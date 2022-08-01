package pb

// 实现ProgressHolder接口
func (x *QuestData) SetProgress(progress int32) {
	if x != nil {
		x.Progress = progress
	}
}