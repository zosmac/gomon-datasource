// Copyright © 2021-2023 The Gomon Project.

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/message"
	"github.com/zosmac/gomon/process"
)

type (
	// Instance of the datasource.
	Instance struct {
		ctx    context.Context
		Health struct {
			Checks int `json:"checks"`
		} `json:"health"`
		Query struct {
			Requests int `json:"requests"`
			Queries  int `json:"count"`
		} `json:"query"`
		Stream struct {
			Streams       int `json:"count"`
			Messages      int `json:"messages"`
			Subscriptions int `json:"subscriptions"`
			Published     int `json:"published"`
			Errors        int `json:"errors"`
		} `json:"stream"`
	}
)

var (
	instance Instance
)

func Factory(ctx context.Context) datasource.InstanceFactoryFunc {
	gocore.Error("DataSourceInstanceFactory", nil).Info()

	if err := message.Encoder(ctx); err != nil {
		gocore.Error("encoder", err).Err()
		return nil
	}

	logs.Flags.LogEvent = logs.LevelTrace
	if err := logs.Observer(ctx); err != nil {
		gocore.Error("logs Observer", err).Err()
		return nil
	}

	if err := process.Observer(ctx); err != nil {
		gocore.Error("processes Observer", err).Err()
		return nil
	}

	<-time.After(time.Second) // await root processes startup
	gocore.Setuid()           // set to grafana user so Serve() opens grpc unix socket accessibly

	go func() {
		<-time.After(time.Second)
		gocore.Seteuid() // after Serve() restore root access to OS services
	}()

	return func(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		gocore.Error("create datasource instance", nil, map[string]string{
			"id":       strconv.Itoa(int(settings.ID)),
			"uid":      settings.UID,
			"type":     settings.Type,
			"name":     settings.Name,
			"jsonData": string(settings.JSONData),
		},
			settings.DecryptedSecureJSONData,
		).Info()

		instance.ctx = ctx

		gocore.Error("datasource instance", nil, map[string]string{
			"id": strconv.Itoa(int(settings.ID)),
		}).Info()

		return &instance, nil
	}
}

// Dispose run when instance cleaned up.
func (instance *Instance) Dispose() {
	gocore.Error("Dispose", nil, map[string]string{
		"datasource": fmt.Sprint(*instance),
	}).Info()

	*instance = Instance{}
}

// CheckHealth run when "save and test" of data source run.
func (instance *Instance) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			gocore.Error("CheckHealth panic", fmt.Errorf("%v", r), map[string]string{
				"stacktrace": string(buf),
			}).Err()
		}
	}()

	instance.Health.Checks += 1

	gocore.Error("CheckHealth", nil, map[string]string{
		"id":             strconv.Itoa(int(req.PluginContext.DataSourceInstanceSettings.ID)),
		"health_checks":  strconv.Itoa(instance.Health.Checks),
		"query_requests": strconv.Itoa(instance.Query.Requests),
		"total_queries":  strconv.Itoa(instance.Query.Queries),
	}).Info()

	status := backend.HealthStatusOk
	message := "instance healthy, see log for details"

	gocore.Error("CheckHealth results", nil, map[string]string{
		"status":  status.String(),
		"message": message,
	}).Info()

	jsonDetails, _ := json.Marshal(instance)

	gocore.Error("check health details", errors.New(string(jsonDetails))).Info()

	return &backend.CheckHealthResult{
		Status:      status,
		Message:     message,
		JSONDetails: jsonDetails,
	}, nil
}

// CallResource of data source.
func (instance *Instance) CallResource(_ context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	gocore.Error("CallResource", nil, map[string]string{
		"instance": fmt.Sprint(*instance),
		"request":  fmt.Sprint(*req),
		"sender":   fmt.Sprint(sender),
	}).Info()

	return nil
}

// QueryData handler for data source.
func (instance *Instance) QueryData(_ context.Context, req *backend.QueryDataRequest) (resp *backend.QueryDataResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			gocore.Error("QueryData panic", nil, map[string]string{
				"request":    fmt.Sprint(*req),
				"panic":      fmt.Sprint(r),
				"stacktrace": string(buf),
			}).Err()
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("QueryData panic: %v", r)
			}
		}
	}()

	instance.Query.Requests += 1
	resp = backend.NewQueryDataResponse()

	for _, query := range req.Queries {
		instance.Query.Queries += 1
		q := struct {
			Pid process.Pid `json:"pid"`
		}{}
		if err = json.Unmarshal(query.JSON, &q); err != nil {
			resp.Responses[query.RefID] = backend.DataResponse{Error: err}
			continue
		}

		to := time.Now()
		from := to.Add(-5 * time.Minute)

		gocore.Error("Query", nil, map[string]string{
			"pid":  q.Pid.String(),
			"from": from.Format("2006-01-02T15:04:05Z07:00"),
			"to":   to.Format("2006-01-02T15:04:05Z07:00"),
		}).Info()

		link := fmt.Sprintf(
			`http://localhost:3000/explore?orgId=${__org}&left={"datasource":%q,"range":{"from":%q,"to":%q},"queries":[{"graph":{"label":"processes"},"pid":${__value.raw}}]}`,
			req.PluginContext.DataSourceInstanceSettings.Name,
			"now-5m",
			"now",
		)

		resp.Responses[query.RefID] = Nodegraph(link, q.Pid)
	}

	return resp, nil
}
