package main

import (
	"context"
	"expvar"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // Bundled timezone data support

	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/syumai/workers"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"absensholat-api/database"
	"absensholat-api/docs"
	"absensholat-api/handlers"
	"absensholat-api/middleware"
	"absensholat-api/routes"
	"absensholat-api/utils"

	_ "net/http/pprof" // Import for pprof

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Force redeploy: jadwal_sholat updated

var sugar *zap.SugaredLogger
var db *gorm.DB

func initLogger() {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := cfg.Build()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	sugar = logger.Sugar()
}

//	@title			API Sistem Absensi Sholat
//	@version		1.0.0
//	@description	API untuk sistem pencatatan absensi sholat siswa
//	@termsOfService	http://swagger.io/terms/
//	@contact.name	API Support
//	@license.name	MIT
//
//	@BasePath	/api
//	@schemes	http https
//
//	@securityDefinitions.apikey	BearerAuth
//	@in								header
//	@name							Authorization
//	@description					Type "Bearer" followed by a space and JWT token.

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading")
	}

	initLogger()
	defer func() {
		if err := sugar.Sync(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()

	env := os.Getenv("ENVIRONMENT")
	isProduction := env == "production"

	// Validate critical environment variables in production
	if isProduction {
		requiredVars := []string{"DATABASE_URL", "JWT_SECRET", "ALLOWED_ORIGINS"}
		missing := []string{}
		for _, v := range requiredVars {
			if os.Getenv(v) == "" {
				missing = append(missing, v)
			}
		}
		if len(missing) > 0 {
			sugar.Fatalf("Missing required environment variables in production: %s", strings.Join(missing, ", "))
		}
	}

	// Initialize Firebase for OTP functionality
	ctx := context.Background()
	if err := utils.InitFirebase(ctx); err != nil {
		sugar.Warnf("Firebase initialization failed: %v. OTP functionality will be unavailable.", err)
	} else {
		sugar.Info("Firebase initialized successfully")
		defer func() {
			if err := utils.CloseFirebase(); err != nil {
				log.Printf("Error closing Firebase: %v", err)
			}
		}()
		// Start OTP cleanup every 5 minutes
		utils.StartOTPCleanup(5 * time.Minute)
	}

	// Initialize Redis cache (optional)
	if err := utils.InitCache(sugar); err != nil {
		sugar.Warnf("Redis cache initialization failed: %v. Caching disabled.", err)
	} else if utils.CacheEnabled() {
		defer func() {
			if err := utils.CloseCache(); err != nil {
				log.Printf("Error closing Cache: %v", err)
			}
		}()
	}

	sugar.Info("Starting up database...")

	conn := os.Getenv("DATABASE_URL")
	if conn == "" {
		sugar.Fatal("DATABASE_URL environment variable is required")
	}

	var err error
	db, err = gorm.Open(postgres.Open(conn), &gorm.Config{})

	if err != nil {
		sugar.Fatal("Failed to connect to database:", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		sugar.Fatal("Failed to get database connection:", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	sugar.Info("Database connected with connection pooling configured")

	// Add database connection pool metrics
	expvar.Publish("db_idle_conns", expvar.Func(func() interface{} { return sqlDB.Stats().Idle }))
	expvar.Publish("db_in_use_conns", expvar.Func(func() interface{} { return sqlDB.Stats().InUse }))
	expvar.Publish("db_open_conns", expvar.Func(func() interface{} { return sqlDB.Stats().OpenConnections }))
	expvar.Publish("db_wait_count", expvar.Func(func() interface{} { return sqlDB.Stats().WaitCount }))
	expvar.Publish("db_wait_duration", expvar.Func(func() interface{} { return sqlDB.Stats().WaitDuration }))

	if err := database.EnsureTablesCreated(db, sugar); err != nil {
		sugar.Fatal("Failed to create tables:", err)
	}

	// Start background task to record missed prayers (check every 5 minutes)
	utils.StartMissedPrayerRecorder(db, sugar, 5*time.Minute)
	sugar.Info("Missed prayer recorder started - will check for ended prayers every 5 minutes")

	// Start background task to clean up backed-up data (check every hour)
	handlers.StartBackupCleanupScheduler(db, sugar)
	sugar.Info("Backup cleanup scheduler started - will check for expired backups every hour")

	// Initialize centralized App Engine
	router := routes.SetupEngine(db, sugar, isProduction)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	// Swagger documentation (disable in production for security)
	if !isProduction {
		// Configure Swagger host based on environment
		swaggerHost := os.Getenv("SWAGGER_HOST")
		if swaggerHost == "" {
			swaggerHost = "localhost:" + port
		}
		docs.SwaggerInfo.Host = swaggerHost
		docs.SwaggerInfo.Schemes = []string{"http"}
		sugar.Infof("Swagger configured for host: %s", swaggerHost)

		router.GET("/swagger/*any", middleware.SwaggerAuthMiddleware(), ginSwagger.WrapHandler(swaggerFiles.Handler))
		sugar.Info("Swagger documentation enabled at /swagger/index.html")
	} else {
		// Production: Set production host if Swagger is ever enabled
		swaggerHost := os.Getenv("SWAGGER_HOST")
		if swaggerHost != "" {
			docs.SwaggerInfo.Host = swaggerHost
			docs.SwaggerInfo.Schemes = []string{"https"}
		}
	}

	// Check if running as a Cloudflare Worker
	if os.Getenv("WORKER_MODE") == "true" {
		sugar.Info("Running in Cloudflare Worker mode")
		workers.Serve(router)
		return
	}

	// Create HTTP server with timeouts for standard environments
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		sugar.Infof("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown (only for standard environments)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	sugar.Info("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		sugar.Fatalf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if sqlDB != nil {
		if err := sqlDB.Close(); err != nil {
			sugar.Errorf("Error closing database connection: %v", err)
		}
	}

	sugar.Info("Server exited gracefully")
}
