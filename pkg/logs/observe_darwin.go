// Copyright © 2021 The Gomon Project.

package logs

/*
#include <libproc.h>
*/
import "C"

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	// osLogLevels maps gomon log levels to OSLog message types
	osLogLevels = map[string]int{
		"trace": 0,  // Default
		"debug": 0,  // Default
		"info":  1,  // Info
		"warn":  2,  // Debug
		"error": 16, // Error
		"fatal": 17, // Fault
	}

	// syslogLevels maps gomon log levels to syslog log levels
	syslogLevels = map[string]string{
		"trace": "7", // Debug
		"debug": "7", // Debug
		"info":  "6", // Info, Notice
		"warn":  "4", // Warning
		"error": "3", // Error, Critical
		"fatal": "1", // Alert, Emergency
	}

	// logRegex for parsing output from the syslog -w -T utc.3 command.
	logRegex = regexp.MustCompile(
		`^(?P<timestamp>\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d\d\d\d\d\d[+-]\d\d\d\d) ` +
			`(?P<thread>[^ ]+)[ ]+` +
			`(?P<level>[^ ]+)[ ]+` +
			`(?P<activity>[^ ]+)[ ]+` +
			`(?P<pid>\d+)[ ]+` +
			`(?P<ttl>\d+)[ ]+` +
			`(?P<process>[^:]+): ` +
			`\((?P<sender>[^\)]+)\) ` +
			`(?:\[(?P<subcat>[^\]]+)\] |)` +
			`(?P<message>.*)$`,
	)

	// syslogRegex for parsing output from the syslog -w -T utc.3 command.
	syslogRegex = regexp.MustCompile(
		`^(?P<timestamp>\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d\d\dZ) ` +
			`(?P<host>[^ ]+) ` +
			`(?P<process>[^\[]+)\[` +
			`(?P<pid>\d+)\] ` +
			`(?:\((?P<sender>(?:\[[\d]+\]|)[^\)]+|)\) |)` +
			`<(?P<level>[A-Z][a-z]*)>: ` +
			`(?P<message>.*)$`,
	)
)

// init starts the macOS log and syslog commands as sub-processes to stream log entries.
func init() {
	logCommand()
	syslogCommand()
}

// logCommand starts the log command to capture OSLog entries (using OSLogStore API directly is MUCH slower)
func logCommand() {
	predicate := fmt.Sprintf(
		"(eventType == 'logEvent') AND (messageType >= %d) AND (NOT eventMessage BEGINSWITH[cd] '%s')",
		osLogLevels["info"],
		"System Policy: gomon",
	)

	sc, err := startCommand(append(strings.Fields("log stream --predicate"), predicate))
	if err != nil {
		log.DefaultLogger.Error("log command failed", "err", err)
		return
	}

	sc.Scan() // ignore first output line from log command
	sc.Text() //  (it just echoes the filter)
	sc.Scan() // ignore second output line
	sc.Text() //  (it is column headers)

	go parseLog(sc, logRegex, "2006-01-02 15:04:05Z0700")
}

// syslogCommand starts the syslog command to capture syslog entries
func syslogCommand() {
	sc, err := startCommand(append(strings.Fields("syslog -w 0 -T utc.3 -k Level Nle"),
		syslogLevels["info"]),
	)
	if err != nil {
		log.DefaultLogger.Error("syslog command failed", "err", err)
		return
	}

	go parseLog(sc, syslogRegex, "2006-01-02 15:04:05Z")
}

func startCommand(cmdline []string) (*bufio.Scanner, error) {
	cmd := exec.Command(cmdline[0], cmdline[1:]...)

	// ensure that no open descriptors propagate to child
	if n := C.proc_pidinfo(
		C.int(os.Getpid()),
		C.PROC_PIDLISTFDS,
		0,
		nil,
		0,
	); n >= 3*C.PROC_PIDLISTFD_SIZE {
		cmd.ExtraFiles = make([]*os.File, (n/C.PROC_PIDLISTFD_SIZE)-3) // close gomon files in child
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe failed %w", err)
	}
	cmd.Stderr = nil // sets to /dev/null
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start failed %w", err)
	}

	log.DefaultLogger.Info(fmt.Sprintf("start %q[%d]", cmd.String(), cmd.Process.Pid))

	return bufio.NewScanner(stdout), nil
}
