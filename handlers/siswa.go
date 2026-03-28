package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

// SiswaFilterRequest represents query parameters for filtering siswa
type SiswaFilterRequest struct {
	Search   string `form:"search"`    // Search by NIS or nama_siswa
	Kelas    string `form:"kelas"`     // Filter by kelas (exact match)
	Jurusan  string `form:"jurusan"`   // Filter by jurusan (exact match)
	JK       string `form:"jk"`        // Filter by jenis kelamin
	Page     int    `form:"page"`      // Page number (1-based)
	PageSize int    `form:"page_size"` // Items per page
	SortBy   string `form:"sort_by"`   // Sort field: nis, nama_siswa, kelas, jurusan
	SortDir  string `form:"sort_dir"`  // Sort direction: asc, desc
}

type SiswaListResponse struct {
	Message string         `json:"message"`
	Data    []models.Siswa `json:"data"`
}

type SiswaListPaginatedResponse struct {
	Message    string         `json:"message"`
	Data       []models.Siswa `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
	Filters    AppliedFilters `json:"filters"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type AppliedFilters struct {
	Search  string `json:"search,omitempty"`
	Kelas   string `json:"kelas,omitempty"`
	Jurusan string `json:"jurusan,omitempty"`
	JK      string `json:"jk,omitempty"`
	SortBy  string `json:"sort_by"`
	SortDir string `json:"sort_dir"`
}

type SiswaDetailResponse struct {
	Message string       `json:"message"`
	Data    models.Siswa `json:"data"`
}

type SiswaCreateResponse struct {
	Message string       `json:"message"`
	Data    models.Siswa `json:"data"`
}

type SiswaUpdateResponse struct {
	Message string       `json:"message"`
	Data    models.Siswa `json:"data"`
}

type SiswaDeleteResponse struct {
	Message string `json:"message"`
}

type SiswaErrorResponse struct {
	Message string      `json:"message"`
	Error   interface{} `json:"error,omitempty"`
}

// GetSiswa godoc
// @Summary Ambil semua data siswa dengan filter dan pagination
// @Description Mengambil daftar siswa dengan dukungan filter, pencarian, sorting, dan pagination
// @Tags siswa
// @Accept json
// @Produce json
// @Param search query string false "Cari berdasarkan NIS atau nama siswa"
// @Param kelas query string false "Filter berdasarkan kelas (exact match)"
// @Param jurusan query string false "Filter berdasarkan jurusan (exact match)"
// @Param jk query string false "Filter berdasarkan jenis kelamin (Laki-laki/Perempuan)"
// @Param page query int false "Nomor halaman (default: 1)"
// @Param page_size query int false "Jumlah item per halaman (default: 20, max: 100)"
// @Param sort_by query string false "Urutkan berdasarkan: nis, nama_siswa, kelas, jurusan (default: nama_siswa)"
// @Param sort_dir query string false "Arah pengurutan: asc, desc (default: asc)"
// @Success 200 {object} SiswaListPaginatedResponse "Data siswa berhasil diambil dengan metadata pagination"
// @Failure 500 {object} SiswaErrorResponse "Kesalahan server internal - Gagal query database"
// @Router /siswa [get]
func GetSiswa(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var filter SiswaFilterRequest
		if err := c.ShouldBindQuery(&filter); err != nil {
			logger.Warnw("Invalid filter parameters",
				"error", err.Error(),
			)
		}

		// Set defaults
		if filter.Page < 1 {
			filter.Page = 1
		}
		if filter.PageSize < 1 || filter.PageSize > 100 {
			filter.PageSize = 20
		}
		if filter.SortBy == "" {
			filter.SortBy = "nama_siswa"
		}
		if filter.SortDir == "" {
			filter.SortDir = "asc"
		}

		// Validate sort_by field
		validSortFields := map[string]bool{
			"nis": true, "nama_siswa": true, "kelas": true, "jurusan": true,
		}
		if !validSortFields[filter.SortBy] {
			filter.SortBy = "nama_siswa"
		}

		// Validate sort_dir
		if filter.SortDir != "asc" && filter.SortDir != "desc" {
			filter.SortDir = "asc"
		}

		// Build query
		query := db.Model(&models.Siswa{})

		// Role-based filtering for Wali Kelas (handle variations like 'wali kelas' or 'wali_kelas')
		role, _ := c.Get("role")
		if role != nil {
			roleStr := strings.ToLower(strings.TrimSpace(role.(string)))
			if roleStr == "wali_kelas" {
				nip, _ := c.Get("nip")
				if nip != nil {
					var guru models.Guru
					if err := db.Where("nip = ?", nip.(string)).First(&guru).Error; err == nil {
						query = query.Where("kelas = ?", guru.KelasWali)
					}
				}
			}
		}

		// Apply search filter (NIS or nama_siswa)
		if filter.Search != "" {
			searchTerm := "%" + strings.ToLower(filter.Search) + "%"
			query = query.Where("LOWER(nis) LIKE ? OR LOWER(nama_siswa) LIKE ?", searchTerm, searchTerm)
		}

		// Apply exact filters
		if filter.Kelas != "" {
			query = query.Where("kelas = ?", filter.Kelas)
		}
		if filter.Jurusan != "" {
			query = query.Where("jurusan = ?", filter.Jurusan)
		}
		if filter.JK != "" {
			query = query.Where("jk = ?", filter.JK)
		}

		// Count total items before pagination
		var totalItems int64
		if err := query.Count(&totalItems).Error; err != nil {
			logger.Errorw("Failed to count students",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menghitung data siswa",
			})
			return
		}

		// Calculate pagination
		totalPages := int((totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize))
		offset := (filter.Page - 1) * filter.PageSize

		// Apply sorting and pagination
		orderClause := filter.SortBy + " " + filter.SortDir
		var siswaList []models.Siswa
		if err := query.Order(orderClause).Offset(offset).Limit(filter.PageSize).Find(&siswaList).Error; err != nil {
			logger.Errorw("Failed to fetch students",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data siswa",
			})
			return
		}

		logger.Infow("Students fetched successfully",
			"count", len(siswaList),
			"total", totalItems,
			"page", filter.Page,
			"filters", filter,
		)

		c.JSON(http.StatusOK, SiswaListPaginatedResponse{
			Message: "Data siswa berhasil diambil",
			Data:    siswaList,
			Pagination: PaginationMeta{
				Page:       filter.Page,
				PageSize:   filter.PageSize,
				TotalItems: totalItems,
				TotalPages: totalPages,
			},
			Filters: AppliedFilters{
				Search:  filter.Search,
				Kelas:   filter.Kelas,
				Jurusan: filter.Jurusan,
				JK:      filter.JK,
				SortBy:  filter.SortBy,
				SortDir: filter.SortDir,
			},
		})
	}
}

