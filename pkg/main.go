// Copyright © 2021 The Gomon Project.

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/zosmac/gomon-datasource/pkg/logs"
	"github.com/zosmac/gomon-datasource/pkg/plugin"
	"github.com/zosmac/gomon-datasource/pkg/process"
)

// go generate creates version.go to set vmmp and package dependencies for version.
//go:generate ./generate.sh

func main() {
	log.DefaultLogger.Info(
		"gomon data source plugin starting",
		"plugin", executable,
		"version", vmmp,
		"build_date", buildDate,
		"compiler", fmt.Sprintf("%s %s_%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	process.Endpoints(ctx)
	logs.Observer(ctx, logs.Level("info"))

	ip := datasource.NewInstanceProvider(plugin.DataSourceInstanceFactory)
	ds := &plugin.DataSource{
		IM:    instancemgmt.New(ip),
		Level: logs.Level("info"),
	}

	if err := datasource.Serve(datasource.ServeOpts{
		CheckHealthHandler:  ds,
		CallResourceHandler: ds,
		QueryDataHandler:    ds,
		StreamHandler:       ds,
	}); err != nil {
		log.DefaultLogger.Error(
			"Serve()",
			"err", err,
		)
		os.Exit(1)
	}
}
