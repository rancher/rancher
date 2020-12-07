package nodepool

import "testing"

func TestNaturalLess(t *testing.T) {
	testset := []struct {
		s1, s2 string
		less   bool
	}{
		{
			"daxworker9",
			"daxworker10",
			true,
		},
		{
			"dax10worker9",
			"daxworker10",
			true,
		},
		{
			"string-with-hyphens",
			"stringwithouthyphens",
			true,
		},
		{"0", "00", true},
		{"00", "0", false},
		{"aa", "ab", true},
		{"ab", "abc", true},
		{"abc", "ad", true},
		{"ab1", "ab2", true},
		{"ab1c", "ab1c", false},
		{"ab12", "abc", true},
		{"ab2a", "ab10", true},
		{"a0001", "a0000001", true},
		{"a10", "abcdefgh2", true},
		{"аб2аб", "аб10аб", true},
		{"2аб", "3аб", true},
		//
		{"a1b", "a01b", true},
		{"a01b", "a1b", false},
		{"ab01b", "ab010b", true},
		{"ab010b", "ab01b", false},
		{"a01b001", "a001b01", true},
		{"a001b01", "a01b001", false},
		{"a1", "a1x", true},
		{"1ax", "1b", true},
		{"1b", "1ax", false},
		//
		{"082", "83", true},
		//
		{"083a", "9a", false},
		{"9a", "083a", true},
	}
	for _, v := range testset {
		if res := NaturalLess(v.s1, v.s2); res != v.less {
			t.Errorf("Compared %#q to %#q: expected %v, got %v",
				v.s1, v.s2, v.less, res)
		}
	}
}
