// Copyright © 2021 The Gomon Project.

package process

import (
	"context"
	"fmt"
	"math"
	"net"
	"path/filepath"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var (
	// host/proc specify the arc for the circle drawn around a node.
	// Each arc has a specific color set in its field metadata to create a circle that identifies the node type.
	hostArc = []interface{}{1.0, 0.0} // red
	procArc = []interface{}{0.0, 1.0} // blue
)

// nodeGraph produces the process connections node graph.
func nodeGraph(ctx context.Context, dq *backend.DataQuery) backend.DataResponse {
	nodes, edges := frames()

	pt := buildTable()
	conns := connections(pt)

	nm := map[string][]interface{}{}
	em := map[string][]interface{}{}

	for i, conn := range conns {
		i := Pid(-1 - i)
		if conn.self.pid == -1 { // external network connections (self.pid/fd = -1/-1)
			host, port, _ := net.SplitHostPort(conn.self.name)
			self := conn.ftype + ":" + conn.self.name
			name := hostname(host)
			if name == host {
				name = ""
			}
			nm[self] = append([]interface{}{conn.ftype + ":" + port, host, name}, hostArc...)

			pc := []interface{}{filepath.Base(pt[conn.peer.pid].Exec), conn.peer.pid.String(), pt[conn.peer.pid].Exec}
			peer := fmt.Sprintf("%s[%s]", pc[:2]...)
			nm[peer] = append(pc, procArc...)

			host, _, _ = net.SplitHostPort(conn.peer.name)
			em[fmt.Sprintf("%s->%d", self, conn.peer.pid)] = []interface{}{
				self,
				peer,
				interfaces[host],
				conn.peer.name,
			}

			// create pseudo process to incorporate host node into process tree
			pid := conn.peer.pid
			var ppid Pid
			for ppid = pt[pid].Ppid; ppid > 1; ppid = pt[ppid].Ppid {
				pid = ppid
			}
			pt[i] = &process{
				Id: id{
					Name: self,
					Pid:  i,
				},
				Props: Props{
					Ppid: ppid,
				},
			}
			pt[pid].Ppid = i
		} else if conn.peer.pid == math.MaxInt32 { // peer is file, add node after all processes identified
		} else if conn.self.pid == 0 || conn.self.pid == 1 || conn.peer.pid == 0 || conn.peer.pid == 1 {
		} else { // peer is process
			sc := []interface{}{filepath.Base(pt[conn.self.pid].Exec), conn.self.pid.String(), pt[conn.self.pid].Exec}
			self := fmt.Sprintf("%s[%s]", sc[:2]...)
			nm[self] = append(sc, procArc...)

			pc := []interface{}{filepath.Base(pt[conn.peer.pid].Exec), conn.peer.pid.String(), pt[conn.peer.pid].Exec}
			peer := fmt.Sprintf("%s[%s]", pc[:2]...)
			nm[peer] = append(pc, procArc...)

			t := conn.ftype
			n := conn.name
			if t == "TCP" || t == "UDP" {
				ip, sp, _ := net.SplitHostPort(conn.self.name)
				_, pp, _ := net.SplitHostPort(conn.peer.name)
				t = interfaces[ip] + ":"
				if strings.IndexByte(ip, ':') < 0 {
					t += ip
				} else {
					t += "[" + ip + "]"
				}
				n = ":" + sp + "->:" + pp
			}

			em[fmt.Sprintf("%d->%d", conn.self.pid, conn.peer.pid)] = []interface{}{
				self,
				peer,
				t,
				n,
			}
		}
	}

	nodes.Meta.Stats[0].Value = float64(len(nm))
	edges.Meta.Stats[0].Value = float64(len(em))

	for _, pid := range flatTree(buildTree(pt), 0) {
		p := pt[pid]
		var id string
		if pid < 0 {
			id = p.Id.Name
		} else { // process
			id = filepath.Base(p.Exec) + "[" + pid.String() + "]" // process
		}
		if values, ok := nm[id]; ok {
			log.DefaultLogger.Debug("Peer node found",
				"pid", pid.String(), // to format as int rather than float
				"node", id,
			)
			nodes.AppendRow(append([]interface{}{id}, values...)...)
			delete(nm, id)
		}
	}
	for id := range nm {
		log.DefaultLogger.Warn("UNRESOLVED EXTRA NODE!!!",
			"node", id,
			"frame", nm[id],
		)
	}

	for id, values := range em {
		edges.AppendRow(append([]interface{}{id}, values...)...)
	}

	return backend.DataResponse{
		Frames: data.Frames{nodes, edges},
	}
}

func frames() (nodes *data.Frame, edges *data.Frame) {
	nodes = data.NewFrameOfFieldTypes("nodes", 0, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString, data.FieldTypeFloat64, data.FieldTypeFloat64)
	nodes.SetMeta(&data.FrameMeta{
		Path:                   "process_node",
		PreferredVisualization: data.VisType("nodeGraph"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Node Count",
			},
		}},
	})
	nodes.SetFieldNames("id", "mainStat", "secondaryStat", "title", "arc__host", "arc__process")
	nodes.Fields[0].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
	}
	nodes.Fields[1].Config = &data.FieldConfig{
		DisplayName: "Command/Service",
		Path:        "command",
	}
	nodes.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Process/Host",
		Path:        "process",
	}
	nodes.Fields[3].Config = &data.FieldConfig{
		DisplayName: "Name",
		Path:        "name",
	}
	nodes.Fields[4].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"mode": "fixed", "fixedColor": "red"},
		DisplayName: "Host",
		Path:        "host",
	}
	nodes.Fields[5].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"mode": "fixed", "fixedColor": "blue"},
		DisplayName: "Process",
		Path:        "process",
	}

	edges = data.NewFrameOfFieldTypes("edges", 0, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString)
	edges.SetMeta(&data.FrameMeta{
		Path:                   "relation",
		PreferredVisualization: data.VisType("nodeGraph"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Edge Count",
			},
		}},
	})
	edges.SetFieldNames("id", "source", "target", "mainStat", "secondaryStat")
	edges.Fields[0].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
	}
	edges.Fields[1].Config = &data.FieldConfig{
		DisplayName: "Source ID",
		Path:        "source",
	}
	edges.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Target ID",
		Path:        "target",
	}
	edges.Fields[3].Config = &data.FieldConfig{
		DisplayName: "Type",
		Path:        "type",
	}
	edges.Fields[4].Config = &data.FieldConfig{
		DisplayName: "Relation",
		Path:        "relation",
	}

	return
}
