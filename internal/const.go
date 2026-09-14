package internal

const (
	// 客户端回调接口函数名前缀
	ClientHandlerMethodNamePrefix = "On"
	// 其他回调接口函数名前缀
	HandlerMethodNamePrefix = "Handle"
	// 事件响应接口函数名前缀
	EventHandlerMethodNamePrefix = "Trigger"
	// 事件分发嵌套层次限制
	SameEventLoopLimit = int32(3)
	// 单次添加唯一物品的最大数量,防止客户端传入超大值导致CPU/内存耗尽
	MaxBatchAddUniqueElemCount = int32(1000)
	
	// DefaultGameServerMaxOnline Game服未配置MaxOnline时参与选服权重计算的默认容量
	// 存量Game服未配置MaxOnline时若按0计算会被误判为满载,导致全部登录被拒,因此必须有默认值兜底
	DefaultGameServerMaxOnline int32 = 10000

	// DefaultServerActiveTimeoutMs 服务器活跃判定阈值(毫秒),超过该时长未上报心跳即判定为不活跃
	// 心跳每秒上报一次,10秒=连续10次心跳丢失
	// 该阈值直接影响登录服对"宕机"服务器的判定与在线记录清理(login_handler粘性分支):
	// 阈值过短(如3秒)时,游戏服GC停顿/Redis慢查询/网络抖动即可造成误判,
	// 把仍在内存中的在线玩家记录清掉,玩家重登后SetNX成功,出现同一玩家在两个
	// 游戏服同时在线、SaveDb相互覆盖的数据丢失事故——数据安全的优先级高于宕机切换速度;
	// 真宕机后的代价:登录服在该时长内仍可能粘性分配到死服,客户端连接失败重试即可自愈
	DefaultServerActiveTimeoutMs int32 = 10 * 1000
)
