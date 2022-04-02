// Copyright © 2021 The Gomon Project.

package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/zosmac/gomon-datasource/pkg/logs"
)

func Logs(link string) (resp backend.DataResponse) {
	logs.MsgLock.Lock()
	messages := logs.Messages
	logs.Messages = nil
	logs.MsgLock.Unlock()

	resp.Frames = logFrames(link, messages)

	return
}
