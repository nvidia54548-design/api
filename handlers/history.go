package handlers

import (
	"net/http"
	"strings"
	"time"

	"absensholat-api/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
)

type HistorySiswaResponse struct {
	Message string           `json:"message"`
	Data    HistorySiswaData `json:"data"`
}

type HistorySiswaData struct {
	Siswa     SiswaInfo            `json:"siswa"`
	Periode   string               `json:"periode"`
	StartDate string               `json:"start_date"`
	EndDate   string               `json:"end_date"`
	Statistik HistoryStatistik     `json:"statistik"`
	Absensi   []AbsensiHistoryItem `json:"absensi"`
}

type SiswaInfo struct {
	NIS       string `json:"nis"`
	NamaSiswa string `json:"nama_siswa"`
	Kelas     string `json:"kelas"`
	Jurusan   string `json:"jurusan"`
}

type HistoryStatistik struct {
	TotalAbsensi        int64   `json:"total_absensi"`
	TotalHadir          int64   `json:"total_hadir"`
	TotalIzin           int64   `json:"total_izin"`
	TotalSakit          int64   `json:"total_sakit"`
	TotalAlpha          int64   `json:"total_alpha"`
	PersentaseKehadiran float64 `json:"persentase_kehadiran"`
}

type AbsensiHistoryItem struct {
	IDAbsen     int    `json:"id_absen"`
	Tanggal     string `json:"tanggal"`
	Hari        string `json:"hari"`
	JenisSholat string `json:"jenis_sholat"`
	Status      string `json:"status"`
	Deskripsi   string `json:"deskripsi"`
}

type HistorySiswaRequest struct {
	Week int `form:"week"` // Number of weeks back (0 = current week, 1 = last week, etc.)
}

// GetHistorySiswa godoc
// @Summary Ambil riwayat absensi siswa (per minggu)
// @Description Mengambil riwayat absensi sholat untuk siswa yang sedang login. Dapat dilihat per minggu
// @Tags history
// @Accept json
// @Produce json
// @Param week query int false "Minggu ke berapa (0 = minggu ini, 1 = minggu lalu, dst). Default: 0"
// @Security BearerAuth
// @Success 200 {object} HistorySiswaResponse "Riwayat absensi berhasil diambil"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi"
// @Failure 404 {object} ErrorResponse "Siswa tidak ditemukan"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal"
// @Router /history/siswa [get]
func GetHistorySiswa(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get NIS from JWT context
		nis, exists := c.Get("nis")
		if !exists || nis == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "NIS tidak ditemukan di token",
			})
			return
		}

		var req HistorySiswaRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			req.Week = 0 // Default to current week
		}

		// Get siswa info
		var siswa models.Siswa
		if err := db.First(&siswa, "nis = ?", nis).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"message": "Siswa tidak ditemukan",
				})
				return
			}
			logger.Errorw("Failed to fetch siswa",
				"error", err.Error(),
				"nis", nis,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data siswa",
			})
			return
		}

		// Calculate week range
		now := utils.GetJakartaTime()
		// Get start of current week (Monday)
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday is 7
		}
		startOfCurrentWeek := now.AddDate(0, 0, -(weekday - 1))
		startOfCurrentWeek = time.Date(startOfCurrentWeek.Year(), startOfCurrentWeek.Month(), startOfCurrentWeek.Day(), 0, 0, 0, 0, now.Location())

		// Adjust for requested week
		startDate := startOfCurrentWeek.AddDate(0, 0, -7*req.Week)
		endDate := startDate.AddDate(0, 0, 6) // End of the week (Sunday)
		endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 0, now.Location())

		// Get absensi for the week with jadwal info
		var absensiList []models.Absensi
		if err := db.Model(&models.Absensi{}).
			Where("nis = ? AND tanggal >= ? AND tanggal <= ?", nis, startDate, endDate).
			Order("tanggal DESC").
			Find(&absensiList).Error; err != nil {
			logger.Errorw("Failed to fetch absensi history",
				"error", err.Error(),
				"nis", nis,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil riwayat absensi",
			})
			return
		}

		// Get jadwal info for each absensi
		var historyItems []AbsensiHistoryItem
		for _, absensi := range absensiList {
			var jadwal models.JadwalSholat
			jenisSholat := ""
			if err := db.First(&jadwal, "id_jadwal = ?", absensi.IDJadwal).Error; err == nil {
				jenisSholat = jadwal.JenisSholat
			}

			// Get day name in Indonesian
			hari := getHariName(absensi.Tanggal.Weekday())

			historyItems = append(historyItems, AbsensiHistoryItem{
				IDAbsen:     absensi.IDAbsen,
				Tanggal:     absensi.Tanggal.Format("2006-01-02"),
				Hari:        hari,
				JenisSholat: jenisSholat,
				Status:      absensi.Status,
				Deskripsi:   absensi.Deskripsi,
			})
		}

		// Calculate statistics for this period
		var stats HistoryStatistik
		stats.TotalAbsensi = int64(len(absensiList))

		for _, absensi := range absensiList {
			switch absensi.Status {
			case "hadir":
				stats.TotalHadir++
			case "izin":
				stats.TotalIzin++
			case "sakit":
				stats.TotalSakit++
			case "alpha":
				stats.TotalAlpha++
			}
		}

		if stats.TotalAbsensi > 0 {
			stats.PersentaseKehadiran = float64(stats.TotalHadir) / float64(stats.TotalAbsensi) * 100
		}

		// Determine period label
		periode := "Minggu Ini"
		if req.Week == 1 {
			periode = "Minggu Lalu"
		} else if req.Week > 1 {
			periode = formatWeekPeriod(req.Week)
		}

		logger.Infow("History siswa fetched successfully",
			"nis", nis,
			"week", req.Week,
			"total_records", len(historyItems),
		)

		c.JSON(http.StatusOK, HistorySiswaResponse{
			Message: "Riwayat absensi berhasil diambil",
			Data: HistorySiswaData{
				Siswa: SiswaInfo{
					NIS:       siswa.NIS,
					NamaSiswa: siswa.NamaSiswa,
					Kelas:     siswa.Kelas,
					Jurusan:   siswa.Jurusan,
				},
				Periode:   periode,
				StartDate: startDate.Format("2006-01-02"),
				EndDate:   endDate.Format("2006-01-02"),
				Statistik: stats,
				Absensi:   historyItems,
			},
		})
	}
}

