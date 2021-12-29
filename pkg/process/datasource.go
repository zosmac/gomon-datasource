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

	// queryModel of the datasource.
	queryModel struct {
		Pid     `json:"pid"`
		Kernel  bool `json:"kernel"`
		Daemons bool `json:"daemons"`
		Files   bool `json:"files"`
	}
)

func NewDataSourceInstance(dsis backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
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

	for _, q := range req.Queries {
		log.DefaultLogger.Info("DataQuery JSON",
			"json", q.JSON,
		)
		resp.Responses[q.RefID] = query(ctx, q)
	}

	return resp, nil
}
