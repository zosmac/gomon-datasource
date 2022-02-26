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
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	// regex for parsing lsof output lines from lsof command.
	regex = regexp.MustCompile(
		`^(?:(?P<header>COMMAND.*)|====(?P<trailer>\d\d:\d\d:\d\d)====.*|` +
			`(?P<command>[^ ]+)[ ]+` +
			`(?P<pid>[^ ]+)[ ]+` +
			`(?:[^ ]+)[ ]+` + // USER
			`(?:(?P<fd>\d+)|fp\.|mem|cwd|rtd)` +
			`(?P<mode> |[rwu-][rwuNRWU]?)[ ]+` +
			`(?P<type>(?:[^ ]+|))[ ]+` +
			`(?P<device>(?:0x[0-9a-f]+|\d+,\d+|kpipe|upipe|))[ ]+` +
			`(?:[^ ]+|)[ ]+` + // SIZE/OFF
			`(?P<node>(?:\d+|TCP|UDP|))[ ]+` +
			`(?P<name>.*))$`,
	)

	// rgxgroups maps names of capture groups to indices.
	rgxgroups = func() map[captureGroup]int {
		g := map[captureGroup]int{}
		for _, name := range regex.SubexpNames() {
			g[captureGroup(name)] = regex.SubexpIndex(name)
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
	groupHeader  captureGroup = "header"
	groupTrailer captureGroup = "trailer"
	groupCommand captureGroup = "command"
	groupPid     captureGroup = "pid"
	groupFd      captureGroup = "fd"
	groupMode    captureGroup = "mode"
	groupType    captureGroup = "type"
	groupDevice  captureGroup = "device"
	groupNode    captureGroup = "node"
	groupName    captureGroup = "name"
)

type (
	// captureGroup is the name of a reqular expression capture group.
	captureGroup string
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

	log.DefaultLogger.Info("start command to capture each processes' open descriptors",
		"command", fmt.Sprintf("%s[%d]", cmd.String(), cmd.Process.Pid),
	)

	go parseOutput(stdout)

	return nil
}

// parseOutput reads the stdout of the command.
func parseOutput(stdout io.ReadCloser) {
	epm := map[Pid][]Connection{}

	sc := bufio.NewScanner(stdout)
	var i Pid
	for sc.Scan() {
		i++

		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}
		if header := match[rgxgroups[groupHeader]]; header != "" {
			continue
		}
		if trailer := match[rgxgroups[groupTrailer]]; trailer != "" {
			epLock.Lock()
			epMap = epm
			epm = map[Pid][]Connection{}
			epLock.Unlock()
			continue
		}

		command := match[rgxgroups[groupCommand]]
		pid, _ := strconv.Atoi(match[rgxgroups[groupPid]])
		fd, _ := strconv.Atoi(match[rgxgroups[groupFd]])
		mode := match[rgxgroups[groupMode]][0]
		fdType := match[rgxgroups[groupType]]
		device := match[rgxgroups[groupDevice]]
		node := match[rgxgroups[groupNode]]
		name := match[rgxgroups[groupName]]

		var self, peer string

		switch fdType {
		case "FSEVENT", "NEXUS", "NPOLICY", "unknown":
		case "ndrv", "systm":
			self = device
			peer = name
		case "CHAN":
			fdType = device
			peer = name
		case "BLK", "DIR", "LINK", "REG", "PSXSHM", "KQUEUE":
			peer = name
		case "CHR":
			peer = name
			if name == os.DevNull {
				fdType = "NUL"
			}
		case "key", "PSXSEM":
			self = device
		case "FIFO":
			if mode == 'w' {
				peer = name
			} else {
				self = name
			}
		case "PIPE", "unix":
			self = device
			if len(name) > 2 && name[:2] == "->" {
				peer = name[2:] // strip "->"
			}
		case "IPv4", "IPv6":
			fdType = node
			split := strings.Split(name, " ")
			split = strings.Split(split[0], "->")
			self = addZone(split[0])
			if len(split) > 1 {
				peer = addZone(split[1])
			}
		}

		ep := Connection{
			Type: fdType,
			Self: Endpoint{Name: self, Pid: Pid(pid)},
			Peer: Endpoint{Name: peer},
		}

		log.DefaultLogger.Debug(fmt.Sprintf("%s[%d:%d] %s %s %s", command, pid, fd, fdType, self, peer))

		if fdType != "NUL" {
			epm[Pid(pid)] = append(epm[Pid(pid)], ep)
		}
	}

	panic(fmt.Errorf("stdout closed %v", sc.Err()))
}
