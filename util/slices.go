package util

// 只删除slice的一个元素
func DeleteOne[S ~[]E, E comparable](s S, delELem E) S {
	for i, v := range s {
		if v == delELem {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// 只删除slice的一个元素
func DeleteOneFunc[S ~[]E, E any](s S, del func(E) bool) S {
	for i, v := range s {
		if del(v) {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
