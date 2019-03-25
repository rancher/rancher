// +build !windows,!nacl,!plan9

package utils

import (
	"log/syslog"
)

func getSeverity(severityStr string) syslog.Priority {
	severityMap := map[string]syslog.Priority{
		"emerg":   syslog.LOG_EMERG,
		"alert":   syslog.LOG_ALERT,
		"crit":    syslog.LOG_CRIT,
		"err":     syslog.LOG_ERR,
		"warning": syslog.LOG_WARNING,
		"notice":  syslog.LOG_NOTICE,
		"info":    syslog.LOG_INFO,
		"debug":   syslog.LOG_DEBUG,
	}

	if severity, ok := severityMap[severityStr]; ok {
		return severity
	}

	return syslog.LOG_INFO
}
