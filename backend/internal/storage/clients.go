package storage

import (
	"context"

	"github.com/ai-prism/backend/internal/config"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func OpenMySQL(cfg config.MySQLConfig) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, nil
	}
	return gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
}

func OpenVectorPostgres(cfg config.VectorDBConfig) (*gorm.DB, error) {
	if cfg.PostgresDSN == "" {
		return nil, nil
	}
	return gorm.Open(postgres.Open(cfg.PostgresDSN), &gorm.Config{})
}

func OpenRedis(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

func OpenMongo(ctx context.Context, cfg config.MongoConfig) (*mongo.Client, error) {
	if cfg.URI == "" {
		return nil, nil
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return nil, err
	}
	return client, nil
}