type HistoryStaffRequest struct {
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
	Tanggal   string `form:"tanggal"`
	Kelas     string `form:"kelas"`
	Jurusan   string `form:"jurusan"`
	NIS       string `form:"nis"`
	Status    string `form:"status"`
	Page      int    `form:"page"`
	Limit     int    `form:"limit"`
}

type HistoryStaffResponse struct {
	Message string           `json:"message"`
	Data    HistoryStaffData `json:"data"`
}

type HistoryStaffData struct {
	Filters    HistoryFilters     `json:"filters"`
	Statistik  LaporanStatistik   `json:"statistik"`
	Pagination PaginationInfo     `json:"pagination"`
	Absensi    []AbsensiStaffItem `json:"absensi"`
}

type HistoryFilters struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Kelas     string `json:"kelas"`
	Jurusan   string `json:"jurusan"`
	NIS       string `json:"nis"`
	Status    string `json:"status"`
}

type PaginationInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type AbsensiStaffItem struct {
	IDAbsen     int    `json:"id_absen"`
	NIS         string `json:"nis"`
	NamaSiswa   string `json:"nama_siswa"`
	Kelas       string `json:"kelas"`
	Jurusan     string `json:"jurusan"`
	Tanggal     string `json:"tanggal"`
	Hari        string `json:"hari"`
	JenisSholat string `json:"jenis_sholat"`
	Status      string `json:"status"`
	Deskripsi   string `json:"deskripsi"`
}

