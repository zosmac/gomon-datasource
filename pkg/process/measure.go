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

func FlatTree(t Tree) []Pid {
	return flatTree(t, 0)
}

func flatTree(t Tree, indent int) []Pid {
	if len(t) == 0 {
		return nil
	}
	var flat []Pid

	var pids []Pid
	for pid := range t {
		pids = append(pids, pid)
	}

	sort.Slice(pids, func(i, j int) bool {
		dti := depthTree(t[pids[i]])
		dtj := depthTree(t[pids[j]])
		return dti > dtj ||
			dti == dtj && pids[i] < pids[j]
	})

	for _, pid := range pids {
		flat = append(flat, pid)
		flat = append(flat, flatTree(t[pid], indent+3)...)
	}

	return flat
}

// depthTree enables sort of deepest process trees first.
func depthTree(t Tree) int {
	depth := 0
	for _, t := range t {
		dt := depthTree(t) + 1
		if depth < dt {
			depth = dt
		}
	}
	return depth
}

// FindTree finds the process tree parented by a specific process.
func FindTree(t Tree, parent Pid) Tree {
	for pid, t := range t {
		if pid == parent {
			return Tree{parent: t}
		}
		if t = FindTree(t, parent); t != nil {
			return t
		}
	}

	return nil
}
