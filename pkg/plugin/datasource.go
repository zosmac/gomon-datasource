// Copyright © 2021 The Gomon Project.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/zosmac/gomon-datasource/pkg/logs"
)

const (
	// graphs.
	graphMetrics   Graph = "metrics"
	graphLogs      Graph = "logs"
	graphProcesses Graph = "processes"
)

type (
	// Graph type.
	Graph string
)

func (g *Graph) MarshalJSON() ([]byte, error) {
	log.DefaultLogger.Info(
		"Marshal() graph",
		"graph", string(*g),
	)
	return []byte(`{ "label": "` + string(*g) + `" }`), nil
}

func (g *Graph) UnmarshalJSON(data []byte) error {
	var label map[string]string
	err := json.Unmarshal(data, &label)
	log.DefaultLogger.Info(
		"Unmarshal() graph",
		"data", string(data),
		"err", err,
	)
	*g = Graph(label["label"])
	return err
}

var (
	// Hostname identifies the host.
	Hostname, _ = os.Hostname()
)

type (
	DataSource struct {
		IM       instancemgmt.InstanceManager `json:"-"`
		Host     string                       `json:"host"`
		Path     string                       `json:"path"`
		User     string                       `json:"user"`
		Password string                       `json:"password"`
		Level    logs.Level                   `json:"level"`
		Health   struct {
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

	// instance of the datasource.
	Instance struct {
		ID    int64
		Graph Graph
		Pid   Pid
	}

	// query from data source.
	query struct {
		Graph     Graph      `json:"graph"`
		Level     logs.Level `json:"level"`
		Pid       Pid        `json:"pid"`
		Streaming bool       `json:"streaming"`
	}
)

// DataSourceInstanceFactory creates an gomon data source instance.
func DataSourceInstanceFactory(dsis backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	log.DefaultLogger.Info(
		"DataSourceInstanceFactory()",
		"settings", dsis,
	)

	query := query{}
	if err := json.Unmarshal(dsis.JSONData, &query); err != nil {
		log.DefaultLogger.Error(
			"Unmarshal()",
			"settings", dsis,
			"err", err,
		)
		return nil, err
	}

	return &Instance{
		ID:    dsis.ID,
		Graph: query.Graph,
	}, nil
}

// Dispose run when instance cleaned up.
func (dsi *Instance) Dispose() {
	log.DefaultLogger.Info(
		"Dispose()",
		"datasource", *dsi,
	)

	*dsi = Instance{}
}

// CheckHealth run when "save and test" of data source run.
func (ds *DataSource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	ds.Health.Checks += 1
	log.DefaultLogger.Info(
		"CheckHealth()",
		"datasource", *ds,
		"request", *req,
	)

	err := json.Unmarshal(req.PluginContext.DataSourceInstanceSettings.JSONData, ds)
	log.DefaultLogger.Info(
		"CheckHealth()",
		"datasource", *ds,
		"request", *req,
		"err", err,
	)

	details, _ := json.Marshal(ds)

	if ds.Level == "" {
		return &backend.CheckHealthResult{
			Status:      backend.HealthStatusError,
			Message:     "specify a valid log level",
			JSONDetails: details,
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:      backend.HealthStatusOk,
		Message:     "datasource healthy, see log for details",
		JSONDetails: details,
	}, nil
}

func (ds *DataSource) CallResource(_ context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	log.DefaultLogger.Info(
		"CallResource()",
		"datasource", *ds,
		"request", *req,
		"sender", sender,
	)

	return nil
}

// QueryData handler for instanceSettings.
func (ds *DataSource) QueryData(_ context.Context, req *backend.QueryDataRequest) (resp *backend.QueryDataResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			log.DefaultLogger.Error(
				"QueryData() Panic",
				"datasource", *ds,
				"request", *req,
				"panic", r,
				"stacktrace", string(buf),
			)
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic in QueryData: %v", r)
			}
		}
	}()

	log.DefaultLogger.Info(
		"QueryData()",
		"datasource", *ds,
		"request", *req,
	)

	ds.Query.Requests += 1
	resp = backend.NewQueryDataResponse()

	link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"graph":{"label":"processes"},"pid":${__value.raw}}]`,
		req.PluginContext.DataSourceInstanceSettings.Name,
	)

	err = ds.IM.Do(req.PluginContext, func(settings *Instance) error {
		for _, query := range req.Queries {
			ds.Query.Queries += 1

			log.DefaultLogger.Info(
				"Query",
				"queries", strconv.Itoa(ds.Query.Queries),
				"query", query,
			)

			q, err := parseQuery(query.JSON)
			if err != nil {
				resp.Responses[query.RefID] = backend.DataResponse{Error: err}
				continue
			}

			switch q.Graph {
			case graphMetrics:
			case graphLogs:
				resp.Responses[query.RefID] = logs.Read(link)
			case graphProcesses:
				resp.Responses[query.RefID] = NodeGraph(link, q.Pid)
			}
		}
		return nil
	})

	return resp, nil
}

// parseQuery extracts the query from the request JSON.
func parseQuery(message json.RawMessage) (query query, err error) {
	// Unmarshal the JSON into our queryModel.
	if err = json.Unmarshal(message, &query); err != nil {
		log.DefaultLogger.Error(
			"Unmarshal()",
			"json", string(message),
			"err", err,
		)
		return
	}

	log.DefaultLogger.Info(
		"parseQuery()",
		"query", query,
	)

	return query, nil
}
