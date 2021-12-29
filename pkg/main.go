// Copyright © 2021 The Gomon Project.

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/zosmac/grafana-plugins/gomon-datasource/pkg/process"
)

// go generate creates version.go to set vmmp and package dependencies for version.
//go:generate ./generate.sh

func main() {
	log.DefaultLogger.Info("Starting gomon process nodegraph plugin backend data source",
		"plugin", executable,
		"version", vmmp,
		"build_date", buildDate,
		"compiler", fmt.Sprintf("%s %s_%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	)
	if err := datasource.Manage("zosmac-gomon-datasource", process.NewDataSourceInstance, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error("Startup failed",
			"err", err,
		)
		os.Exit(1)
	}
}
