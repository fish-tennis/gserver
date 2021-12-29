package mongodb

import (
	"context"
	"github.com/fish-tennis/gserver/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// AccountDb和PlayerDb的mongo实现
type MongoDb struct {
	mongoClient    *mongo.Client
	mongoDatabase  *mongo.Database

	uri            string
	dbName         string
	collectionName string

	// 账号id列名(unique index)
	colAccountId   string
	// 账号名列名(unique index)
	colAccountName string

	// 玩家id列名(unique index)
	colPlayerId    string
	// 玩家名列名(unique index)
	colPlayerName  string
	// 玩家区服id列名
	colRegionId    string
}

func NewMongoDb(uri,dbName,collectionName string) *MongoDb {
	return &MongoDb{
		uri:            uri,
		dbName:         dbName,
		collectionName: collectionName,
	}
}

func (this *MongoDb) SetAccountColumnNames(colAccountId, colAccountName string) {
	this.colAccountId = colAccountId
	this.colAccountName = colAccountName
}

func (this *MongoDb) SetPlayerColumnNames(colPlayerId, colPlayerName, colRegionId string) {
	this.colPlayerId = colPlayerId
	this.colPlayerName = colPlayerName
	this.colRegionId = colRegionId
}

func (this *MongoDb) Connect() bool {
	client,err := mongo.Connect(context.TODO(), options.Client().ApplyURI(this.uri))
	if err != nil {
		return false
	}
	// Ping the primary
	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
		logger.Error(err.Error())
		return false
	}
	this.mongoClient = client
	this.mongoDatabase = this.mongoClient.Database(this.dbName)
	col := this.mongoDatabase.Collection(this.collectionName)
	columnNames := []string{this.colAccountId, this.colAccountName, this.colPlayerId, this.colPlayerName}
	var indexModels []mongo.IndexModel
	for _,columnName := range columnNames {
		if columnName != "" {
			indexModel := mongo.IndexModel{
				Keys: bson.D{
					{columnName, 1},
				},
				Options: options.Index().SetUnique(true),
			}
			indexModels = append(indexModels, indexModel)
		}
	}
	if len(indexModels) > 0 {
		indexNames,indexErr := col.Indexes().CreateMany(context.TODO(), indexModels)
		if indexErr != nil {
			logger.Error("create index err:%v", indexErr)
		} else {
			logger.Info("mongo index:%v", indexNames)
		}
	}
	logger.Info("mongo Connected")
	return true
}

func (this *MongoDb) Disconnect() {
	if this.mongoClient == nil {
		return
	}
	if err := this.mongoClient.Disconnect(context.TODO()); err != nil {
		logger.Error(err.Error())
	}
	logger.Info("mongo Disconnected")
}

// 根据账号名查找账号数据
func (this *MongoDb) FindAccount(accountName string, data interface{}) (bool,error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.TODO(), bson.D{{this.colAccountName,accountName}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(data)
	if err != nil {
		return false, err
	}
	return true,nil
}

// 新建账号(insert)
func (this *MongoDb) InsertAccount(accountData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.InsertOne(context.TODO(), accountData)
	return err
}

// 保存账号数据(update account by accountId)
func (this *MongoDb) SaveAccount(accountId int64, accountData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.UpdateOne(context.TODO(), bson.D{{this.colAccountId, accountId}}, accountData)
	return err
}

// 保存账号字段(update account.fieldName by accountId)
func (this *MongoDb) SaveAccountField(accountId int64, fieldName string, fieldData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.UpdateOne(context.TODO(), bson.D{{this.colAccountId, accountId}},
		bson.D{{"$set", bson.D{{fieldName,fieldData}}}})
	return err
}


// 根据账号id查找玩家数据
// 适用于一个账号在一个区服只有一个玩家角色的游戏
func (this *MongoDb) FindPlayerByAccountId(accountId int64, regionId int32, playerData interface{}) (bool,error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.TODO(), bson.D{{this.colAccountId,accountId},{this.colRegionId,regionId}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(playerData)
	if err != nil {
		return false, err
	}
	return true,nil
}

func (this *MongoDb) InsertPlayer(playerId int64, playerData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, insertErr := col.InsertOne(context.TODO(), playerData)
	if insertErr != nil {
		return insertErr
	}
	return nil
}

// 保存玩家数据(update player by playerId)
func (this *MongoDb) SavePlayer(playerId int64, playerData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.UpdateOne(context.TODO(), bson.D{{this.colPlayerId, playerId}}, playerData)
	return err
}

// 保存玩家组件(update by int playerId.componentName)
func (this *MongoDb) SaveComponent(playerId int64, componentName string, componentData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, updateErr := col.UpdateOne(context.TODO(), bson.D{{this.colPlayerId, playerId}},
		bson.D{{"$set", bson.D{{componentName,componentData}}}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}
