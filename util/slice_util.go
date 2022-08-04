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

func ContainsInt32(slice []int32, i int32) bool {
	for _,v := range slice {
		if v == i {
			return true
		}
	}
	return false
}