// Copyright © 2021 The Gomon Project.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"runtime"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type (
	// serveOpts implements the datasource interface methods.
	serveOpts struct {
	}

	// instanceSettings for a datasource instance.
	instanceSettings struct {
		url.URL
	}

	// queryModel of the datasource.
	queryModel struct {
		Pid     `json:"pid"`
		Kernel  bool `json:"kernel"`
		Daemons bool `json:"daemons"`
		Syslog  bool `json:"syslog"`
		Files   bool `json:"files"`
	}
)

// newDatasource initializes the instance manager and returns datasource.ServeOpts.
func newDatasource() datasource.ServeOpts {
	datasource.NewInstanceManager(newDataSourceInstance)

	s := &serveOpts{}

	return datasource.ServeOpts{
		QueryDataHandler:    s,
		CheckHealthHandler:  s,
		CallResourceHandler: s,
	}
}

func newDataSourceInstance(dsis backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var settings map[string]interface{}
	err := json.Unmarshal(dsis.JSONData, &settings)
	if err != nil {
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

func (s *instanceSettings) Dispose() {
	s.URL = url.URL{}
}

// QueryData handler for ServeOpts.
func (s *serveOpts) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
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

	for _, q := range req.Queries {
		log.DefaultLogger.Info("DataQuery JSON",
			"json", q.JSON,
		)
		resp.Responses[q.RefID] = query(ctx, q)
	}

	return resp, nil
}

// CheckHealth handler for ServeOpts.
func (s *serveOpts) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	status := backend.HealthStatusOk
	message := "gomon-datasource is working"
	detail := struct {
		ServeOpts                   *serveOpts `json:"datasource-serveOpts"`
		*backend.CheckHealthRequest `json:"check-health-request"`
	}{
		s,
		req,
	}
	buf, err := json.MarshalIndent(detail, "", "  ")
	return &backend.CheckHealthResult{
		Status:      status,
		Message:     message,
		JSONDetails: buf,
	}, err
}

// CallResource handler for ServeOpts.
func (s *serveOpts) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	log.DefaultLogger.Info("Call resource request",
		"request", *req,
	)

	if _, err := json.Marshal(req); err != nil {
		log.DefaultLogger.Error("Marshaling call resource request failed",
			"err", err,
		)
		return err
	}
	resp := struct {
		ServeOpts                          *serveOpts `json:"datasource-serveOpts"`
		*backend.CallResourceRequest       `json:"call-resource-request"`
		backend.CallResourceResponseSender `json:"call-resource-response-sender"`
	}{
		s,
		req,
		sender,
	}

	buf, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.DefaultLogger.Error("Marshaling call resource response failed",
			"err", err,
		)
		return err
	}

	return sender.Send(&backend.CallResourceResponse{
		Status:  http.StatusOK,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    buf,
	})
}
