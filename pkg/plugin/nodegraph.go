// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon-datasource/pkg/process"
)

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid
)

var (
	// host/proc specify the arc for the circle drawn around a node.
	// Each arc has a specific color set in its field metadata to create a circle that identifies the node type.
	hostArc = []any{1.0, 0.0, 0.0, 0.0, 0.0} // red
	procArc = []any{0.0, 1.0, 0.0, 0.0, 0.0} // blue
	dataArc = []any{0.0, 0.0, 1.0, 0.0, 0.0} // yellow
	sockArc = []any{0.0, 0.0, 0.0, 1.0, 0.0} // magenta
	kernArc = []any{0.0, 0.0, 0.0, 0.0, 1.0} // cyan
	red     = map[string]any{"mode": "fixed", "fixedColor": "red"}
	blue    = map[string]any{"mode": "fixed", "fixedColor": "blue"}
	yellow  = map[string]any{"mode": "fixed", "fixedColor": "yellow"}
	magenta = map[string]any{"mode": "fixed", "fixedColor": "magenta"}
	cyan    = map[string]any{"mode": "fixed", "fixedColor": "cyan"}
)

// NodeGraph produces the process connections node graph.
func NodeGraph(link string, pid Pid) (resp backend.DataResponse) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error(
				"NodeGraph() Panic",
				"panic", r,
				"stacktrace", string(buf),
			)
			if e, ok := r.(error); ok {
				resp.Error = e
			} else {
				resp.Error = fmt.Errorf("panic in NodeGraph: %v", r)
			}
		}
	}()

	log.DefaultLogger.Info(
		"NodeGraph",
		"pid", pid.String(),
	)

	ft := process.Table{}
	pt := process.BuildTable()
	process.Connections(pt)

	if pid != 0 && pt[pid] == nil {
		pid = 0 // reset to default
	}
	if pid > 0 { // build this process' "extended family"
		ft = family(pt, pid)
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

	nm := map[Pid][]any{}
	em := map[string][]any{}
	timestamp := time.Now()

	for _, p := range ft {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				pid == 0 && conn.Peer.Pid >= math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			nm[conn.Self.Pid] = append([]any{
				timestamp,
				int64(conn.Self.Pid),
				pt[conn.Self.Pid].Id.Name,
				conn.Self.Pid.String(),
				longname(pt, conn.Self.Pid),
				longname(pt, pt[conn.Self.Pid].Ppid),
			}, procArc...)

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)

				arc := hostArc
				// name for listen port is device inode: on linux decimal and on darwin hexadecimal
				if _, err := strconv.Atoi(conn.Self.Name); err == nil || conn.Self.Name[0:2] == "0x" { // listen socket
					arc = sockArc
				}

				nm[conn.Peer.Pid] = append([]any{
					timestamp,
					int64(conn.Peer.Pid),
					conn.Type + ":" + port,
					gocore.Hostname(host),
					host,
					gocore.Hostname(host),
				}, arc...)

				// flip the source and target to get Host shown to left in node graph
				id := fmt.Sprintf("%d -> %d", conn.Peer.Pid, conn.Self.Pid)
				em[id] = []any{
					timestamp,
					id,
					int64(conn.Peer.Pid),
					int64(conn.Self.Pid),
					host,
					shortname(pt, conn.Self.Pid),
					fmt.Sprintf(
						"%s:%s ‑> %s", // non-breaking space/hyphen
						conn.Type,
						conn.Peer.Name,
						conn.Self.Name,
					),
				}
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				arc := dataArc
				if conn.Type != "REG" && conn.Type != "DIR" {
					arc = kernArc
				}

				nm[conn.Peer.Pid] = append([]any{
					timestamp,
					int64(conn.Peer.Pid),
					conn.Type,
					conn.Peer.Name,
					peer,
					shortname(pt, conn.Self.Pid),
				}, arc...)

				// show edge for data connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				if _, ok := em[id]; !ok {
					em[id] = []any{
						timestamp,
						id,
						int64(conn.Self.Pid),
						int64(conn.Peer.Pid),
						shortname(pt, conn.Self.Pid),
						url.QueryEscape(peer),
						fmt.Sprintf(
							"%s:%s ‑> %s", // non-breaking space/hyphen
							conn.Type,
							conn.Self.Name,
							conn.Peer.Name,
						),
					}
				}
			} else { // peer is process
				peer := shortname(pt, conn.Peer.Pid)
				nm[conn.Peer.Pid] = append([]any{
					timestamp,
					int64(conn.Peer.Pid),
					pt[conn.Peer.Pid].Id.Name,
					conn.Peer.Pid.String(),
					longname(pt, conn.Peer.Pid),
					longname(pt, pt[conn.Peer.Pid].Ppid),
				}, procArc...)

				// show edge for inter-process connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				di := fmt.Sprintf("%d -> %d", conn.Peer.Pid, conn.Self.Pid)

				_, ok := em[id]
				if ok {
					em[id][6] = (em[id][6]).(string) + fmt.Sprintf(
						"\n%s:%s ‑> %s", // non-breaking space/hyphen
						conn.Type,
						conn.Self.Name,
						conn.Peer.Name,
					)
				} else if _, ok = em[di]; ok {
					em[di][6] = (em[di][6]).(string) + fmt.Sprintf(
						"\n%s:%s ‑> %s", // non-breaking space/hyphen
						conn.Type,
						conn.Peer.Name,
						conn.Self.Name,
					)
				} else {
					em[id] = []any{
						timestamp,
						id,
						int64(conn.Self.Pid),
						int64(conn.Peer.Pid),
						shortname(pt, conn.Self.Pid),
						peer,
						fmt.Sprintf(
							"%s ‑> %s\n%s:%s ‑> %s", // non-breaking space/hyphen
							shortname(pt, conn.Self.Pid),
							shortname(pt, conn.Peer.Pid),
							conn.Type,
							conn.Self.Name,
							conn.Peer.Name,
						),
					}
				}
			}
		}
	}

	ns := make([][]any, len(nm))
	i := 0
	for _, n := range nm {
		ns[i] = n
		i++
	}

	sort.Slice(ns, func(i, j int) bool {
		return ns[i][1].(int64) < ns[j][1].(int64)
	})

	es := make([][]any, len(em))
	i = 0
	for _, e := range em {
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
	pids := process.FlatTree(process.FindTree(process.BuildTree(pt), pid)) // descendants
	for _, pid := range pids {
		ft[pid] = pt[pid]
	}
	return ft
}

// longname formats the full Executable name and pid.
func longname(pt process.Table, pid Pid) string {
	if p, ok := pt[pid]; ok {
		name := p.Executable
		if name == "" {
			name = p.Id.Name
		}
		return fmt.Sprintf("%s[%d]", name, pid)
	}
	return ""
}

// shortname formats process name and pid.
func shortname(pt process.Table, pid Pid) string {
	if p, ok := pt[pid]; ok {
		return fmt.Sprintf("%s[%d]", p.Id.Name, pid)
	}
	return ""
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
