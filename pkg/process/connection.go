// Copyright © 2021 The Gomon Project.

package process

import (
	"math"
	"net"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	// localIps addresses for all local network interfaces on host
	localIps = func() map[string]string {
		l := map[string]string{}
		h, _ := os.Hostname()
		ips, _ := net.LookupIP(h)
		for _, ip := range ips {
			l[ip.String()] = h
		}
		return l
	}()

	// interfaces maps local ip addresses to their network interfaces.
	interfaces = func() map[string]string {
		im := map[string]string{}
		if nis, err := net.Interfaces(); err == nil {
			for _, ni := range nis {
				if addrs, err := ni.Addrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							im[ip.String()] = ni.Name
						}
					}
				}
				if addrs, err := ni.MulticastAddrs(); err == nil {
					for _, addr := range addrs {
						if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
							im[ip.String()] = ni.Name
						}
					}
				}
			}
		}
		return im
	}()

	// hnMap caches resolver host name lookup.
	hnMap  = map[string]string{}
	hnLock sync.RWMutex
)

type (
	endpoint struct {
		name string
		pid  Pid
	}

	connection struct {
		ftype string
		name  string
		self  endpoint
		peer  endpoint
	}
)

// hostname resolves the host name for an ip address.
func hostname(ip string) string {
	hnLock.Lock()
	defer hnLock.Unlock()

	if host, ok := hnMap[ip]; ok {
		return host
	}

	hnMap[ip] = ip
	go func() { // initiate hostname lookup
		if hosts, err := net.LookupAddr(ip); err == nil {
			hnLock.Lock()
			hnMap[ip] = hosts[0]
			hnLock.Unlock()
		}
	}()

	return ip
}

// connections creates an ordered slice of local to remote connections by pid and fd.
func connections(pt processTable) []connection {
	connm := map[[4]int]connection{}
	epm := map[string]map[Pid][]int{}
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error("connections panicked",
				"panic", r,
				"stacktrace", string(buf),
			)
		}
	}()

	// build a map of all remote (peer) intra-host endpoints
	for pid, p := range pt {
		for i, conn := range p.Connections {
			if conn.Self == "" {
				continue
			}
			switch conn.Type {
			case "FIFO", "PIPE", "TCP", "UDP", "unix":
				self := conn.Type + ": " + conn.Self
				if _, ok := epm[self]; !ok {
					epm[self] = map[Pid][]int{}
				}
				epm[self][pid] = append(epm[self][pid], i)
			}
		}
	}

	// determine all inter- and intra- connections for processes
	for pid, p := range pt {
		ppid := p.Ppid
		connm[[4]int{int(ppid), -1, int(pid), -1}] = connection{
			ftype: "parent:" + ppid.String(), // set for edge tooltip
			name:  "child:" + pid.String(),
			self: endpoint{
				pid: ppid,
			},
			peer: endpoint{
				pid: pid,
			},
		}

		for _, conn := range p.Connections {
			fd := conn.Descriptor
			switch conn.Type {
			case "NUL": // ignore /dev/null connection endpoints
			case "DIR", "REG", "PSXSHM":
				connm[[4]int{int(pid), int(fd), math.MaxInt32, 0}] = connection{
					ftype: conn.Type,
					name:  conn.Name,
					self: endpoint{
						pid: pid,
					},
					peer: endpoint{
						pid: math.MaxInt32,
					},
				}
			case "systm":
				connm[[4]int{int(pid), int(fd), 0, 0}] = connection{
					ftype: conn.Type,
					name:  conn.Name,
					self: endpoint{
						pid: pid,
					},
				}
			case "FIFO", "PIPE", "TCP", "UDP", "unix":
				if conn.Peer == "" {
					continue
				}
				key := conn.Type + ": " + conn.Peer

				if _, ok := epm[key]; !ok {
					if conn.Type == "TCP" || conn.Type == "UDP" { // possible external connection
						host, _, _ := net.SplitHostPort(conn.Peer)
						var local bool
						if _, local = localIps[host]; local {
						} else if _, local = interfaces[host]; local {
						} else {
							ip := net.ParseIP(host)
							local = ip.IsLoopback() ||
								ip.IsInterfaceLocalMulticast() ||
								ip.IsLinkLocalMulticast() ||
								ip.IsLinkLocalUnicast()
						}
						if !local {
							connm[[4]int{-1, -1, int(pid), int(fd)}] = connection{
								ftype: conn.Type,
								name:  conn.Name,
								self: endpoint{
									name: conn.Peer,
									pid:  -1,
								},
								peer: endpoint{
									name: conn.Self,
									pid:  pid,
								},
							}
						}
					}
					continue
				}

				rpids := make([]Pid, len(epm[key]))
				i := 0
				for rpid := range epm[key] {
					rpids[i] = rpid
					i++
				}
				sort.Slice(rpids, func(i, j int) bool {
					return rpids[i] < rpids[j]
				})

				for _, rpid := range rpids {
					if pid == rpid { // ignore intra-process connections
						continue
					}
					ix := epm[key][rpid]
					rp := pt[rpid]
					for _, i := range ix {
						rconn := rp.Connections[i]
						if !(conn.Type == rconn.Type &&
							conn.Peer == rconn.Self &&
							(rconn.Peer == conn.Self ||
								rconn.Peer == "" && (conn.Type == "PSXSHM" || conn.Type == "unix"))) {
							continue
						}

						// ignore connection if previously identified
						rfd := rconn.Descriptor
						if _, ok := connm[[4]int{int(pid), int(fd), int(rpid), int(rfd)}]; ok {
							continue
						}
						if _, ok := connm[[4]int{int(rpid), int(rfd), int(pid), int(fd)}]; ok {
							continue
						}

						connm[[4]int{int(pid), int(fd), int(rpid), int(rfd)}] = connection{
							ftype: conn.Type,
							name:  conn.Name,
							self: endpoint{
								name: conn.Self,
								pid:  pid,
							},
							peer: endpoint{
								name: conn.Peer,
								pid:  rpid,
							},
						}

						log.DefaultLogger.Debug("Connection",
							"type", conn.Type,
							"name", conn.Name,
							"self", pid.String(), // to format as int rather than float
							"peer", rpid.String(),
						)
					}
				}
			}
		}
	}

	keys := make([][4]int, len(connm))
	i := 0
	for key := range connm {
		keys[i] = key
		i++
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i][0] < keys[j][0] ||
			keys[i][0] == keys[j][0] && (keys[i][2] < keys[j][2] ||
				keys[i][2] == keys[j][2] && (keys[i][1] < keys[j][1] ||
					keys[i][1] == keys[j][1] && keys[i][3] < keys[j][3]))
	})

	conns := make([]connection, len(keys))
	for i, key := range keys {
		conns[i] = connm[key]
	}

	return conns
}
