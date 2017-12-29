package set

func Diff(desired, actual map[string]bool) (toCreate []string, toDelete []string, same []string) {
	for key := range desired {
		if actual[key] {
			same = append(same, key)
		} else {
			toCreate = append(toCreate, key)
		}
	}
	for key := range actual {
		if !desired[key] {
			toDelete = append(toDelete, key)
		}
	}
	return
}
