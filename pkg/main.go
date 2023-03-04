// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon-datasource/pkg/plugin"
	"github.com/zosmac/gomon-datasource/pkg/process"
)

func main() {
	gocore.Main(Main)
}

func Main(ctx context.Context) error {
	log.DefaultLogger.Info(fmt.Sprintf(
		"start gomon data source plugin %s %s_%s",
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	))

	if err := process.Endpoints(ctx); err != nil {
		return gocore.Error("Remote Host and Inter-Process Connections Unavailable", err)
	}

	ip := datasource.NewInstanceProvider(plugin.Factory(ctx))
	ds := &plugin.DataSource{
		IM: instancemgmt.New(ip),
	}

	return gocore.Error(
		"exit",
		datasource.Serve(datasource.ServeOpts{
			CheckHealthHandler:  ds,
			CallResourceHandler: ds,
			QueryDataHandler:    ds,
			StreamHandler:       ds,
		}),
	)
}
