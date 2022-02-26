// Copyright © 2021 The Gomon Project.

package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func dataframes(link string) (nodes *data.Frame, edges *data.Frame) {
	nodes = data.NewFrameOfFieldTypes("nodes", 0,
		data.FieldTypeString,
		data.FieldTypeTime,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeFloat64,
		data.FieldTypeFloat64,
		data.FieldTypeFloat64,
		data.FieldTypeFloat64,
	)
	nodes.SetFieldNames(
		"id",
		"time",
		"mainStat",
		"secondaryStat",
		"detail__name",
		"detail__parent",
		"query",
		"arc__host",
		"arc__process",
		"arc__data",
		"arc__kernel",
	)
	nodes.SetMeta(&data.FrameMeta{
		Path:                   "node",
		PreferredVisualization: data.VisType("nodeGraph"),
		Stats: []data.QueryStat{{
			FieldConfig: data.FieldConfig{
				DisplayName: "Node Count",
			},
		}},
	})
	nodes.Fields[0].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
	}
	nodes.Fields[1].Config = &data.FieldConfig{
		DisplayName: "Time",
		Path:        "time",
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
		DisplayName: "Query",
		Path:        "query",
		Links: []data.DataLink{{
			Title: "${__value.raw}",
			URL:   link,
		}},
	}
	nodes.Fields[7].Config = &data.FieldConfig{
		Color:       red,
		DisplayName: "Host",
		Path:        "host",
	}
	nodes.Fields[8].Config = &data.FieldConfig{
		Color:       blue,
		DisplayName: "Process",
		Path:        "process",
	}
	nodes.Fields[9].Config = &data.FieldConfig{
		Color:       yellow,
		DisplayName: "Data",
		Path:        "data",
	}
	nodes.Fields[10].Config = &data.FieldConfig{
		Color:       magenta,
		DisplayName: "Kernel",
		Path:        "kernel",
	}

	edges = data.NewFrameOfFieldTypes("edges", 0,
		data.FieldTypeString,
		data.FieldTypeTime,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
	)
	edges.SetFieldNames(
		"id",
		"time",
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
		}},
	})
	edges.Fields[0].Config = &data.FieldConfig{
		DisplayName: "ID",
		Path:        "id",
	}
	edges.Fields[1].Config = &data.FieldConfig{
		DisplayName: "Time",
		Path:        "time",
	}
	edges.Fields[2].Config = &data.FieldConfig{
		DisplayName: "Source_ID",
		Path:        "source",
	}
	edges.Fields[3].Config = &data.FieldConfig{
		DisplayName: "Target_ID",
		Path:        "target",
	}
	edges.Fields[4].Config = &data.FieldConfig{
		DisplayName: "Source",
		Path:        "self",
		Links: []data.DataLink{{
			Title: `${__value.raw}`,
			URL:   link,
		}},
	}
	edges.Fields[5].Config = &data.FieldConfig{
		DisplayName: "Target",
		Path:        "peer",
		Links: []data.DataLink{{
			Title: "${__value.raw}",
			URL:   link,
		}},
	}
	edges.Fields[6].Config = &data.FieldConfig{
		DisplayName: "Relations",
		Path:        "relations",
	}

	return
}
