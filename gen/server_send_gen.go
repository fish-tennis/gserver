// Code generated by proto_code_gen. DO NOT EDIT.
// https://github.com/fish-tennis/proto_code_gen
// 对应的proto规范:
//  xxx.proto
//  enum CmdXxx {
//    Cmd_Xyz = 1102; // 格式: Cmd_MessageName
//  }
//
//  // @Server
//  message Xyz {
//    int32 abc = 1;
//  }
package gen

import (
 . "github.com/fish-tennis/gnet"
 "github.com/fish-tennis/gserver/pb"
 . "github.com/fish-tennis/gserver/internal"
)

// 踢玩家下线
// @Server表示是服务器用的普通消息,工具会生成相应的辅助代码
func SendKickPlayer(serverId int32, message *pb.KickPlayer) bool {
   return GetServerList().Send(serverId, PacketCommand(pb.CmdInner_Cmd_KickPlayer), message)
}

