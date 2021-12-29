// Copyright © 2021 The Gomon Project.

package process

/*
#include <libproc.h>
#include <sys/sysctl.h>
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"
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

// props captures the properties of a process.
func (pid Pid) props() (id, Props) {
	var bsi C.struct_proc_bsdshortinfo
	if n := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDT_SHORTBSDINFO,
		0,
		unsafe.Pointer(&bsi),
		C.int(C.PROC_PIDT_SHORTBSDINFO_SIZE),
	); n != C.int(C.PROC_PIDT_SHORTBSDINFO_SIZE) {
		return id{Pid: pid}, Props{}
	}

	return id{
			Name: C.GoString(&bsi.pbsi_comm[0]),
			Pid:  pid,
		},
		Props{
			Ppid:        Pid(bsi.pbsi_ppid),
			Pgid:        int(bsi.pbsi_pgid),
			UID:         int(bsi.pbsi_uid),
			GID:         int(bsi.pbsi_gid),
			Username:    users.name(int(bsi.pbsi_uid)),
			Groupname:   groups.name(int(bsi.pbsi_gid)),
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

	l := int(binary.BigEndian.Uint32(buf[:4]))
	ss := bytes.Split(buf[4:size], []byte{0})
	var i int
	var exec string
	var args, envs []string
	for _, s := range ss {
		if len(s) == 0 { // strings in command line are null padded, so Split will yield many zero length "arg" strings
			continue
		}
		if i == 0 {
			exec = C.GoString((*C.char)(unsafe.Pointer(&s[0])))
		} else if i <= l {
			args = append(args, C.GoString((*C.char)(unsafe.Pointer(&s[0]))))
		} else {
			envs = append(envs, C.GoString((*C.char)(unsafe.Pointer(&s[0]))))
		}
		i++
	}

	cl = CommandLine{
		Exec: exec,
		Args: args,
		Envs: envs,
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
