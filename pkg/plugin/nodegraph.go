// Copyright © 2021 The Gomon Project.

package plugin

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/zosmac/gomon-datasource/pkg/process"
)

var (
	// host/proc specify the arc for the circle drawn around a node.
	// Each arc has a specific color set in its field metadata to create a circle that identifies the node type.
	hostArc = []interface{}{1.0, 0.0, 0.0, 0.0} // red
	procArc = []interface{}{0.0, 1.0, 0.0, 0.0} // blue
	dataArc = []interface{}{0.0, 0.0, 1.0, 0.0} // yellow
	kernArc = []interface{}{0.0, 0.0, 0.0, 1.0} // magenta
	red     = map[string]interface{}{"mode": "fixed", "fixedColor": "red"}
	blue    = map[string]interface{}{"mode": "fixed", "fixedColor": "blue"}
	yellow  = map[string]interface{}{"mode": "fixed", "fixedColor": "yellow"}
	magenta = map[string]interface{}{"mode": "fixed", "fixedColor": "magenta"}

	// hnMap caches resolver host name lookup.
	hnMap  = map[string]string{}
	hnLock sync.Mutex

	// queryRegex used to read the pid from the query.
	queryRegex = regexp.MustCompile(
		`^(?P<executable>[^\[]*)\[(?P<pid>\d+)\]$`,
	)
)

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid

	// query
	query struct {
		pid  Pid
		Node struct {
			Process string `json:"process"`
			Host    string `json:"host"`
			Data    string `json:"data"`
		} `json:"node"`
		Streaming bool `json:"streaming"`
	}
)

