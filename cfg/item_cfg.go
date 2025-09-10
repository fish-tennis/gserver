package cfg

import "github.com/fish-tennis/gserver/pb"

func ConvertToAddElemArg(itemNum *pb.ItemNum) *pb.AddElemArg {
	return &pb.AddElemArg{
		CfgId: itemNum.CfgId,
		Num:   itemNum.Num,
	}
}

func ConvertToAddElemArgs(itemNums []*pb.ItemNum) []*pb.AddElemArg {
	var s []*pb.AddElemArg
	for _, itemNum := range itemNums {
		s = append(s, ConvertToAddElemArg(itemNum))
	}
	return s
}

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
