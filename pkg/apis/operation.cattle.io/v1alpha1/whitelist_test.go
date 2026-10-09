package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhitelistEntries(t *testing.T) {
	assert.Nil(t, WhitelistEntries(""))
	assert.Nil(t, WhitelistEntries(" , ,"))
	assert.Equal(t, []string{"a.example.io"}, WhitelistEntries("a.example.io"))
	assert.Equal(t, []string{"a.example.io", "b.example.io"}, WhitelistEntries(" a.example.io , b.example.io,"))
}

func TestWhitelisted(t *testing.T) {
	const save = "etcdsnapshotsaves.operation.cattle.io"

	for name, tc := range map[string]struct {
		annotations map[string]string
		restricted  bool
		restore     bool
		save        bool
	}{
		"no annotations":         {annotations: nil, restore: true, save: true},
		"no whitelist":           {annotations: map[string]string{"other": "x"}, restore: true, save: true},
		"an empty whitelist":     {annotations: map[string]string{WhitelistedAnnotation: ""}, restore: true, save: true},
		"a whitelist of commas":  {annotations: map[string]string{WhitelistedAnnotation: " , "}, restore: true, save: true},
		"restores only":          {annotations: map[string]string{WhitelistedAnnotation: ETCDSnapshotRestoreResource}, restricted: true, restore: true},
		"restores, with spacing": {annotations: map[string]string{WhitelistedAnnotation: " " + ETCDSnapshotRestoreResource + " "}, restricted: true, restore: true},
		"two entries":            {annotations: map[string]string{WhitelistedAnnotation: ETCDSnapshotRestoreResource + "," + save}, restricted: true, restore: true, save: true},
		"only something else":    {annotations: map[string]string{WhitelistedAnnotation: "other.example.io"}, restricted: true},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.restricted, HasWhitelist(tc.annotations), "HasWhitelist")
			assert.Equal(t, tc.restore, Whitelisted(tc.annotations, ETCDSnapshotRestoreResource), "restore")
			assert.Equal(t, tc.save, Whitelisted(tc.annotations, save), "save")
		})
	}

	// A resource is matched as a whole entry, not as a substring of one.
	assert.False(t, Whitelisted(map[string]string{WhitelistedAnnotation: "x" + ETCDSnapshotRestoreResource}, ETCDSnapshotRestoreResource))
}

func TestAppendToWhitelist(t *testing.T) {
	for name, tc := range map[string]struct {
		value   string
		want    string
		changed bool
	}{
		"absent or empty":            {value: "", want: ETCDSnapshotRestoreResource, changed: true},
		"only commas":                {value: " , ", want: ETCDSnapshotRestoreResource, changed: true},
		"already present":            {value: ETCDSnapshotRestoreResource, want: ETCDSnapshotRestoreResource},
		"already present, spaced":    {value: " a.example.io , " + ETCDSnapshotRestoreResource, want: " a.example.io , " + ETCDSnapshotRestoreResource},
		"missing from other entries": {value: "a.example.io, b.example.io", want: "a.example.io,b.example.io," + ETCDSnapshotRestoreResource, changed: true},
	} {
		t.Run(name, func(t *testing.T) {
			got, changed := AppendToWhitelist(tc.value, ETCDSnapshotRestoreResource)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.changed, changed)
		})
	}
}
