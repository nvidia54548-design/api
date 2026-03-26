package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CronMiddleware verifies requests from Vercel Cron jobs using CRON_SECRET
func CronMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		expectedSecret := os.Getenv("CRON_SECRET")

		// If CRON_SECRET is set, we must validate it
		if expectedSecret != "" {
			var tokenString string
			parts := strings.Split(authHeader, " ")

			// Extract token from "Bearer <token>" or just "<token>"
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			} else if len(parts) == 1 {
				tokenString = parts[0]
			}

			if tokenString != expectedSecret {
				response := gin.H{"message": "Unauthorized cron trigger"}
				if !isProduction() {
					response["debug"] = "Invalid CRON_SECRET"
				}
				c.JSON(http.StatusUnauthorized, response)
				c.Abort()
				return
			}
		} else if isProduction() {
			// In production, CRON_SECRET MUST be set for security
			c.JSON(http.StatusInternalServerError, gin.H{"message": "CRON_SECRET environment variable is not configured"})
			c.Abort()
			return
		}

		c.Next()
	}
}
