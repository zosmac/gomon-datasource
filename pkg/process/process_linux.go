// Copyright © 2021 The Gomon Project.

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

// props captures the properties of a process.
func (pid Pid) props() (id, Props) {
	buf, err := os.ReadFile("/proc/" + strconv.Itoa(int(pid)) + "/stat")
	if err != nil {
		log.DefaultLogger.Error("Cannot read proc file",
			"path", "/proc/"+strconv.Itoa(int(pid))+"/stat",
		)
		return id{Pid: pid}, Props{}
	}
	fields := strings.Fields(string(buf))

	m, _ := measures(filepath.Join("/proc", strconv.Itoa(int(pid)), "status"))

	ppid, _ := strconv.Atoi(fields[3])
	pgid, _ := strconv.Atoi(fields[4])
	uid, _ := strconv.Atoi(m["Uid"])
	gid, _ := strconv.Atoi(m["Gid"])

	return id{
			Name: fields[1][1 : len(fields[1])-1],
			Pid:  pid,
		},
		Props{
			Ppid:        Pid(ppid),
			Pgid:        pgid,
			UID:         uid,
			GID:         gid,
			Username:    users.name(int(uid)),
			Groupname:   groups.name(int(gid)),
			Status:      status[fields[2][0]],
			CommandLine: pid.commandLine(),
		}
}

// commandLine retrieves process command, arguments, and environment.
func (pid Pid) commandLine() CommandLine {
	clLock.RLock()
	cl, ok := clMap[pid]
	clLock.RUnlock()
	if ok {
		return cl
	}

	cl.Exec, _ = os.Readlink(filepath.Join("/proc", strconv.Itoa(int(pid)), "exe"))

	if arg, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(int(pid)), "cmdline")); err == nil {
		cl.Args = strings.Split(string(arg[:len(arg)-2]), "\000")
		cl.Args = cl.Args[1:]
	}

	if env, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(int(pid)), "environ")); err == nil {
		cl.Envs = strings.Split(string(env), "\000")
	}

	clLock.Lock()
	clMap[pid] = cl
	clLock.Unlock()

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

	pids := make([]Pid, 0, len(ns))
	i := 0
	for _, n := range ns {
		if pid, err := strconv.Atoi(n); err == nil {
			pids[i] = Pid(pid)
			i++
		}
	}

	return pids, nil
}

// measures reads a /proc filesystem file and produces a map of name:value pairs.
func measures(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := strings.SplitN(sc.Text(), ":", 2)
		if len(s) == 2 {
			k := s[0]
			v := strings.Fields(s[1])
			if len(v) > 0 {
				m[k] = v[0]
			}
		}
	}

	return m, nil
}
