// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// InternalAuth returns a middleware that validates the X-Internal-Secret header.
// If secret is empty, authentication is skipped (for development).
func InternalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth if no secret configured (development mode)
		if secret == "" {
			c.Next()
			return
		}

		providedSecret := c.GetHeader("X-Internal-Secret")
		if providedSecret == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing X-Internal-Secret header",
			})
			return
		}

		if providedSecret != secret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid internal secret",
			})
			return
		}

		c.Next()
	}
}
