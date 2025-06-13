package mongo

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoConfig struct {
	URI               string
	Database          string
	ConnectTimeout    time.Duration
	MaxPoolSize       uint64
	MinPoolSize       uint64
	MaxConnIdleTime   time.Duration
	MaxConnecting     uint64 // Controls concurrency
	EnableCompression bool
	RetryWrites       bool
	RetryReads        bool
}

var (
	Mongo_Client   *mongo.Client
	Mongo_Database *mongo.Database
)

func init() {
	config := loadMongoConfig()

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)

	opts := options.Client().
		ApplyURI(config.URI).
		SetServerAPIOptions(serverAPI).
		SetMaxPoolSize(config.MaxPoolSize).
		SetMinPoolSize(config.MinPoolSize).
		SetMaxConnIdleTime(config.MaxConnIdleTime).
		SetMaxConnecting(config.MaxConnecting).
		SetCompressors([]string{"zstd", "snappy", "zlib"}).
		SetRetryWrites(config.RetryWrites).
		SetRetryReads(config.RetryReads)

	if config.EnableCompression {
		opts.SetCompressors([]string{"zstd", "snappy", "zlib"})
	}

	var err error
	fmt.Println(config.URI)

	Mongo_Client, err = mongo.Connect(opts)
	if err != nil {
		log.Fatalf("Fatal error connecting to MongoDB: %s", err)
	}

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if err := Mongo_Client.Ping(pingCtx, nil); err != nil {
		log.Printf("Warning: Could not verify MongoDB connection: %s", err)
	} else {
		log.Println("Successfully connected to MongoDB")
	}

	Mongo_Database = Mongo_Client.Database(config.Database)

	log.Printf("MongoDB initialized - Database: %s, Max Pool Size: %d",
		config.Database, config.MaxPoolSize)
}

func DisconnectMongo() {
	if Mongo_Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := Mongo_Client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %s", err)
		} else {
			log.Println("Successfully disconnected from MongoDB")
		}
	}
}

func loadMongoConfig() *MongoConfig {
	// Set default values
	maxPoolSize := uint64(100)
	minPoolSize := uint64(10)
	connectTimeout := 30 * time.Second
	maxIdleTime := 60 * time.Second
	maxConnecting := uint64(2)
	enableCompression := true
	retryWrites := true
	retryReads := true

	return &MongoConfig{
		URI:               os.Getenv("MONGO_URI"),
		Database:          os.Getenv("AUTH_SERVICE_MONGO_DB"),
		ConnectTimeout:    connectTimeout,
		MaxPoolSize:       maxPoolSize,
		MinPoolSize:       minPoolSize,
		MaxConnIdleTime:   maxIdleTime,
		MaxConnecting:     maxConnecting,
		EnableCompression: enableCompression,
		RetryWrites:       retryWrites,
		RetryReads:        retryReads,
	}
}

func GetCollection(name string) *mongo.Collection {
	return Mongo_Database.Collection(name)
}

func IsConnected() bool {
	if Mongo_Client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Mongo_Client.Ping(ctx, nil)
	return err == nil
}
