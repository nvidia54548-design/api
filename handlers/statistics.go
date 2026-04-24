package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/utils"
)

type StatisticsResponse struct {
	Data StatisticsData `json:"data"`
}

type StatisticsData struct {
	Tanggal                string  `json:"tanggal"`
	TotalSiswa             int64   `json:"total_siswa"`
	TotalAbsenHariIni      int64   `json:"total_absen_hari_ini"`
	TotalKehadiranHariIni  int64   `json:"total_kehadiran_hari_ini"`
	TotalIzinHariIni       int64   `json:"total_izin_hari_ini"`
	TotalSakitHariIni      int64   `json:"total_sakit_hari_ini"`
	TotalTidakHadirHariIni int64   `json:"total_tidak_hadir_hari_ini"`
	TotalAlphaHariIni      int64   `json:"total_alpha_hari_ini"`
	PersentaseKehadiran    float64 `json:"persentase_kehadiran"`
	PersentaseIzin         float64 `json:"persentase_izin"`
	PersentaseSakit        float64 `json:"persentase_sakit"`
	PersentaseAlpha        float64 `json:"persentase_alpha"`
	RataRataKehadiran      float64 `json:"rata_rata_kehadiran"`
}

// GetStatistics godoc
// @Summary Ambil statistik absensi hari ini
// @Description Mengambil statistik absensi sholat untuk hari ini termasuk total absen, total kehadiran, izin, sakit, alpha, persentase kehadiran, dan rata-rata kehadiran
// @Tags statistics
// @Accept json
// @Produce json
// @Success 200 {object} StatisticsResponse "Statistik berhasil diambil"
// @Failure 500 {object} SiswaErrorResponse "Kesalahan server internal"
// @Router /statistics [get]
func GetStatistics(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		cacheKey := "stats:" + time.Now().Format("2006-01-02")

		var response StatisticsResponse
		cache := utils.GetCache()

		// Role-based filtering for Wali Kelas
		role, _ := c.Get("role")
		var kelasIDFilter *int
		if role != nil && role.(string) == "guru" {
			// Check if guru is wali_kelas
			nip, _ := c.Get("nip")
			if nip != nil {
				var wali models.WaliKelas
				if err := db.Where("id_staff = (SELECT id_staff FROM staff WHERE nip = ?) AND is_active = true", nip.(string)).First(&wali).Error; err == nil {
					kelasIDFilter = &wali.IDKelas
					cacheKey += ":" + string(rune(wali.IDKelas))
				}
			}
		}

		// Try to get from cache first, or compute if not cached
		err := cache.GetOrSet(ctx, cacheKey, &response, utils.CacheTTLStatistics, func() (interface{}, error) {
			return computeStatistics(db, logger, kelasIDFilter)
		})

		if err != nil {
			logger.Errorw("Failed to get statistics", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal mengambil statistik",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": response.Data,
		})
	}
}

