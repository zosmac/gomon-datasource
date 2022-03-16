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
	"github.com/grafana/grafana-plugin-sdk-go/data"

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

	log.DefaultLogger.Info("NodeGraph requested", "node", query.Node)

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

	pids := make([]Pid, len(ft))
	i := 0
	for pid := range ft {
		pids[i] = pid
		i++
	}
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})
	fs := make([]*process.Process, len(ft))
	for i, pid := range pids {
		fs[i] = ft[pid]
	}

	nm := map[string][]interface{}{}
	em := map[string][]interface{}{}

	for _, p := range fs {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.pid == 0 && conn.Peer.Pid > math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			self := shortname(pt, conn.Self.Pid)
			nm[self] = append([]interface{}{
				self,
				self,
				conn.Self.Pid.String(),
				longname(pt, conn.Self.Pid),
				longname(pt, pt[conn.Self.Pid].Ppid),
				self,
			}, procArc...)

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)
				peer := conn.Type + ":" + conn.Peer.Name

				arc := hostArc
				if conn.Self.Name[0:2] == "0x" { // listen socket
					arc = sockArc
				}

				nm[peer] = append([]interface{}{
					peer,
					conn.Type + ":" + port,
					hostname(host),
					host,
					hostname(host),
					host,
				}, arc...)

				// flip the source and target to get Host shown to left in node graph
				id := fmt.Sprintf("%s->%d", peer, conn.Self.Pid)
				if e, ok := em[id]; ok {
					e[5].(map[string]struct{})[conn.Type+":"+conn.Peer.Name+"->"+conn.Self.Name] = struct{}{}
				} else {
					em[id] = []interface{}{
						id,
						peer,
						self,
						host,
						self,
						map[string]struct{}{peer + "->" + conn.Self.Name: {}},
					}
				}
			} else if conn.Peer.Pid > math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				arc := dataArc
				if conn.Type != "REG" && conn.Type != "DIR" {
					arc = kernArc
				}

				nm[peer] = append([]interface{}{
					peer,
					conn.Type,
					conn.Peer.Name,
					peer,
					self,
					url.QueryEscape(peer),
				}, arc...)

				id := fmt.Sprintf("%d->%s", conn.Self.Pid, peer)
				if e, ok := em[id]; ok {
					e[5].(map[string]struct{})[self+"->"+peer] = struct{}{}
				} else {
					em[id] = []interface{}{
						id,
						self,
						peer,
						self,
						url.QueryEscape(peer),
						map[string]struct{}{self + "->" + peer: {}},
					}
				}
			} else { // peer is process
				peer := shortname(pt, conn.Peer.Pid)
				nm[peer] = append([]interface{}{
					peer,
					peer,
					conn.Peer.Pid.String(),
					longname(pt, conn.Peer.Pid),
					longname(pt, pt[conn.Peer.Pid].Ppid),
					peer,
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
					e, ok = em[di]
				}
				if ok {
					_, ok := e[5].(map[string]struct{})[s2p]
					if !ok {
						_, ok = e[5].(map[string]struct{})[p2s]
					}
					if !ok {
						e[5].(map[string]struct{})[s2p] = struct{}{}
					}
				} else {
					em[id] = []interface{}{
						id,
						self,
						peer,
						self,
						peer,
						map[string]struct{}{s2p: {}},
					}
				}
			}
		}
	}

	nodes, edges := nodeFrames(link, len(nm), len(em))
	timestamp := time.Now()

	i = 0
	rows := make([][]interface{}, len(nm))
	for _, n := range nm {
		rows[i] = n
		i++
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0].(string) < rows[j][0].(string)
	})
	for i, row := range rows {
		nodes.SetRow(i, append([]interface{}{timestamp}, row...)...)
	}

	i = 0
	rows = make([][]interface{}, len(em))
	for _, e := range em {
		var ss []string
		for s := range e[5].(map[string]struct{}) {
			ss = append(ss, s)
		}
		e[5] = strings.Join(ss, ", ")
		rows[i] = e
		i++
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0].(string) < rows[j][0].(string)
	})
	for i, row := range rows {
		edges.SetRow(i, append([]interface{}{timestamp}, row...)...)
	}

	resp.Frames = data.Frames{nodes, edges}

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

// longname formats the full Executable name and pid.
func longname(pt process.Table, pid Pid) string {
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
