// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/zosmac/gocore"
)

// TODO: flesh this out so it actually works. So far, this is just placeholder code.

// RunStream initiates data source's stream to channel.
func (dsi *Instance) RunStream(ctx context.Context, req *backend.RunStreamRequest, sender *backend.StreamSender) error {
	dsi.Stream.Streams += 1
	gocore.Error("RunStream", nil, map[string]string{
		"instance": fmt.Sprint(*dsi),
		"request":  fmt.Sprint(*req),
	}).Info()

	for {
		select {
		case <-ctx.Done():
			gocore.Error("RunStream Cancelled", nil, map[string]string{
				"path": req.Path,
			}).Err()
			return nil
		case <-time.After(time.Second * 10):
			dsi.Stream.Messages += 1
			gocore.Error("RunStream", nil, map[string]string{
				"path":     req.Path,
				"streams":  strconv.Itoa(dsi.Stream.Streams),
				"messages": strconv.Itoa(dsi.Stream.Messages),
				"request":  fmt.Sprint(*req),
			}).Info()

			link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"node":"${__value.raw}"}]`,
				req.PluginContext.DataSourceInstanceSettings.Name,
			)

			resp := Nodegraph(link, 0)
			for _, frame := range resp.Frames {
				if err := sender.SendFrame(frame, data.IncludeAll); err != nil {
					gocore.Error("SendFrame", nil, map[string]string{
						"frame": frame.Name,
						"err":   err.Error(),
					}).Err()
					dsi.Stream.Errors += 1
					break
				}
			}
		}
	}
}

// SubscribeStream connects client to stream.
func (dsi *Instance) SubscribeStream(_ context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	dsi.Stream.Subscriptions += 1
	gocore.Error("SubscribeStream", nil, map[string]string{
		"subscriptions": strconv.Itoa(dsi.Stream.Subscriptions),
		"request":       fmt.Sprint(*req),
	}).Info()

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
func (dsi *Instance) PublishStream(_ context.Context, req *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	dsi.Stream.Published += 1
	gocore.Error("PublishStream", nil, map[string]string{
		"published": strconv.Itoa(dsi.Stream.Published),
		"request":   fmt.Sprint(*req),
	}).Info()

	// Do not allow publishing at all.
	return &backend.PublishStreamResponse{
		Status: backend.PublishStreamStatusPermissionDenied,
	}, nil
}
