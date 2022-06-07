package functions

import "strconv"

func OutputToInt(output string) int {
	outputInt64, _ := strconv.ParseInt(output, 10, 0)
	outputInt := int(outputInt64)
	return outputInt
}