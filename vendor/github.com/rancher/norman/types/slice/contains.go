package slice

func ContainsString(slice []string, item string) bool {
	for _, j := range slice {
		if j == item {
			return true
		}
	}
	return false
}
