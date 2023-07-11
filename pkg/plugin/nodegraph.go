// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"fmt"
	"math"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/process"
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

// color defines the color for grafana node arcs.
func color(conn process.Connection) []any {
	var arc []any
	if conn.Peer.Pid < 0 {
		arc = hostArc
		// name for listen port is device inode: on linux decimal and on darwin hexadecimal
		if _, err := strconv.Atoi(conn.Self.Name); err == nil || conn.Self.Name[0:2] == "0x" { // listen socket
			arc = sockArc
		}
	} else if conn.Peer.Pid >= math.MaxInt32 {
		arc = dataArc
		if conn.Type != "REG" && conn.Type != "DIR" {
			arc = kernArc
		}
	} else {
		arc = procArc
	}
	return arc
}

// Nodegraph produces the process connections node graph.
func Nodegraph(link string, queryPid Pid) (resp backend.DataResponse) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			gocore.Error("nodegraph", fmt.Errorf("%v", r), map[string]string{
				"stacktrace": string(buf),
			}).Err()
			if e, ok := r.(error); ok {
				resp.Error = e
			} else {
				resp.Error = fmt.Errorf("nodegraph panic: %v", r)
			}
		}
	}()

	var (
		timestamp    = time.Now()
		hosts        = map[Pid][]any{}         // The host (IP) nodes in the leftmost cluster.
		prcss        = map[int]map[Pid][]any{} // The process nodes in the process clusters.
		datas        = map[Pid][]any{}         // The file, unix socket, pipe and kernel connections in the rightmost cluster.
		include      = map[Pid]struct{}{}      // Processes that have a connection to include in report.
		edges        = map[[2]Pid][]any{}
		edgeTooltips = map[[2]Pid]map[string]struct{}{}
	)

	tb := process.BuildTable()
	tr := process.BuildTree(tb)
	process.Connections(tb)

	if queryPid != 0 && tb[queryPid] == nil {
		queryPid = 0 // reset to default
	}

	gocore.Error("nodegraph", nil, map[string]string{
		"pid": queryPid.String(),
	}).Info()

	pt := process.Table{}
	if queryPid > 0 { // build this process' "extended family"
		pt = family(tb, tr, queryPid)
	} else { // only consider non-daemon and remote host connected processes
		for pid, p := range tb {
			if p.Ppid > 1 {
				for pid, p := range family(tb, tr, pid) {
					pt[pid] = p
				}
			}
			for _, conn := range p.Connections {
				if conn.Peer.Pid < 0 {
					pt[conn.Self.Pid] = tb[conn.Self.Pid]
				}
			}
		}
	}

	pids := make([]Pid, 0, len(pt))
	for pid := range pt {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})

	for _, pid := range pids {
		p := pt[pid]
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				queryPid == 0 && conn.Peer.Pid >= math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			include[conn.Self.Pid] = struct{}{}

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)

				if _, ok := hosts[conn.Peer.Pid]; !ok {
					hosts[conn.Peer.Pid] = append([]any{
						timestamp,
						int64(conn.Peer.Pid),
						conn.Type + ":" + port,
						gocore.Hostname(host),
						host,
						"",
					}, color(conn)...)
				}

				// flip the source and target to get Host shown to left in node graph
				id := [2]Pid{conn.Peer.Pid, conn.Self.Pid}
				edges[id] = []any{nil, "", 0, 0, host, shortname(tb, id[1])}
				if _, ok := edgeTooltips[id]; !ok {
					edgeTooltips[id] = map[string]struct{}{}
				}
				edgeTooltips[id][fmt.Sprintf(
					"%s:%s->%s",
					conn.Type,
					conn.Peer.Name,
					conn.Self.Name,
				)] = struct{}{}
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				if _, ok := datas[conn.Peer.Pid]; !ok {
					datas[conn.Peer.Pid] = append([]any{
						timestamp,
						int64(conn.Peer.Pid),
						conn.Type,
						conn.Peer.Name,
						peer,
						"",
					}, color(conn)...)
				}

				id := [2]Pid{conn.Self.Pid, conn.Peer.Pid}
				edges[id] = []any{nil, "", 0, 0, shortname(tb, id[0]), peer}
				if _, ok := edgeTooltips[id]; !ok {
					edgeTooltips[id] = map[string]struct{}{}
				}
				edgeTooltips[id][fmt.Sprintf(
					"%s->%s",
					shortname(tb, conn.Self.Pid),
					peer,
				)] = struct{}{}
			} else { // peer is process
				include[conn.Peer.Pid] = struct{}{}
				pids := append(conn.Peer.Pid.Ancestors(tb), conn.Peer.Pid)
				for i := range pids[:len(pids)-1] { // add "in-laws"
					include[pids[i]] = struct{}{}
					id := [2]Pid{pids[i], pids[i+1]}
					edges[id] = []any{nil, "", 0, 0, shortname(tb, id[0]), shortname(tb, id[1])}
					if _, ok := edgeTooltips[id]; !ok {
						edgeTooltips[id] = map[string]struct{}{}
					}
					edgeTooltips[id][fmt.Sprintf(
						"parent:%s->%s",
						shortname(tb, pids[i]),
						shortname(tb, pids[i+1]),
					)] = struct{}{}
				}

				// show edge for inter-process connections only once
				self, peer := conn.Self.Name, conn.Peer.Name
				selfPid, peerPid := conn.Self.Pid, conn.Peer.Pid
				if len(selfPid.Ancestors(tb)) > len(peerPid.Ancestors(tb)) ||
					len(selfPid.Ancestors(tb)) == len(peerPid.Ancestors(tb)) && conn.Self.Pid > conn.Peer.Pid {
					selfPid, peerPid = peerPid, selfPid
					self, peer = peer, self
				}
				id := [2]Pid{selfPid, peerPid}
				edges[id] = []any{nil, "", 0, 0, shortname(tb, id[0]), shortname(tb, id[1])}
				if _, ok := edgeTooltips[id]; !ok {
					edgeTooltips[id] = map[string]struct{}{}
				}
				edgeTooltips[id][fmt.Sprintf(
					"%s:%s->%s",
					conn.Type,
					self,
					peer,
				)] = struct{}{}
			}
		}
	}

	pids = make([]Pid, 0, len(include))
	for pid := range include {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})

	maxConnections := 0
	for _, pid := range pids {
		depth := len(pid.Ancestors(tb))
		if _, ok := prcss[depth]; !ok {
			prcss[depth] = map[Pid][]any{}
		}

		prcss[depth][pid] = append([]any{
			timestamp,
			int64(pid),
			tb[pid].Id.Name,
			pid.String(),
			longname(tb, pid),
			longname(tb, tb[pid].Ppid),
		}, procArc...)

		for id, tooltip := range edgeTooltips {
			self := id[0]
			peer := id[1]
			if self == pid || self < 0 && peer == pid {
				if len(tooltip) > 0 {
					var tts []any
					for tt := range tooltip {
						tts = append(tts, tt)
					}
					sort.Slice(tts, func(i, j int) bool {
						if strings.HasPrefix(tts[i].(string), "parent") {
							return true
						} else if strings.HasPrefix(tts[j].(string), "parent") {
							return false
						} else {
							return tts[i].(string) < tts[j].(string)
						}
					})
					if maxConnections < len(tooltip) {
						maxConnections = len(tooltip)
					}

					edges[id] = []any{
						timestamp,
						fmt.Sprintf("%d -> %d", id[0], id[1]),
						int64(id[0]),
						int64(id[1]),
						edges[id][4],
						edges[id][5],
					}
					edges[id] = append(edges[id], tts...)
				}
			}
		}
	}

	ns := cluster(hosts)

	var depths []int
	for depth := range prcss {
		depths = append(depths, depth)
	}
	sort.Ints(depths)

	for _, depth := range depths {
		ns = append(ns, cluster(prcss[depth])...)
	}

	ns = append(ns, cluster(datas)...)

	ids := make([][2]Pid, 0, len(edges))
	for id := range edges {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		a, b, c, d := ids[i][0], ids[j][0], ids[i][1], ids[j][1]
		return a < b ||
			a == b && c < d
	})

	es := make([][]any, 0, len(edges))
	for _, id := range ids {
		es = append(es, edges[id])
	}

	resp.Frames = nodeFrames(link, ns, es, maxConnections)

	return
}

