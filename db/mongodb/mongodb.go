package mongodb

import (
	"context"
	"github.com/fish-tennis/gnet"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoDb struct {
	mongoClient    *mongo.Client
	mongoDatabase  *mongo.Database
	uri            string
	dbName         string
	collectionName string
	// 可以同时有string key和int key,典型的如账号表和玩家表,名字和id都是unique key
	// int key的列名
	intKeyName     string
	// string key的列名
	stringKeyName  string
	// 二进制值的列名
	valueName      string
}

func NewMongoDb(uri,dbName,collectionName,intKeyName,stringKeyName,valueName string) *MongoDb {
	return &MongoDb{
		uri:            uri,
		dbName:         dbName,
		collectionName: collectionName,
		intKeyName:     intKeyName,
		stringKeyName:  stringKeyName,
		valueName:      valueName,
	}
}

func (this *MongoDb) Connect() bool {
	client,err := mongo.Connect(context.TODO(), options.Client().ApplyURI(this.uri))
	if err != nil {
		return false
	}
	// Ping the primary
	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
		gnet.LogError(err.Error())
		return false
	}
	this.mongoClient = client
	this.mongoDatabase = this.mongoClient.Database(this.dbName)
	col := this.mongoDatabase.Collection(this.collectionName)
	if this.stringKeyName != "" {
		indexModel := mongo.IndexModel{
			Keys: bson.D{
				{this.stringKeyName, 1},
			},
			Options: options.Index().SetUnique(true),
		}
		indexName,indexErr := col.Indexes().CreateOne(context.TODO(), indexModel)
		if indexErr == nil {
			gnet.LogInfo("mongo stringIndexName:%v", indexName)
		}
	}
	if this.intKeyName != "" {
		indexModel := mongo.IndexModel{
			Keys: bson.D{
				{this.intKeyName, 1},
			},
			Options: options.Index().SetUnique(true),
		}
		indexName,indexErr := col.Indexes().CreateOne(context.TODO(), indexModel)
		if indexErr == nil {
			gnet.LogInfo("mongo intIndexName:%v", indexName)
		}
	}
	gnet.LogInfo("mongo Connected")
	return true
}

func (this *MongoDb) Disconnect() {
	if this.mongoClient == nil {
		return
	}
	if err := this.mongoClient.Disconnect(context.TODO()); err != nil {
		gnet.LogError(err.Error())
	}
	gnet.LogInfo("mongo Disconnected")
}

func (this *MongoDb) FindString(key string, data interface{}) (bool,error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.TODO(), bson.D{{this.stringKeyName,key}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(data)
	if err != nil {
		return false, err
	}
	return true,nil
}

func (this *MongoDb) InsertString(key string, data interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, insertErr := col.InsertOne(context.TODO(),
		data)
	if insertErr != nil {
		return insertErr
	}
	return nil
}

func (this *MongoDb) UpdateString(key string, data interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, updateErr := col.UpdateOne(context.TODO(), bson.D{{this.stringKeyName, key}},
		data)
	if updateErr != nil {
		return updateErr
	}
	return nil
}

func (this *MongoDb) FindInt64(key int64, data interface{}) (bool,error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.TODO(), bson.D{{this.intKeyName,key}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(data)
	if err != nil {
		return false, err
	}
	return true,nil
}

func (this *MongoDb) InsertInt64(key int64, data interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, insertErr := col.InsertOne(context.TODO(), data)
	if insertErr != nil {
		return insertErr
	}
	return nil
}

func (this *MongoDb) UpdateInt64(key int64, data interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, updateErr := col.UpdateOne(context.TODO(), bson.D{{this.intKeyName, key}},
		data)
	if updateErr != nil {
		return updateErr
	}
	return nil
}

func (this *MongoDb) LoadFieldInt64(key int64, fieldName string, fieldData interface{}) (bool,error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	opts := options.FindOne().SetProjection(bson.D{{fieldName,1}})
	result := col.FindOne(context.TODO(), bson.D{{this.intKeyName,key}}, opts)
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(fieldData)
	if err != nil {
		return false, err
	}
	return true,nil
}

func (this *MongoDb) SaveFieldInt64(key int64, fieldName string, fieldData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, updateErr := col.UpdateOne(context.TODO(), bson.D{{this.intKeyName, key}},
		bson.D{{"$set", bson.D{{fieldName,fieldData}}}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}

// 根据账号id查找玩家数据
// 适用于一个账号在一个区服只有一个玩家角色的游戏
func (this *MongoDb) FindPlayerByAccountId(accountId int64, regionId int32, playerData interface{}) (bool,error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.TODO(), bson.D{{"accountid",accountId},{"regionid",regionId}})
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
