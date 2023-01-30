// Copyright © 2021-2023 The Gomon Project.

package core

import (
	"context"
	"os/exec"
	"strconv"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	Ctx, Cancel = context.WithCancel(context.Background())
)

func Wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	state := cmd.ProcessState
	var stderr string
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			stderr = string(err.Stderr)
		}
	}
	log.DefaultLogger.Info(
		"Wait()",
		"command", cmd.String(),
		"pid", strconv.Itoa(cmd.Process.Pid),
		"err", err,
		"rc", strconv.Itoa(state.ExitCode()),
		"systime", state.SystemTime(),
		"usrtime", state.UserTime(),
		"sys", state.Sys(),
		"usage", state.SysUsage(),
		"stderr", stderr,
	)
}
