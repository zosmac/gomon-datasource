// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/zosmac/gomon-datasource/pkg/core"
	"github.com/zosmac/gomon-datasource/pkg/plugin"
	"github.com/zosmac/gomon-datasource/pkg/process"
)

func main() {
	log.DefaultLogger.Info(
		"gomon data source plugin starting",
		"plugin", executable,
		"version", vmmp,
		"build_date", buildDate,
		"compiler", fmt.Sprintf("%s %s_%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	)

	defer core.Cancel()

	process.Endpoints()

	ip := datasource.NewInstanceProvider(plugin.DataSourceInstanceFactory)
	ds := &plugin.DataSource{
		IM: instancemgmt.New(ip),
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
