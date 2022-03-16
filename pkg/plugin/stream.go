// Copyright © 2021 The Gomon Project.

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
func (dsi *instance) RunStream(ctx context.Context, req *backend.RunStreamRequest, sender *backend.StreamSender) error {
	dsi.Stream.Streams += 1
	log.DefaultLogger.Info("RunStream called",
		"datasource", dsi,
		"request", req,
	)

	for {
		select {
		case <-ctx.Done():
			log.DefaultLogger.Info("Context done, finish streaming", "path", req.Path)
			return nil
		case <-time.After(time.Second * 10):
			dsi.Stream.Messages += 1
			log.DefaultLogger.Info("Stream message",
				"streams", strconv.Itoa(dsi.Stream.Streams),
				"messages", strconv.Itoa(dsi.Stream.Messages),
				"request", req,
			)

			link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"node":"${__value.raw}"}]`,
				req.PluginContext.DataSourceInstanceSettings.Name,
			)

			resp := NodeGraph(link, query{Streaming: true})
			for _, frame := range resp.Frames {
				if err := sender.SendFrame(frame, data.IncludeAll); err != nil {
					log.DefaultLogger.Error("Error sending frame",
						"frame", frame.Name,
						"err", err,
					)
					dsi.Stream.Errors += 1
					break
				}
			}
		}
	}
}

// SubscribeStream connects client to stream.
func (dsi *instance) SubscribeStream(_ context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	dsi.Stream.Subscriptions += 1
	log.DefaultLogger.Info("SubscribeStream called",
		"subscriptions", strconv.Itoa(dsi.Stream.Subscriptions),
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
func (dsi *instance) PublishStream(_ context.Context, req *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	dsi.Stream.Published += 1
	log.DefaultLogger.Info("PublishStream called",
		"published", strconv.Itoa(dsi.Stream.Published),
		"request", req,
	)

	// Do not allow publishing at all.
	return &backend.PublishStreamResponse{
		Status: backend.PublishStreamStatusPermissionDenied,
	}, nil
}
