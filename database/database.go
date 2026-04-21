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
			if err := ensureStudentControlSchema(db, sugar); err != nil {
				if sugar != nil {
					sugar.Warnf("Student control schema migration warning: %v", err)
				}
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

		if err := ensureStudentControlSchema(db, sugar); err != nil {
			if sugar != nil {
				sugar.Warnf("Student control schema migration warning: %v", err)
			}
		}
	})

	return nil
}

func ensureStudentControlSchema(db *gorm.DB, sugar *zap.SugaredLogger) error {
	statements := []string{
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS academic_year VARCHAR(9) DEFAULT ''`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS current_semester SMALLINT DEFAULT 1`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS class_status VARCHAR(30) DEFAULT 'active'`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS last_promotion_at TIMESTAMP NULL`,
		`CREATE TABLE IF NOT EXISTS student_class_transitions (
			id SERIAL PRIMARY KEY,
			nis VARCHAR(20) NOT NULL,
			action VARCHAR(30) NOT NULL,
			from_class VARCHAR(50),
			to_class VARCHAR(50),
			from_academic_year VARCHAR(9),
			to_academic_year VARCHAR(9),
			from_semester SMALLINT,
			to_semester SMALLINT,
			from_status VARCHAR(30),
			to_status VARCHAR(30),
			decided_by VARCHAR(255),
			note TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (nis) REFERENCES siswa(nis) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_siswa_kelas ON siswa(kelas)`,
		`CREATE INDEX IF NOT EXISTS idx_siswa_academic_year ON siswa(academic_year)`,
		`CREATE INDEX IF NOT EXISTS idx_siswa_class_status ON siswa(class_status)`,
		`CREATE INDEX IF NOT EXISTS idx_student_class_transitions_nis ON student_class_transitions(nis)`,
		`CREATE INDEX IF NOT EXISTS idx_student_class_transitions_created_at ON student_class_transitions(created_at)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	if sugar != nil {
		sugar.Info("Student control schema is ready")
	}

	return nil
}