// NodeGraph produces the process connections node graph.
func NodeGraph(link string, message json.RawMessage) (resp backend.DataResponse) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error("NodeGraph panicked",
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

	query, err := parseQuery(message)
	if err != nil {
		resp.Error = err
		return
	}

	ft := process.ProcessTable{}
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

	timestamp := time.Now()

	for _, p := range fs {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore kernel and launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.pid == 0 && // for the "all process" query:
					(conn.Peer.Pid == 0 || conn.Peer.Pid > math.MaxInt32) { // ignore kernel and data connections
				continue
			}

			if _, ok := ft[conn.Peer.Pid]; !ok && conn.Peer.Pid > 0 && conn.Peer.Pid < math.MaxInt32 {
				continue
			}

			self := shortname(pt, conn.Self.Pid)
			nm[self] = append([]interface{}{
				timestamp,
				filepath.Base(pt[conn.Self.Pid].Executable),
				conn.Self.Pid.String(),
				longname(pt, conn.Self.Pid),
				longname(pt, pt[conn.Self.Pid].Ppid),
				fmt.Sprintf(`"process": %q`, self),
			}, procArc...)

			if conn.Peer.Pid < 0 { // external network connections
				host, port, _ := net.SplitHostPort(conn.Peer.Name)
				peer := conn.Type + ":" + conn.Peer.Name
				nm[peer] = append([]interface{}{
					timestamp,
					conn.Type + ":" + port,
					hostname(host),
					host,
					hostname(host),
					fmt.Sprintf(`"host": %q`, host),
				}, hostArc...)

				// flip the source and target to get Host shown to left in node graph
				key := fmt.Sprintf("%s->%d", peer, conn.Self.Pid)
				if e, ok := em[key]; ok {
					e[5].(map[string]struct{})[conn.Type+":"+conn.Peer.Name+"->"+conn.Self.Name] = struct{}{}
				} else {
					em[key] = []interface{}{
						timestamp,
						peer,
						self,
						fmt.Sprintf(`"host": %q`, host),
						fmt.Sprintf(`"process": %q`, self),
						map[string]struct{}{conn.Type + ":" + conn.Peer.Name + "->" + conn.Self.Name: {}},
					}
				}

				// create pseudo process to incorporate host node into process tree
				ft[conn.Peer.Pid] = &process.Process{
					Id: process.Id{
						Name: peer,
						Pid:  conn.Peer.Pid,
					},
				}
			} else if conn.Peer.Pid == 0 {
				ft[0] = pt[0]
				peer := "kernel[0]"
				nm[peer] = append([]interface{}{
					timestamp,
					"kernel",
					"0",
					peer,
					"",
					fmt.Sprintf(`"process": %q`, peer),
				}, kernArc...)

				key := fmt.Sprintf("%d->0", conn.Self.Pid)
				if e, ok := em[key]; ok {
					e[5].(map[string]struct{})[conn.Type+":"+conn.Self.Name+"->"+conn.Peer.Name] = struct{}{}
				} else {
					em[key] = []interface{}{
						timestamp,
						self,
						peer,
						fmt.Sprintf(`"process": %q`, self),
						fmt.Sprintf(`"process": %q`, peer),
						map[string]struct{}{conn.Type + ":" + conn.Self.Name + "->" + conn.Peer.Name: {}},
					}
				}
			} else if conn.Peer.Pid > math.MaxInt32 {
				peer := conn.Type + ":" + conn.Peer.Name
				nm[peer] = append([]interface{}{
					timestamp,
					conn.Type,
					conn.Peer.Name,
					peer,
					self,
					fmt.Sprintf(`"data": %q`, url.QueryEscape(peer)),
				}, dataArc...)

				key := fmt.Sprintf("%d->%s", conn.Self.Pid, peer)
				if e, ok := em[key]; ok {
					e[5].(map[string]struct{})[self+"->"+peer] = struct{}{}
				} else {
					em[key] = []interface{}{
						timestamp,
						self,
						peer,
						fmt.Sprintf(`"process": %q`, self),
						fmt.Sprintf(`"data": %q`, url.QueryEscape(peer)),
						map[string]struct{}{self + "->" + peer: {}},
					}
				}

				// create pseudo process to incorporate data node into process tree
				ft[conn.Peer.Pid] = &process.Process{
					Id: process.Id{
						Name: peer,
						Pid:  conn.Peer.Pid,
					},
				}
			} else {
				peer := shortname(pt, conn.Peer.Pid)
				nm[peer] = append([]interface{}{
					timestamp,
					filepath.Base(pt[conn.Peer.Pid].Executable),
					conn.Peer.Pid.String(),
					longname(pt, conn.Peer.Pid),
					longname(pt, pt[conn.Peer.Pid].Ppid),
					fmt.Sprintf(`"process": %q`, peer),
				}, procArc...)

				var s2p, p2s string
				switch conn.Type {
				case "parent":
				default:
					s2p = conn.Type + ":" + conn.Self.Name + "->" + conn.Peer.Name
					p2s = conn.Type + ":" + conn.Peer.Name + "->" + conn.Self.Name
				}

				key := fmt.Sprintf("%d->%d", conn.Self.Pid, conn.Peer.Pid)
				yek := fmt.Sprintf("%d->%d", conn.Peer.Pid, conn.Self.Pid)

				e, ok := em[key]
				if !ok {
					e, ok = em[yek]
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
					em[key] = []interface{}{
						timestamp,
						self,
						peer,
						fmt.Sprintf(`"process": %q`, self),
						fmt.Sprintf(`"process": %q`, peer),
						map[string]struct{}{s2p: {}},
					}
				}
			}
		}
	}

	nodes, edges := dataframes(link)
	nodes.Meta.Stats[0].Value = float64(len(nm))
	edges.Meta.Stats[0].Value = float64(len(em))
	resp.Frames = data.Frames{nodes, edges}

	for pid, p := range ft {
		var id string
		if pid < 0 || pid > math.MaxInt32 { // host or data
			id = p.Id.Name
		} else if pid == 0 {
			id = "kernel[0]"
		} else { // process
			id = shortname(pt, pid)
		}
		if values, ok := nm[id]; ok {
			log.DefaultLogger.Debug("Peer node found",
				"pid", pid.String(), // to format as int rather than float
				"node", id,
				"values", values,
			)
			nodes.AppendRow(append([]interface{}{id}, values...)...)
			delete(nm, id)
		}
	}
	for id, node := range nm {
		log.DefaultLogger.Error("UNRESOLVED EXTRA NODE!!!",
			"id", id,
			"node", node,
		)
	}

	for id, e := range em {
		var ss []string
		for s := range e[5].(map[string]struct{}) {
			ss = append(ss, s)
		}
		e[5] = strings.Join(ss, ", ")
		edges.AppendRow(append([]interface{}{id}, e...)...)
	}

	return
}

// parseQuery extracts the query from the request JSON.
func parseQuery(message json.RawMessage) (query query, err error) {
	// Unmarshal the JSON into our queryModel.
	if err = json.Unmarshal(message, &query); err != nil {
		log.DefaultLogger.Error("Query unmarshaling failed",
			"json", string(message),
			"err", err,
		)
		return
	}

	log.DefaultLogger.Info("Node Graph query properties",
		"pid", query.pid,
		"process", query.Node.Process,
		"host", query.Node.Host,
		"data", query.Node.Data,
		"streaming", query.Streaming,
	)

	if query.Node.Process != "" {
		match := queryRegex.FindStringSubmatch(query.Node.Process)
		executable := match[1]
		pid, err := strconv.Atoi(match[2])
		if err == nil {
			query.pid = Pid(pid)
		}
		log.DefaultLogger.Info(fmt.Sprintf("process: %s[%d]", executable, pid), "err", err)
	}

	return
}

// family identifies all of the processes related to a process.
func family(pt process.ProcessTable, pid Pid) process.ProcessTable {
	ft := process.ProcessTable{pid: pt[pid]}
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
func longname(pt process.ProcessTable, pid Pid) string {
	return fmt.Sprintf("%s[%d]", pt[pid].Executable, pid)
}

// shortname formats the base Executable name and pid.
func shortname(pt process.ProcessTable, pid Pid) string {
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