// GetHistoryStaff godoc
// @Summary Ambil riwayat absensi untuk staff (admin/guru/wali_kelas)
// @Description Mengambil riwayat absensi semua siswa dengan filter. Hanya untuk admin, guru, dan wali_kelas
// @Tags history
// @Accept json
// @Produce json
// @Param start_date query string false "Tanggal mulai (format: YYYY-MM-DD)"
// @Param end_date query string false "Tanggal akhir (format: YYYY-MM-DD)"
// @Param tanggal query string false "Filter tanggal tunggal (YYYY-MM-DD). Prioritas lebih tinggi daripada start/end date."
// @Param kelas query string false "Filter berdasarkan kelas"
// @Param jurusan query string false "Filter berdasarkan jurusan"
// @Param nis query string false "Filter berdasarkan NIS siswa tertentu"
// @Param status query string false "Filter berdasarkan status (hadir/izin/sakit/alpha)"
// @Param page query int false "Nomor halaman (default: 1)"
// @Param limit query int false "Jumlah per halaman (default: 20, max: 100)"
// @Security BearerAuth
// @Success 200 {object} HistoryStaffResponse "Riwayat absensi berhasil diambil"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi"
// @Failure 403 {object} ErrorResponse "Akses ditolak"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal"
// @Router /history/staff [get]
func GetHistoryStaff(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req HistoryStaffRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Warnw("Invalid history request",
				"error", err.Error(),
			)
		}

		// Log received parameters
		logger.Infow("History request received", "params", req)

		// Set defaults
		if req.Page < 1 {
			req.Page = 1
		}
		if req.Limit < 1 {
			req.Limit = 20
		}
		if req.Limit > 100 {
			req.Limit = 100
		}

		// Role-based filtering for Wali Kelas (handle variations)
		role, _ := c.Get("role")
		if role != nil {
			roleStr := strings.ToLower(strings.TrimSpace(role.(string)))
			if roleStr == "wali_kelas" {
				nip, _ := c.Get("nip")
				if nip != nil {
					var guru models.Guru
					if err := db.Where("nip = ?", nip.(string)).First(&guru).Error; err == nil {
						// Force the class filter to be the teacher's class
						req.Kelas = guru.KelasWali
						logger.Infow("Wali Kelas class filter enforced", "class", req.Kelas)
					}
				}
			}
		}

		// Build base query
		baseQuery := db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		// Apply filters using range comparisons for timestamps to enable index usage
		if req.Tanggal != "" {
			logger.Infow("FILTERING BY SINGLE DATE", "received_tanggal_param", req.Tanggal)
			// Assuming req.Tanggal is YYYY-MM-DD, compare as range [T 00:00:00, T 23:59:59]
			baseQuery = baseQuery.Where("absensi.tanggal >= ? AND absensi.tanggal <= ?", req.Tanggal+" 00:00:00", req.Tanggal+" 23:59:59")
		} else {
			logger.Infow("NO SINGLE DATE FILTER APPLIED", "received_tanggal_param", req.Tanggal)
			if req.StartDate != "" {
				baseQuery = baseQuery.Where("absensi.tanggal >= ?", req.StartDate+" 00:00:00")
			}
			if req.EndDate != "" {
				baseQuery = baseQuery.Where("absensi.tanggal <= ?", req.EndDate+" 23:59:59")
			}
		}
		if req.Kelas != "" {
			baseQuery = baseQuery.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			baseQuery = baseQuery.Where("siswa.jurusan = ?", req.Jurusan)
		}
		if req.NIS != "" {
			baseQuery = baseQuery.Where("absensi.nis = ?", req.NIS)
		}
		if req.Status != "" {
			baseQuery = baseQuery.Where("absensi.status = ?", req.Status)
		}

		// Count total items
		var totalItems int64
		if err := baseQuery.Count(&totalItems).Error; err != nil {
			logger.Errorw("Failed to count history items",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menghitung data",
			})
			return
		}

		// Calculate statistics
		var stats LaporanStatistik

		// Get total siswa based on filters
		siswaQuery := db.Model(&models.Siswa{})
		if req.Kelas != "" {
			siswaQuery = siswaQuery.Where("kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			siswaQuery = siswaQuery.Where("jurusan = ?", req.Jurusan)
		}
		if req.NIS != "" {
			siswaQuery = siswaQuery.Where("nis = ?", req.NIS)
		}
		siswaQuery.Count(&stats.TotalSiswa)

		// Consolidate status counts into a single efficient query
		type StatusStats struct {
			Hadir int64
			Izin  int64
			Sakit int64
			Alpha int64
		}
		var statusStats StatusStats

		statsQuery := db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Select("SUM(CASE WHEN absensi.status = 'hadir' THEN 1 ELSE 0 END) as hadir, " +
				"SUM(CASE WHEN absensi.status = 'izin' THEN 1 ELSE 0 END) as izin, " +
				"SUM(CASE WHEN absensi.status = 'sakit' THEN 1 ELSE 0 END) as sakit, " +
				"SUM(CASE WHEN absensi.status = 'alpha' THEN 1 ELSE 0 END) as alpha")

		// Apply the SAME filters as baseQuery
		if req.Tanggal != "" {
			statsQuery = statsQuery.Where("absensi.tanggal >= ? AND absensi.tanggal <= ?", req.Tanggal+" 00:00:00", req.Tanggal+" 23:59:59")
		} else {
			if req.StartDate != "" {
				statsQuery = statsQuery.Where("absensi.tanggal >= ?", req.StartDate+" 00:00:00")
			}
			if req.EndDate != "" {
				statsQuery = statsQuery.Where("absensi.tanggal <= ?", req.EndDate+" 23:59:59")
			}
		}
		if req.Kelas != "" {
			statsQuery = statsQuery.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			statsQuery = statsQuery.Where("siswa.jurusan = ?", req.Jurusan)
		}
		if req.NIS != "" {
			statsQuery = statsQuery.Where("absensi.nis = ?", req.NIS)
		}

		if err := statsQuery.Scan(&statusStats).Error; err != nil {
			logger.Errorw("Failed to fetch status statistics", "error", err.Error())
		}

		stats.TotalHadir = statusStats.Hadir
		stats.TotalIzin = statusStats.Izin
		stats.TotalSakit = statusStats.Sakit
		stats.TotalAlpha = statusStats.Alpha

		if stats.TotalAbsensi > 0 {
			stats.PersentaseHadir = float64(stats.TotalHadir) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseIzin = float64(stats.TotalIzin) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseSakit = float64(stats.TotalSakit) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseAlpha = float64(stats.TotalAlpha) / float64(stats.TotalAbsensi) * 100
			stats.RataRataKehadiran = stats.PersentaseHadir
		}

		// Get paginated absensi data
		offset := (req.Page - 1) * req.Limit
		var absensiList []models.Absensi

		dataQuery := db.Model(&models.Absensi{}).
			Preload("Siswa").
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		if req.Tanggal != "" {
			dataQuery = dataQuery.Where("absensi.tanggal >= ? AND absensi.tanggal <= ?", req.Tanggal+" 00:00:00", req.Tanggal+" 23:59:59")
		} else {
			if req.StartDate != "" {
				dataQuery = dataQuery.Where("absensi.tanggal >= ?", req.StartDate+" 00:00:00")
			}
			if req.EndDate != "" {
				dataQuery = dataQuery.Where("absensi.tanggal <= ?", req.EndDate+" 23:59:59")
			}
		}
		if req.Kelas != "" {
			dataQuery = dataQuery.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			dataQuery = dataQuery.Where("siswa.jurusan = ?", req.Jurusan)
		}
		if req.NIS != "" {
			dataQuery = dataQuery.Where("absensi.nis = ?", req.NIS)
		}
		if req.Status != "" {
			dataQuery = dataQuery.Where("absensi.status = ?", req.Status)
		}

		if err := dataQuery.
			Order("absensi.tanggal DESC, siswa.kelas, siswa.nis").
			Offset(offset).
			Limit(req.Limit).
			Find(&absensiList).Error; err != nil {
			logger.Errorw("Failed to fetch history data",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data riwayat",
			})
			return
		}

		// Transform to response items
		var items []AbsensiStaffItem
		for _, absensi := range absensiList {
			namaSiswa := ""
			kelas := ""
			jurusan := ""
			if absensi.Siswa != nil {
				namaSiswa = absensi.Siswa.NamaSiswa
				kelas = absensi.Siswa.Kelas
				jurusan = absensi.Siswa.Jurusan
			}

			var jadwal models.JadwalSholat
			jenisSholat := ""
			if err := db.First(&jadwal, "id_jadwal = ?", absensi.IDJadwal).Error; err == nil {
				jenisSholat = jadwal.JenisSholat
			}

			hari := getHariName(absensi.Tanggal.Weekday())

			items = append(items, AbsensiStaffItem{
				IDAbsen:     absensi.IDAbsen,
				NIS:         absensi.NIS,
				NamaSiswa:   namaSiswa,
				Kelas:       kelas,
				Jurusan:     jurusan,
				Tanggal:     absensi.Tanggal.Format("2006-01-02"),
				Hari:        hari,
				JenisSholat: jenisSholat,
				Status:      absensi.Status,
				Deskripsi:   absensi.Deskripsi,
			})
		}

		// Calculate total pages
		totalPages := int(totalItems) / req.Limit
		if int(totalItems)%req.Limit > 0 {
			totalPages++
		}

		logger.Infow("History staff fetched successfully",
			"filters", req,
			"total_items", totalItems,
			"page", req.Page,
		)

		c.JSON(http.StatusOK, HistoryStaffResponse{
			Message: "Riwayat absensi berhasil diambil",
			Data: HistoryStaffData{
				Filters: HistoryFilters{
					StartDate: req.StartDate,
					EndDate:   req.EndDate,
					Kelas:     req.Kelas,
					Jurusan:   req.Jurusan,
					NIS:       req.NIS,
					Status:    req.Status,
				},
				Statistik: stats,
				Pagination: PaginationInfo{
					Page:       req.Page,
					Limit:      req.Limit,
					TotalItems: totalItems,
					TotalPages: totalPages,
				},
				Absensi: items,
			},
		})
	}
}

// Helper function to get Indonesian day name
func getHariName(weekday time.Weekday) string {
	days := map[time.Weekday]string{
		time.Sunday:    "Minggu",
		time.Monday:    "Senin",
		time.Tuesday:   "Selasa",
		time.Wednesday: "Rabu",
		time.Thursday:  "Kamis",
		time.Friday:    "Jumat",
		time.Saturday:  "Sabtu",
	}
	return days[weekday]
}

// Helper function to format week period
func formatWeekPeriod(week int) string {
	now := utils.GetJakartaTime()
	return now.AddDate(0, 0, -7*week).Format("2 Jan 2006") + " - " +
		now.AddDate(0, 0, -7*week+6).Format("2 Jan 2006")
}
