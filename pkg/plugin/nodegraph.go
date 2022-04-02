// Copyright © 2021 The Gomon Project.

package plugin

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/zosmac/gomon-datasource/pkg/process"
)

var (
	// hnMap caches resolver host name lookup.
	hnMap  = map[string]string{}
	hnLock sync.Mutex

	// host/proc specify the arc for the circle drawn around a node.
	// Each arc has a specific color set in its field metadata to create a circle that identifies the node type.
	hostArc = []interface{}{1.0, 0.0, 0.0, 0.0, 0.0} // red
	procArc = []interface{}{0.0, 1.0, 0.0, 0.0, 0.0} // blue
	dataArc = []interface{}{0.0, 0.0, 1.0, 0.0, 0.0} // yellow
	sockArc = []interface{}{0.0, 0.0, 0.0, 1.0, 0.0} // magenta
	kernArc = []interface{}{0.0, 0.0, 0.0, 0.0, 1.0} // cyan
	red     = map[string]interface{}{"mode": "fixed", "fixedColor": "red"}
	blue    = map[string]interface{}{"mode": "fixed", "fixedColor": "blue"}
	yellow  = map[string]interface{}{"mode": "fixed", "fixedColor": "yellow"}
	magenta = map[string]interface{}{"mode": "fixed", "fixedColor": "magenta"}
	cyan    = map[string]interface{}{"mode": "fixed", "fixedColor": "cyan"}
)

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid
)

// NodeGraph produces the process connections node graph.
func NodeGraph(link string, query query) (resp backend.DataResponse) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error("NodeGraph() panicked",
				"panic", r,
				"stacktrace", string(buf),
			)
			if r, ok := r.(error); ok {
				resp.Error = r
			} else {
				resp.Error = fmt.Errorf("panic in NodeGraph: %v", r)
			}
		}
	}()

	log.DefaultLogger.Info("NodeGraph requested", "node", query.Query)

	ft := process.Table{}
	pt := process.BuildTable()
	process.Connections(pt)

	if query.pid > 0 && pt[query.pid] == nil {
		query.pid = 0 // reset to default
	}
	if query.pid > 0 { // build this process' "extended family"
		ft = family(pt, query.pid)
	} else { // only consider non-daemon and remote host connected processes
		for pid, p := range pt {
			if p.Ppid > 1 {
				for pid, p := range family(pt, pid) {
					ft[pid] = p
				}
			}
			for _, conn := range p.Connections {
				if conn.Peer.Pid < 0 {
					ft[conn.Self.Pid] = pt[conn.Self.Pid]
				}
			}
		}
	}

	nm := map[string][]interface{}{}
	em := map[string][]interface{}{}
	timestamp := time.Now()

	for _, p := range ft {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.pid == 0 && conn.Peer.Pid >= math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			self := shortname(pt, conn.Self.Pid)
			nm[self] = append([]interface{}{
				timestamp,
				int64(conn.Self.Pid),
				filepath.Base(pt[conn.Self.Pid].Executable),
				conn.Self.Pid.String(),
				fullname(pt, conn.Self.Pid),
				fullname(pt, pt[conn.Self.Pid].Ppid),
			}, procArc...)

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)
				peer := conn.Type + ":" + conn.Peer.Name

				arc := hostArc
				if conn.Self.Name[0:2] == "0x" { // listen socket
					arc = sockArc
				}

				nm[peer] = append([]interface{}{
					timestamp,
					int64(conn.Peer.Pid),
					conn.Type + ":" + port,
					hostname(host),
					host,
					hostname(host),
				}, arc...)

				// flip the source and target to get Host shown to left in node graph
				id := fmt.Sprintf("%d->%d", conn.Peer.Pid, conn.Self.Pid)
				if e, ok := em[id]; ok {
					e[6].(map[string]struct{})[conn.Type+":"+conn.Peer.Name+"->"+conn.Self.Name] = struct{}{}
				} else {
					em[id] = []interface{}{
						timestamp,
						id,
						int64(conn.Peer.Pid),
						int64(conn.Self.Pid),
						host,
						self,
						map[string]struct{}{peer + "->" + conn.Self.Name: {}},
					}
				}
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				arc := dataArc
				if conn.Type != "REG" && conn.Type != "DIR" {
					arc = kernArc
				}

				nm[peer] = append([]interface{}{
					timestamp,
					int64(conn.Peer.Pid),
					conn.Type,
					conn.Peer.Name,
					peer,
					self,
				}, arc...)

				id := fmt.Sprintf("%d->%d", conn.Self.Pid, conn.Peer.Pid)
				if e, ok := em[id]; ok {
					e[6].(map[string]struct{})[self+"->"+peer] = struct{}{}
				} else {
					em[id] = []interface{}{
						timestamp,
						id,
						int64(conn.Self.Pid),
						int64(conn.Peer.Pid),
						self,
						url.QueryEscape(peer),
						map[string]struct{}{self + "->" + peer: {}},
					}
				}
			} else { // peer is process
				peer := shortname(pt, conn.Peer.Pid)
				nm[peer] = append([]interface{}{
					timestamp,
					int64(conn.Peer.Pid),
					filepath.Base(pt[conn.Peer.Pid].Executable),
					conn.Peer.Pid.String(),
					fullname(pt, conn.Peer.Pid),
					fullname(pt, pt[conn.Peer.Pid].Ppid),
				}, procArc...)

				var s2p, p2s string
				switch conn.Type {
				case "parent":
				default:
					s2p = conn.Type + ":" + conn.Self.Name + "->" + conn.Peer.Name
					p2s = conn.Type + ":" + conn.Peer.Name + "->" + conn.Self.Name
				}

				// show bidirectional connection only once
				id := fmt.Sprintf("%d->%d", conn.Self.Pid, conn.Peer.Pid)
				di := fmt.Sprintf("%d->%d", conn.Peer.Pid, conn.Self.Pid)

				e, ok := em[id]
				if !ok {
					if e, ok = em[di]; ok && conn.Type == "parent" {
						em[id] = []interface{}{
							timestamp,
							id,
							int64(conn.Self.Pid),
							int64(conn.Peer.Pid),
							self,
							peer,
							e[6],
						}
						delete(em, di)
					}
				}
				if ok {
					if conn.Type != "parent" {
						_, ok := e[6].(map[string]struct{})[s2p]
						if !ok {
							_, ok = e[6].(map[string]struct{})[p2s]
						}
						if !ok {
							e[6].(map[string]struct{})[s2p] = struct{}{}
						}
					}
				} else {
					em[id] = []interface{}{
						timestamp,
						id,
						int64(conn.Self.Pid),
						int64(conn.Peer.Pid),
						self,
						peer,
						map[string]struct{}{},
					}
					if conn.Type != "parent" {
						em[id][6].(map[string]struct{})[s2p] = struct{}{}
					}
				}
			}
		}
	}

	ns := make([][]interface{}, len(nm))
	i := 0
	for _, n := range nm {
		ns[i] = n
		i++
	}

	sort.Slice(ns, func(i, j int) bool {
		return ns[i][1].(int64) < ns[j][1].(int64)
	})

	es := make([][]interface{}, len(em))
	i = 0
	for _, e := range em {
		var ss []string
		for s := range e[6].(map[string]struct{}) {
			ss = append(ss, s)
		}
		e[6] = strings.Join(ss, ", ")
		es[i] = e
		i++
	}

	sort.Slice(es, func(i, j int) bool {
		return es[i][2].(int64) < es[j][2].(int64) ||
			es[i][2].(int64) == es[j][2].(int64) && es[i][3].(int64) < es[j][3].(int64)
	})

	resp.Frames = nodeFrames(link, ns, es)

	return
}

