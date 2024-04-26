package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"strings"
)

const (
	// 组件名
	ComponentNamePendingMessages = "PendingMessages"
)

// 利用go的init进行组件的自动注册
func init() {
	RegisterPlayerComponentCtor(ComponentNamePendingMessages, 100, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &PendingMessages{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   ComponentNamePendingMessages,
			},
			Messages: make(map[int64]*pb.RoutePlayerMessage),
		}
		gentity.LoadData(component, playerData.GetPendingMessages())
		return component
	})
}

// 待处理消息
// 该组件不会在玩家下线时保存数据库,也不保存缓存
type PendingMessages struct {
	BasePlayerComponent
	Messages map[int64]*pb.RoutePlayerMessage `db:""`
}

func NewPendingMessages(player *Player) *PendingMessages {
	component := &PendingMessages{
		BasePlayerComponent: BasePlayerComponent{
			player: player,
			name:   "PendingMessages",
		},
		Messages: make(map[int64]*pb.RoutePlayerMessage),
	}
	return component
}

// 事件接口
func (this *PendingMessages) OnEvent(event interface{}) {
	switch event.(type) {
	case *internal.EventPlayerEntryGame:
		for _, req := range this.Messages {
			message, err := req.PacketData.UnmarshalNew()
			if err != nil {
				logger.Error("UnmarshalNew %v err:%v", req.ToPlayerId, err)
				continue
			}
			err = req.PacketData.UnmarshalTo(message)
			if err != nil {
				logger.Error("UnmarshalTo %v err:%v", req.ToPlayerId, err)
				continue
			}
			this.GetPlayer().processMessage(gnet.NewProtoPacket(gnet.PacketCommand(req.PacketCommand), message))
			// 处理过的消息,单独删除数据
			db.GetPlayerDb().DeleteComponentField(this.GetPlayerId(), strings.ToLower(this.GetName()), util.Itoa(req.MessageId))
			logger.Debug("%v delete pending message:%v", this.GetPlayerId(), req.MessageId)
		}
	}
}
