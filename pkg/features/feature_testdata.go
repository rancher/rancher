//go:build test
// +build test

package features

var (
	IsDefFalse = newFeature("isfalse", "", false, false, true)
)
