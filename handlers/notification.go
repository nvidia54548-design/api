package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
)

type NotificationItem struct {
	NIS         string `json:"nis"`
	NamaSiswa   string `json:"nama_siswa"`
	Kelas       string `json:"kelas"`
	Jurusan     string `json:"jurusan"`
	JenisSholat string `json:"jenis_sholat"`
	IDTemplate  int    `json:"id_template"`
}

type NotificationResponse struct {
	Message string             `json:"message"`
	Data    []NotificationItem `json:"data"`
	Count   int                `json:"count"`
}

// GetPendingNotifications returns students who haven't marked attendance for active/ended prayers today
// @Summary Get pending attendance notifications
// @Description Returns students who need to mark attendance for prayers that have started or ended today
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} NotificationResponse
// @Failure 500 {object} ErrorResponse
// @Router /notifications [get]
func GetPendingNotifications(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get current time in WIB
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			loc = time.FixedZone("WIB", 7*3600)
			logger.Warnw("failed to load Asia/Jakarta timezone, using fixed fallback", "error", err.Error())
		}
		now := time.Now().In(loc)
		currentDay := getDayName(now.Weekday())
		today := now.Format("2006-01-02")

		// Find prayer templates for current day
		var templates []models.JadwalSholatTemplate
		if filterErr := db.Preload("JenisSholat").Where("hari = ?", currentDay).
			Find(&templates).Error; filterErr != nil {
			logger.Errorw("Failed to fetch prayer templates", "error", filterErr.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data template jadwal",
			})
			return
		}

		if len(templates) == 0 {
			c.JSON(http.StatusOK, NotificationResponse{
				Message: "Tidak ada template jadwal sholat untuk hari ini",
				Data:    []NotificationItem{},
				Count:   0,
			})
			return
		}

		// Get template IDs
		templateIDs := make([]int, len(templates))
		for i, t := range templates {
			templateIDs[i] = t.IDTemplate
		}

		// Get all students with kelas info
		var students []models.Siswa
		if findErr := db.Preload("KelasRef").Where("deleted_at IS NULL").Find(&students).Error; findErr != nil {
			logger.Errorw("Failed to fetch students", "error", findErr.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data siswa",
			})
			return
		}

		// Get existing attendance records for today
		tanggal, err := time.Parse("2006-01-02", today)
		if err != nil {
			logger.Errorw("Failed to parse today's date", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Kesalahan internal (tanggal)",
			})
			return
		}
		var existingRecords []models.Absensi
		db.Where("tanggal = ? AND id_template IN (?)", tanggal, templateIDs).Find(&existingRecords)

		// Map of attended: NIS-TemplateID
		attended := make(map[string]bool)
		for _, r := range existingRecords {
			attended[strconv.Itoa(r.IDSiswa)+"-"+strconv.Itoa(r.IDTemplate)] = true
		}

		var notifications []NotificationItem
		for _, template := range templates {
			for _, student := range students {
				// Skip if already attended
				if attended[strconv.Itoa(student.IDSiswa)+"-"+strconv.Itoa(template.IDTemplate)] {
					continue
				}

				// Gender-based Friday filtering
				if now.Weekday() == time.Friday {
					if student.JK == "L" && template.JenisSholat.NamaJenis == "Dzuhur" {
						continue
					}
					if student.JK == "P" && template.JenisSholat.NamaJenis == "Jumat" {
						continue
					}
				} else {
					if template.JenisSholat.NamaJenis == "Jumat" {
						continue
					}
				}

				// Get kelas info
				kelas := ""
				jurusan := ""
				if student.KelasRef != nil {
					kelas = strconv.Itoa(student.KelasRef.Tingkatan) + student.KelasRef.Part
					jurusan = student.KelasRef.Jurusan
				}

				notifications = append(notifications, NotificationItem{
					NIS:         student.NIS,
					NamaSiswa:   student.NamaSiswa,
					Kelas:       kelas,
					Jurusan:     jurusan,
					JenisSholat: template.JenisSholat.NamaJenis,
					IDTemplate:  template.IDTemplate,
				})
			}
		}

		c.JSON(http.StatusOK, NotificationResponse{
			Message: "Notifikasi kehadiran berhasil diambil",
			Data:    notifications,
			Count:   len(notifications),
		})
	}
}

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var s string
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
