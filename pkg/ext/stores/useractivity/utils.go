package useractivity

import (
	"fmt"
	"strings"
)

func setUserActivityName(user, token string) (string, error) {
	if user == "" {
		return "", fmt.Errorf("user argument must not be empty")
	}
	if token == "" {
		return "", fmt.Errorf("token argument must not be empty")
	}
	// UserActivity name must have the following format:
	// ua_$(USER)_$(TOKEN)
	return strings.Join([]string{"ua", user, token}, "_"), nil
}

func getUserActivityName(uaName string) (string, string, error) {
	if !strings.HasPrefix(uaName, "ua_") {
		return "", "", fmt.Errorf("invalid prefix")
	}
	result := strings.Split(uaName, "_")
	if len(result) != 3 {
		return "", "", fmt.Errorf("invalid format")
	}
	return result[1], result[2], nil
}
