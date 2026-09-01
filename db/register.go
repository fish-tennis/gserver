package db

import (
	"github.com/fish-tennis/gentity"
)

// 集合注册统一入口
//
// 设计目的:把每个集合的"分片策略+唯一键"固化在db包这一处,
// 各服务器(gameserver/loginserver)的initDb只调用这里的函数组合,
// 避免不同进程对同一集合传入不同的ShardKeyType导致元数据互相覆盖。

// RegisterPlayerDb 注册player集合(全项目唯一分片集合)
//
// 分片键_id=playerId(ShardKeyHashed):
// 存档写/玩家加载(框架内建按_id操作)单shard直达;账号维度查询靠{AccountId,RegionId}
// 复合索引消化扇出代价(见EnsurePlayerAccountRegionIndex)。
// 为什么player适合hashed分片键:playerId由KV自增分配,值单调递增,ranged分片会
// 让所有新写入持续打在最后一个chunk(hotspot),hashed彻底打散。
func RegisterPlayerDb(mongoDb *gentity.MongoDb) gentity.PlayerDb {
	return mongoDb.RegisterPlayerDb(PlayerDbName, gentity.ShardKeyHashed, UniqueIdName, PlayerAccountId, PlayerRegionId)
}

// RegisterAccountDb 注册account集合(不分片)
//
// _id=账号名(uniqueId=Name):账号名唯一性由不分片集合的_id主键在数据库层全局原子保证。
// Connect时会自动为Name建唯一索引(双保险,可拦截"_id与Name写得不一致"的代码错误);
// 业务账号Id(仍由KV自增分配)需手动建普通索引(见loginserver.initDb)。
func RegisterAccountDb(mongoDb *gentity.MongoDb) gentity.EntityDb {
	return mongoDb.RegisterEntityDb(AccountDbName, gentity.ShardKeyNone, AccountName)
}

// RegisterGuildDb 注册guild集合(不分片)
//
// _id=guildId:公会名唯一性由游戏进程建会流程保证(insert失败回滚,见game/guild.go)。
func RegisterGuildDb(mongoDb *gentity.MongoDb) gentity.EntityDb {
	return mongoDb.RegisterEntityDb(GuildDbName, gentity.ShardKeyNone, UniqueIdName)
}

// RegisterGlobalEntityDb 注册global集合的EntityDb形态(不分片)
//
// global集合兼具两种用途:KvDb(自增计数器,如AccountId/GuildId)
// 与EntityDb(GlobalEntity/Regions等全局文档),需要同时调用RegisterGlobalKvDb。
func RegisterGlobalEntityDb(mongoDb *gentity.MongoDb) gentity.EntityDb {
	return mongoDb.RegisterEntityDb(GlobalDbName, gentity.ShardKeyNone, GlobalDbKeyName)
}

// RegisterGlobalKvDb 注册global集合的KvDb形态(不分片)
//
// Connect时框架自动为Key建唯一索引——这是KV计数器的防重号防线:
// KvDb.Inc是FindOneAndUpdate+upsert,无唯一索引时upsert并发竞态(两个进程首次对
// 同一key自增)可能插入两条同Key文档,各自递增返回重复值,导致playerId/guildId/
// accountId等ID分配器重号(ID撞号级别的数据错乱);唯一索引存在时,MongoDB对
// upsert的duplicate key错误自动重试为update,竞态闭合。
func RegisterGlobalKvDb(mongoDb *gentity.MongoDb) gentity.KvDb {
	return mongoDb.RegisterKvDb(GlobalDbName, gentity.ShardKeyNone, GlobalDbKeyName, GlobalDbValueName)
}

// RegisterBanDb 注册封禁记录集合(不分片)
//
// _id={accountId}_{regionId}复合键:同一账号区服维度的封禁记录唯一。
func RegisterBanDb(mongoDb *gentity.MongoDb) gentity.EntityDb {
	return mongoDb.RegisterEntityDb(BanDbName, gentity.ShardKeyNone, UniqueIdName)
}
// RegisterPlayerAccountDb 注册账号区服注册表(不分片)
//
// _id={accountId}_{regionId}(业务复合键):建角防重的原子抢占集合,
// 唯一性由不分片集合的_id主键全局保证(设计说明见player_account.go)。
// 只有GameServer处理建角,其余进程(login)不访问该集合,无需注册。
func RegisterPlayerAccountDb(mongoDb *gentity.MongoDb) gentity.EntityDb {
	return mongoDb.RegisterEntityDb(PlayerAccountDbName, gentity.ShardKeyNone, UniqueIdName)
}
