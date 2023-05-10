// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/logs"
)

func init() {
	gocore.LoggingLevel = func() gocore.LogLevel {
		switch log.DefaultLogger.Level() {
		case log.Trace:
			return gocore.LevelTrace
		case log.Debug:
			return gocore.LevelDebug
		case log.Info:
			return gocore.LevelInfo
		case log.Warn:
			return gocore.LevelWarn
		case log.Error:
			return gocore.LevelError
		default:
			return gocore.LevelInfo
		}
	}()

	gocore.Log = func(msg gocore.LogMessage, level gocore.LogLevel) {
		if level >= logs.EventMap[logs.Flags.LogEvent] {
			switch level {
			case gocore.LevelTrace,
				gocore.LevelDebug:
				log.DefaultLogger.Debug(msg.Source, grafanaDetail(msg)...)
			case gocore.LevelInfo:
				log.DefaultLogger.Info(msg.Source, grafanaDetail(msg)...)
			case gocore.LevelWarn:
				log.DefaultLogger.Warn(msg.Source, grafanaDetail(msg)...)
			case gocore.LevelError:
				log.DefaultLogger.Error(msg.Source, grafanaDetail(msg)...)
			}
		}
	}
}

// grafanaDetail pulls key,value pairs from message details into a slice.
func grafanaDetail(msg gocore.LogMessage) []interface{} {
	var detail []interface{}
	if msg.E != nil {
		detail = append(detail, "err", msg.E.Error())
	}
	detail = append(detail, "location", fmt.Sprintf("%s:%d", msg.File, msg.Line))

	for key, val := range msg.Detail {
		if val == "" {
			continue
		}
		detail = append(detail, key, val)
	}

	return detail
}
