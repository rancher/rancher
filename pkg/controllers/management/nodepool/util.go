package nodepool

import (
	"strconv"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/sirupsen/logrus"
)

// use pointer to avoid copying large number of structs

type byNodeRequestedHostName []*v3.Node

/*
 Expecting to see strings styled [Prefix] + [some decimal value] which means we can do some specific things
Inspired by https://github.com/facette/natsort/blob/2cd4dd1e2dcba4d85d6d3ead4adf4cfd2b70caf2/natsort.go
*/

func (s byNodeRequestedHostName) Len() int {
	return len(s) //returns len of slice of structs
}

func (s byNodeRequestedHostName) Less(a, b int) bool {
	return Compare(s[a].Spec.RequestedHostname, s[b].Spec.RequestedHostname)
}

func (s byNodeRequestedHostName) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

// Compare returns true if the first string precedes the second one according to natural order
func Compare(a, b string) bool {
	s1 := nameRegexp.FindStringSubmatch(a)
	s2 := nameRegexp.FindStringSubmatch(b)

	if len(s1) < 2 || len(s2) < 2 {
		// fallback onto lexicographical sorting
		logrus.Warnf("Unable to sort nodes %s  %s naturally, fallback onto lexicographical sort", a, b)
		return a < b
	}

	s1Int, aErr := strconv.Atoi(s1[2])
	s2Int, bErr := strconv.Atoi(s2[2])

	if aErr == nil && bErr == nil {
		return s1Int < s2Int
	}
	// fallback on lexicographical sorting
	logrus.Warnf("Unable to sort nodes %s  %s naturally, fallback onto lexicographical sort", a, b)
	return a < b
}