// GetSiswaByID godoc
// @Summary Ambil data siswa berdasarkan NIS
// @Description Mengambil detail siswa spesifik berdasarkan nomor induk siswa (NIS). Mengembalikan profil lengkap siswa termasuk nama, jenis kelamin, jurusan, dan kelas
// @Tags siswa
// @Accept json
// @Produce json
// @Param nis path string true "NIS Siswa - nomor induk siswa 4-20 digit"
// @Success 200 {object} SiswaDetailResponse "Data siswa ditemukan dan berhasil diambil"
// @Failure 400 {object} SiswaErrorResponse "NIS tidak valid atau kosong - parameter path tidak diberikan"
// @Failure 404 {object} SiswaErrorResponse "Siswa tidak ditemukan - NIS tidak ada dalam database"
// @Failure 500 {object} SiswaErrorResponse "Kesalahan server internal - Database connection error atau query error"
// @Router /siswa/{nis} [get]
func GetSiswaByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract NIS from the full URL path: /api/siswa/{nis}
		fullPath := c.Request.URL.Path
		nis := strings.TrimPrefix(fullPath, "/api/siswa/")

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "NIS tidak valid",
			})
			return
		}

		var siswa models.Siswa
		if err := db.First(&siswa, "nis = ?", nis).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Student not found",
					"nis", nis,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"message": "Siswa tidak ditemukan",
				})
				return
			}
			logger.Errorw("Failed to fetch student",
				"error", err.Error(),
				"nis", nis,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data siswa",
			})
			return
		}

		logger.Infow("Student retrieved",
			"nis", nis,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "Data siswa berhasil diambil",
			"data":    siswa,
		})
	}
}

