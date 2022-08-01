package testclient

import (
	"context"
	"flag"
	"fmt"
	. "github.com/fish-tennis/gnet"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"sync"
)

var (
	_ Server = (*TestClient)(nil)
	_testClient *TestClient
)

// 客户端测试,也作为一个Server启动
// 可以管理多个模拟的客户端
type TestClient struct {
	ctx context.Context
	wg sync.WaitGroup

	clientCodec *ProtoCodec
	clientHandler *MockClientHandler

	serverAddr string
	mockClientAccountPrefix string
	mockClientNum int
	mockClientBeginId int

	mockClients map[string]*MockClient
	mockClientsMutex sync.RWMutex
}

func (this *TestClient) GetServerId() int32 {
	return 0
}

func (this *TestClient) GetContext() context.Context {
	return this.ctx
}

func (this *TestClient) GetWaitGroup() *sync.WaitGroup {
	return &this.wg
}

func (this *TestClient) parseCmdArgs() {
	flag.StringVar(&this.serverAddr, "server", "127.0.0.1:10002", "server's ip:port")
	flag.IntVar(&this.mockClientNum, "num", 1, "num of mock client")
	flag.IntVar(&this.mockClientBeginId, "begin", 1, "begin id of mock client")
	flag.StringVar(&this.mockClientAccountPrefix, "prefix", "mock", "prefix of mock client's accountName")
	flag.Parse()
	logger.Info("server:%v num:%v prefix:%v beginId:%v", this.serverAddr, this.mockClientNum,
		this.mockClientAccountPrefix, this.mockClientBeginId)
}

func (this *TestClient) Init(ctx context.Context, configFile string) bool {
	this.parseCmdArgs()
	_testClient = this
	this.ctx = ctx
	this.mockClients = make(map[string]*MockClient)

	this.clientCodec = NewProtoCodec(nil)
	this.clientHandler = NewMockClientHandler(this.clientCodec)
	this.clientHandler.autoRegister()
	this.clientHandler.SetOnDisconnectedFunc(func(connection Connection) {
		accountName := connection.GetTag().(string)
		mockClient := _testClient.getMockClientByAccountName(accountName)
		if mockClient == nil {
			return
		}
		_testClient.removeMockClient(accountName)
		logger.Debug("client disconnect %v", accountName)
	})

	for i := 0; i < this.mockClientNum; i++ {
		accountName := fmt.Sprintf("%v%v",this.mockClientAccountPrefix,i+1)
		client := newMockClient(accountName)
		this.addMockClient(client)
		client.start()
	}
	return true
}

func (this *TestClient) Run(ctx context.Context) {
}

func (this *TestClient) OnUpdate(ctx context.Context, updateCount int64) {
}

func (this *TestClient) Exit() {
	this.mockClientsMutex.RLock()
	defer this.mockClientsMutex.RUnlock()
	for _,mockClient := range this.mockClients {
		if mockClient.conn != nil {
			mockClient.conn.Close()
		}
	}
}

//func (this *TestClient) registerClientHandler(handler ConnectionHandler) {
//	handler.RegisterHeartBeat(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
//		return &pb.HeartBeatReq{
//			Timestamp: uint64(util.GetCurrentMS()),
//		}
//	})
//}

func (this *TestClient) getMockClientByAccountName(accountName string) *MockClient {
	this.mockClientsMutex.RLock()
	defer this.mockClientsMutex.RUnlock()
	return this.mockClients[accountName]
}

func (this *TestClient) addMockClient(client *MockClient) {
	this.mockClientsMutex.Lock()
	defer this.mockClientsMutex.Unlock()
	this.mockClients[client.accountName] = client
}

func (this *TestClient) removeMockClient(accountName string) {
	this.mockClientsMutex.Lock()
	defer this.mockClientsMutex.Unlock()
	delete(this.mockClients, accountName)
}

func (this *TestClient) OnInputCmd(cmd string) {
	this.mockClientsMutex.RLock()
	defer this.mockClientsMutex.RUnlock()
	for _,mockClient := range this.mockClients {
		mockClient.OnInputCmd(cmd)
	}
}