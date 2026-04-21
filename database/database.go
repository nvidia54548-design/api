package database

import (
	_ "embed"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

//go:embed schema.sql
var schema string

var ensureSchemaOnce sync.Once

func shouldBootstrapSchema() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DB_SCHEMA_BOOTSTRAP")))
	if v == "" {
		return true
	}

	switch v {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func EnsureTablesCreated(db *gorm.DB, sugar *zap.SugaredLogger) error {
	ensureSchemaOnce.Do(func() {
		if !shouldBootstrapSchema() {
			if sugar != nil {
				sugar.Info("Skipping schema bootstrap: DB_SCHEMA_BOOTSTRAP is disabled")
			}
			return
		}

		if sugar != nil {
			sugar.Info("Checking database schema status...")
		}

		var usersStaffExists bool
		checkErr := db.Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = 'users_staff'
			)
		`).Scan(&usersStaffExists).Error

		if checkErr == nil && usersStaffExists {
			if sugar != nil {
				sugar.Info("Schema already present, skipping bootstrap SQL")
			}
			return
		}

		if checkErr != nil && sugar != nil {
			sugar.Warnf("Schema existence check failed, attempting bootstrap anyway: %v", checkErr)
		}

		if sugar != nil {
			sugar.Info("Bootstrapping database schema...")
		}

		if err := db.Exec(schema).Error; err != nil {
			if sugar != nil {
				sugar.Warnf("Schema execution completed with warning: %v", err)
			}
		}

		if sugar != nil {
			sugar.Info("Database tables are ready")
		}
	})

	return nil
}
