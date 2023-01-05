// Copyright © 2021 The Gomon Project.

package process

import (
	"strconv"
	"time"
)

type (
	// Pid is the identifier for a process.
	Pid int

	// Id identifies the message.
	Id struct {
		Name      string    `json:"name" gomon:"property"`
		Pid       Pid       `json:"pid" gomon:"property"`
		Starttime time.Time `json:"starttime" gomon:"property"`
	}
)

func (pid Pid) String() string {
	return strconv.Itoa(int(pid))
}