// CreateSiswa godoc
// @Summary Tambah data siswa baru
// @Description Menambahkan siswa baru ke dalam sistem. NIS harus unik dan belum terdaftar. Semua field (NIS, nama_siswa, jk) wajib diisi
// @Tags siswa
// @Accept json
// @Produce json
// @Param student body models.Siswa true "Data siswa baru - NIS (unique), nama_siswa, jk (Laki-laki/Perempuan), jurusan (optional), kelas (optional)"
// @Success 201 {object} SiswaCreateResponse "Siswa berhasil ditambahkan - Mengembalikan data siswa yang baru dibuat"
// @Failure 400 {object} SiswaErrorResponse "Permintaan tidak valid - Format JSON salah, field required kosong, atau duplicate NIS"
// @Failure 500 {object} SiswaErrorResponse "Kesalahan server internal - Database constraint violation atau connection error"
// @Router /siswa [post]
func CreateSiswa(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var siswa models.Siswa
		if err := c.ShouldBindJSON(&siswa); err != nil {
			logger.Warnw("Invalid JSON format",
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format JSON tidak valid",
				"error":   err.Error(),
			})
			return
		}

		if err := db.Create(&siswa).Error; err != nil {
			logger.Errorw("Failed to create student",
				"error", err.Error(),
				"nis", siswa.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menambahkan siswa",
			})
			return
		}

		logger.Infow("Student created",
			"nis", siswa.NIS,
			"nama_siswa", siswa.NamaSiswa,
		)

		c.JSON(http.StatusCreated, gin.H{
			"message": "Siswa berhasil ditambahkan",
			"data":    siswa,
		})
	}
}

// UpdateSiswa godoc
// @Summary Ubah data siswa
// @Description Memperbarui informasi siswa yang sudah terdaftar. NIS tidak dapat diubah (primary key). Hanya nama_siswa, jk, jurusan, dan kelas yang dapat diperbarui
// @Tags siswa
// @Accept json
// @Produce json
// @Param nis path string true "NIS Siswa yang akan diupdate"
// @Param student body models.Siswa true "Data siswa yang diperbarui - nama_siswa, jk, jurusan, kelas dapat diubah"
// @Success 200 {object} SiswaUpdateResponse "Siswa berhasil diperbarui - Mengembalikan data siswa terbaru"
// @Failure 400 {object} SiswaErrorResponse "Permintaan tidak valid - Format JSON salah atau NIS kosong"
// @Failure 404 {object} SiswaErrorResponse "Siswa tidak ditemukan - NIS tidak ada dalam database"
// @Failure 500 {object} SiswaErrorResponse "Kesalahan server internal - Database error atau constraint violation"
// @Router /siswa/{nis} [put]
func UpdateSiswa(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract NIS from the full URL path: /api/siswa/{nis}
		fullPath := c.Request.URL.Path
		nis := strings.TrimPrefix(fullPath, "/api/siswa/")

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "NIS tidak valid",
			})
			return
		}

		var siswa models.Siswa
		if err := c.ShouldBindJSON(&siswa); err != nil {
			logger.Warnw("Invalid JSON format",
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format JSON tidak valid",
				"error":   err.Error(),
			})
			return
		}

		siswa.NIS = nis
		result := db.Model(&models.Siswa{}).Where("nis = ?", nis).Updates(&siswa)
		if result.Error != nil {
			logger.Errorw("Failed to update student",
				"error", result.Error.Error(),
				"nis", nis,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memperbarui siswa",
			})
			return
		}

		if result.RowsAffected == 0 {
			logger.Warnw("Student not found for update",
				"nis", nis,
			)
			c.JSON(http.StatusNotFound, gin.H{
				"message": "Siswa tidak ditemukan",
			})
			return
		}

		logger.Infow("Student updated",
			"nis", nis,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "Siswa berhasil diperbarui",
			"data":    siswa,
		})
	}
}

