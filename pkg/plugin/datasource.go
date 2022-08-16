// Copyright © 2021 The Gomon Project.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/zosmac/gomon-datasource/pkg/logs"
)

const (
	// query regular expression capture group names.
	groupGraph = "graph"
)

var (
	// Hostname identifies the host.
	Hostname, _ = os.Hostname()

	// queryRegex used to read the pid from the query.
	queryRegex = regexp.MustCompile(
		`^(?P<graph>metrics|logs|processes|-?\d+)$`,
	)

	queryGroups = func() map[string]int {
		g := map[string]int{}
		for _, name := range queryRegex.SubexpNames() {
			g[name] = queryRegex.SubexpIndex(name)
		}
		return g
	}()
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

	// query from data source.
	query struct {
		QueryText string `json:"queryText"`
		Streaming bool   `json:"streaming"`
		pid       Pid
		metrics   bool
		logs      bool
		processes bool
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
			log.DefaultLogger.Error("QueryData() panicked",
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

	log.DefaultLogger.Info("Query request",
		"datasource", dsi,
		"request", req,
	)

	dsi.Query.Requests += 1
	resp = backend.NewQueryDataResponse()

	link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"queryText":"${__value.raw}"}]`,
		req.PluginContext.DataSourceInstanceSettings.Name,
	)

	for _, query := range req.Queries {
		dsi.Query.Queries += 1

		log.DefaultLogger.Info("Query",
			"queries", strconv.Itoa(dsi.Query.Queries),
			"query", query,
		)

		q, err := parseQuery(query.JSON)
		if err != nil {
			resp.Responses[query.RefID] = backend.DataResponse{Error: err}
			continue
		}

		if q.metrics {
		} else if q.logs {
			resp.Responses[query.RefID] = logs.Read(link)
		} else if q.processes {
			resp.Responses[query.RefID] = NodeGraph(link, q)
		}
	}

	return resp, nil
}

// parseQuery extracts the query from the request JSON.
func parseQuery(message json.RawMessage) (query query, err error) {
	// Unmarshal the JSON into our queryModel.
	if err = json.Unmarshal(message, &query); err != nil {
		log.DefaultLogger.Error("Query unmarshaling failed",
			"json", string(message),
			"err", err,
		)
		return
	}

	log.DefaultLogger.Info("Data source query",
		"query", query,
	)

	match := queryRegex.FindStringSubmatch(query.QueryText)
	if len(match) == 0 || match[0] == "" {
		return
	}

	switch m := match[queryGroups[groupGraph]]; m {
	case "metrics":
		query.metrics = true
	case "logs":
		query.logs = true
	case "processes":
		query.processes = true
	default:
		if pid, err := strconv.Atoi(m); err == nil {
			query.processes = true
			query.pid = Pid(pid)
		}
	}

	log.DefaultLogger.Info("query regex match",
		"metrics", query.metrics,
		"logs", query.logs,
		"processes", query.processes,
		"pid", strconv.Itoa(int(query.pid)),
	)

	return query, nil
}
