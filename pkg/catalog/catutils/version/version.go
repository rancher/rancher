package version

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	numberRe = regexp.MustCompile("[0-9]+")
	wordRe   = regexp.MustCompile("[a-z]+")
)

func GreaterThan(a, b string) bool {
	a = stripMetadata(a)
	b = stripMetadata(b)

	a = strings.TrimLeft(a, "v")
	b = strings.TrimLeft(b, "v")

	aSplit := periodDashSplit(a)
	bSplit := periodDashSplit(b)

	if len(bSplit) > len(aSplit) {
		return !GreaterThan(b, a) && a != b
	}

	for i := 0; i < len(aSplit); i++ {
		if i == len(bSplit) {
			if _, err := strconv.Atoi(aSplit[i]); err == nil {
				return true
			}
			return false
		}
		aWord := wordRe.FindString(aSplit[i])
		bWord := wordRe.FindString(bSplit[i])
		if aWord != "" && bWord != "" {
			if strings.Compare(aWord, bWord) > 0 {
				return true
			}
			if strings.Compare(bWord, aWord) > 0 {
				return false
			}
		}
		aMatch := numberRe.FindString(aSplit[i])
		bMatch := numberRe.FindString(bSplit[i])
		if aMatch == "" || bMatch == "" {
			if strings.Compare(aSplit[i], bSplit[i]) > 0 {
				return true
			}
			if strings.Compare(bSplit[i], aSplit[i]) > 0 {
				return false
			}
		}
		aNum, _ := strconv.Atoi(aMatch)
		bNum, _ := strconv.Atoi(bMatch)
		if aNum > bNum {
			return true
		}
		if bNum > aNum {
			return false
		}
	}

	return false
}

func stripMetadata(v string) string {
	split := strings.Split(v, "+")
	if len(split) > 1 {
		return split[0]
	}
	return v
}

func periodDashSplit(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '.', '-':
			return true
		}
		return false
	})
}
