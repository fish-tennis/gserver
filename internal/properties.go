package internal

import "github.com/fish-tennis/gserver/pb"

type PropertyInt32 interface {
	GetPropertyInt32(propertyName string, conditionCfg *pb.ConditionCfg) int32
}
