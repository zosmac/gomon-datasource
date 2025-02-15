// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func nodeFrames(link string, ns, es [][]any, maxConnections int) []*data.Frame {
	timestamp := time.Now()

	nodes := data.NewFrameOfFieldTypes("nodes", len(ns),
		data.FieldTypeTime,
		data.FieldTypeInt64,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeFloat64,
		data.FieldTypeFloat64,
		data.FieldTypeFloat64,
		data.FieldTypeFloat64,
		data.FieldTypeFloat64,
	)
	nodes.SetFieldNames(
		"time",
		"id",
		"mainStat",
		"secondaryStat",
		"detail__name",
		"arc__host",
		"arc__process",
		"arc__data",
		"arc__socket",
		"arc__kernel",
	)
	nodes.SetMeta(&data.FrameMeta{
		Path:                   "node",
		PreferredVisualization: data.VisType("nodeGraph"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Node Count",
			},
			Value: float64(len(ns)),
		}},
	})

	nodes.Fields[0].Config = &data.FieldConfig{
		DisplayName: "Time",
		Path:        "time",
	}
	nodes.Fields[1].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
		Links: []data.DataLink{{
			Title: "${__value.raw}",
			URL:   link,
		}},
	}
	nodes.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Service",
		Path:        "service",
	}
	nodes.Fields[3].Config = &data.FieldConfig{
		DisplayName: "Instance",
		Path:        "instance",
	}
	nodes.Fields[4].Config = &data.FieldConfig{
		DisplayName: "Name",
		Path:        "name",
	}
	nodes.Fields[5].Config = &data.FieldConfig{
		Color:       red,
		DisplayName: "Host",
		Path:        "host",
	}
	nodes.Fields[6].Config = &data.FieldConfig{
		Color:       blue,
		DisplayName: "Process",
		Path:        "process",
	}
	nodes.Fields[7].Config = &data.FieldConfig{
		Color:       yellow,
		DisplayName: "Data",
		Path:        "data",
	}
	nodes.Fields[8].Config = &data.FieldConfig{
		Color:       magenta,
		DisplayName: "Socket",
		Path:        "socket",
	}
	nodes.Fields[9].Config = &data.FieldConfig{
		Color:       cyan,
		DisplayName: "Kernel",
		Path:        "kernel",
	}

	for i, n := range ns {
		nodes.SetRow(i, append([]any{timestamp}, n...)...)
	}

	flds := []data.FieldType{
		data.FieldTypeTime,
		data.FieldTypeString,
		data.FieldTypeInt64,
		data.FieldTypeInt64,
		data.FieldTypeString,
		data.FieldTypeString,
	}
	names := []string{
		"time",
		"id",
		"source",
		"target",
		"mainStat",
		"secondaryStat",
	}
	for i := range maxConnections {
		flds = append(flds, data.FieldTypeString)
		names = append(names, "detail__connection_"+strconv.Itoa(i))
	}

	edges := data.NewFrameOfFieldTypes("edges", len(es), flds...)
	edges.SetFieldNames(names...)

	edges.SetMeta(&data.FrameMeta{
		Path:                   "edge",
		PreferredVisualization: data.VisType("nodeGraph"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Edge Count",
			},
			Value: float64(len(es)),
		}},
	})

	edges.Fields[0].Config = &data.FieldConfig{
		DisplayName: "Time",
		Path:        "time",
	}
	edges.Fields[1].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
	}
	edges.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Source_ID",
		Path:        "source",
		Links: []data.DataLink{{
			Title: `${__value.raw}`,
			URL:   link,
		}},
	}
	edges.Fields[3].Config = &data.FieldConfig{
		DisplayName: "Target_ID",
		Path:        "target",
		Links: []data.DataLink{{
			Title: `${__value.raw}`,
			URL:   link,
		}},
	}
	edges.Fields[4].Config = &data.FieldConfig{
		DisplayName: "Source",
		Path:        "self",
	}
	edges.Fields[5].Config = &data.FieldConfig{
		DisplayName: "Target",
		Path:        "peer",
	}

	for i := range maxConnections {
		edges.Fields[i+6].Config = &data.FieldConfig{
			DisplayName: fmt.Sprintf("Connection %d", i+1),
			Path:        fmt.Sprintf("connection %d", i+1),
		}
	}

	for i, e := range es {
		edges.SetRow(i, append([]any{timestamp}, e...)...)
	}

	return []*data.Frame{nodes, edges}
}
