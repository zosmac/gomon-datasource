// Copyright © 2021 The Gomon Project.

package logs

import (
	"bufio"
	"fmt"
	"regexp"
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
	levelMap = map[string]string{
		"emerg":      "fatal", // Apache
		"emergency":  "fatal", // syslog
		"fatal":      "fatal",
		"fault":      "fatal", // macOS
		"panic":      "fatal", // syslog, Postgres
		"alert":      "error", // syslog, Apache
		"crash":      "error", // RabbitMQ
		"crit":       "error", // syslog, Apache
		"critical":   "error", // syslog, RabbitMQ
		"err":        "error", // syslog, Consul, Vault
		"error":      "error",
		"supervisor": "warn", // RabbitMQ
		"warn":       "warn",
		"warning":    "warn", // syslog, Postgres
		"info":       "info",
		"":           "info", // treat unknown as info
		"log":        "info", // Postgres
		"notice":     "info", // syslog, Postgres, Apache, macOS
		"statement":  "info", // Postgres
		"debug":      "debug",
		"debug1":     "debug", // Postgres
		"debug2":     "debug", // Postgres
		"debug3":     "debug", // Postgres
		"debug4":     "debug", // Postgres
		"debug5":     "debug", // Postgres
		"default":    "debug", // macOS
		"trace":      "trace",
		"trace1":     "trace", // Apache
		"trace2":     "trace", // Apache
		"trace3":     "trace", // Apache
		"trace4":     "trace", // Apache
		"trace5":     "trace", // Apache
		"trace6":     "trace", // Apache
		"trace7":     "trace", // Apache
		"trace8":     "trace", // Apache
	}
)

func init() {
	go observe()
}

func Read(link string) (resp backend.DataResponse) {
	requestChan <- struct{}{}
	resp.Frames = logFrames(link, <-messagesChan)
	return
}

func observe() {
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

	messageChan <- []interface{}{
		t,
		match[groups[groupMessage]],
		levelMap[strings.ToLower(match[groups[groupLevel]])],
		fmt.Sprintf("%s[%s]", match[groups[groupProcess]], match[groups[groupPid]]),
		sender,
	}
}
