package util

import "strconv"

func Atoi(s string) int {
	i,err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func Atoi64(s string) int64 {
	i,err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}
