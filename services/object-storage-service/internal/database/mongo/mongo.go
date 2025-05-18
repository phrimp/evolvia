package mongo

import (
	"context"
	"log"
	"object-storage-service/internal/config"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var (
	Client   *mongo.Client
	Database *mongo.Database
)

func InitMongoDB(cfg *config.MongoDBConfig) error {
	// Create MongoDB client
	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetMaxPoolSize(cfg.PoolSize)

	var err error
	Client, err = mongo.Connect(clientOptions)
	if err != nil {
		log.Printf("Error connecting to MongoDB: %v", err)
		return err
	}

	// Ping the MongoDB server to verify connection
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := Client.Ping(pingCtx, readpref.Primary()); err != nil {
		log.Printf("Error pinging MongoDB: %v", err)
		return err
	}

	// Set the database
	Database = Client.Database(cfg.Database)
	log.Printf("Successfully connected to MongoDB database: %s", cfg.Database)

	return nil
}

// CloseDB closes the MongoDB connection
func CloseDB() {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := Client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}
}

// GetCollection returns a MongoDB collection
func GetCollection(name string) *mongo.Collection {
	return Database.Collection(name)
}
