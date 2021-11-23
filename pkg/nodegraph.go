// Copyright © 2021 The Gomon Project.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var (
	// host/proc/fileArc specify the arc for the circle drawn around a node.
	// Each arc has a specific color set in its field metadata to create a circle that identifies the node type.
	hostArc = func() []interface{} { return []interface{}{1.0, 0.0, 0.0} } // red
	procArc = func() []interface{} { return []interface{}{0.0, 1.0, 0.0} } // green
	fileArc = func() []interface{} { return []interface{}{0.0, 0.0, 1.0} } // blue
)

func query(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var qm queryModel
	if err := json.Unmarshal(query.JSON, &qm); err != nil {
		return backend.DataResponse{
			Error: err,
		}
	}

	// create nodegraph nodes and edges data frames for response
	nodes, edges := nodeGraph(qm)

	return backend.DataResponse{
		Frames: data.Frames{nodes, edges},
	}
}

// nodeGraph produces the process connections node graph.
func nodeGraph(qm queryModel) (*data.Frame, *data.Frame) {
	nodes, edges := frames()

	pt := buildTable()
	if qm.Pid > 0 && pt[qm.Pid] == nil {
		qm = queryModel{} // reset to default
	}
	if qm.Pid > 0 {
		ft := processTable{0: pt[0], 1: pt[1], qm.Pid: pt[qm.Pid]}
		for pid := pt[qm.Pid].Ppid; pid > 1; pid = pt[pid].Ppid {
			ft[pid] = pt[pid]
		}
		ps := flatTree(findTree(buildTree(pt), qm.Pid), 0)
		for _, pid := range ps {
			ft[pid] = pt[pid]
		}
		pt = ft
	}

	conns := connections(pt)

	nm := map[string][]interface{}{}
	em := map[string][]interface{}{}

	i := Pid(-1)
	j := Pid(1)
	for _, conn := range conns {
		if conn.self.pid == -1 { // external network connections (self.pid/fd = -1/-1)
			self := conn.ftype + ":" + conn.self.name
			peer := fmt.Sprintf("%s[%d]", filepath.Base(conn.peer.command), conn.peer.pid)

			nm[self] = append([]interface{}{self, conn.self.command}, hostArc()...)
			nm[peer] = append([]interface{}{peer, ""}, procArc()...)

			host, _, _ := net.SplitHostPort(conn.peer.name)

			em[fmt.Sprintf("%s->%s:%d", self, peer, conn.peer.fd)] = []interface{}{
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
					Name: conn.ftype + ":" + conn.self.name,
					Pid:  i,
				},
				Props: Props{
					Ppid: ppid,
				},
			}
			pt[pid].Ppid = i
			i--
		} else if conn.peer.pid == math.MaxInt32 { // peer is file
			if conn.self.pid > 1 && qm.Files && (qm.Daemons || pt[conn.self.pid].Ppid > 1) {
				self := fmt.Sprintf("%s[%d]", filepath.Base(conn.self.command), conn.self.pid)
				peer := conn.peer.name

				nm[self] = append([]interface{}{self, ""}, procArc()...)
				nm[peer] = append([]interface{}{peer, ""}, fileArc()...)

				dir := filepath.Dir(conn.peer.name) + string(filepath.Separator)
				em[fmt.Sprintf("%d:%d->%s", conn.self.pid, conn.self.fd, conn.peer.name)] = []interface{}{
					self,
					peer,
					filepath.Dir(dir),
					filepath.Base(dir),
				}

				// create pseudo process to incorporate file node into process tree
				pt[math.MaxInt32+j] = &process{
					Id: id{
						Name: conn.peer.name,
						Pid:  math.MaxInt32 + j,
					},
					Props: Props{
						Ppid: conn.self.pid,
					},
				}
				j++
			}
		} else if conn.self.pid == 0 { // ignore kernel
			continue
		} else if conn.self.pid == 1 {
			if qm.Daemons {
				peer := fmt.Sprintf("%s[%d]", filepath.Base(conn.peer.command), conn.peer.pid)
				nm[peer] = append([]interface{}{peer, ""}, procArc()...)
			}
		} else if conn.peer.pid == 1 {
			if qm.Daemons {
				self := fmt.Sprintf("%s[%d]", filepath.Base(conn.self.command), conn.self.pid)
				nm[self] = append([]interface{}{self, ""}, procArc()...)
			}
		} else { // peer is process
			if !qm.Kernel && conn.peer.pid == 0 {
				continue
			}
			if !qm.Syslog && conn.ftype == "UNIX" && strings.HasSuffix(conn.name, filepath.Join("var", "run", "syslog")) {
				continue
			}

			self := fmt.Sprintf("%s[%d]", filepath.Base(conn.self.command), conn.self.pid)
			peer := fmt.Sprintf("%s[%d]", filepath.Base(conn.peer.command), conn.peer.pid)

			nm[self] = append([]interface{}{self, ""}, procArc()...)
			nm[peer] = append([]interface{}{peer, ""}, procArc()...)

			em[fmt.Sprintf("%d:%d->%d:%d", conn.self.pid, conn.self.fd, conn.peer.pid, conn.peer.fd)] = []interface{}{
				self,
				peer,
				conn.ftype,
				conn.name,
			}
		}
	}

	nodes.Meta.Stats[0].Value = float64(len(nm))
	edges.Meta.Stats[0].Value = float64(len(em))

	for pid, p := range pt {
		var id string
		if pid < 0 || pid >= math.MaxInt32 { // host or file
			id = p.Id.Name
		} else if pid == 0 {
			id = "kernel[0]"
		} else { // process
			id = filepath.Base(p.Exec) + "[" + strconv.Itoa(int(pid)) + "]" // process
		}
		if values, ok := nm[id]; ok {
			log.DefaultLogger.Debug("Peer node found",
				"node", id,
			)
			nodes.AppendRow(append([]interface{}{id}, values...)...)
			delete(nm, id)
		}
	}
	for id := range nm {
		log.DefaultLogger.Warn("UNRESOLVED EXTRA NODE!!!",
			"node", id,
		)
	}

	for id, values := range em {
		edges.AppendRow(append([]interface{}{id}, values...)...)
	}

	return nodes, edges
}

func frames() (nodes *data.Frame, edges *data.Frame) {
	nodes = data.NewFrameOfFieldTypes("nodes", 0, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString, data.FieldTypeFloat64, data.FieldTypeFloat64, data.FieldTypeFloat64)
	nodes.SetMeta(&data.FrameMeta{
		Path:                   "process_node",
		PreferredVisualization: data.VisType("nodeGraph"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Node Count",
			},
		}},
	})
	nodes.SetFieldNames("id", "mainStat", "secondaryStat", "arc__host", "arc__process", "arc__file")
	nodes.Fields[0].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
	}
	nodes.Fields[1].Config = &data.FieldConfig{
		DisplayName: "Host/Command/File",
		Path:        "command",
	}
	nodes.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Host/Command/File",
		Path:        "command",
	}
	nodes.Fields[3].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"fixedColor": "red"},
		DisplayName: "Host",
		Path:        "host",
	}
	nodes.Fields[4].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"fixedColor": "green"},
		DisplayName: "Process",
		Path:        "process",
	}
	nodes.Fields[5].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"fixedColor": "blue"},
		DisplayName: "File",
		Path:        "file",
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
