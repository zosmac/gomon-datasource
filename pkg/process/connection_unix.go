// Copyright © 2021 The Gomon Project.

//go:build !windows
// +build !windows

package process

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	// regex for parsing lsof output lines from lsof command.
	regex = regexp.MustCompile(
		`^(?P<command>[^ ]+)[ ]+` +
			`(?P<pid>\d+)[ ]+` +
			`(?:\d+)[ ]+` + // USER
			`(?:(?P<fd>\d+)|fp\.|mem|cwd|rtd)` +
			`(?P<mode> |[rwu-][rwuNRWU]?)[ ]+` +
			`(?P<type>(?:[^ ]+|))[ ]+` +
			`(?P<device>(?:0x[0-9a-f]+|\d+,\d+|kpipe|upipe|))[ ]+` +
			`(?:[^ ]+|)[ ]+` + // SIZE/OFF
			`(?P<node>(?:[^ ]+|))`,
	)

	// rgxgroups maps names of capture groups to indices.
	rgxgroups = func() map[string]int {
		g := map[string]int{}
		for _, name := range regex.SubexpNames() {
			g[name] = regex.SubexpIndex(name)
		}
		return g
	}()

	// zoneregex determines if a link local address embeds a zone index.
	zoneregex = regexp.MustCompile(`^((fe|FE)80):(\d{1,2})(::.*)$`)

	// Zones maps local ip addresses to their network zones.
	zones = func() map[string]string {
		zm := map[string]string{}
		if nis, err := net.Interfaces(); err == nil {
			for _, ni := range nis {
				zm[strconv.FormatUint(uint64(ni.Index), 16)] = ni.Name
				if addrs, err := ni.Addrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							zm[ip.String()] = ni.Name
						}
					}
				}
				if addrs, err := ni.MulticastAddrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							zm[ip.String()] = ni.Name
						}
					}
				}
			}
		}
		return zm
	}()
)

const (
	// lsof line regular expressions named capture groups.
	groupCommand = "command"
	groupPid     = "pid"
	groupFd      = "fd"
	groupMode    = "mode"
	groupType    = "type"
	groupDevice  = "device"
	groupNode    = "node"
)

func init() {
	err := lsofCommand()
	setuid() // after lsof command starts, set to the grafana user
	if err != nil {
		log.DefaultLogger.Error("command to capture open process descriptors failed",
			"error", err,
		)
	}
}

func addZone(addr string) string {
	ip, port, _ := net.SplitHostPort(addr)
	match := zoneregex.FindStringSubmatch(ip)
	if match != nil { // strip the zone index from the ipv6 link local address
		ip = match[1] + match[4]
		if zone, ok := zones[match[3]]; ok {
			ip += "%" + zone
		}
	} else if zone, ok := zones[ip]; ok {
		ip += "%" + zone
	}
	return net.JoinHostPort(ip, port)
}

// lsofCommand starts the lsof command to capture process connections.
func lsofCommand() error {
	cmd := hostCommand() // perform OS specific customizations for command
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe failed %w", err)
	}
	cmd.Stderr = nil // sets to /dev/null
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start failed %w", err)
	}

	log.DefaultLogger.Info(fmt.Sprintf("start [%d] %q", cmd.Process.Pid, cmd.String()))

	go parseLsof(stdout)

	return nil
}

// parseLsof parses each line of stdout from the command.
func parseLsof(stdout io.ReadCloser) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error("parseLsof() panicked",
				"panic", r,
				"stacktrace", string(buf),
			)
		}
	}()

	epm := map[Pid][]Connection{}
	var nameIndex int
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		text := sc.Text()
		if strings.HasPrefix(text, "COMMAND") {
			nameIndex = strings.Index(text, "NAME")
			continue
		} else if strings.HasPrefix(text, "====") {
			epLock.Lock()
			epMap = epm
			epm = map[Pid][]Connection{}
			epLock.Unlock()
			continue
		}
		match := regex.FindStringSubmatch(text[:nameIndex])
		if len(match) == 0 || match[0] == "" {
			continue
		}

		command := match[rgxgroups[groupCommand]]
		pid, _ := strconv.Atoi(match[rgxgroups[groupPid]])
		fd, _ := strconv.Atoi(match[rgxgroups[groupFd]])
		mode := match[rgxgroups[groupMode]][0]
		fdType := match[rgxgroups[groupType]]
		device := match[rgxgroups[groupDevice]]
		node := match[rgxgroups[groupNode]]
		peer := text[nameIndex:]

		var self string

		switch fdType {
		case "REG":
		case "BLK", "CHR", "DIR", "LINK", "PSXSHM", "KQUEUE":
		case "FSEVENT", "NEXUS", "NPOLICY", "ndrv", "systm", "unknown":
		case "CHAN":
			fdType = device
		case "key", "PSXSEM":
			peer = device
		case "FIFO":
			if mode != 'w' {
				self = peer
				peer = ""
			}
		case "PIPE", "unix":
			self = device
			if len(peer) > 2 && peer[:2] == "->" {
				peer = peer[2:] // strip "->"
			}
		case "IPv4", "IPv6":
			fdType = node
			split := strings.Split(peer, " ")
			split = strings.Split(split[0], "->")
			if len(split) > 1 {
				self = addZone(split[0])
				peer = addZone(split[1])
			} else {
				self = device
				peer = addZone((split[0]))
			}
		}

		if self == "" && peer == "" {
			peer = fdType // treat like data connection
		}

		log.DefaultLogger.Debug(fmt.Sprintf("%s[%d:%d] %s %s %s", command, pid, fd, fdType, self, peer))

		if peer != os.DevNull {
			epm[Pid(pid)] = append(epm[Pid(pid)],
				Connection{
					Type: fdType,
					Self: Endpoint{Name: self, Pid: Pid(pid)},
					Peer: Endpoint{Name: peer},
				},
			)
		}
		if fdType == "unix" && peer[:2] != "0x" {
			epm[Pid(pid)] = append(epm[Pid(pid)],
				Connection{
					Type: fdType,
					Self: Endpoint{Pid: Pid(pid)},
					Peer: Endpoint{Name: peer},
				},
			)
		}
	}

	panic(fmt.Errorf("stdout closed %v", sc.Err()))
}
