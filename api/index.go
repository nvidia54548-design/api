package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"absensholat-api/database"
	"absensholat-api/routes"
	"absensholat-api/utils"

	"os"

	"github.com/gin-gonic/gin"
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
	cfg.Encoding = "console"
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, err := cfg.Build()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	sugar = logger.Sugar()
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

	// Initialize Redis cache (optional)
	if err := utils.InitCache(sugar); err != nil {
		if sugar != nil {
			sugar.Warnf("Redis cache initialization failed: %v. Caching disabled.", err)
		}
	}

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

	// Configure connection pool for serverless
	sqlDB, err := db.DB()
	if err != nil {
		initErr = fmt.Errorf("failed to get database connection: %w", err)
		if sugar != nil {
			sugar.Error(initErr)
		}
		return
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if sugar != nil {
		sugar.Info("Database connected with serverless pool configuration")
	}

	if err := database.EnsureTablesCreated(db, sugar); err != nil {
		initErr = fmt.Errorf("failed to create tables: %w", err)
		if sugar != nil {
			sugar.Error(initErr)
		}
		return
	}

	// Use SetupEngine for full middleware stack (CORS, gzip, security headers, rate limiting, etc.)
	ginEngine = routes.SetupEngine(db, sugar, true)
}

// Handler is the entrypoint for Vercel serverless function
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initGin)

	if initErr != nil {
		http.Error(w, "Internal Server Error: "+initErr.Error(), http.StatusInternalServerError)
		return
	}

	ginEngine.ServeHTTP(w, r)
}
