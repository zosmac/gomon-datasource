// Copyright © 2021 The Gomon Project.

package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func nodeFrames(link string, ns, es [][]interface{}) []*data.Frame {
	nodes := data.NewFrameOfFieldTypes("nodes", len(ns),
		data.FieldTypeTime,
		data.FieldTypeInt64,
		data.FieldTypeString,
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
		"detail__parent",
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
		DisplayName: " ",
		Path:        "command",
	}
	nodes.Fields[3].Config = &data.FieldConfig{
		DisplayName: " ",
		Path:        "process",
	}
	nodes.Fields[4].Config = &data.FieldConfig{
		DisplayName: "Name",
		Path:        "name",
	}
	nodes.Fields[5].Config = &data.FieldConfig{
		DisplayName: "Parent",
		Path:        "parent",
	}
	nodes.Fields[6].Config = &data.FieldConfig{
		Color:       red,
		DisplayName: "Host",
		Path:        "host",
	}
	nodes.Fields[7].Config = &data.FieldConfig{
		Color:       blue,
		DisplayName: "Process",
		Path:        "process",
	}
	nodes.Fields[8].Config = &data.FieldConfig{
		Color:       yellow,
		DisplayName: "Data",
		Path:        "data",
	}
	nodes.Fields[9].Config = &data.FieldConfig{
		Color:       magenta,
		DisplayName: "Socket",
		Path:        "socket",
	}
	nodes.Fields[10].Config = &data.FieldConfig{
		Color:       cyan,
		DisplayName: "Kernel",
		Path:        "kernel",
	}

	for i, n := range ns {
		nodes.SetRow(i, n...)
	}

	edges := data.NewFrameOfFieldTypes("edges", len(es),
		data.FieldTypeTime,
		data.FieldTypeString,
		data.FieldTypeInt64,
		data.FieldTypeInt64,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
	)
	edges.SetFieldNames(
		"time",
		"id",
		"source",
		"target",
		"mainStat",
		"secondaryStat",
		"detail__edges",
	)
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
	edges.Fields[6].Config = &data.FieldConfig{
		DisplayName: "Relations",
		Path:        "relations",
	}

	for i, e := range es {
		edges.SetRow(i, e...)
	}

	return []*data.Frame{nodes, edges}
}

func logFrames(link string, ms [][]interface{}) []*data.Frame {
	logs := data.NewFrameOfFieldTypes("logs", len(ms),
		data.FieldTypeTime,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
	)
	logs.SetFieldNames(
		"time",
		"message",
		"level",
		"process",
		"sender",
	)
	logs.SetMeta(&data.FrameMeta{
		Path:                   "log",
		PreferredVisualization: data.VisType("logs"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Log Count",
			},
			Value: float64(len(ms)),
		}},
	})
	logs.Fields[0].Config = &data.FieldConfig{
		DisplayName: "Time",
		Path:        "time",
	}
	logs.Fields[1].Config = &data.FieldConfig{
		DisplayName: "Message",
		Path:        "message",
	}
	logs.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Level",
		Path:        "level",
	}
	logs.Fields[3].Config = &data.FieldConfig{
		DisplayName: "Process",
		Path:        "process",
		Links: []data.DataLink{{
			Title: "${__value.raw}",
			URL:   link,
		}},
	}
	logs.Fields[4].Config = &data.FieldConfig{
		DisplayName: "Sender",
		Path:        "sender",
	}

	for i, m := range ms {
		logs.SetRow(i, m...)
	}

	return []*data.Frame{logs}
}
