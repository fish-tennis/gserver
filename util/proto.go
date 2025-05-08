package util

import "google.golang.org/protobuf/proto"

// proto.Clone(e).(*pb.Xxx)
func CloneMessage[E proto.Message](e E) E {
	return proto.Clone(e).(E)
}
