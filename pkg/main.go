// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"context"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"

	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon-datasource/pkg/plugin"
	"github.com/zosmac/gomon/file"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/process"
	"github.com/zosmac/gomon/serve"
)

func main() {
	gocore.Main(Main)
}

func Main(ctx context.Context) error {
	gocore.Error("start", nil, map[string]string{
		"plugin":  "gomon data source",
		"version": gocore.Version,
	}).Info()

	go func() {
		<-time.After(time.Second) // await datasource manage/serve startup to limit message flood

		if err := message.Encoder(ctx); err != nil {
			gocore.Error("encoder", err).Err()
		}

		if err := logs.Observer(ctx); err != nil {
			gocore.Error("logs Observer", err).Err()
		}

		serve.Serve(ctx)

		<-time.After(time.Second) // await encoder and observer startup
		gocore.Seteuid()          // after startup restore root access to OS services

		if err := file.Observer(ctx); err != nil {
			gocore.Error("files Observer", err).Err()
		}

		if err := process.Observer(ctx); err != nil {
			gocore.Error("processes Observer", err).Err()
		}

		if err := serve.Measure(ctx); err != nil {
			gocore.Error("system Measure", err).Err()
		}
	}()

	gocore.Setuid() // set to grafana user for open of the grpc unix socket in Manage()
	return gocore.Error(
		"datasource exit",
		datasource.Manage("zosmac-gomon-datasource",
			plugin.Factory(ctx),
			datasource.ManageOpts{},
		),
	)
}