// computeStatistics calculates statistics data using optimized queries
func computeStatistics(db *gorm.DB, logger *zap.SugaredLogger, kelasID *int) (StatisticsResponse, error) {
	today := utils.GetJakartaDateString()

	// Use a single raw query to get all statistics including izin/sakit breakdown
	type StatsResult struct {
		TotalSiswa      int64
		TodayTotal      int64
		TodayHadir      int64
		TodayIzin       int64
		TodaySakit      int64
		TodayTidakHadir int64
		TodayAlpha      int64
		AllTimeTotal    int64
		AllTimeHadir    int64
	}

	var stats StatsResult
	var query string
	var args []interface{}

	if kelasID != nil {
		// Filtered by class for Wali Kelas
		query = `
			SELECT
				(SELECT COUNT(*) FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) as total_siswa,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) AND DATE(tanggal) = ?) as today_total,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) AND DATE(tanggal) = ? AND status = 'hadir') as today_hadir,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) AND DATE(tanggal) = ? AND status = 'izin') as today_izin,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) AND DATE(tanggal) = ? AND status = 'sakit') as today_sakit,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) AND DATE(tanggal) = ? AND status != 'hadir') as today_tidak_hadir,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) AND DATE(tanggal) = ? AND status = 'alpha') as today_alpha,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL)) as all_time_total,
				(SELECT COUNT(*) FROM absensi WHERE id_siswa IN (SELECT id_siswa FROM siswa WHERE id_kelas = ? AND deleted_at IS NULL) AND status = 'hadir') as all_time_hadir
		`
		args = []interface{}{
			*kelasID,
			*kelasID, today,
			*kelasID, today,
			*kelasID, today,
			*kelasID, today,
			*kelasID, today,
			*kelasID, today,
			*kelasID,
			*kelasID,
		}
	} else {
		// Global stats for Admin
		query = `
			SELECT
				(SELECT COUNT(*) FROM siswa WHERE deleted_at IS NULL) as total_siswa,
				(SELECT COUNT(*) FROM absensi WHERE DATE(tanggal) = ?) as today_total,
				(SELECT COUNT(*) FROM absensi WHERE DATE(tanggal) = ? AND status = 'hadir') as today_hadir,
				(SELECT COUNT(*) FROM absensi WHERE DATE(tanggal) = ? AND status = 'izin') as today_izin,
				(SELECT COUNT(*) FROM absensi WHERE DATE(tanggal) = ? AND status = 'sakit') as today_sakit,
				(SELECT COUNT(*) FROM absensi WHERE DATE(tanggal) = ? AND status != 'hadir') as today_tidak_hadir,
				(SELECT COUNT(*) FROM absensi WHERE DATE(tanggal) = ? AND status = 'alpha') as today_alpha,
				(SELECT COUNT(*) FROM absensi) as all_time_total,
				(SELECT COUNT(*) FROM absensi WHERE status = 'hadir') as all_time_hadir
		`
		args = []interface{}{today, today, today, today, today, today}
	}

	if err := db.Raw(query, args...).Scan(&stats).Error; err != nil {
		return StatisticsResponse{}, err
	}

	// Calculate percentages
	var persentaseKehadiran, persentaseIzin, persentaseSakit, persentaseAlpha float64
	if stats.TodayTotal > 0 {
		persentaseKehadiran = (float64(stats.TodayHadir) / float64(stats.TodayTotal)) * 100
		persentaseIzin = (float64(stats.TodayIzin) / float64(stats.TodayTotal)) * 100
		persentaseSakit = (float64(stats.TodaySakit) / float64(stats.TodayTotal)) * 100
		persentaseAlpha = (float64(stats.TodayAlpha) / float64(stats.TodayTotal)) * 100
	}

	var rataRataKehadiran float64
	if stats.AllTimeTotal > 0 {
		rataRataKehadiran = (float64(stats.AllTimeHadir) / float64(stats.AllTimeTotal)) * 100
	}

	logger.Infow("Statistics computed successfully",
		"tanggal", today,
		"total_siswa", stats.TotalSiswa,
		"total_absen_hari_ini", stats.TodayTotal,
		"total_kehadiran_hari_ini", stats.TodayHadir,
		"total_izin", stats.TodayIzin,
		"total_sakit", stats.TodaySakit,
		"total_alpha", stats.TodayAlpha,
	)

	return StatisticsResponse{
		Message: "Statistik berhasil diambil",
		Data: StatisticsData{
			Tanggal:                today,
			TotalSiswa:             stats.TotalSiswa,
			TotalAbsenHariIni:      stats.TodayTotal,
			TotalKehadiranHariIni:  stats.TodayHadir,
			TotalIzinHariIni:       stats.TodayIzin,
			TotalSakitHariIni:      stats.TodaySakit,
			TotalTidakHadirHariIni: stats.TodayTidakHadir,
			TotalAlphaHariIni:      stats.TodayAlpha,
			PersentaseKehadiran:    persentaseKehadiran,
			PersentaseIzin:         persentaseIzin,
			PersentaseSakit:        persentaseSakit,
			PersentaseAlpha:        persentaseAlpha,
			RataRataKehadiran:      rataRataKehadiran,
		},
	}, nil
}
