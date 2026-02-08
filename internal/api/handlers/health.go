// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/db"
)

// HealthResponse is the response for the health endpoint.
type HealthResponse struct {
	Status   string                 `json:"status"`
	Database map[string]interface{} `json:"database,omitempty"`
}

// Health returns a health check handler.
func Health(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbHealth, err := database.Health(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, HealthResponse{
				Status: "unhealthy",
				Database: map[string]interface{}{
					"status": "unhealthy",
					"error":  err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusOK, HealthResponse{
			Status:   "healthy",
			Database: dbHealth,
		})
	}
}
