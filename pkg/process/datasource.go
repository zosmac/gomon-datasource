// Copyright © 2021 The Gomon Project.

package process

import (
	"context"
	"encoding/json"
	"net/url"
	"runtime"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type (
	// instanceSettings for a datasource instance.
	instanceSettings struct {
		url.URL
	}
)

func NewDataSourceInstance(dsis backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var settings map[string]interface{}
	if err := json.Unmarshal(dsis.JSONData, &settings); err != nil {
		log.DefaultLogger.Error("Unmarshaling instance settings failed",
			"err", err,
		)
		return nil, err
	}

	host, _ := settings["host"].(string)
	path, _ := settings["path"].(string)
	secret := dsis.DecryptedSecureJSONData
	u, err := url.Parse(host)
	if err != nil {
		log.DefaultLogger.Error("Parsing host url failed",
			"err", err,
		)
		return nil, err
	}
	u.Path = path
	u.User = url.UserPassword(secret["user"], secret["password"])

	return &instanceSettings{
		URL: *u,
	}, nil
}

// CheckHealth run when "save and test" of data source run.
func (s *instanceSettings) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	log.DefaultLogger.Info("CheckHealth request received",
		"request", req,
	)

	return &backend.CheckHealthResult{
		Status:      backend.HealthStatusOk,
		Message:     "Instance healthy",
		JSONDetails: nil,
	}, nil
}

// Dispose run when instance cleaned up.
func (s *instanceSettings) Dispose() {
	log.DefaultLogger.Info("Dispose Called",
		"url", s.URL,
	)
	s.URL = url.URL{} // reset the url
}

// QueryData handler for instanceSettings.
func (s *instanceSettings) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	log.DefaultLogger.Info("QueryData request received",
		"request", req,
	)

	// create response struct
	resp := backend.NewQueryDataResponse()
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error("QueryData panicked",
				"panic", r,
				"stacktrace", string(buf),
			)
		}
	}()

	for _, dq := range req.Queries {
		log.DefaultLogger.Info("DataQuery JSON",
			"json", dq.JSON,
		)
		resp.Responses[dq.RefID] = nodeGraph(ctx, &dq)
	}

	return resp, nil
}
