// Copyright © 2021 The Gomon Project.

package main

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

// seteuid toggles effective userid of gomon-datasource.
func seteuid(id int) {
	log.DefaultLogger.Debug("Current uid and euid",
		"uid", os.Getuid(),
		"euid", os.Geteuid(),
	)

	if uid != euid {
		err := syscall.Seteuid(id)
		log.DefaultLogger.Debug("Seteuid results",
			"uid", os.Getuid(),
			"euid", os.Geteuid(),
			"err", err,
		)
	}
}

var (
	uid  = os.Getuid()
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
	epMap  = map[Pid]Connections{}
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

	// id identifies the process.
	id struct {
		Name      string    `json:"name" gomon:"property"`
		Pid       Pid       `json:"pid" gomon:"property"`
		Starttime time.Time `json:"starttime" gomon:"property"`
	}

	// CommandLine contains a process' command line arguments.
	CommandLine struct {
		Exec string   `json:"exec" gomon:"property"`
		Args []string `json:"args" gomon:"property"`
		Envs []string `json:"envs" gomon:"property"`
	}

	// Props defines process properties.
	Props struct {
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

	// Connection represents a process connection to a data source.
	Connection struct {
		Descriptor int    `json:"descriptor" gomon:"property"`
		Type       string `json:"type" gomon:"property"`
		Name       string `json:"name" gomon:"property"`
		Self       string `json:"self" gomon:"property"`
		Peer       string `json:"peer" gomon:"property"`
	}

	// Connections records all the process' data connections.
	Connections []Connection

	// process contains a process' properties and connections.
	process struct {
		Id          id `json:"id" gomon:""`
		Props       `gomon:""`
		Connections `json:"connections" gomon:""`
	}

	// processTable defines a process table as a map of pids to processes.
	processTable map[Pid]*process

	// processTree organizes the process into a hierarchy
	processTree map[Pid]processTree
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

func (p *process) ID() string {
	return p.Id.Name + "[" + p.Id.Pid.String() + "]"
}

// buildTable builds a process table and captures current process state
func buildTable() processTable {
	seteuid(euid)
	defer seteuid(uid)

	pids, err := getPids()
	if err != nil {
		panic(fmt.Errorf("could not build process table %v", err))
	}

	var epm map[Pid]Connections
	epLock.RLock()
	if len(epMap) > 0 {
		epm = epMap
	}
	epLock.RUnlock()

	pt := make(map[Pid]*process, len(pids))
	for _, pid := range pids {
		id, props := pid.props()
		pt[pid] = &process{
			Id:          id,
			Props:       props,
			Connections: epm[pid],
		}
	}

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

func buildTree(pt processTable) processTree {
	t := processTree{}

	for pid, p := range pt {
		var ancestors []Pid
		for pid := p.Ppid; pid > 1; pid = pt[pid].Ppid {
			ancestors = append([]Pid{pid}, ancestors...)
		}
		addPid(t, append(ancestors, pid))
	}

	return t
}

func addPid(t processTree, ancestors []Pid) {
	if len(ancestors) == 0 {
		return
	}
	if _, ok := t[ancestors[0]]; !ok {
		t[ancestors[0]] = processTree{}
	}
	addPid(t[ancestors[0]], ancestors[1:])
}

func flatTree(t processTree, indent int) []Pid {
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
		flat = append(flat, flatTree(t[pid], indent+2)...)
	}

	return flat
}

func depthTree(t processTree) int {
	depth := 0
	for _, tree := range t {
		dt := depthTree(tree) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

func findTree(t processTree, pid Pid) processTree {
	if t, ok := t[pid]; ok {
		return t
	}
	for _, t := range t {
		if findTree(t, pid) != nil {
			return t
		}
	}

	return nil
}
