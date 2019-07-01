package nodepool

import "regexp"

// use pointer to avoid copying large number of structs
// could also use strings here maybe?
// type nodes []*v3.Node

type stringSlice []string

// implement sort interface

/*
 Expecting to see strings styled [Prefix] + [some decimal value] which means we can do some specific things
Inspired by https://github.com/facette/natsort/blob/2cd4dd1e2dcba4d85d6d3ead4adf4cfd2b70caf2/natsort.go
*/

func (s stringSlice) Len() int {
	return len(s)
}

func (s stringSlice) Less(a, b int) bool {
	return Compare(s[a], s[b])
}

func (s stringSlice) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

var chunifyRegexp = regexp.MustCompile(`(\d+|\D+)`)

func Compare(a, b string) bool {
	return true
}
