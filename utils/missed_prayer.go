package utils

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
)

// StartMissedPrayerRecorder starts a background goroutine to record missed prayers
// It checks every minute if any prayer sessions have ended and auto-marks students who didn't attend
func StartMissedPrayerRecorder(db *gorm.DB, logger *zap.SugaredLogger, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := RecordMissedPrayers(db, logger); err != nil {
				logger.Errorw("Failed to record missed prayers",
					"error", err.Error(),
				)
			}
		}
	}()
}

// RecordMissedPrayers checks for ended prayer sessions and records missed attendance
// Gender-aware: On Fridays, males are assigned to Jumat and females to Dzuhur
func RecordMissedPrayers(db *gorm.DB, logger *zap.SugaredLogger) error {
	now := GetJakartaTime()
	currentTime := now.Format("15:04:05")

	// We check for prayers that ended TODAY or YESTERDAY to be safe against midnight transitions
	// or server downtime at the end of a day.
	daysToCheck := []time.Time{now, now.AddDate(0, 0, -1)}

	for _, t := range daysToCheck {
		dayName := GetIndonesianDayName(t)
		dateStr := t.Format("2006-01-02")
		tanggal, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			logger.Errorw("Failed to parse date string", "date", dateStr, "error", err.Error())
			continue
		}
		isFriday := t.Weekday() == time.Friday

		// Find all prayer templates for this day
		var templates []models.JadwalSholatTemplate
		if err := db.Preload("JenisSholat").Where("hari = ?", dayName).Find(&templates).Error; err != nil {
			logger.Errorw("Failed to fetch prayer templates for missed recording", "day", dayName, "error", err.Error())
			continue
		}

		if len(templates) == 0 {
			continue
		}

		// Get current active semester
		var semester models.SemesterAkademik
		if err := db.Where("tanggal_mulai <= ? AND tanggal_selesai >= ?", tanggal, tanggal).First(&semester).Error; err != nil {
			logger.Warnw("No active semester found for date", "date", dateStr)
			continue
		}

		// Get all students with kelas info
		var students []models.Siswa
		if err := db.Preload("KelasRef").Where("deleted_at IS NULL").Find(&students).Error; err != nil {
			logger.Errorw("Failed to fetch students", "error", err.Error())
			return err
		}

		// Collect template IDs for this day
		templateIDs := make([]int, len(templates))
		for i, tmpl := range templates {
			templateIDs[i] = tmpl.IDTemplate
		}

		// Get all existing attendances for these templates on this date
		var existingAttendances []models.Absensi
		if err := db.Where("tanggal = ? AND id_template IN (?)",
			tanggal, templateIDs).
			Find(&existingAttendances).Error; err != nil {
			logger.Errorw("Failed to fetch existing attendances", "date", dateStr, "error", err.Error())
			continue
		}

		// Map of existing records: SISWA-TEMPLATE
		existingMap := make(map[string]bool)
		for _, att := range existingAttendances {
			key := fmt.Sprintf("%d-%d", att.IDSiswa, att.IDTemplate)
			existingMap[key] = true
		}

		var missingRecords []models.Absensi
		for _, tmpl := range templates {
			for _, student := range students {
				// Gender-based filtering on Fridays
				if isFriday {
					if student.JK == "L" && tmpl.JenisSholat.NamaJenis == "Dzuhur" {
						continue
					}
					if student.JK == "P" && tmpl.JenisSholat.NamaJenis == "Jumat" {
						continue
					}
				} else {
					if tmpl.JenisSholat.NamaJenis == "Jumat" {
						continue
					}
				}

				// For Dhuha, check if student needs giliran (but we'll handle in attendance creation)
				if tmpl.JenisSholat.ButuhGiliran && tmpl.JenisSholat.NamaJenis == "Dhuha" {
					// Skip auto-marking Dhuha if no giliran specified - should be marked manually
					continue
				}

				key := fmt.Sprintf("%d-%d", student.IDSiswa, tmpl.IDTemplate)
				if !existingMap[key] {
					absensi := models.Absensi{
						IDSiswa:    student.IDSiswa,
						IDSemester: semester.IDSemester,
						IDTemplate: tmpl.IDTemplate,
						Tanggal:    tanggal,
						Status:     "alpha",
					}
					missingRecords = append(missingRecords, absensi)
				}
			}
		}

		// Bulk insert missing records for this day
		if len(missingRecords) > 0 {
			if err := db.CreateInBatches(missingRecords, 100).Error; err != nil {
				logger.Errorw("Failed to bulk create missed records", "date", dateStr, "error", err.Error())
			} else {
				logger.Infow("Bulk recorded missed prayers",
					"count", len(missingRecords),
					"date", dateStr,
					"templates_processed", len(templates),
				)
			}
		}
	}

	return nil
}
