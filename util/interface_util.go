package util

import "reflect"

// interface{}判断是否为空 不能简单的==nil
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	} else {
		switch v := reflect.ValueOf(i); v.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
			return v.IsNil()
		}
	}
	return false
}