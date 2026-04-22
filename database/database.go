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

		var accountsExists bool
		checkErr := db.Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = 'accounts'
			)
		`).Scan(&accountsExists).Error

		if checkErr == nil && accountsExists {
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
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS academic_year CHAR(9)`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS current_semester SMALLINT DEFAULT 1`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS class_status VARCHAR(20) DEFAULT 'active'`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS last_promotion_at TIMESTAMP NULL`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS id_account INTEGER`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS id_kelas INTEGER`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS jurusan VARCHAR(20)`,
		`ALTER TABLE siswa ADD COLUMN IF NOT EXISTS kelas VARCHAR(50)`,
		`ALTER TABLE jadwal_sholat ADD COLUMN IF NOT EXISTS jurusan VARCHAR(100)`,
		`ALTER TABLE jadwal_sholat ADD COLUMN IF NOT EXISTS kelas VARCHAR(50)`,
		`ALTER TABLE absensi ADD COLUMN IF NOT EXISTS nis VARCHAR(20)`,
		`CREATE TABLE IF NOT EXISTS student_class_transitions (
			id SERIAL PRIMARY KEY,
			id_siswa INTEGER NOT NULL,
			action VARCHAR(30) NOT NULL,
			from_class INTEGER,
			to_class INTEGER,
			from_academic_year VARCHAR(9),
			to_academic_year VARCHAR(9),
			from_semester SMALLINT,
			to_semester SMALLINT,
			from_status VARCHAR(30),
			to_status VARCHAR(30),
			decided_by INTEGER,
			note TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (id_siswa) REFERENCES siswa(id_siswa) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_siswa_id_kelas ON siswa(id_kelas)`,
		`CREATE INDEX IF NOT EXISTS idx_siswa_academic_year ON siswa(academic_year)`,
		`CREATE INDEX IF NOT EXISTS idx_siswa_class_status ON siswa(class_status)`,
		`CREATE INDEX IF NOT EXISTS idx_student_class_transitions_id_siswa ON student_class_transitions(id_siswa)`,
		`CREATE INDEX IF NOT EXISTS idx_student_class_transitions_created_at ON student_class_transitions(created_at)`,
		`UPDATE siswa s
		 SET jurusan = k.jurusan,
		     kelas = CONCAT(k.tingkatan::text, k.part)
		 FROM kelas k
		 WHERE s.id_kelas = k.id_kelas
		   AND (s.jurusan IS NULL OR s.kelas IS NULL)`,
		`UPDATE absensi a
		 SET nis = s.nis
		 FROM siswa s
		 WHERE a.id_siswa = s.id_siswa
		   AND a.nis IS NULL`,
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