// DeleteSiswa godoc
// @Summary Hapus data siswa
// @Description Menghapus siswa dari sistem secara permanen. Operasi ini akan menghapus semua data terkait siswa termasuk akun login dan absensi. Hati-hati karena operasi ini tidak dapat dibatalkan
// @Tags siswa
// @Accept json
// @Produce json
// @Param nis path string true "NIS Siswa yang akan dihapus"
// @Success 200 {object} SiswaDeleteResponse "Siswa berhasil dihapus - Data siswa telah dihapus dari sistem"
// @Failure 400 {object} SiswaErrorResponse "Permintaan tidak valid - NIS kosong atau tidak diberikan"
// @Failure 404 {object} SiswaErrorResponse "Siswa tidak ditemukan - NIS tidak ada dalam database"
// @Failure 500 {object} SiswaErrorResponse "Kesalahan server internal - Database error atau cascade delete error"
// @Router /siswa/{nis} [delete]
func DeleteSiswa(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract NIS from the full URL path: /api/siswa/{nis}
		fullPath := c.Request.URL.Path
		nis := strings.TrimPrefix(fullPath, "/api/siswa/")

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "NIS tidak valid",
			})
			return
		}

		result := db.Delete(&models.Siswa{}, "nis = ?", nis)
		if result.Error != nil {
			logger.Errorw("Failed to delete student",
				"error", result.Error.Error(),
				"nis", nis,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menghapus siswa",
			})
			return
		}

		if result.RowsAffected == 0 {
			logger.Warnw("Student not found for deletion",
				"nis", nis,
			)
			c.JSON(http.StatusNotFound, gin.H{
				"message": "Siswa tidak ditemukan",
			})
			return
		}

		logger.Infow("Student deleted",
			"nis", nis,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "Siswa berhasil dihapus",
		})
	}
}

type CreateAbsensiRequest struct {
	IDJadwal  int    `json:"id_jadwal" binding:"required"`
	Tanggal   string `json:"tanggal" binding:"required"`
	Status    string `json:"status" binding:"required,oneof=hadir izin sakit alpha"`
	Deskripsi string `json:"deskripsi"`
}

type AbsensiResponse struct {
	IDAbsen   int    `json:"id_absen"`
	NIS       string `json:"nis"`
	IDJadwal  int    `json:"id_jadwal"`
	Tanggal   string `json:"tanggal"`
	Status    string `json:"status"`
	Deskripsi string `json:"deskripsi"`
	CreatedAt string `json:"created_at"`
}

// HandleSiswaPath routes POST requests to either absensi or other handlers based on path
func HandleSiswaPath(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")

		// Check if this is an absensi request
		if len(path) > 8 && path[len(path)-8:] == "/absensi" {
			CreateAbsensi(db, logger)(c)
			return
		}

		// Otherwise treat as regular siswa POST (though not typical)
		c.JSON(400, gin.H{
			"message": "Invalid path",
		})
	}
}

