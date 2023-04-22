// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"fmt"
	"sync"

	"github.com/zosmac/gocore"
)

type (
	// Table defines a process table as a map of pids to processes.
	Table = gocore.Table[Pid, *Process]

	// Tree organizes the processes into a hierarchy.
	Tree = gocore.Tree[Pid, int, *Process]
)

var (
	// clMap caches process command lines, which can be expensive to query.
	clMap  = map[Pid]CommandLine{}
	clLock sync.Mutex

	// oldPids identifies pids no longer active.
	oldPids map[Pid]struct{}

	// endpoints of processes periodically populated by lsof.
	epMap  = map[Pid][]Connection{}
	epLock sync.RWMutex
)

// BuildTable builds a process table and captures current process state.
func BuildTable() Table {
	gocore.Seteuid()
	defer gocore.Setuid()

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

// BuildTree builds the process tree.
func BuildTree(tb Table) Tree {
	tr := Tree{}
	for pid := range tb {
		var pids []Pid
		for ; pid > 0; pid = tb[pid].Ppid {
			pids = append([]Pid{pid}, pids...)
		}
		tr.Add(pids...)
	}
	return tr
}
