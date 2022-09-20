// Copyright © 2021 The Gomon Project.

package process

/*
#include <libproc.h>
#include <sys/sysctl.h>
*/
import "C"

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/zosmac/gomon-datasource/pkg/core"
)

var (
	status = map[C.uint]string{
		C.SIDL:   "Idle",
		C.SRUN:   "Running",
		C.SSLEEP: "Sleeping",
		C.SSTOP:  "Stopped",
		C.SZOMB:  "Zombie",
	}
)

// properties captures the properties of a process.
func (pid Pid) properties() (Id, Properties) {
	var bsi C.struct_proc_bsdshortinfo
	if n := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDT_SHORTBSDINFO,
		0,
		unsafe.Pointer(&bsi),
		C.int(C.PROC_PIDT_SHORTBSDINFO_SIZE),
	); n != C.int(C.PROC_PIDT_SHORTBSDINFO_SIZE) {
		return Id{Pid: pid}, Properties{}
	}

	return Id{
			Name: C.GoString(&bsi.pbsi_comm[0]),
			Pid:  pid,
		},
		Properties{
			Ppid:        Pid(bsi.pbsi_ppid),
			Pgid:        int(bsi.pbsi_pgid),
			UID:         int(bsi.pbsi_uid),
			GID:         int(bsi.pbsi_gid),
			Username:    core.Username(int(bsi.pbsi_uid)),
			Groupname:   core.Groupname(int(bsi.pbsi_gid)),
			Status:      status[bsi.pbsi_status],
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

	size := C.size_t(C.ARG_MAX)
	buf := make([]byte, size)

	if rv := C.sysctl(
		(*C.int)(unsafe.Pointer(&[3]C.int{C.CTL_KERN, C.KERN_PROCARGS2, C.int(pid)})),
		3,
		unsafe.Pointer(&buf[0]),
		&size,
		unsafe.Pointer(nil),
		0,
	); rv != 0 {
		return CommandLine{}
	}

	l := int(*(*uint32)(unsafe.Pointer(&buf[0])))
	ss := bytes.FieldsFunc(buf[4:size], func(r rune) bool { return r == 0 })
	var executable string
	var args, envs []string
	for i, s := range ss {
		if i == 0 {
			executable = string(s)
		} else if i <= l {
			args = append(args, string(s))
		} else {
			envs = append(envs, string(s))
		}
	}

	cl = CommandLine{
		Executable: executable,
		Args:       args,
		Envs:       envs,
	}
	clLock.Lock()
	clMap[pid] = cl
	clLock.Unlock()
	return cl
}

// getPids gets the list of active processes by pid.
func getPids() ([]Pid, error) {
	n, err := C.proc_listpids(C.PROC_ALL_PIDS, 0, nil, 0)
	if n <= 0 {
		return nil, fmt.Errorf("proc_listpids PROC_ALL_PIDS failed %w", err)
	}

	var pid C.int
	buf := make([]C.int, n/C.int(unsafe.Sizeof(pid))+10)
	if n, err = C.proc_listpids(C.PROC_ALL_PIDS, 0, unsafe.Pointer(&buf[0]), n); n <= 0 {
		return nil, fmt.Errorf("proc_listpids PROC_ALL_PIDS failed %w", err)
	}
	n /= C.int(unsafe.Sizeof(pid))
	if int(n) < len(buf) {
		buf = buf[:n]
	}

	pids := make([]Pid, len(buf))
	for i, pid := range buf {
		pids[int(n)-i-1] = Pid(pid) // Darwin returns pids in descending order, so reverse the order
	}
	return pids, nil
}
