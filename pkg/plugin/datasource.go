// Copyright © 2021 The Gomon Project.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type (
	// instance of the datasource.
	instance struct {
		Host   string
		Path   string
		User   string
		Pswd   string
		Health struct {
			Checks int
		}
		Query struct {
			Requests int
			Queries  int
		}
		Stream struct {
			Streams       int
			Messages      int
			Subscriptions int
			Published     int
			Errors        int
		}
	}
)

// NewDataSourceInstance creates an instance of the gomon datasource.
func NewDataSourceInstance(dsis backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	log.DefaultLogger.Info("Creating data source instance",
		"settings", dsis,
	)

	var settings map[string]interface{}
	if err := json.Unmarshal(dsis.JSONData, &settings); err != nil {
		log.DefaultLogger.Error("Unmarshaling datasource instance settings failed",
			"err", err,
		)
		return nil, err
	}

	return &instance{
		Host: settings["host"].(string),
		Path: settings["path"].(string),
		User: dsis.DecryptedSecureJSONData["user"],
		Pswd: dsis.DecryptedSecureJSONData["password"],
	}, nil
}

// Dispose run when instance cleaned up.
func (dsi *instance) Dispose() {
	log.DefaultLogger.Info("Dispose of datasource instance",
		"datasource", dsi,
	)

	*dsi = instance{}
}

// CheckHealth run when "save and test" of data source run.
func (dsi *instance) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	dsi.Health.Checks += 1
	log.DefaultLogger.Info("Check health of datasource",
		"datasource", dsi,
		"request", req,
	)

	details, _ := json.Marshal(dsi)

	return &backend.CheckHealthResult{
		Status:      backend.HealthStatusOk,
		Message:     "datasource healthy, see log for details",
		JSONDetails: details,
	}, nil
}

// QueryData handler for instanceSettings.
func (dsi *instance) QueryData(ctx context.Context, req *backend.QueryDataRequest) (resp *backend.QueryDataResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error("QueryData panicked",
				"panic", r,
				"stacktrace", string(buf),
			)
			if r, ok := r.(error); ok {
				err = r
			} else {
				err = fmt.Errorf("panic in QueryData: %v", r)
			}
		}
	}()

	log.DefaultLogger.Info("Query request",
		"datasource", dsi,
		"request", req,
	)

	dsi.Query.Requests += 1
	resp = backend.NewQueryDataResponse()

	link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"node":{${__value.raw}}}]`,
		req.PluginContext.DataSourceInstanceSettings.Name,
	)

	for _, query := range req.Queries {
		dsi.Query.Queries += 1

		log.DefaultLogger.Info("Query",
			"queries", strconv.Itoa(dsi.Query.Queries),
			"query", query,
		)

		resp.Responses[query.RefID] = NodeGraph(link, query.JSON)
	}

	return resp, nil
}

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

			link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"node":{${__value.raw}}}]`,
				req.PluginContext.DataSourceInstanceSettings.Name,
			)

			resp := NodeGraph(link, []byte(`{"streaming": true}`))
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
