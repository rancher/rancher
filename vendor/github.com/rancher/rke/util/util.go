package util

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/coreos/go-semver/semver"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	WorkerThreads = 50
	// SupportedSyncToolsVersion this should be kept at the latest version of rke released with
	// rancher 2.2.0.
	SupportedSyncToolsVersion = "0.1.25"
)

func StrToSemVer(version string) (*semver.Version, error) {
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return nil, err
	}
	return v, nil
}

func GetObjectQueue(l interface{}) chan interface{} {
	s := reflect.ValueOf(l)
	c := make(chan interface{}, s.Len())

	for i := 0; i < s.Len(); i++ {
		c <- s.Index(i).Interface()
	}
	close(c)
	return c
}

func ErrList(e []error) error {
	if len(e) > 0 {
		return fmt.Errorf("%v", e)
	}
	return nil
}

// UniqueStringSlice - Input slice, retrun slice with unique elements. Will not maintain order.
func UniqueStringSlice(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if !encountered[elements[v]] {
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}
	return result
}

func IsSymlink(file string) (bool, error) {
	f, err := os.Lstat(file)
	if err != nil {
		return false, err
	}
	if f.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}
	return false, nil
}

func GetDefaultRKETools() string {
	return v3.AllK8sVersions[v3.DefaultK8s].Alpine
}

// IsRancherBackupSupported  with rancher 2.2.0 and rke 0.2.0, etcdbackup was completely refactored
// and the interface for the rke-tools backup command changed significantly.
// This function is used to check the the release rke-tools version to choose
// between the new backup or the legacy backup code paths.
// The released version of rke-tools should be set in the const SupportedSyncToolsVersion
func IsRancherBackupSupported(image string) bool {
	v := strings.Split(image, ":")
	last := v[len(v)-1]

	sv, err := StrToSemVer(last)
	if err != nil {
		return false
	}

	supported, err := StrToSemVer(SupportedSyncToolsVersion)
	if err != nil {
		return false
	}
	if sv.LessThan(*supported) {
		return false
	}
	return true
}

func GetTagMajorVersion(tag string) string {
	splitTag := strings.Split(tag, ".")
	if len(splitTag) < 2 {
		return ""
	}
	return strings.Join(splitTag[:2], ".")
}
