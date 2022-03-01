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
	// hnMap caches resolver host name lookup.
	hnMap  = map[string]string{}
	hnLock sync.Mutex

	// queryRegex used to read the pid from the query.
	queryRegex = regexp.MustCompile(
		`^(?P<executable>[^\[]*)\[(?P<pid>\d+)\]$`,
	)

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

	// query from data source.
	query struct {
		Pid
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

	query, err := parseQuery(message)
	if err != nil {
		resp.Error = err
		return
	}

	ft := process.Table{}
	pt := process.BuildTable()
	process.Connections(pt)

	if query.Pid > 0 && pt[query.Pid] == nil {
		query.Pid = 0 // reset to default
	}
	if query.Pid > 0 { // build this process' "extended family"
		ft = family(pt, query.Pid)
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

	timestamp := time.Now()

	nm := map[string][]interface{}{}
	em := map[string][]interface{}{}

	for _, p := range fs {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.Pid == 0 && conn.Peer.Pid > math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			self := shortname(pt, conn.Self.Pid)
			nm[self] = append([]interface{}{
				timestamp,
				shortname(pt, conn.Self.Pid),
				conn.Self.Pid.String(),
				longname(pt, conn.Self.Pid),
				longname(pt, pt[conn.Self.Pid].Ppid),
				fmt.Sprintf(`"process": %q`, self),
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
					conn.Type + ":" + port,
					hostname(host),
					host,
					hostname(host),
					fmt.Sprintf(`"host": %q`, host),
				}, arc...)

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
					timestamp,
					conn.Type,
					conn.Peer.Name,
					peer,
					self,
					fmt.Sprintf(`"data": %q`, url.QueryEscape(peer)),
				}, arc...)

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
			} else { // peer is process
				peer := shortname(pt, conn.Peer.Pid)
				nm[peer] = append([]interface{}{
					timestamp,
					shortname(pt, conn.Peer.Pid),
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

				// show bidirectional connection only once
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

	for id, n := range nm {
		nodes.AppendRow(append([]interface{}{id}, n...)...)
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
		"pid", query.Pid,
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
			query.Pid = Pid(pid)
		}
		log.DefaultLogger.Info(fmt.Sprintf("process: %s[%d]", executable, pid), "err", err)
	}

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
