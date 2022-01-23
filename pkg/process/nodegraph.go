// Copyright © 2021 The Gomon Project.

package process

import (
	"context"
	"encoding/json"
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
	// host/proc/fileArc specify the arc for the circle drawn around a node.
	// Each arc has a specific color set in its field metadata to create a circle that identifies the node type.
	hostArc = []interface{}{1.0, 0.0, 0.0, 0.0, 0.0} // red
	kernArc = []interface{}{0.0, 1.0, 0.0, 0.0, 0.0} // magenta
	procArc = []interface{}{0.0, 0.0, 1.0, 0.0, 0.0} // blue
	dirArc  = []interface{}{0.0, 0.0, 0.0, 1.0, 0.0} // green
	fileArc = []interface{}{0.0, 0.0, 0.0, 0.0, 1.0} // yellow
)

type (
	// query model of the datasource.
	query struct {
		Pid     `json:"pid"`
		Kernel  bool `json:"kernel"`
		Daemons bool `json:"daemons"`
		Files   bool `json:"files"`
	}
)

func parse(dq *backend.DataQuery) (query, error) {
	var q query
	if err := json.Unmarshal(dq.JSON, &q); err != nil {
		return q, err
	}

	return query{
		Pid:     q.Pid,
		Kernel:  q.Kernel || q.Pid > 0,
		Daemons: q.Daemons || q.Pid > 0,
		Files:   q.Files || q.Pid > 0,
	}, nil

}

// nodeGraph produces the process connections node graph.
func nodeGraph(ctx context.Context, dq *backend.DataQuery) backend.DataResponse {
	nodes, edges := frames()

	pt := buildTable()
	conns := connections(pt)

	q, err := parse(dq)
	if err != nil {
		return backend.DataResponse{
			Error: err,
		}
	}
	if q.Pid > 0 && pt[q.Pid] == nil {
		q = query{} // reset to default
	}
	if q.Pid > 0 {
		ft := map[Pid]struct{}{q.Pid: {}}
		for pid := pt[q.Pid].Ppid; pid > 1; pid = pt[pid].Ppid { // ancestors
			ft[pid] = struct{}{}
		}
		ps := flatTree(findTree(buildTree(pt), q.Pid), 0) // descendants
		for _, pid := range ps {
			ft[pid] = struct{}{}
		}
		var cs []connection
		for _, conn := range conns {
			_, ok := ft[conn.self.pid]
			if !ok {
				_, ok = ft[conn.peer.pid]
			}
			if ok {
				cs = append(cs, conn)
			}
		}
		conns = cs
	}

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
		} else if conn.self.pid == 0 { // ignore kernel
		} else if conn.self.pid == 1 {
			if q.Daemons {
				pc := []interface{}{filepath.Base(pt[conn.peer.pid].Exec), conn.peer.pid.String(), pt[conn.peer.pid].Exec}
				peer := fmt.Sprintf("%s[%s]", pc[:2]...)
				nm[peer] = append(pc, procArc...)
			}
		} else if conn.peer.pid == 1 {
			if q.Daemons {
				sc := []interface{}{filepath.Base(pt[conn.self.pid].Exec), conn.self.pid.String(), pt[conn.self.pid].Exec}
				self := fmt.Sprintf("%s[%s]", sc[:2]...)
				nm[self] = append(sc, procArc...)
			}
		} else { // peer is process
			var peerExec string
			var arc []interface{}
			if conn.peer.pid == 0 {
				if !q.Kernel {
					continue
				}
				peerExec = "kernel"
				arc = kernArc
			} else {
				peerExec = filepath.Base(pt[conn.peer.pid].Exec)
				arc = procArc
			}

			sc := []interface{}{filepath.Base(pt[conn.self.pid].Exec), conn.self.pid.String(), pt[conn.self.pid].Exec}
			self := fmt.Sprintf("%s[%s]", sc[:2]...)
			nm[self] = append(sc, procArc...)

			pc := []interface{}{peerExec, conn.peer.pid.String(), pt[conn.peer.pid].Exec}
			peer := fmt.Sprintf("%s[%s]", pc[:2]...)
			nm[peer] = append(pc, arc...)

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

	if q.Files {
		for i, conn := range conns {
			i := Pid(math.MaxInt32 + i)
			if conn.peer.pid == math.MaxInt32 { // peer is file
				self := fmt.Sprintf("%s[%d]", filepath.Base(pt[conn.self.pid].Exec), conn.self.pid)
				if _, ok := nm[self]; !ok {
					continue
				}

				log.DefaultLogger.Debug("FILE NAME",
					"pid", conn.self.pid.String(), // to format as int rather than float
					"name", conn.name,
				)

				peer := conn.name
				var dir, file string
				var arc []interface{}
				switch conn.ftype {
				case "DIR":
					dir = peer + string(filepath.Separator)
					arc = dirArc
				case "REG":
					dir = filepath.Dir(peer)
					file = filepath.Base(peer)
					arc = fileArc
				}
				nm[peer] = append([]interface{}{dir, file, ""}, arc...)

				em[fmt.Sprintf("%d->%s", conn.self.pid, conn.name)] = []interface{}{
					self,
					peer,
					conn.ftype,
					pt[conn.self.pid].Exec,
				}

				// create pseudo process to incorporate file node into process tree
				pt[i] = &process{
					Id: id{
						Name: conn.name,
						Pid:  i,
					},
					Props: Props{
						Ppid: conn.self.pid,
					},
				}
			}
		}
	}

	nodes.Meta.Stats[0].Value = float64(len(nm))
	edges.Meta.Stats[0].Value = float64(len(em))

	// pids := make([]Pid, len(pt))
	// i = 0
	// for pid := range pt {
	// 	pids[i] = pid
	// 	i++
	// }

	// sort.Slice(pids, func(i, j int) bool {
	// 	return pids[i] < pids[j]
	// })

	// for _, pid := range pids {
	// for pid, p := range pt {
	for _, pid := range flatTree(buildTree(pt), 0) {
		p := pt[pid]
		var id string
		if pid < 0 || pid >= math.MaxInt32 { // host or file
			id = p.Id.Name
		} else if pid == 0 {
			id = "kernel[0]"
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
	nodes = data.NewFrameOfFieldTypes("nodes", 0, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString, data.FieldTypeString, data.FieldTypeFloat64, data.FieldTypeFloat64, data.FieldTypeFloat64, data.FieldTypeFloat64, data.FieldTypeFloat64)
	nodes.SetMeta(&data.FrameMeta{
		Path:                   "process_node",
		PreferredVisualization: data.VisType("nodeGraph"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Node Count",
			},
		}},
	})
	nodes.SetFieldNames("id", "mainStat", "secondaryStat", "title", "arc__host", "arc__kernel", "arc__process", "arc__directory", "arc__file")
	nodes.Fields[0].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
	}
	nodes.Fields[1].Config = &data.FieldConfig{
		DisplayName: "Command/Service/Directory",
		Path:        "command",
	}
	nodes.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Process/Host/File",
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
		Color:       map[string]interface{}{"mode": "fixed", "fixedColor": "magenta"},
		DisplayName: "Kernel",
		Path:        "kernel",
	}
	nodes.Fields[6].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"mode": "fixed", "fixedColor": "blue"},
		DisplayName: "Process",
		Path:        "process",
	}
	nodes.Fields[7].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"mode": "fixed", "fixedColor": "green"},
		DisplayName: "Directory",
		Path:        "directory",
	}
	nodes.Fields[8].Config = &data.FieldConfig{
		Color:       map[string]interface{}{"mode": "fixed", "fixedColor": "yellow"},
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
