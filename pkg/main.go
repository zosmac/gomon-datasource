// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon-datasource/pkg/plugin"
)

func main() {
	gocore.Main(Main)
}

func Main(ctx context.Context) error {
	gocore.Error("start", nil, map[string]string{
		"plugin":  "gomon data source",
		"version": gocore.Version,
	}).Info()

	return gocore.Error(
		"datasource exit",
		datasource.Manage("zosmac-gomon-datasource",
			plugin.Factory(ctx),
			datasource.ManageOpts{},
		),
	)
}
