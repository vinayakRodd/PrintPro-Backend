package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"print-pro-backend/internal/infrastructure"
)

// CreateHealthCheck creates a health check handler with database connectivity checks
func CreateHealthCheck(redisClient *infrastructure.RedisClient, postgresClient *infrastructure.PostgresClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		healthStatus := map[string]interface{}{
			"status":    "ok",
			"message":   "Server is responding",
			"timestamp": time.Now().Format(time.RFC3339),
		}

		// Check Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		redisStatus := "connected"
		if err := redisClient.GetClient().Ping(ctx).Err(); err != nil {
			redisStatus = "disconnected"
			healthStatus["status"] = "degraded"
		}
		healthStatus["redis"] = map[string]interface{}{
			"status": redisStatus,
		}

		// Check PostgreSQL connection
		postgresStatus := "connected"
		if err := postgresClient.Ping(ctx); err != nil {
			postgresStatus = "disconnected"
			healthStatus["status"] = "degraded"
		}
		healthStatus["postgres"] = map[string]interface{}{
			"status": postgresStatus,
		}

		// Set HTTP status code
		statusCode := http.StatusOK
		if healthStatus["status"] == "degraded" {
			statusCode = http.StatusServiceUnavailable
		}

		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(healthStatus)
	}
}

