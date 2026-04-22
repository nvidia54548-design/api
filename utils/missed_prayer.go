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

		// Find all prayer sessions for this day
		var prayers []models.JadwalSholat
		query := db.Where("hari = ?", dayName)

		// If it's today, only look at prayers that have finished
		if dateStr == now.Format("2006-01-02") {
			query = query.Where("waktu_selesai < ?", currentTime)
		}

		if err := query.Find(&prayers).Error; err != nil {
			logger.Errorw("Failed to fetch prayers for missed recording", "day", dayName, "error", err.Error())
			continue
		}

		if len(prayers) == 0 {
			continue
		}

		// Get all students
		var students []models.Siswa
		if err := db.Find(&students).Error; err != nil {
			logger.Errorw("Failed to fetch students", "error", err.Error())
			return err
		}

		// Collect prayer IDs for this day
		prayerIDs := make([]int, len(prayers))
		for i, p := range prayers {
			prayerIDs[i] = p.IDJadwal
		}

		// Get all existing attendances for these prayers on this date
		var existingAttendances []models.Absensi
		if err := db.Where("tanggal = ? AND id_jadwal IN (?)",
			tanggal, prayerIDs).
			Find(&existingAttendances).Error; err != nil {
			logger.Errorw("Failed to fetch existing attendances", "date", dateStr, "error", err.Error())
			continue
		}

		// Map of existing records: NIS-JadwalID
		existingMap := make(map[string]bool)
		for _, att := range existingAttendances {
			key := fmt.Sprintf("%s-%d", att.NIS, att.IDJadwal)
			existingMap[key] = true
		}

		var missingRecords []models.Absensi
		for _, prayer := range prayers {
			for _, student := range students {
				// Gender-based filtering on Fridays
				if isFriday {
					if student.JK == "L" && prayer.JenisSholat == "Dzuhur" {
						continue
					}
					if student.JK == "P" && prayer.JenisSholat == "Jumat" {
						continue
					}
				} else {
					if prayer.JenisSholat == "Jumat" {
						continue
					}
				}

				// Jurusan filtering (if defined for the prayer)
				if prayer.Jurusan != "" && prayer.Jurusan != "Semua Jurusan" {
					if student.Jurusan != prayer.Jurusan {
						continue
					}
				}

				key := fmt.Sprintf("%s-%d", student.NIS, prayer.IDJadwal)
				if !existingMap[key] {
					absensi := models.Absensi{
						IDSiswa:   student.IDSiswa,
						NIS:       student.NIS,
						IDJadwal:  prayer.IDJadwal,
						Tanggal:   tanggal,
						Status:    "alpha",
						Deskripsi: fmt.Sprintf("Absensi otomatis (%s) - tidak hadir", prayer.JenisSholat),
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
					"prayers_processed", len(prayers),
				)
			}
		}
	}

	return nil
}
