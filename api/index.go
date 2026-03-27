package handler

import (
	"context"
	"fmt"
	"log" // Added log import

	// Added math/rand import
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"absensholat-api/database"
	_ "absensholat-api/docs"
	"absensholat-api/routes"
	"absensholat-api/utils"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	ginEngine *gin.Engine
	once      sync.Once
	sugar     *zap.SugaredLogger
	initErr   error
)

func initLogger() {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := cfg.Build()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	sugar = logger.Sugar()
}

func GinMiddleware() gin.HandlerFunc {
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

func healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "API is running on Vercel",
	})
}

func initGin() {
	initLogger()

	// Initialize Firebase for OTP functionality (non-fatal if fails)
	ctx := context.Background()
	if err := utils.InitFirebase(ctx); err != nil {
		if sugar != nil {
			sugar.Warnf("Firebase initialization failed: %v. OTP functionality will be unavailable.", err)
		}
	} else {
		if sugar != nil {
			sugar.Info("Firebase initialized successfully")
		}
		// Start OTP cleanup every 5 minutes
		utils.StartOTPCleanup(5 * time.Minute)
	}

	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)

	// Initialize router
	ginEngine = gin.New()

	// CORS middleware
	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "HEAD"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Accept", "Origin", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           86400,
	}

	// Allow specific origins in production
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		corsConfig.AllowOrigins = []string{"*"}
	} else {
		corsConfig.AllowOrigins = []string{allowedOrigins}
	}

	ginEngine.Use(cors.New(corsConfig))
	ginEngine.Use(GinMiddleware(), gin.Recovery())

	// Health check endpoint (before DB, so it works even if DB fails)
	ginEngine.GET("/health", healthCheck)

	// Connect to database
	conn := os.Getenv("DATABASE_URL")
	if conn == "" {
		initErr = fmt.Errorf("DATABASE_URL environment variable not set")
		if sugar != nil {
			sugar.Error(initErr)
		}
		return
	}

	db, err := gorm.Open(postgres.Open(conn), &gorm.Config{})
	if err != nil {
		initErr = fmt.Errorf("failed to connect to database: %w", err)
		if sugar != nil {
			sugar.Error(initErr)
		}
		return
	}

	if sugar != nil {
		sugar.Info("Database connected")
	}

	if err := database.EnsureTablesCreated(db, sugar); err != nil {
		initErr = fmt.Errorf("failed to create tables: %w", err)
		if sugar != nil {
			sugar.Error(initErr)
		}
		return
	}

	routes.SetupRoutes(ginEngine, db, sugar)

	// Swagger documentation
	ginEngine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// Handler is the entrypoint for Vercel serverless function
func Handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Vercel Request Path:", r.URL.Path)
	once.Do(initGin)

	if initErr != nil {
		http.Error(w, "Internal Server Error: "+initErr.Error(), http.StatusInternalServerError)
		return
	}

	ginEngine.ServeHTTP(w, r)
}
