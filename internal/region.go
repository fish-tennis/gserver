package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/fish-tennis/gserver/cache"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/pb"
)

// 区服完整数据缓存
// key=regionId, value=完整的 pb.Region 数据(含 Id、Name、Status、CreateTimestamp、UpdateTimestamp)
// 使用读写锁保护,因为GameServer中每个玩家在独立协程中处理消息,
// 多个玩家协程可能同时查询不同区服的数据
var (
	regionCache      = make(map[int32]*pb.Region)
	regionCacheMutex sync.RWMutex
)

// InitRegionCache 启动时从MongoDB加载所有区服数据到内存缓存,并订阅区服变更通知
// 在GameServer.Init与LoginServer.Init中调用(initDb/initCache之后),保证后续查询只读内存不再访问MongoDB;
// 订阅逻辑收敛在本函数内,调用方(GameServer/LoginServer)无需各自维护订阅样板,也避免快照实现重复
// 当Regions文档尚不存在时(如GameServer先于首次初始化启动),db.LoadAllRegions返回空map而非错误,
// 此时以空缓存启动,后续LoginServer初始化或GM新增区服后会通过RefreshRegionCache补齐
func InitRegionCache(ctx context.Context) error {
	regions, err := db.LoadAllRegions()
	if err != nil {
		return err
	}

	regionCacheMutex.Lock()
	regionCache = regions
	regionCacheMutex.Unlock()
	slog.Info("InitRegionCache", "count", len(regions))

	// 订阅Redis Pub/Sub区服变更通知:GM修改区服落库MongoDB后广播,收到后重读MongoDB刷新快照;
	// 替代原服务器间基于TCP消息的刷新通知;订阅断线自动重连,
	// at-most-once丢失风险与补偿方式见cache/region_notify.go
	cache.SubscribeRegionUpdate(ctx, func(regionId int32) {
		slog.Info("region cache refresh notify received", "regionId", regionId)
		RefreshRegionCache(regionId)
	})
	return nil
}

// GetRegion 获取区服完整数据(纯内存读取,区服数据在启动时已由InitRegionCache预加载)
func GetRegion(regionId int32) (*pb.Region, error) {
	regionCacheMutex.RLock()
	defer regionCacheMutex.RUnlock()
	region, ok := regionCache[regionId]
	if !ok {
		return nil, fmt.Errorf("region %d not found in cache", regionId)
	}
	return region, nil
}

// RefreshRegionCache 从MongoDB重新加载指定区服数据并更新内存缓存
// 通过Redis Pub/Sub频道 notify:region_update 通知触发:
// GM后台修改区服落库MongoDB后发布通知,各订阅进程收到后调用本函数重读MongoDB刷新内存快照
// 与InitRegionCache一样统一走db.LoadAllRegions全量加载,保证与db包的单份解析实现一致
func RefreshRegionCache(regionId int32) {
	regions, err := db.LoadAllRegions()
	if err != nil {
		slog.Error("RefreshRegionCache failed", "regionId", regionId, "error", err)
		return
	}

	regionCacheMutex.Lock()
	defer regionCacheMutex.Unlock()
	// 批量更新缓存,保持其他区服数据同步最新
	regionCache = regions
	slog.Info("RefreshRegionCache", "regionId", regionId, "totalRegions", len(regions))
}

// GetAllRegions 获取所有区服列表(纯内存读取,快照的slice视图)
// 供LoginServer组装返回给客户端的区服列表等需要遍历全部区服的场景
func GetAllRegions() []*pb.Region {
	regionCacheMutex.RLock()
	defer regionCacheMutex.RUnlock()
	list := make([]*pb.Region, 0, len(regionCache))
	for _, region := range regionCache {
		list = append(list, region)
	}
	return list
}

// GetAllRegionIds 获取所有区服Id列表
// 直接从内存缓存返回,不再访问MongoDB
func GetAllRegionIds() ([]int32, error) {
	regionCacheMutex.RLock()
	defer regionCacheMutex.RUnlock()
	regionIds := make([]int32, 0, len(regionCache))
	for regionId := range regionCache {
		regionIds = append(regionIds, regionId)
	}
	return regionIds, nil
}
