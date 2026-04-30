package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// HTTPSEnforcement middleware redirects HTTP requests to HTTPS in production
func HTTPSEnforcement() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip in development
		if os.Getenv("ENVIRONMENT") != "production" {
			c.Next()
			return
		}

		// Check various headers that indicate HTTPS
		// X-Forwarded-Proto is set by most reverse proxies/load balancers
		proto := c.GetHeader("X-Forwarded-Proto")
		if proto == "" {
			proto = c.GetHeader("X-Forwarded-Protocol")
		}
		if proto == "" && c.GetHeader("X-Forwarded-Ssl") == "on" {
			proto = "https"
		}

		// If behind a proxy and not HTTPS, redirect
		if proto != "" && proto != "https" {
			// Build HTTPS URL
			host := c.Request.Host
			path := c.Request.URL.RequestURI()
			httpsURL := "https://" + host + path

			c.Redirect(http.StatusMovedPermanently, httpsURL)
			c.Abort()
			return
		}

		// Set HSTS header for HTTPS connections
		if proto == "https" || c.Request.TLS != nil {
			// Strict-Transport-Security: enforce HTTPS for 1 year, include subdomains
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		c.Next()
	}
}

// SecurityHeaders adds security headers to all responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS filter
		c.Header("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy
		// Keep API responses locked down by default, but allow Scalar assets on /docs.
		c.Header("Content-Security-Policy", contentSecurityPolicy(c.Request.URL.Path))

		// Prevent caching of sensitive data
		if isSensitivePath(c.Request.URL.Path) {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		c.Next()
	}
}

func contentSecurityPolicy(path string) string {
	if strings.HasPrefix(path, "/docs") {
		return strings.Join([]string{
			"default-src 'self'",
			"script-src 'self' https://cdn.jsdelivr.net",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: https:",
			"font-src 'self' data: https:",
			"connect-src 'self' https://api.scalar.com https://cdn.jsdelivr.net",
			"object-src 'none'",
			"base-uri 'none'",
			"frame-ancestors 'none'",
		}, "; ")
	}

	return "default-src 'none'; frame-ancestors 'none'"
}

// isSensitivePath checks if the path contains sensitive data
func isSensitivePath(path string) bool {
	sensitivePaths := []string{
		"/auth/",
		"/me",
		"/export/",
	}

	for _, p := range sensitivePaths {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID already exists (from load balancer/proxy)
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a simple request ID using timestamp and random component
			requestID = generateRequestID()
		}

		// Set request ID in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// Use timestamp + random for uniqueness
	// In production, consider using github.com/google/uuid
	return strings.ReplaceAll(
		strings.ReplaceAll(
			time.Now().Format("20060102150405.000000"),
			".", ""),
		"-", "")
}

// ForwardedProtoMiddleware handles X-Forwarded-Proto header from reverse proxies
func ForwardedProtoMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if the request came through Cloudflare or other reverse proxy
		if cfRay := c.GetHeader("CF-RAY"); cfRay != "" {
			// Force HTTPS for Cloudflare Tunnel requests
			c.Request.Header.Set("X-Forwarded-Proto", "https")
		} else if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto != "" {
			// Trust the forwarded proto from other proxies
			// Could add validation here if needed
		}

		c.Next()
	}
}

// DocumentationSecurityMiddleware configures security headers for API documentation
func DocumentationSecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to documentation routes
		if c.Request.URL.Path == "/docs" || c.Request.URL.Path == "/openapi.json" {
			// Allow connections to Cloudflare tunnel domains (both HTTP and HTTPS)
			// This resolves mixed content issues when Scalar documentation loads JSON specs
			tunnelDomain := c.Request.Host
			c.Writer.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
				"script-src 'self' 'unsafe-eval' https://cdn.jsdelivr.net; "+
				"connect-src 'self' https://"+tunnelDomain+" http://"+tunnelDomain+" https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net;")

			// Enable CORS for API documentation access
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Handle preflight OPTIONS requests
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}

		c.Next()
	}
}
