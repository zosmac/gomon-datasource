// Copyright © 2021 The Gomon Project.

package process

import (
	"math"
	"net"
	"runtime"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Connections creates a slice of local to remote connections.
func Connections(pt ProcessTable) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error("Connections panicked",
				"panic", r,
				"stacktrace", string(buf),
			)
		}
	}()

	epm := map[[3]string]Pid{} // is distinguishing dup'd and inherited descriptors an issue?

	// build a map for identifying intra-host peer Endpoints
	for _, p := range pt {
		for _, conn := range p.Connections {
			epm[[3]string{conn.Type, conn.Self.Name, conn.Peer.Name}] = conn.Self.Pid
		}
	}

	hdpid := Pid(0) // -hdix for host "pid", math.MaxInt32+hdix for data "pid"
	for _, p := range pt {
		pid := p.Pid
		for i, conn := range p.Connections {
			hdpid++

			if conn.Peer.Name == "" {
				continue // listener or unix named socket
			}

			if conn.Self.Name == "" { // data, already set
				p.Connections[i].Peer.Pid = hdpid + math.MaxInt32
				continue
			}

			rpid, ok := epm[[3]string{conn.Type, conn.Peer.Name, conn.Self.Name}]
			if !ok {
				rpid, ok = epm[[3]string{conn.Type, conn.Peer.Name, ""}] // partner with unix named socket
				for i, cn := range pt[rpid].Connections {
					if cn.Self.Name == conn.Peer.Name {
						pt[rpid].Connections[i].Peer.Name = conn.Self.Name
						pt[rpid].Connections[i].Peer.Pid = pid
					}
				}
			}
			if ok {
				p.Connections[i].Peer.Pid = rpid
			} else if _, _, err := net.SplitHostPort(conn.Peer.Name); err == nil { // remote host
				p.Connections[i].Peer.Pid = -hdpid
			} else { // kernel
				p.Connections[i].Peer.Pid = 0
			}
		}
		if p.Ppid > 0 {
			p.Connections = append(p.Connections,
				Connection{
					Type: "parent",
					Self: Endpoint{
						Name: pt[p.Ppid].Executable,
						Pid:  p.Ppid,
					},
					Peer: Endpoint{
						Name: p.Executable,
						Pid:  p.Pid,
					},
				},
			)
		}
	}
}
