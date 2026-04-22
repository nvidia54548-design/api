package routes

import (
	"net/http"
	"os"
	"strings"
	"time"

	"absensholat-api/docs"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/middleware"
)

func GinMiddleware(sugar *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		if sugar != nil {
			sugar.Infow("request",
				"status", c.Writer.Status(),
				"method", c.Request.Method,
				"path", path,
				"query", query,
				"ip", c.ClientIP(),
				"latency", latency,
				"user-agent", c.Request.UserAgent(),
			)
		}
	}
}

func healthCheck(c *gin.Context, db *gorm.DB) {
	health := gin.H{
		"status":    "ok",
		"message":   "API is running",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			health["database"] = "error"
			health["status"] = "degraded"
		} else if err := sqlDB.Ping(); err != nil {
			health["database"] = "disconnected"
			health["status"] = "degraded"
		} else {
			health["database"] = "connected"
		}
	}

	statusCode := http.StatusOK
	if health["status"] == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, health)
}

func SetupEngine(db *gorm.DB, logger *zap.SugaredLogger, isProduction bool) *gin.Engine {
	if isProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.MaxMultipartMemory = 8 << 20

	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "HEAD"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Accept", "Origin", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           86400,
	}

	if isProduction {
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
		if allowedOrigins != "" {
			origins := strings.Split(allowedOrigins, ",")
			for i := range origins {
				origins[i] = strings.TrimSpace(origins[i])
			}
			corsConfig.AllowOrigins = origins
			if logger != nil {
				logger.Infof("CORS configured for origins: %v", origins)
			}
		} else {
			corsConfig.AllowOrigins = []string{"*"}
		}
	} else {
		corsConfig.AllowOrigins = []string{"*"}
	}

	router.Use(cors.New(corsConfig))
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RequestID())

	if isProduction {
		router.Use(middleware.HTTPSEnforcement())
	}

	router.Use(middleware.RateLimitMiddleware())
	router.Use(middleware.PrometheusMiddleware())

	if logger != nil {
		router.Use(middleware.AuditMiddleware(logger))
		router.Use(GinMiddleware(logger), gin.Recovery())
	} else {
		router.Use(gin.Recovery())
	}

	SetupRoutes(router, db, logger)

	if !isProduction {
		// Only wrap DefaultServeMux for pprof
		router.GET("/debug/pprof/*profile", gin.WrapH(http.DefaultServeMux))
		router.GET("/openapi.json", docs.OpenAPIJSONHandler(router, isProduction))
		router.GET("/docs", docs.ScalarDocsHandler("Absensholat API Reference (Development)", "/openapi.json"))
		if logger != nil {
			logger.Info("Scalar API docs enabled at /docs")
			logger.Info("pprof profiling enabled at /debug/pprof/")
		}
	}

	router.GET("/health", func(c *gin.Context) {
		healthCheck(c, db)
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return router
}
