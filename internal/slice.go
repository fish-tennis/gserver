package internal

import (
	"github.com/fish-tennis/gserver/util"
	"strconv"
	"strings"
)

//type SliceInterface struct {
//	s []interface{}
//	// reflect.type 加一个类型检查
//}
//
//func (this *SliceInterface) Append(elem interface{}) {
//	this.s = append(this.s, elem)
//}
//
//func (this *SliceInterface) DeleteByIndex(index int) {
//	if index < 0 || index >= len(this.s) {
//		return
//	}
//	this.s = append(this.s[0:index], this.s[index+1:]...)
//}
//
//func (this *SliceInterface) Len() int {
//	return len(this.s)
//}
//
//func (this *SliceInterface) Cap() int {
//	return cap(this.s)
//}
//
//func (this *SliceInterface) Range(f func(index int,elem interface{}) bool) {
//	for index,elem := range this.s {
//		if !f(index,elem) {
//			break
//		}
//	}
//}

// 由于slice的操作可能返回新地址,所以无法用于Saveable的接口
// SliceInt32提供了一个不变的引用地址,把slice的操作封装在内部
// SliceInt32只能和DirtyMark配合使用,暂不支持MapDirtyMark
type SliceInt32 struct {
	s []int32
}

func (this *SliceInt32) Append(elem int32) {
	this.s = append(this.s, elem)
}

func (this *SliceInt32) DeleteByIndex(index int) {
	if index < 0 || index >= len(this.s) {
		return
	}
	this.s = append(this.s[0:index], this.s[index+1:]...)
}

func (this *SliceInt32) Len() int {
	return len(this.s)
}

func (this *SliceInt32) Cap() int {
	return cap(this.s)
}

func (this *SliceInt32) Data() []int32 {
	return this.s
}

func (this *SliceInt32) Range(f func(index int,elem int32) bool) {
	for index,elem := range this.s {
		if !f(index,elem) {
			break
		}
	}
}

func (this *SliceInt32) ToString() string {
	builder := strings.Builder{}
	for i := 0; i < len(this.s); i++ {
		builder.WriteString(strconv.Itoa(int(this.s[i])))
		if i != len(this.s) - 1 {
			builder.WriteString(",")
		}
	}
	return builder.String()
}

func (this *SliceInt32) FromString(str string) {
	strs := strings.Split(str,",")
	this.s = make([]int32, len(strs), len(strs))
	for i,s := range strs {
		this.s[i] = int32(util.Atoi(s))
	}
}