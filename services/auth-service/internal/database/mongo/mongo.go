package mongo

import (
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoConfig struct {
	URI      string
	Database string
}

var (
	Mongo_Client   *mongo.Client
	Mongo_Database *mongo.Database
)

func init() {
	config := loadMongoConfig()
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(config.URI).SetServerAPIOptions(serverAPI)
	var err error
	Mongo_Client, err = mongo.Connect(opts)
	if err != nil {
		log.Printf("\n Error connecting to Mongo: %s", err)
	}
	var result bson.M
	if err := Mongo_Client.Database("admin").RunCommand(context.TODO(), bson.D{{Key: "ping", Value: 1}}).Decode(&result); err != nil {
		panic(err)
	}
	log.Println("Pinged current deployment. Auth Service successfully connected to MongoDB!")
	Mongo_Database = Mongo_Client.Database(config.Database)
}

func DisconnectMongo() {
	Mongo_Client.Disconnect(context.TODO())
}

func loadMongoConfig() *MongoConfig {
	return &MongoConfig{
		URI:      os.Getenv("MONGO_URI"),
		Database: os.Getenv("AUTH_SERVICE_MONGO_DB"),
	}
}