// family identifies all of the processes related to a process.
func family(pt process.Table, pid Pid) process.Table {
	ft := process.Table{pid: pt[pid]}
	for pid := pt[pid].Ppid; pid > 1; pid = pt[pid].Ppid { // ancestors
		ft[pid] = pt[pid]
	}
	pids := process.FlatTree(process.FindTree(process.BuildTree(pt), pid), 0) // descendants
	for _, pid := range pids {
		ft[pid] = pt[pid]
	}
	return ft
}

// fullname formats the full Executable name and pid.
func fullname(pt process.Table, pid Pid) string {
	return fmt.Sprintf("%s[%d]", pt[pid].Executable, pid)
}

// shortname formats the base Executable name and pid.
func shortname(pt process.Table, pid Pid) string {
	return fmt.Sprintf("%s[%d]", filepath.Base(pt[pid].Executable), pid)
}

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

// if query.Streaming {
// 	// create data frame response.
// 	stream := data.NewFrame("stream")

// 	// add fields.
// 	stream.Fields = append(stream.Fields,
// 		data.NewField("time", nil, []time.Time{query.TimeRange.From, query.TimeRange.To}),
// 		data.NewField("values", nil, []int64{10, 20}),
// 	)

// 	channel := live.Channel{
// 		Scope:     live.ScopeDatasource,
// 		Namespace: pctx.DataSourceInstanceSettings.UID,
// 		Path:      "stream",
// 	}
// 	stream.SetMeta(&data.FrameMeta{Channel: channel.String()})
// }
