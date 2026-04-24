package middleware

import (
	"net/http"
	"os"
	"strings"

	"absensholat-api/utils"

	"github.com/gin-gonic/gin"
)

// isProduction checks if the application is running in production mode
func isProduction() bool {
	return os.Getenv("ENVIRONMENT") == "production"
}

func AuthMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response := gin.H{"message": "Authorization header required"}
			if !isProduction() {
				response["debug"] = "No Authorization header found"
			}
			c.JSON(http.StatusUnauthorized, response)
			c.Abort()
			return
		}

		var tokenString string
		parts := strings.Split(authHeader, " ")

		// Handle both formats: "Bearer <token>" and just "<token>"
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString = parts[1]
		} else if len(parts) == 1 {
			// Accept token without Bearer prefix for flexibility
			tokenString = parts[0]
		} else {
			response := gin.H{"message": "Invalid authorization format"}
			if !isProduction() {
				response["debug"] = "Expected 'Bearer <token>' or just '<token>'"
			}
			c.JSON(http.StatusUnauthorized, response)
			c.Abort()
			return
		}

		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			response := gin.H{"message": "Invalid or expired token"}
			if !isProduction() {
				response["debug"] = err.Error()
			}
			c.JSON(http.StatusUnauthorized, response)
			c.Abort()
			return
		}

		// Check if the user's role is among the allowed roles
		if len(allowedRoles) > 0 {
			roleFound := false
			for _, allowedRole := range allowedRoles {
				// Special handling: wali_kelas requires guru role (additional checks in handlers)
				if allowedRole == "wali_kelas" && claims.Role == "guru" {
					roleFound = true
					break
				} else if claims.Role == allowedRole {
					roleFound = true
					break
				}
			}
			if !roleFound {
				response := gin.H{"message": "Access denied: Insufficient permissions"}
				if !isProduction() {
					response["debug"] = "Role '" + claims.Role + "' not authorized"
				}
				c.JSON(http.StatusForbidden, response)
				c.Abort()
				return
			}
		}

		c.Set("nis", claims.NIS)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role) // Set role in context as well
		c.Set("name", claims.Name) // Set name in context
		c.Set("nip", claims.NIP)   // Set NIP in context
		c.Next()
	}
}
