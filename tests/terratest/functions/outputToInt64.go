package functions

import "strconv"

func OutputToInt64(output string) int64 {
	outputInt64, _ := strconv.ParseInt(output, 10, 0)
	return outputInt64
}