// CreateAbsensi godoc
// @Summary Buat absensi sholat baru untuk siswa
// @Description Siswa membuat pencatatan absensi (kehadiran/ketidakhadiran) untuk sesi sholat. Status yang valid adalah: hadir, izin, sakit, alpha
// @Tags absensi
// @Accept json
// @Produce json
// @Param nis path string true "NIS Siswa - nomor induk siswa"
// @Param request body CreateAbsensiRequest true "Data absensi siswa"
// @Success 201 {object} AbsensiResponse "Absensi berhasil dibuat"
// @Failure 400 {object} SiswaErrorResponse "Request tidak valid atau data tidak lengkap"
// @Failure 404 {object} SiswaErrorResponse "Siswa atau jadwal sholat tidak ditemukan"
// @Failure 500 {object} SiswaErrorResponse "Kesalahan server internal"
// @Router /siswa/{nis}/absensi [post]
func CreateAbsensi(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract NIS from the full URL path
		// Path format: /api/siswa/{nis}/absensi
		fullPath := c.Request.URL.Path

		logger.Infow("CreateAbsensi called",
			"full_path", fullPath,
			"raw_path_param", c.Param("path"),
		)

		// Remove /api/siswa/ prefix and /absensi suffix
		nis := fullPath
		nis = strings.TrimPrefix(nis, "/api/siswa/")
		nis = strings.TrimSuffix(nis, "/absensi")

		logger.Infow("NIS extracted", "nis", nis)

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "NIS tidak valid",
			})
			return
		}

		// Check if student exists
		var siswa models.Siswa
		if err := db.First(&siswa, "nis = ?", nis).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Student not found for absensi",
					"nis", nis,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"message": "Siswa tidak ditemukan",
				})
				return
			}
			logger.Errorw("Failed to check student",
				"nis", nis,
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memeriksa data siswa",
			})
			return
		}

		var req CreateAbsensiRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Warnw("Invalid absensi request",
				"nis", nis,
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Data absensi tidak valid",
				"error":   err.Error(),
			})
			return
		}

		// Check if jadwal exists
		var jadwal models.JadwalSholat
		if err := db.First(&jadwal, "id_jadwal = ?", req.IDJadwal).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Schedule not found for absensi",
					"id_jadwal", req.IDJadwal,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"message": "Jadwal sholat tidak ditemukan",
				})
				return
			}
			logger.Errorw("Failed to check schedule",
				"id_jadwal", req.IDJadwal,
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memeriksa jadwal sholat",
			})
			return
		}

		// Parse tanggal
		tanggal, err := time.Parse("2006-01-02", req.Tanggal)
		if err != nil {
			logger.Warnw("Invalid date format",
				"tanggal", req.Tanggal,
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format tanggal tidak valid. Gunakan format YYYY-MM-DD",
			})
			return
		}

		// Role-based restrictions for Wali Kelas
		role, _ := c.Get("role")
		userRole := strings.ToLower(strings.TrimSpace(role.(string)))

		if userRole == "wali_kelas" {
			nip, _ := c.Get("nip")
			userNip := nip.(string)

			var guru models.Guru
			if err := db.Where("nip = ?", userNip).First(&guru).Error; err != nil {
				logger.Errorw("Failed to fetch guru info for wali_kelas", "nip", userNip, "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memverifikasi data wali kelas"})
				return
			}

			if guru.KelasWali != siswa.Kelas {
				logger.Warnw("Wali kelas attempted to edit student outside their class",
					"guru_nip", userNip,
					"guru_kelas", guru.KelasWali,
					"siswa_nis", nis,
					"siswa_kelas", siswa.Kelas,
				)
				c.JSON(http.StatusForbidden, gin.H{
					"message": "Anda hanya diperbolehkan mengelola absensi untuk kelas " + guru.KelasWali,
				})
				return
			}
		}

		// Check if an absensi record already exists for this day/jadwal
		var absensi models.Absensi
		var existingAbsensi models.Absensi
		err = db.Where("nis = ? AND id_jadwal = ? AND tanggal = ?", nis, req.IDJadwal, tanggal).First(&existingAbsensi).Error

		if err == nil {
			// Record exists. Update it if the new status is izin/sakit (or any valid status override)
			// User specifically mentioned converting 'alpha' to 'izin/sakit'
			existingAbsensi.Status = req.Status
			existingAbsensi.Deskripsi = req.Deskripsi

			if err := db.Save(&existingAbsensi).Error; err != nil {
				logger.Errorw("Failed to update existing absensi", "nis", nis, "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui data absensi"})
				return
			}
			absensi = existingAbsensi
		} else if err == gorm.ErrRecordNotFound {
			// Record doesn't exist, create new one
			absensi = models.Absensi{
				NIS:       nis,
				IDJadwal:  req.IDJadwal,
				Tanggal:   tanggal,
				Status:    req.Status,
				Deskripsi: req.Deskripsi,
			}

			if err := db.Create(&absensi).Error; err != nil {
				logger.Errorw("Failed to create absensi", "nis", nis, "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat absensi"})
				return
			}
		} else {
			logger.Errorw("Failed to check existing absensi", "nis", nis, "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengecek data absensi"})
			return
		}

		// Invalidate statistics cache
		if utils.CacheEnabled() {
			if err := utils.GetCache().DeletePattern(context.Background(), "stats:*"); err != nil {
				logger.Warnw("Failed to invalidate statistics cache", "error", err.Error())
			}
		}

		logger.Infow("Absensi processed successfully",
			"nis", nis,
			"id_absen", absensi.IDAbsen,
			"status", absensi.Status,
		)

		c.JSON(http.StatusCreated, AbsensiResponse{
			IDAbsen:   absensi.IDAbsen,
			NIS:       absensi.NIS,
			IDJadwal:  absensi.IDJadwal,
			Tanggal:   absensi.Tanggal.Format("2006-01-02"),
			Status:    absensi.Status,
			Deskripsi: absensi.Deskripsi,
			CreatedAt: absensi.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
}
