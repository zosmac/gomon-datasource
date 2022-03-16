// Copyright © 2021 The Gomon Project.

package logs

import (
	"sync"
)

var (
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

	Messages [][]interface{}
	MsgLock  sync.Mutex
)
