// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"cmp"
	"fmt"
	"math"
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/process"
)

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid

	// query parameters for request.
	Query struct {
		pid  Pid
		link string
	}
)

var (
	// host/proc specify the arc for the circle drawn around a node.
	// Each arc has a specific color set in its field metadata to create a circle that identifies the node type.
	hostColor = []any{1.0, 0.0, 0.0, 0.0, 0.0} // red
	procColor = []any{0.0, 1.0, 0.0, 0.0, 0.0} // blue
	dataColor = []any{0.0, 0.0, 1.0, 0.0, 0.0} // yellow
	sockColor = []any{0.0, 0.0, 0.0, 1.0, 0.0} // magenta
	kernColor = []any{0.0, 0.0, 0.0, 0.0, 1.0} // cyan
	red       = map[string]any{"mode": "fixed", "fixedColor": "red"}
	blue      = map[string]any{"mode": "fixed", "fixedColor": "blue"}
	yellow    = map[string]any{"mode": "fixed", "fixedColor": "yellow"}
	magenta   = map[string]any{"mode": "fixed", "fixedColor": "magenta"}
	cyan      = map[string]any{"mode": "fixed", "fixedColor": "cyan"}
)

// color defines the color for grafana nodes.
func color(conn process.Connection) []any {
	var color []any
	if conn.Peer.Pid < 0 {
		color = hostColor
		// name for listen port is device inode: on linux decimal and on darwin hexadecimal
		if _, err := strconv.Atoi(conn.Self.Name); err == nil || conn.Self.Name[0:2] == "0x" { // listen socket
			color = sockColor
		}
	} else if conn.Peer.Pid >= math.MaxInt32 {
		color = dataColor
		if conn.Type != "REG" && conn.Type != "DIR" {
			color = kernColor
		}
	} else {
		color = procColor
	}
	return color
}

// Nodegraph produces the process connections node graph.
func Nodegraph(link string, queryPid Pid) backend.DataResponse {
	return backend.DataResponse{
		Frames: process.Nodegraph(Query{pid: queryPid, link: link}),
	}
}

// Pid returns the query's pid.
func (query Query) Pid() Pid {
	return query.pid
}

// Arrow returns the character to use in edges' tooltip connections list.
func (query Query) Arrow() string {
	return " -> "
}

func (query Query) BuildGraph(
	tb process.Table,
	itr process.Tree,
	hosts map[Pid][]any,
	prcss map[int]map[Pid][]any,
	datas map[Pid][]any,
	edges map[[2]Pid][]any,
) []*data.Frame {
	maxConnections := 0

	// add process nodes to each cluster, sort connections for tooltip
	for depth, pid := range itr.All() {
		prcss[depth][pid] = query.ProcNode(tb[pid])
		for id, edge := range edges {
			self := id[0]
			peer := id[1]
			if self == pid || self < 0 && peer == pid {
				slices.SortFunc(edge[5:], func(a, b any) int { // tooltips list edge's connection endpoints
					if strings.HasPrefix(a.(string), "parent") {
						return -1
					} else if strings.HasPrefix(b.(string), "parent") {
						return 1
					} else {
						return cmp.Compare(a.(string), b.(string))
					}
				})
				if maxConnections < len(edge)-5 {
					maxConnections = len(edge) - 5
				}
			}
		}
	}

	// build hosts cluster
	ns := cluster(tb, hosts)

	// build processes clusters
	for depth := range len(prcss) {
		ns = append(ns, cluster(tb, prcss[depth])...)
	}

	// build datas (files, sockets, pipes, ...) cluster
	ns = append(ns, cluster(tb, datas)...)

	// add the edges
	var es [][]any
	// for id, edge := range edges { // does sorting improve graph consistency?
	for _, edge := range gocore.Ordered(edges, func(a, b [2]Pid) int {
		return cmp.Or(
			cmp.Compare(a[0], b[0]),
			cmp.Compare(a[1], b[1]),
		)
	}) {
		es = append(es, edge)
	}

	return nodeFrames(query.link, ns, es, maxConnections)
}

func (query Query) HostNode(conn process.Connection) []any {
	host, port, _ := net.SplitHostPort(conn.Peer.Name)
	return append([]any{
		int64(conn.Peer.Pid),
		conn.Type + ":" + port,
		gocore.Hostname(host),
		host,
	}, color(conn)...)
}

func (query Query) HostEdge(tb process.Table, conn process.Connection) []any {
	host, _, _ := net.SplitHostPort(conn.Peer.Name)
	return []any{
		fmt.Sprintf("%d -> %d", conn.Peer.Pid, conn.Self.Pid),
		int64(conn.Peer.Pid),
		int64(conn.Self.Pid),
		host,
		tb[conn.Self.Pid].Shortname(),
	}
}

func (query Query) DataNode(conn process.Connection) []any {
	return append([]any{
		int64(conn.Peer.Pid),
		conn.Type,
		conn.Peer.Name,
		conn.Type + ":" + conn.Peer.Name,
	}, color(conn)...)
}

func (query Query) DataEdge(tb process.Table, conn process.Connection) []any {
	peer := conn.Type + ":" + conn.Peer.Name
	return []any{
		fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid),
		int64(conn.Self.Pid),
		int64(conn.Peer.Pid),
		tb[conn.Self.Pid].Shortname(),
		peer,
	}
}

func (query Query) ProcNode(p *process.Process) []any {
	return append([]any{
		int64(p.Pid),
		p.Id.Name,
		p.Pid.String(),
		p.Longname(),
	}, procColor...)
}

func (query Query) ProcEdge(tb process.Table, self, peer Pid) []any {
	return []any{
		fmt.Sprintf("%d -> %d", self, peer),
		int64(self),
		int64(peer),
		tb[self].Shortname(),
		tb[peer].Shortname(),
	}
}

// cluster returns list of nodes in cluster and id of first node.
func cluster(tb process.Table, nodes map[Pid][]any) [][]any {
	if len(nodes) == 0 {
		return [][]any{}
	}

	var ns [][]any
	// for _, node := range nodes { // does sorting improve graph consistency?
	for _, node := range gocore.Ordered(nodes, func(a, b Pid) int {
		if a >= 0 && a < math.MaxInt32 { // processes
			if n := cmp.Compare(
				filepath.Base(tb[a].Executable),
				filepath.Base(tb[b].Executable),
			); n != 0 {
				return n
			}
		}
		return cmp.Compare(a, b)
	}) {
		ns = append(ns, node)
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
