// Copyright © 2021-2023 The Gomon Project.

package process

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	// status maps status codes to state names.
	status = map[byte]string{
		'R': "Running",
		'S': "Sleeping",
		'D': "Waiting",
		'Z': "Zombie",
		'T': "Stopped",
		'X': "Dead",
	}
)

// properties captures the properties of a process.
func (pid Pid) properties() (Id, Properties) {
	path := filepath.Join("/proc", pid.String(), "stat"))
	buf, err := os.ReadFile(path)
	if err != nil {
		log.DefaultLogger.Error(
			"ReadFile()",
			"file", path,
			"err", err,
		)
		return Id{Pid: pid}, Properties{}
	}
	fields := strings.Fields(string(buf))

	m, _ := gocore.Measures(filepath.Join("/proc", pid.String(), "status"))

	ppid, _ := strconv.Atoi(fields[3])
	pgid, _ := strconv.Atoi(fields[4])
	uid, _ := strconv.Atoi(m["Uid"])
	gid, _ := strconv.Atoi(m["Gid"])

	return Id{
			Name: fields[1][1 : len(fields[1])-1],
			Pid:  pid,
		},
		Properties{
			Ppid:        Pid(ppid),
			Pgid:        pgid,
			UID:         uid,
			GID:         gid,
			Username:    gocore.Username(uid),
			Groupname:   gocore.Groupname(gid),
			Status:      status[fields[2][0]],
			CommandLine: pid.commandLine(),
		}
}

// commandLine retrieves process command, arguments, and environment.
func (pid Pid) commandLine() CommandLine {
	clLock.Lock()
	defer clLock.Unlock()
	if cl, ok := clMap[pid]; ok {
		return cl
	}

	cl := CommandLine{}
	cl.Executable, _ = os.Readlink(filepath.Join("/proc", pid.String(), "exe"))
	if arg, err := os.ReadFile(filepath.Join("/proc", pid.String(), "cmdline")); err == nil && len(arg) > 2 {
		cl.Args = strings.Split(string(arg[:len(arg)-2]), "\x00")
		cl.Args = cl.Args[1:]
	}
	if env, err := os.ReadFile(filepath.Join("/proc", pid.String(), "environ")); err == nil {
		cl.Envs = strings.Split(string(env), "\x00")
	}

	clMap[pid] = cl
	return cl
}

// getPids gets the list of active processes by pid.
func getPids() ([]Pid, error) {
	dir, err := os.Open("/proc")
	if err != nil {
		return nil, fmt.Errorf("/proc open error %w", err)
	}
	ns, err := dir.Readdirnames(0)
	dir.Close()
	if err != nil {
		return nil, fmt.Errorf("/proc read error %w", err)
	}

	pids := make([]Pid, len(ns))
	i := 0
	for _, n := range ns {
		if pid, err := strconv.Atoi(n); err == nil {
			pids[i] = Pid(pid)
			i++
		}
	}

	return pids[:i], nil
}
