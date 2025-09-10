package cfg

import "github.com/fish-tennis/gserver/pb"

func ConvertToDelElemArg(itemNum *pb.ItemNum) *pb.DelElemArg {
	return &pb.DelElemArg{
		CfgId: itemNum.CfgId,
		Num:   itemNum.Num,
	}
}

func ConvertToDelElemArgs(itemNums []*pb.ItemNum) []*pb.DelElemArg {
	var s []*pb.DelElemArg
	for _, itemNum := range itemNums {
		s = append(s, ConvertToDelElemArg(itemNum))
	}
	return s
}
