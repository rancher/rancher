package deployer

import (
	"testing"
)

func TestVersion(t *testing.T) {
	expectedVersion := "system-library-rancher-logging-initializing"
	loggingService := &LoggingService{}
	version, err := loggingService.Version()
	if err != nil {
		t.Error(err)
		return
	}

	if version != expectedVersion {
		t.Errorf("output version %s isn't equal to expected version %s", version, expectedVersion)
	}
}
