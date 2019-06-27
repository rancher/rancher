package util

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/coreos/go-semver/semver"
	ref "github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

const (
	WorkerThreads = 50

	DefaultRKETools = "rancher/rke-tools:v0.1.34"
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

func GetTagMajorVersion(tag string) string {
	splitTag := strings.Split(tag, ".")
	if len(splitTag) < 2 {
		return ""
	}
	return strings.Join(splitTag[:2], ".")
}

func IsFileExists(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func GetImageTagFromImage(image string) (string, error) {
	parsedImage, err := ref.ParseNormalizedNamed(image)
	if err != nil {
		return "", err
	}
	imageTag := parsedImage.(ref.Tagged).Tag()
	logrus.Debugf("Extracted version [%s] from image [%s]", imageTag, image)
	return imageTag, nil
}
