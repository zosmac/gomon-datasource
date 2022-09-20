// Copyright © 2021 The Gomon Project.

//go:build !windows

package core

import (
	"os"
	"strconv"
	"syscall"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	// euid gets the executable's owner id.
	euid = os.Geteuid()
)

// init sets the process owner to user. When a datasource instance is created it cannot be running as root.
func init() {
	Setuid()
}

// seteuid gomon-datasource to owner.
func Seteuid() {
	err := syscall.Seteuid(euid)
	log.DefaultLogger.Debug(
		"Seteuid results",
		"uid", strconv.Itoa(os.Getuid()), // to format as int rather than float
		"euid", strconv.Itoa(os.Geteuid()), // to format as int rather than float
		"err", err,
	)
}

// setuid gomon-datasource to grafana user.
func Setuid() {
	err := syscall.Seteuid(os.Getuid())
	log.DefaultLogger.Debug(
		"Setuid results",
		"uid", strconv.Itoa(os.Getuid()), // to format as int rather than float
		"euid", strconv.Itoa(os.Geteuid()), // to format as int rather than float
		"err", err,
	)
}
