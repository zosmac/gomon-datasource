// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon-datasource/pkg/logs"
)

type (
	// Graph type.
	Graph string
)

const (
	// graph types supported.
	graphMetrics   Graph = "metrics"
	graphLogs      Graph = "logs"
	graphProcesses Graph = "processes"
)

var (
	validGraphs = map[Graph]struct{}{
		graphMetrics:   {},
		graphLogs:      {},
		graphProcesses: {},
	}
)

func (g *Graph) MarshalJSON() ([]byte, error) {
	if _, ok := validGraphs[*g]; !ok {
		*g = graphMetrics
	}
	return []byte(fmt.Sprintf(`{"label":%q}`, *g)), nil
}

func (g *Graph) UnmarshalJSON(data []byte) error {
	var label map[string]string
	json.Unmarshal(data, &label)
	graph, ok := label["label"]
	if ok {
		_, ok = validGraphs[Graph(graph)]
	}
	if !ok {
		graph = string(graphMetrics)
	}
	*g = Graph(graph)
	return nil
}

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
		ID       int64
		Settings struct {
			Host     string     `json:"host"`
			Path     string     `json:"path"`
			User     string     `json:"user"`
			Password string     `json:"password"`
			Level    logs.Level `json:"level"`
		}
	}
)

func Factory(ctx context.Context) datasource.InstanceFactoryFunc {
	return func(dsis backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		log.DefaultLogger.Info(
			"DataSourceInstanceFactory()",
			"settings", dsis,
		)

		instance := Instance{
			ID: dsis.ID,
		}

		if err := json.Unmarshal(dsis.JSONData, &instance.Settings); err != nil {
			log.DefaultLogger.Error(
				"Unmarshal()",
				"settings", dsis,
				"err", err,
			)
			return nil, gocore.Error("Unmarshal", err)
		}

		logs.Observer(ctx, instance.Settings.Level)

		return &instance, nil
	}
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

	var err error
	status := backend.HealthStatusOk
	message := "datasource healthy, see log for details"
	var details []byte

	if err = json.Unmarshal(req.PluginContext.DataSourceInstanceSettings.JSONData, ds); err == nil {
		details, err = json.Marshal(ds)
	}
	if err != nil {
		status = backend.HealthStatusError
		message = err.Error()
	}

	log.DefaultLogger.Info(
		"CheckHealth()",
		"datasource", *ds,
		"request", *req,
		"status", status,
		"message", message,
		"err", err,
	)

	return &backend.CheckHealthResult{
		Status:      status,
		Message:     message,
		JSONDetails: details,
	}, err
}

// CallResource of data source.
func (ds *DataSource) CallResource(_ context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	log.DefaultLogger.Info(
		"CallResource()",
		"datasource", *ds,
		"request", *req,
		"sender", sender,
	)

	return nil
}

// QueryData handler for data source.
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

	ds.Query.Requests += 1
	resp = backend.NewQueryDataResponse()

	err = ds.IM.Do(req.PluginContext, func(instance *Instance) error {
		log.DefaultLogger.Info(
			"Query datasource instance",
			"id", strconv.FormatInt(instance.ID, 10),
			"level", instance.Settings.Level,
		)

		for _, query := range req.Queries {
			ds.Query.Queries += 1

			q := struct {
				Graph Graph `json:"graph"`
				Pid   Pid   `json:"pid"`
			}{}
			if err = json.Unmarshal(query.JSON, &q); err != nil {
				resp.Responses[query.RefID] = backend.DataResponse{Error: err}
				continue
			}

			to := time.Now()
			from := to.Add(-5 * time.Minute)

			log.DefaultLogger.Info(
				"Query",
				"graph", q.Graph,
				"pid", q.Pid.String(),
				"from", from.Format("2006-01-02T15:04:05Z07:00"),
				"to", to.Format("2006-01-02T15:04:05Z07:00"),
			)

			link := fmt.Sprintf(
				`http://localhost:3000/explore?orgId=${__org}&left={"datasource":%q,"range":{"from":%q,"to":%q},"queries":[{"graph":{"label":"processes"},"pid":${__value.raw}}]}`,
				req.PluginContext.DataSourceInstanceSettings.Name,
				"now-5m",
				"now",
			)

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
