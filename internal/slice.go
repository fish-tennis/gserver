package internal

import (
	"github.com/fish-tennis/gserver/util"
	"strconv"
	"strings"
)

// SliceInt32提供了一个不变的引用地址,把slice的操作封装在内部
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

func (this *SliceInt32) Contains(v int32) bool {
	for _,elem := range this.s {
		if elem == v {
			return true
		}
	}
	return false
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
	// TODO: use json
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
	// TODO: use json
	strs := strings.Split(str,",")
	this.s = make([]int32, len(strs), len(strs))
	for i,s := range strs {
		this.s[i] = int32(util.Atoi(s))
	}
}

// SliceInt64
// SliceString
// SliceProto