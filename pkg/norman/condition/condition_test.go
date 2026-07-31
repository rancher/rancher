package condition

import "testing"

func TestRegexp(t *testing.T) {
	testInputs := []string{
		"create file /tmp/file-uy76324 ",
		"/tmp/file-y6123tsd",
	}
	testOutputs := []string{
		"create file file_path_redacted ",
		"file_path_redacted",
	}
	for i, s := range testInputs {
		if testOutputs[i] != temfileRegexp.ReplaceAllString(s, "file_path_redacted") {
			t.Fatalf("Regexp failed to check %s", testInputs[i])
		}
	}
}