// family identifies all of the ancestor and children processes of a process.
func family(tb process.Table, tr process.Tree, pid Pid) process.Table {
	pt := process.Table{}
	for _, pid := range pid.Ancestors(tb) { // ancestors
		pt[pid] = tb[pid]
	}

	tr = tr.FindTree(pid)
	for _, pid := range tr.Flatten(tb, func(node Pid, pt process.Table) int { return order(tr) }) {
		pt[pid] = tb[pid]
	}

	return pt
}

// order returns the process' depth in the tree for sorting.
func order(tr process.Tree) int {
	var depth int
	for _, tr := range tr {
		dt := order(tr) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

// longname formats the full Executable name and pid.
func longname(tb process.Table, pid Pid) string {
	if p, ok := tb[pid]; ok {
		name := p.Executable
		if name == "" {
			name = p.Id.Name
		}
		return fmt.Sprintf("%s[%d]", name, pid)
	}
	return ""
}

// shortname formats process name and pid.
func shortname(tb process.Table, pid Pid) string {
	if p, ok := tb[pid]; ok {
		return fmt.Sprintf("%s[%d]", p.Id.Name, pid)
	}
	return ""
}

func cluster(nodes map[Pid][]any) [][]any {
	pids := make([]Pid, 0, len(nodes))
	for pid := range nodes {
		pids = append(pids, pid)
	}

	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})

	var ns [][]any
	for _, pid := range pids {
		ns = append(ns, nodes[pid])
	}

	return ns
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
