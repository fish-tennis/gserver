package util

import "google.golang.org/protobuf/proto"

func CloneMessage[E proto.Message](e E) E {
	return proto.Clone(e).(E)
}
