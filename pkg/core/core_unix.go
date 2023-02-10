// Copyright © 2021-2023 The Gomon Project.

//go:build !windows

package core

import (
	"os"

	"github.com/zosmac/gocore"
)

var (
	// euid gets the executable's owner id.
	euid = os.Geteuid()
)

// init sets the process owner to user. When a datasource instance is created it cannot be running as root.
func init() {
	gocore.Setuid()
}
