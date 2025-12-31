package app

import (
	"log"
	"print-pro-backend/internal/config"
	"print-pro-backend/internal/infrastructure"
)

// setupRedis initializes Redis connection
func setupRedis(cfg *config.Config) (*infrastructure.RedisClient, error) {
	redisClient, err := infrastructure.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return nil, err
	}
	log.Printf("Connected to Redis at %s", cfg.RedisAddr)
	return redisClient, nil
}

