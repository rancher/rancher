package utils

func getSeverity(severityStr string) int {
	severityMap := map[string]int{
		"emerg":   0,
		"alert":   1,
		"crit":    2,
		"err":     3,
		"warning": 4,
		"notice":  5,
		"info":    6,
		"debug":   7,
	}

	if severity, ok := severityMap[severityStr]; ok {
		return severity
	}

	return 6
}
