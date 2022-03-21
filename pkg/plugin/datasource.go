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
)

const (
	// query regular expression capture group names.
	groupType    = "type"
	groupName    = "name"
	groupProcess = "process"
	groupPid     = "pid"
)

var (
	// Hostname identifies the host.
	Hostname, _ = os.Hostname()

	// queryRegex used to read the pid from the query.
	queryRegex = regexp.MustCompile(
		`^(?:` +
			`(?P<type>[A-Za-z]+):(?P<name>.+)|` +
			`(?P<process>[^\[]*)\[(?P<pid>\d+)\]|` +
			`)$`,
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
		process   string
		pid       Pid
		nodeType  string
		name      string
		Node      string `json:"node"`
		Streaming bool   `json:"streaming"`
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

	link := fmt.Sprintf(`http://localhost:3000/explore?orgId=${__org}&left=["now-5m","now","%s",{"node":"${__value.raw}"}]`,
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

		if q.pid != 0 {
			resp.Responses[query.RefID] = NodeGraph(link, q)
		} else {
			resp.Responses[query.RefID] = Logs(link)
		}
	}

	return resp, nil
}

// parseQuery extracts the query from the request JSON.
func parseQuery(message json.RawMessage) (query, error) {
	q := query{}
	// Unmarshal the JSON into our queryModel.
	if err := json.Unmarshal(message, &q); err != nil {
		log.DefaultLogger.Error("Query unmarshaling failed",
			"json", string(message),
			"err", err,
		)
		return query{}, err
	}

	log.DefaultLogger.Info("Data source query",
		"query", q,
	)

	match := queryRegex.FindStringSubmatch(q.Node)
	if len(match) == 0 || match[0] == "" {
		return q, nil
	}

	q.process = match[queryGroups[groupProcess]]
	q.nodeType = match[queryGroups[groupType]]
	q.name = match[queryGroups[groupName]]
	pid, err := strconv.Atoi(match[queryGroups[groupPid]])
	if err == nil {
		q.pid = Pid(pid)
	}

	log.DefaultLogger.Info("query regex match",
		"type", q.nodeType,
		"name", q.name,
		"process", q.process,
		"pid", match[queryGroups[groupPid]],
	)

	if pid == 0 && q.nodeType == "" {
		q.Node = ""
	}

	return q, nil
}
