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

func (p *ProgressEventMapping) CheckProgress(event any, progress internal.CfgData) {
	switch v := progress.(type) {
	case *pb.QuestData:
		questCfg := cfg.GetQuestCfgMgr().GetQuestCfg(v.GetCfgId())
		if cfg.GetQuestCfgMgr().GetProgressMgr().UpdateProgress(v, event, questCfg.Progress) {
			p.player.GetQuest().Quests.SetDirty(v.GetCfgId(), true)
			p.player.Send(&pb.QuestUpdate{
				QuestCfgId: v.GetCfgId(),
				Data:       util.CloneMessage(v),
			})
			slog.Debug("QuestProgressUpdate", "questId", v.GetCfgId(), "progress", v.GetProgress(), "activityId", v.GetActivityId())
		}
	default:
		slog.Error("CheckProgressErr", "progress", progress)
	}
}

// 事件分发后,检查进度更新
func (p *ProgressEventMapping) OnTriggerEvent(event any) {
	// 属性值更新
	key := reflect.TypeOf(event).Elem().Name()
	if progressSlice, ok := p.mapping[key]; ok {
		for _, progress := range progressSlice {
			p.CheckProgress(event, progress)
		}
	}
}
