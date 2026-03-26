package main

import (
	"context"
	"os"

	"absensholat-api/utils"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 1. Initialize Zap logger
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := cfg.Build()
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to initialize logger"}, nil
	}
	sugar := logger.Sugar()

	sugar.Info("[Netlify Cron] Starting scheduled alpha attendance check...")

	// 2. Initialize Database Connection
	conn := os.Getenv("DATABASE_URL")
	if conn == "" {
		sugar.Error("DATABASE_URL environment variable is required")
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "DATABASE_URL is missing"}, nil
	}

	db, err := gorm.Open(postgres.Open(conn), &gorm.Config{})
	if err != nil {
		sugar.Error("Failed to connect to database:", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "DB Connection failed"}, nil
	}

	// 3. Trigger the Alpha recording logic
	err = utils.RecordMissedPrayers(db, sugar)
	if err != nil {
		sugar.Errorw("[Netlify Cron] Failed to record missed prayers", "error", err.Error())
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to record missed prayers"}, nil
	}

	sugar.Info("[Netlify Cron] Scheduled alpha attendance check completed successfully")

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "{\"message\": \"Auto-absen alpha berhasil dijalankan oleh Netlify Cron\"}",
	}, nil
}

func main() {
	lambda.Start(Handler)
}
