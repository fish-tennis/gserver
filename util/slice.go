package util

func IndexString(slice []string, s string) int {
	for idx,str := range slice {
		if str == s {
			return idx
		}
	}
	return -1
}

func HasString(slice []string, s string) bool {
	return IndexString(slice,s) >= 0
}
