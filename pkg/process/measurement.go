// Copyright © 2021-2023 The Gomon Project.

package process

type (
	// CommandLine contains a process' command line arguments.
	CommandLine struct {
		Executable string   `json:"executable" gomon:"property"`
		Args       []string `json:"args" gomon:"property"`
		Envs       []string `json:"envs" gomon:"property"`
	}

	// Properties defines measurement properties.
	Properties struct {
		Ppid        Pid    `json:"ppid" gomon:"property"`
		Pgid        int    `json:"pgid,omitempty" gomon:"property,,!windows"`
		Tgid        int    `json:"tgid,omitempty" gomon:"property,,linux"`
		Tty         string `json:"tty,omitempty" gomon:"property,,!windows"`
		UID         int    `json:"uid,omitempty" gomon:"property,,!windows"`
		GID         int    `json:"gid,omitempty" gomon:"property,,!windows"`
		Username    string `json:"username" gomon:"property"`
		Groupname   string `json:"groupname,omitempty" gomon:"property,,!windows"`
		Status      string `json:"status" gomon:"enum,none"`
		CommandLine `gomon:""`
	}

	// Endpoint identifies one end of a connection.
	Endpoint struct {
		Name string `json:"name" gomon:"property"`
		Pid  Pid    `json:"pid" gomon:"property"`
	}

	// Connection represents an inter-process or host/data connection.
	Connection struct {
		Type string   `json:"type" gomon:"property"`
		Self Endpoint `json:"self" gomon:"property"`
		Peer Endpoint `json:"peer" gomon:"property"`
	}

	// Process contains a process' properties and connections.
	Process struct {
		Id          `json:"id" gomon:""`
		Properties  `gomon:""`
		Connections []Connection `json:"connections" gomon:""`
	}
)

// ID returns the identifier for a process message.
func (p *Process) ID() string {
	return p.Id.Name + "[" + p.Id.Pid.String() + "]"
}
