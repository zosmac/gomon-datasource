// Copyright © 2021 The Gomon Project.

package process

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// seteuid gomon-datasource to owner.
func seteuid() {
	err := syscall.Seteuid(euid)
	log.DefaultLogger.Debug("Seteuid results",
		"uid", strconv.Itoa(os.Getuid()), // to format as int rather than float
		"euid", strconv.Itoa(os.Geteuid()), // to format as int rather than float
		"err", err,
	)
}

// setuid gomon-datasource to grafana user.
func setuid() {
	err := syscall.Seteuid(os.Getuid())
	log.DefaultLogger.Debug("Setuid results",
		"uid", strconv.Itoa(os.Getuid()), // to format as int rather than float
		"euid", strconv.Itoa(os.Geteuid()), // to format as int rather than float
		"err", err,
	)
}

var (
	// euid gets the executable's owner id.
	euid = os.Geteuid()

	// users caches user names for uids.
	users = names{
		lookup: func(id int) string {
			name := strconv.Itoa(id)
			if u, err := user.LookupId(name); err == nil {
				name = u.Name
			}
			return name
		},
		names: map[int]string{},
	}

	// groups caches group names for gids.
	groups = names{
		lookup: func(id int) string {
			name := strconv.Itoa(id)
			if g, err := user.LookupGroupId(name); err == nil {
				name = g.Name
			}
			return name
		},
		names: map[int]string{},
	}

	// clMap caches process command lines, which are expensive to query.
	clMap  = map[Pid]CommandLine{}
	clLock sync.RWMutex

	// oldPids identifies pids no longer active.
	oldPids map[Pid]struct{}

	// endpoints of processes periodically populated by lsof.
	epMap  = map[Pid][]Connection{}
	epLock sync.RWMutex
)

type (
	// names defines a cache type for mapping ids to names.
	names struct {
		sync.RWMutex
		lookup func(int) string
		names  map[int]string
	}

	// Pid is the identifier for a process.
	Pid int

	// Id identifies the process.
	Id struct {
		Name      string    `json:"name" gomon:"property"`
		Pid       Pid       `json:"pid" gomon:"property"`
		Starttime time.Time `json:"starttime" gomon:"property"`
	}

	// CommandLine contains a process' command line arguments.
	CommandLine struct {
		Executable string   `json:"executable" gomon:"property"`
		Args       []string `json:"args" gomon:"property"`
		Envs       []string `json:"envs" gomon:"property"`
	}

	// Properties defines process properties.
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

	Endpoint struct {
		Name string `json:"name" gomon:"property"`
		Pid  Pid    `json:"pid" gomon:"property"`
	}

	// Connection represents an inter-process or host/data connection
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

	// Table defines a process table as a map of pids to processes.
	Table map[Pid]*Process

	// Tree organizes the processes into a hierarchy.
	Tree map[Pid]Tree
)

// lookup retrieves and caches name for id.
func (ns *names) name(id int) string {
	ns.RLock()
	name, ok := ns.names[id]
	ns.RUnlock()
	if !ok {
		name = ns.lookup(id)
		ns.Lock()
		ns.names[id] = name
		ns.Unlock()
	}
	return name
}

func (pid Pid) String() string {
	return strconv.Itoa(int(pid))
}

func (p *Process) ID() string {
	return p.Id.Name + "[" + p.Id.Pid.String() + "]"
}

// BuildTable builds a process table and captures current process state
func BuildTable() Table {
	seteuid()
	defer setuid()

	pids, err := getPids()
	if err != nil {
		panic(fmt.Errorf("could not build process table %v", err))
	}

	epLock.RLock()
	pt := make(map[Pid]*Process, len(pids))
	for _, pid := range pids {
		id, props := pid.properties()
		pt[pid] = &Process{
			Id:          id,
			Properties:  props,
			Connections: epMap[pid],
		}
	}
	epLock.RUnlock()

	newPids := make(map[Pid]struct{}, len(pids))
	for pid := range pt {
		newPids[pid] = struct{}{}
		delete(oldPids, pid)
	}

	for pid := range oldPids { // process exited
		delete(clMap, pid) // remove command line from cache
	}

	oldPids = newPids

	return pt
}

func BuildTree(pt Table) Tree {
	t := Tree{}

	for pid, p := range pt {
		var ancestors []Pid
		for pid := p.Ppid; pid > 1; pid = pt[pid].Ppid {
			ancestors = append([]Pid{pid}, ancestors...)
		}
		addPid(t, append(ancestors, pid))
	}

	return t
}

func addPid(t Tree, ancestors []Pid) {
	if len(ancestors) == 0 {
		return
	}
	if _, ok := t[ancestors[0]]; !ok {
		t[ancestors[0]] = Tree{}
	}
	addPid(t[ancestors[0]], ancestors[1:])
}

func FlatTree(t Tree, indent int) []Pid {
	var flat []Pid

	pids := make([]Pid, len(t))
	var i int
	for pid := range t {
		pids[i] = pid
		i++
	}

	sort.Slice(pids, func(i, j int) bool {
		dti := depthTree(t[pids[i]])
		dtj := depthTree(t[pids[j]])
		return dti > dtj ||
			dti == dtj && pids[i] < pids[j]
	})

	for _, pid := range pids {
		flat = append(flat, pid)
		flat = append(flat, FlatTree(t[pid], indent+2)...)
	}

	return flat
}

func depthTree(t Tree) int {
	depth := 0
	for _, tree := range t {
		dt := depthTree(tree) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

func FindTree(t Tree, pid Pid) Tree {
	if t, ok := t[pid]; ok {
		return t
	}
	for _, t := range t {
		if FindTree(t, pid) != nil {
			return t
		}
	}

	return nil
}
