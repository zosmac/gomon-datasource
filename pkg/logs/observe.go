// Copyright © 2021 The Gomon Project.

package logs

import (
	"bufio"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	// log record regular expressions capture group names.
	groupTimestamp = "timestamp"
	groupLevel     = "level"
	groupHost      = "host"
	groupProcess   = "process"
	groupPid       = "pid"
	groupThread    = "thread"
	groupSender    = "sender"
	groupSubCat    = "subcat"
	groupMessage   = "message"
)

var (
	// requestChan signals that Grafana requests the latest log messages.
	requestChan = make(chan struct{})

	// messagesChan posts the queued messages when requested.
	messagesChan = make(chan [][]interface{})

	// messageChan queues log messages to convey to Grafana.
	messageChan = make(chan []interface{}, 100)

	// levelMap maps various applications' log levels to a common set fatal/error/warn/info/debug/trace.
	levelMap = map[string]Level{
		"emerg":      levelFatal, // Apache
		"emergency":  levelFatal, // syslog
		"fatal":      levelFatal,
		"fault":      levelFatal, // macOS
		"panic":      levelFatal, // syslog, Postgres
		"alert":      levelError, // syslog, Apache
		"crash":      levelError, // RabbitMQ
		"crit":       levelError, // syslog, Apache
		"critical":   levelError, // syslog, RabbitMQ
		"err":        levelError, // syslog, Consul, Vault
		"error":      levelError,
		"supervisor": levelWarn, // RabbitMQ
		"warn":       levelWarn,
		"warning":    levelWarn, // syslog, Postgres
		"info":       levelInfo,
		"":           levelInfo, // treat unknown as info
		"log":        levelInfo, // Postgres
		"notice":     levelInfo, // syslog, Postgres, Apache, macOS
		"statement":  levelInfo, // Postgres
		"debug":      levelDebug,
		"debug1":     levelDebug, // Postgres
		"debug2":     levelDebug, // Postgres
		"debug3":     levelDebug, // Postgres
		"debug4":     levelDebug, // Postgres
		"debug5":     levelDebug, // Postgres
		"default":    levelDebug, // macOS
		"trace":      levelTrace,
		"trace1":     levelTrace, // Apache
		"trace2":     levelTrace, // Apache
		"trace3":     levelTrace, // Apache
		"trace4":     levelTrace, // Apache
		"trace5":     levelTrace, // Apache
		"trace6":     levelTrace, // Apache
		"trace7":     levelTrace, // Apache
		"trace8":     levelTrace, // Apache
	}
)

// Observer starts the log monitor.
func Observer(ctx context.Context, level Level) {
	go observe(ctx, level)

	go func() {
		var messages [][]interface{}
		for {
			select {
			case <-requestChan:
				messagesChan <- messages
				messages = nil
			case message := <-messageChan:
				messages = append(messages, message)
			}
		}
	}()
}

func Read(link string) (resp backend.DataResponse) {
	requestChan <- struct{}{}
	resp.Frames = logFrames(link, <-messagesChan)
	return
}

func parseLog(sc *bufio.Scanner, regex *regexp.Regexp, format string) {
	groups := func() map[string]int {
		g := map[string]int{}
		for _, name := range regex.SubexpNames() {
			g[name] = regex.SubexpIndex(name)
		}
		return g
	}()

	for sc.Scan() {
		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}
		queue(groups, format, match)
	}
}

func queue(groups map[string]int, format string, match []string) {
	t, _ := time.Parse(format, match[groups[groupTimestamp]])

	sender := match[groups[groupSender]]
	if cg, ok := groups[groupSubCat]; ok {
		sender = match[cg] + ":" + sender
	}

	pid, _ := strconv.Atoi(match[groups[groupPid]])
	messageChan <- []interface{}{
		t,
		match[groups[groupMessage]],
		string(levelMap[strings.ToLower(match[groups[groupLevel]])]),
		match[groups[groupProcess]],
		int32(pid),
		sender,
	}
}
