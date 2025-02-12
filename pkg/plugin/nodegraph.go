// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"cmp"
	"fmt"
	"math"
	"net"
	"path/filepath"
	"runtime"
	"slices"
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

	// prevCPU is used to limit reporting only of processes that consumed CPU since the previous measurement.
	prevCPU = map[Pid]time.Duration{}
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
		include      = process.Table{}         // Processes that have a connection to include in report.
		edges        = map[[2]Pid][]any{}
		edgeTooltips = map[[2]Pid]map[string]struct{}{}
	)

	tb := process.BuildTable()
	tr := process.BuildTree(tb)
	process.Connections(tb)

	currCPU := map[Pid]time.Duration{}
	for pid, p := range tb {
		currCPU[pid] = p.Total
	}

	if queryPid != 0 && tb[queryPid] == nil {
		queryPid = 0 // reset to default
	}

	gocore.Error("nodegraph", nil, map[string]string{
		"pid": queryPid.String(),
	}).Info()

	pt := process.Table{}
	if queryPid > 0 { // build this process' "extended family"
		for _, pid := range tr.Family(queryPid).All() {
			pt[pid] = tb[pid]
		}
		for _, p := range tb {
			for _, conn := range p.Connections {
				if conn.Peer.Pid == queryPid {
					for _, pid := range tr.Ancestors(conn.Self.Pid) {
						pt[pid] = tb[pid]
					}
					pt[conn.Self.Pid] = tb[conn.Self.Pid]
				}
			}
		}
	} else { // only report non-daemon, remote host connected, and cpu consuming processes
		for pid, p := range tb {
			if pcpu, ok := prevCPU[pid]; !ok || pcpu < currCPU[pid] {
				pt[pid] = tb[pid]
			}
			if p.Ppid > 1 {
				for _, pid := range tr.Family(pid).All() {
					pt[pid] = tb[pid]
				}
			}
			for _, conn := range p.Connections {
				if conn.Peer.Pid < 0 {
					pt[conn.Self.Pid] = tb[conn.Self.Pid]
				}
			}
		}
	}

	prevCPU = currCPU

	for pid, p := range pt {
		include[pid] = p
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd process
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				queryPid == 0 && conn.Peer.Pid >= math.MaxInt32 || // ignore data connections for the "all process" query
				(queryPid > 0 && queryPid != conn.Self.Pid && // ignore hosts and datas of connected processes
					(conn.Peer.Pid < 0 || conn.Peer.Pid >= math.MaxInt32)) {
				continue
			}

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
					"%s:%s->%s[%d]",
					conn.Type,
					conn.Peer.Name,
					conn.Self.Name,
					conn.Self.Pid,
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
				include[conn.Peer.Pid] = tb[conn.Peer.Pid]
				for _, pid := range tr.Ancestors(conn.Peer.Pid) {
					include[pid] = tb[pid] // add ancestor for BuildTree
				}

				// show edge for inter-process connections only once
				self, peer := conn.Self.Name, conn.Peer.Name
				selfPid, peerPid := conn.Self.Pid, conn.Peer.Pid
				if len(tr.Ancestors(selfPid)) > len(tr.Ancestors(peerPid)) ||
					len(tr.Ancestors(selfPid)) == len(tr.Ancestors(peerPid)) && conn.Self.Pid > conn.Peer.Pid {
					selfPid, peerPid = peerPid, selfPid
					self, peer = peer, self
				}
				id := [2]Pid{selfPid, peerPid}
				edges[id] = []any{nil, "", 0, 0, shortname(tb, id[0]), shortname(tb, id[1])}
				if _, ok := edgeTooltips[id]; !ok {
					edgeTooltips[id] = map[string]struct{}{}
				}
				edgeTooltips[id][fmt.Sprintf(
					"%s:%s[%d]->%s[%d]",
					conn.Type,
					self,
					selfPid,
					peer,
					peerPid,
				)] = struct{}{}
			}
		}
	}

	itr := process.BuildTree(include)

	// connect the parents to their children
	var parents []Pid
	for depth, pid := range itr.All() {
		if depth == 0 {
			parents = []Pid{pid}
			continue
		} else if depth < len(parents) {
			parents = parents[:depth]
			parents = append(parents, pid)
		} else if depth == len(parents) {
			parents = append(parents, pid)
		}
		id := [2]Pid{parents[depth-1], parents[depth]}
		edges[id] = []any{nil, "", 0, 0, shortname(tb, id[0]), shortname(tb, id[1])}
		if _, ok := edgeTooltips[id]; !ok {
			edgeTooltips[id] = map[string]struct{}{}
		}
		edgeTooltips[id][fmt.Sprintf(
			"parent:%s->%s",
			shortname(tb, id[0]),
			shortname(tb, id[1]),
		)] = struct{}{}
	}

	for i := range itr.DepthTree() {
		prcss[i] = map[Pid][]any{}
	}

	maxConnections := 0
	for depth, pid := range itr.All() {
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
					slices.SortFunc(tts, func(a, b any) int {
						if strings.HasPrefix(a.(string), "parent") {
							return -1
						} else if strings.HasPrefix(b.(string), "parent") {
							return 1
						} else {
							return cmp.Compare(a.(string), b.(string))
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

	ns := cluster(tb, hosts)

	for depth := range len(prcss) {
		ns = append(ns, cluster(tb, prcss[depth])...)
	}

	ns = append(ns, cluster(tb, datas)...)

	ids := make([][2]Pid, 0, len(edges))
	for id := range edges {
		ids = append(ids, id)
	}

	slices.SortFunc(ids, func(a, b [2]Pid) int {
		return cmp.Or(
			cmp.Compare(a[0], b[0]),
			cmp.Compare(a[1], b[1]),
		)
	})

	es := make([][]any, 0, len(edges))
	for _, id := range ids {
		es = append(es, edges[id])
	}

	resp.Frames = nodeFrames(link, ns, es, maxConnections)

	return
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

// cluster returns list of nodes in cluster and id of first node.
func cluster(tb process.Table, nodes map[Pid][]any) [][]any {
	if len(nodes) == 0 {
		return [][]any{}
	}

	pids := make([]Pid, 0, len(nodes))
	for pid := range nodes {
		pids = append(pids, pid)
	}

	slices.SortFunc(pids, func(a, b Pid) int {
		if a >= 0 && a < math.MaxInt32 { // processes
			if n := cmp.Compare(
				filepath.Base(tb[a].Executable),
				filepath.Base(tb[b].Executable),
			); n != 0 {
				return n
			}
		}
		return cmp.Compare(a, b)
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
