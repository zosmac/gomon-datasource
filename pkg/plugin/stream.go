// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// TODO: flesh this out so it actually works. So far, this is just placeholder code.

// RunStream initiates data source's stream to channel.
func (ds *DataSource) RunStream(ctx context.Context, req *backend.RunStreamRequest, sender *backend.StreamSender) error {
	ds.Stream.Streams += 1
	log.DefaultLogger.Info(
		"RunStream()",
		"datasource", *ds,
		"request", *req,
	)

	for {
		select {
		case <-ctx.Done():
			log.DefaultLogger.Info(
				"RunStream Cancelled",
				"path", req.Path,
			)
			return nil
		case <-time.After(time.Second * 10):
			ds.Stream.Messages += 1
			log.DefaultLogger.Info(
				"RunStream()",
				"path", req.Path,
				"streams", strconv.Itoa(ds.Stream.Streams),
				"messages", strconv.Itoa(ds.Stream.Messages),
				"request", *req,
			)

			link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"node":"${__value.raw}"}]`,
				req.PluginContext.DataSourceInstanceSettings.Name,
			)

			resp := NodeGraph(link, 0)
			for _, frame := range resp.Frames {
				if err := sender.SendFrame(frame, data.IncludeAll); err != nil {
					log.DefaultLogger.Error(
						"SendFrame()",
						"frame", frame.Name,
						"err", err,
					)
					ds.Stream.Errors += 1
					break
				}
			}
		}
	}
}

// SubscribeStream connects client to stream.
func (ds *DataSource) SubscribeStream(_ context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	ds.Stream.Subscriptions += 1
	log.DefaultLogger.Info(
		"SubscribeStream()",
		"subscriptions", strconv.Itoa(ds.Stream.Subscriptions),
		"request", req,
	)

	status := backend.SubscribeStreamStatusPermissionDenied
	if req.Path == "stream" {
		// Allow subscribing only on expected path.
		status = backend.SubscribeStreamStatusOK
	}
	return &backend.SubscribeStreamResponse{
		Status: status,
	}, nil
}

// PublishStream sends client message to the stream.
func (ds *DataSource) PublishStream(_ context.Context, req *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	ds.Stream.Published += 1
	log.DefaultLogger.Info(
		"PublishStream()",
		"published", strconv.Itoa(ds.Stream.Published),
		"request", req,
	)

	// Do not allow publishing at all.
	return &backend.PublishStreamResponse{
		Status: backend.PublishStreamStatusPermissionDenied,
	}, nil
}
