package game

import (
	"fmt"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"reflect"
)

// 增加活动id,以便于更新进度后,调用Activity.SetDirty
type ActivityQuestDataWrapper struct {
	*pb.ActivityQuestData
	ActivityId int32
}

// 进度事件映射
type ProgressEventMapping struct {
	player *Player
	// 关联事件名的进度slice
	// 如果是通用事件名匹配,则key就是事件名
	// 如果是属性值事件,则key就是property:propertyName
	mapping map[string][]internal.CfgData // key:eventName or property:propertyName
}

func (p *ProgressEventMapping) getKey(progressCfg *pb.ProgressCfg) string {
	if progressCfg.GetType() == int32(pb.ProgressType_ProgressType_PlayerProperty) ||
		progressCfg.GetType() == int32(pb.ProgressType_ProgressType_ActivityProperty) {
		// 属性映射
		property := progressCfg.GetProperties()[PropertyKey]
		if property == "" {
			slog.Error("getKeyErr", "progressCfg", progressCfg)
			return ""
		}
		return fmt.Sprintf("property:%v", property)
	} else {
		// 事件映射
		return fmt.Sprintf("Event%v", progressCfg.GetEvent())
	}
}

func (p *ProgressEventMapping) addProgress(progressCfg *pb.ProgressCfg, progress internal.CfgData) {
	key := p.getKey(progressCfg)
	if key == "" {
		slog.Error("addProgressErr", "progressCfg", progressCfg, "progress", progress)
		return
	}
	if p.mapping == nil {
		p.mapping = make(map[string][]internal.CfgData)
	}
	progressSlice, _ := p.mapping[key]
	progressSlice = append(progressSlice, progress)
	p.mapping[key] = progressSlice
	slog.Debug("AddQuest", "key", key, "questId", progress.GetCfgId())
}

func (p *ProgressEventMapping) removeProgress(progressCfg *pb.ProgressCfg, questId int32) {
	key := p.getKey(progressCfg)
	progressSlice, _ := p.mapping[key]
	for i := 0; i < len(progressSlice); i++ {
		if progressSlice[i].GetCfgId() == questId {
			progressSlice = append(progressSlice[:i], progressSlice[i+1:]...)
			p.mapping[key] = progressSlice
			slog.Debug("removeProgress", "event", key, "questId", questId)
			break
		}
	}
}

func (p *ProgressEventMapping) CheckProgress(event any, progress internal.CfgData) {
	switch v := progress.(type) {
	case *pb.QuestData:
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(v.GetCfgId())
		if cfg.GetQuestCfgMgr().GetProgressMgr().CheckProgress(event, questCfg.Progress, v) {
			p.player.GetQuest().Quests.SetDirty(v.GetCfgId(), true)
			p.player.Send(&pb.QuestUpdate{
				QuestCfgId: v.GetCfgId(),
				Data:       v,
			})
			slog.Debug("QuestProgressUpdate", "questId", v.GetCfgId(), "progress", v.GetProgress())
		}
	case *ActivityQuestDataWrapper:
		activity := p.player.GetActivities().GetActivity(v.ActivityId)
		if activity == nil {
			return
		}
		if activityDefault, ok := activity.(*ActivityDefault); ok {
			questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(v.GetCfgId())
			if cfg.GetQuestCfgMgr().GetProgressMgr().CheckProgress(event, questCfg.Progress, v) {
				activityDefault.SetDirty()
				p.player.Send(&pb.ActivityQuestUpdate{
					ActivityId: v.ActivityId,
					QuestCfgId: v.GetCfgId(),
					Data:       v.ActivityQuestData,
				})
				slog.Debug("ActivityProgressUpdate", "activityId", v.ActivityId,
					"questId", v.GetCfgId(), "progress", v.GetProgress())
			}
		}
	default:
		slog.Error("CheckProgressErr", "progress", progress)
	}
}

// 事件分发后,检查进度更新
func (p *ProgressEventMapping) OnTriggerEvent(event any) {
	var key string
	switch evt := event.(type) {
	case *pb.EventPlayerProperty:
		key = fmt.Sprintf("property:%v", evt.GetPropertyName())
	case *pb.EventActivityProperty:
		key = fmt.Sprintf("property:%v", evt.GetPropertyName())
	default:
		// 通用事件
		key = reflect.TypeOf(event).Elem().Name()
	}
	// 属性值更新
	if progressSlice, ok := p.mapping[key]; ok {
		for _, progress := range progressSlice {
			p.CheckProgress(event, progress)
		}
	}
}
