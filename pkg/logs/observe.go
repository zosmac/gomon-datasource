// Copyright © 2021-2023 The Gomon Project.

package logs

import (
	"bufio"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/zosmac/gocore"
)

const (
	// log record regular expressions capture group names.
	groupTimestamp = "timestamp"
	groupUtc       = "utc"
	groupTzoffset  = "tzoffset"
	groupTimezone  = "timezone"
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
	// cancel stops exiting logs observer.
	cancel context.CancelFunc = func() {}

	// currLevel is the current minimum log level captured.
	currLevel Level

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
func Observer(ctx context.Context, level Level) error {
	if level == currLevel {
		return nil // do not restart observer
	}
	currLevel = level

	cancel() // stop current observer if running

	chld, cncl := context.WithCancel(ctx)
	cancel = cncl

	if err := observe(chld); err != nil {
		return gocore.Error("observe", err)
	}

	go func() {
		var messages [][]interface{}
		for {
			select {
			case <-chld.Done():
				return
			case <-requestChan:
				if len(messages) > 1000 {
					messages = messages[len(messages)-1000:]
				}
				messagesChan <- messages
				messages = nil
			case message := <-messageChan:
				messages = append(messages, message)
				if len(messages) > 1000 {
					messages = messages[len(messages)-900:]
				}
			}
		}
	}()

	return nil
}

func Read(link string) (resp backend.DataResponse) {
	requestChan <- struct{}{}
	messages := <-messagesChan
	log.DefaultLogger.Info(
		"Read()",
		"messages", strconv.Itoa(len(messages)),
	)
	resp.Frames = logFrames(link, messages)
	return
}

func parseLog(ctx context.Context, sc *bufio.Scanner, regex *regexp.Regexp, format string) {
	groups := func() map[string]int {
		g := map[string]int{}
		for _, name := range regex.SubexpNames() {
			g[name] = regex.SubexpIndex(name)
		}
		return g
	}()

	readyChan := make(chan struct{})
	go func() {
		for sc.Scan() {
			readyChan <- struct{}{}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-readyChan:
		}
		match := regex.FindStringSubmatch(sc.Text())
		if len(match) == 0 || match[0] == "" {
			continue
		}
		queue(groups, format, match)
	}
}

func queue(groups map[string]int, format string, match []string) {
	t, _ := time.Parse(format, match[groups[groupTimestamp]])
	pid, _ := strconv.Atoi(match[groups[groupPid]])
	sender := match[groups[groupSender]]
	if cg, ok := groups[groupSubCat]; ok {
		sender = match[cg] + ":" + sender
	}

	messageChan <- []interface{}{
		t,
		match[groups[groupMessage]],
		string(levelMap[strings.ToLower(match[groups[groupLevel]])]),
		match[groups[groupProcess]],
		int32(pid),
		sender,
	}
}
