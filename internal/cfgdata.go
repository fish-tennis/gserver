package internal

// 配置数据提供一个统一的接口,以方便做一些统一的处理
type CfgData interface {
	GetCfgId() int32
}