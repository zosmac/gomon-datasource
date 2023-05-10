// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// logFrames formats log messages into grafana data frames.
func logFrames(link string, ms [][]any) []*data.Frame {
	logs := data.NewFrameOfFieldTypes("logs", len(ms),
		data.FieldTypeTime,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeString,
		data.FieldTypeInt32,
		data.FieldTypeString,
	)
	logs.SetFieldNames(
		"time",
		"message",
		"level",
		"process",
		"pid",
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
	}
	logs.Fields[4].Config = &data.FieldConfig{
		DisplayName: "Pid",
		Path:        "pid",
		Links: []data.DataLink{{
			Title: "${__value.raw}",
			URL:   link,
		}},
	}
	logs.Fields[5].Config = &data.FieldConfig{
		DisplayName: "Sender",
		Path:        "sender",
	}

	for i, m := range ms {
		logs.SetRow(i, m...)
	}

	return []*data.Frame{logs}
}
