package systemtokens

import (
	"testing"
)

func Test_getIdentifier(t *testing.T) {
	t.Run("Generate random string", func(t *testing.T) {
		if got := getIdentifier(); len(got) != 5 {
			t.Errorf("getIdentifier() = %v, want length 5, got length %v", got, len(got))
		}
	})
}
