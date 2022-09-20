// Copyright © 2021 The Gomon Project.

package logs

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const (
	// message events.
	levelFatal Level = "fatal"
	levelError Level = "error"
	levelWarn  Level = "warn"
	levelInfo  Level = "info"
	levelDebug Level = "debug"
	levelTrace Level = "trace"
)

type (
	// Level type.
	Level string
)

func (l *Level) MarshalJSON() ([]byte, error) {
	log.DefaultLogger.Info(
		"Marshal() level",
		"level", string(*l),
	)
	return []byte(`{ "label": "` + string(*l) + `" }`), nil
}

func (l *Level) UnmarshalJSON(data []byte) error {
	var label map[string]string
	err := json.Unmarshal(data, &label)
	level, ok := label["label"]
	if ok {
		*l = Level(level)
	} else {
		err = fmt.Errorf("level does not include label")
	}
	log.DefaultLogger.Info(
		"Unmarshal() level",
		"data", string(data),
		"err", err,
	)
	return err
}

func logFrames(link string, ms [][]interface{}) []*data.Frame {
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
