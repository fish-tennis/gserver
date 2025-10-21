package game

import (
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"log/slog"
	"reflect"
)

// 进度事件映射
type ProgressEventMapping struct {
	player *Player
	// 关联事件名的进度对象列表
	mapping map[string][]internal.CfgData // key:eventName
}

func (p *ProgressEventMapping) getKey(progressCfg *pb.ProgressCfg) string {
	return progressCfg.GetEvent()
}

func (p *ProgressEventMapping) AddProgress(progressCfg *pb.ProgressCfg, progress internal.CfgData) {
	if progressCfg == nil {
		return
	}
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
	slog.Debug("AddProgress", "key", key, "cfgId", progress.GetCfgId())
}

func (p *ProgressEventMapping) RemoveProgress(progressCfg *pb.ProgressCfg, cfgId int32) {
	if progressCfg == nil {
		return
	}
	key := p.getKey(progressCfg)
	progressSlice, _ := p.mapping[key]
	for i := 0; i < len(progressSlice); i++ {
		if progressSlice[i].GetCfgId() == cfgId {
			progressSlice = append(progressSlice[:i], progressSlice[i+1:]...)
			p.mapping[key] = progressSlice
			slog.Debug("RemoveProgress", "event", key, "cfgId", cfgId)
			break
		}
	}
}

func (p *ProgressEventMapping) UpdateProgress(event any, progress internal.CfgData) bool {
	switch v := progress.(type) {
	case *pb.QuestData:
		questCfg := cfg.Quests.GetCfg(v.GetCfgId())
		if internal.UpdateProgress(p.player, v, event, questCfg.Progress) {
			p.player.GetQuest().Quests.SetDirty(v.GetCfgId(), true)
			p.player.Send(&pb.QuestUpdate{
				QuestCfgId: v.GetCfgId(),
				Data:       util.CloneMessage(v),
			})
			slog.Debug("QuestProgressUpdate", "name", questCfg.GetName(), "questId", v.GetCfgId(), "progress", v.GetProgress(), "activityId", v.GetActivityId())
			return true
		}
	default:
		slog.Error("CheckProgressErr", "progress", progress)
	}
	return false
}

// 事件分发后,检查进度更新
func (p *ProgressEventMapping) OnTriggerEvent(event any) {
	// 属性值更新
	key := reflect.TypeOf(event).Elem().Name()
	if progressSlice, ok := p.mapping[key]; ok { //快速查询该事件对应的进度对象
		for _, progress := range progressSlice {
			p.UpdateProgress(event, progress)
		}
	}
}
