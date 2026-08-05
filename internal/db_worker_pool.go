package internal

import (
	"hash/fnv"
	"log/slog"
	"sync"
)

const (
	// DbWorkerCount DB操作协程池的worker数量
	// 这些worker主要执行MongoDB查询(I/O密集型),大部分时间在等待DB返回
	// 8个worker足以应对开服冲峰场景,同时不会对MongoDB集群造成过大压力
	DbWorkerCount = 8
	// DbWorkerQueueCap 每个worker的任务队列容量
	DbWorkerQueueCap = 64
	// 如果协程池未初始化,是否丢弃任务
	DropTaskIfPoolNotInited = true
)

// dbWorkerPool DB操作协程池
// 按 hashKey(accountId 或 AccountName hash) 分配worker
// 保证同一标识的请求串行执行,避免并发竞态
var dbWorkerPool *DbWorkerPool

// DbWorkerPool 按 hashKey 分配的固定数量worker协程池
type DbWorkerPool struct {
	workers []chan func()
	wg      sync.WaitGroup
}

// NewDbWorkerPool 创建并启动协程池
func NewDbWorkerPool() *DbWorkerPool {
	pool := &DbWorkerPool{
		workers: make([]chan func(), DbWorkerCount),
	}
	for i := 0; i < DbWorkerCount; i++ {
		pool.workers[i] = make(chan func(), DbWorkerQueueCap)
		pool.wg.Add(1)
		go pool.workerLoop(i)
	}
	slog.Info("DbWorkerPool started", "workerCount", DbWorkerCount, "queueCap", DbWorkerQueueCap)
	return pool
}

// workerLoop worker的主循环,从任务队列消费并执行
// 内置recover:单个任务panic不会杀死worker,后续任务继续执行
func (p *DbWorkerPool) workerLoop(idx int) {
	defer p.wg.Done()
	for f := range p.workers[idx] {
		func() {
			defer func() {
				if err := recover(); err != nil {
					slog.Error("DbWorker panic", "worker", idx, "error", err)
					SendAlert(err)
				}
			}()
			f()
		}()
	}
}

// Submit 按 hashKey 选择worker,非阻塞投递任务
// hashKey 通常是 accountId,保证同一账号的请求串行执行
// 返回false表示对应worker队列已满,调用方应返回TryLater给客户端
func (p *DbWorkerPool) Submit(hashKey int64, f func()) bool {
	workerIdx := int(uint64(hashKey) % DbWorkerCount)
	select {
	case p.workers[workerIdx] <- f:
		return true
	default:
		slog.Warn("DbWorker queue full", "worker", workerIdx, "hashKey", hashKey)
		return false
	}
}

// SubmitByName 按字符串标识(如AccountName) hash 选择worker
// 用于没有 int64 ID 的场景(如注册时还没有 accountId)
func (p *DbWorkerPool) SubmitByName(name string, f func()) bool {
	h := fnv.New64a()
	h.Write([]byte(name))
	return p.Submit(int64(h.Sum64()), f)
}

// Shutdown 关闭协程池,等待所有worker退出
func (p *DbWorkerPool) Shutdown() {
	for i := range p.workers {
		close(p.workers[i])
	}
	p.wg.Wait()
	slog.Info("DbWorkerPool shutdown")
}

// InitDbWorkerPool 初始化全局协程池
func InitDbWorkerPool() {
	dbWorkerPool = NewDbWorkerPool()
}

// ShutdownDbWorkerPool 关闭全局协程池
func ShutdownDbWorkerPool() {
	if dbWorkerPool != nil {
		dbWorkerPool.Shutdown()
	}
}

// SubmitDbTask 按 int64 hashKey 投递DB任务到协程池,队列满时返回false
func SubmitDbTask(hashKey int64, f func()) bool {
	if dbWorkerPool == nil {
		if DropTaskIfPoolNotInited {
			slog.Error("DbWorkerPool not initialized, task dropped", "hashKey", hashKey)
			return false
		} else {
			// // 协程池未初始化,直接同步执行(降级)
			f()
		}
		return true
	}
	return dbWorkerPool.Submit(hashKey, f)
}

// SubmitDbTaskByName 按字符串标识投递DB任务到协程池,队列满时返回false
func SubmitDbTaskByName(name string, f func()) bool {
	if dbWorkerPool == nil {
		if DropTaskIfPoolNotInited {
			slog.Error("DbWorkerPool not initialized, task dropped", "name", name)
			return false
		} else {
			// // 协程池未初始化,直接同步执行(降级)
			f()
		}
		return true
	}
	return dbWorkerPool.SubmitByName(name, f)
}
