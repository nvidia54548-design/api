package handlers

import (
	"context"
	"net/http"
	"path"
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
	Search         string `form:"search"`          // Search by NIS or nama_siswa
	Tingkatan      int    `form:"tingkatan"`       // Filter by tingkatan (10-12)
	Jurusan        string `form:"jurusan"`         // Filter by jurusan (exact match)
	Part           string `form:"part"`            // Filter by part (exact match)
	JK             string `form:"jk"`              // Filter by jenis kelamin (L/P)
	ClassStatus    string `form:"class_status"`    // Filter by class_status
	IDTahunMasuk   int    `form:"id_tahun_masuk"`  // Filter by id_tahun_masuk
	Page           int    `form:"page"`            // Page number (1-based)
	PageSize       int    `form:"page_size"`       // Items per page
	SortBy         string `form:"sort_by"`         // Sort field: nis, nama_siswa, tingkatan, jurusan
	SortDir        string `form:"sort_dir"`        // Sort direction: asc, desc
	IncludeDeleted bool   `form:"include_deleted"` // Include soft-deleted records (admin only)
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
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

type AppliedFilters struct {
	Search         string `json:"search,omitempty"`
	Tingkatan      int    `json:"tingkatan,omitempty"`
	Jurusan        string `json:"jurusan,omitempty"`
	Part           string `json:"part,omitempty"`
	JK             string `json:"jk,omitempty"`
	ClassStatus    string `json:"class_status,omitempty"`
	IDTahunMasuk   int    `json:"id_tahun_masuk,omitempty"`
	SortBy         string `json:"sort_by"`
	SortDir        string `json:"sort_dir"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
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

func extractStudentNIS(c *gin.Context) string {
	if nis := strings.TrimSpace(c.Param("nis")); nis != "" {
		return nis
	}

	fullPath := strings.Trim(c.Request.URL.Path, "/")
	parts := strings.Split(fullPath, "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "siswa" || parts[i] == "students" {
			if i+1 < len(parts) {
				candidate := strings.TrimSpace(parts[i+1])
				if candidate != "" {
					return candidate
				}
			}
		}
	}

	return ""
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
			"nis": true, "nama_siswa": true, "tingkatan": true, "jurusan": true,
		}
		if !validSortFields[filter.SortBy] {
			filter.SortBy = "nama_siswa"
		}

		// Validate sort_dir
		if filter.SortDir != "asc" && filter.SortDir != "desc" {
			filter.SortDir = "asc"
		}

		// Build query
		query := db.Model(&models.Siswa{}).Preload("Account").Preload("KelasRef").Preload("TahunMasuk")

		// Filter soft-deleted unless include_deleted
		if !filter.IncludeDeleted {
			query = query.Where("siswa.deleted_at IS NULL")
		}

		// Role-based filtering for Wali Kelas
		role, _ := c.Get("role")
		if role != nil && role.(string) == "guru" {
			// For guru, check if they are wali_kelas with is_active
			nip, _ := c.Get("nip")
			if nip != nil {
				var wali models.WaliKelas
				if err := db.Where("id_staff = (SELECT id_staff FROM staff WHERE nip = ?) AND is_active = true", nip.(string)).First(&wali).Error; err == nil {
					query = query.Where("id_kelas = ?", wali.IDKelas)
				}
			}
		}

		// Apply search filter (NIS or nama_siswa)
		if filter.Search != "" {
			searchTerm := "%" + filter.Search + "%"
			query = query.Where("nis LIKE ? OR nama_siswa LIKE ?", searchTerm, searchTerm)
		}

		// Apply exact filters
		if filter.Tingkatan > 0 {
			query = query.Joins("LEFT JOIN kelas ON kelas.id_kelas = siswa.id_kelas").Where("kelas.tingkatan = ?", filter.Tingkatan)
		}
		if filter.Jurusan != "" {
			query = query.Joins("LEFT JOIN kelas ON kelas.id_kelas = siswa.id_kelas").Where("kelas.jurusan = ?", filter.Jurusan)
		}
		if filter.Part != "" {
			query = query.Joins("LEFT JOIN kelas ON kelas.id_kelas = siswa.id_kelas").Where("kelas.part = ?", filter.Part)
		}
		if filter.JK != "" {
			query = query.Where("jk = ?", filter.JK)
		}
		if filter.ClassStatus != "" {
			query = query.Where("class_status = ?", filter.ClassStatus)
		}
		if filter.IDTahunMasuk > 0 {
			query = query.Where("id_tahun_masuk = ?", filter.IDTahunMasuk)
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
		var orderClause string
		if filter.SortBy == "tingkatan" || filter.SortBy == "jurusan" {
			orderClause = "kelas." + filter.SortBy + " " + filter.SortDir
		} else {
			orderClause = filter.SortBy + " " + filter.SortDir
		}
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

		c.JSON(http.StatusOK, gin.H{
			"data": siswaList,
			"meta": gin.H{
				"total":       totalItems,
				"total_pages": totalPages,
				"page":        filter.Page,
				"limit":       filter.PageSize,
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
		nis := extractStudentNIS(c)

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "NIS tidak valid",
				},
			})
			return
		}

		var siswa models.Siswa
		if err := db.Preload("Account").Preload("KelasRef").Preload("TahunMasuk").Where("nis = ? AND deleted_at IS NULL", nis).First(&siswa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Student not found",
					"nis", nis,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code": "NOT_FOUND",
						"message": "Siswa tidak ditemukan",
					},
				})
				return
			}
			logger.Errorw("Failed to fetch student",
				"error", err.Error(),
				"nis", nis,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal mengambil data siswa",
				},
			})
			return
		}

		logger.Infow("Student retrieved",
			"nis", nis,
		)

		c.JSON(http.StatusOK, gin.H{
			"data": siswa,
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
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "Format JSON tidak valid",
					"details": []string{err.Error()},
				},
			})
			return
		}

		// Validate enums
		validator := utils.NewValidator()
		validator.Required("nis", siswa.NIS).NIS("nis", siswa.NIS)
		validator.Required("nama_siswa", siswa.NamaSiswa)
		validator.Gender("jk", siswa.JK)
		validator.ClassStatus("class_status", siswa.ClassStatus)
		if siswa.IDKelas != nil {
			// Check if kelas exists
			var kelas models.Kelas
			if err := db.First(&kelas, "id_kelas = ?", *siswa.IDKelas).Error; err != nil {
				validator.AddError("id_kelas", "Kelas tidak ditemukan")
			}
		}
		if siswa.IDTahunMasuk != nil {
			// Check if tahun_masuk exists
			var tahun models.TahunMasuk
			if err := db.First(&tahun, "id_tahun_masuk = ?", *siswa.IDTahunMasuk).Error; err != nil {
				validator.AddError("id_tahun_masuk", "Tahun masuk tidak ditemukan")
			}
		}
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "Data tidak valid",
					"details": validator.Errors(),
				},
			})
			return
		}

		if err := db.Create(&siswa).Error; err != nil {
			logger.Errorw("Failed to create student",
				"error", err.Error(),
				"nis", siswa.NIS,
			)
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code": "CONFLICT",
						"message": "NIS sudah digunakan",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code": "INTERNAL_ERROR",
						"message": "Gagal menambahkan siswa",
					},
				})
			}
			return
		}

		logger.Infow("Student created",
			"nis", siswa.NIS,
			"nama_siswa", siswa.NamaSiswa,
		)

		c.JSON(http.StatusCreated, gin.H{
			"data": siswa,
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
		nis := extractStudentNIS(c)

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "NIS tidak valid",
				},
			})
			return
		}

		var siswa models.Siswa
		if err := c.ShouldBindJSON(&siswa); err != nil {
			logger.Warnw("Invalid JSON format",
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "Format JSON tidak valid",
					"details": []string{err.Error()},
				},
			})
			return
		}

		// Validate enums
		validator := utils.NewValidator()
		validator.Required("nama_siswa", siswa.NamaSiswa)
		validator.Gender("jk", siswa.JK)
		validator.ClassStatus("class_status", siswa.ClassStatus)
		if siswa.IDKelas != nil {
			var kelas models.Kelas
			if err := db.First(&kelas, "id_kelas = ?", *siswa.IDKelas).Error; err != nil {
				validator.AddError("id_kelas", "Kelas tidak ditemukan")
			}
		}
		if siswa.IDTahunMasuk != nil {
			var tahun models.TahunMasuk
			if err := db.First(&tahun, "id_tahun_masuk = ?", *siswa.IDTahunMasuk).Error; err != nil {
				validator.AddError("id_tahun_masuk", "Tahun masuk tidak ditemukan")
			}
		}
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "Data tidak valid",
					"details": validator.Errors(),
				},
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
			if strings.Contains(result.Error.Error(), "duplicate") || strings.Contains(result.Error.Error(), "unique") {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code": "CONFLICT",
						"message": "Data duplikat",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code": "INTERNAL_ERROR",
						"message": "Gagal memperbarui siswa",
					},
				})
			}
			return
		}

		if result.RowsAffected == 0 {
			logger.Warnw("Student not found for update",
				"nis", nis,
			)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code": "NOT_FOUND",
					"message": "Siswa tidak ditemukan",
				},
			})
			return
		}

		logger.Infow("Student updated",
			"nis", nis,
		)

		// Fetch updated siswa
		var updatedSiswa models.Siswa
		if err := db.Preload("Account").Preload("KelasRef").Preload("TahunMasuk").First(&updatedSiswa, "nis = ?", nis).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal mengambil data siswa terbaru",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": updatedSiswa,
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
		nis := extractStudentNIS(c)

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "NIS tidak valid",
				},
			})
			return
		}

		// Soft delete: set deleted_at
		result := db.Model(&models.Siswa{}).Where("nis = ?", nis).Update("deleted_at", "NOW()")
		if result.Error != nil {
			logger.Errorw("Failed to soft delete student",
				"error", result.Error.Error(),
				"nis", nis,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal menghapus siswa",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			logger.Warnw("Student not found for deletion",
				"nis", nis,
			)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code": "NOT_FOUND",
					"message": "Siswa tidak ditemukan",
				},
			})
			return
		}

		logger.Infow("Student soft deleted",
			"nis", nis,
		)

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{"nis": nis},
		})
	}
}

type CreateAbsensiRequest struct {
	IDSemester int     `json:"id_semester" binding:"required"`
	IDTemplate int     `json:"id_template" binding:"required"`
	IDGiliran  *int    `json:"id_giliran,omitempty"`
	Tanggal    string  `json:"tanggal" binding:"required"`
	Status     string  `json:"status" binding:"required"`
}

type AbsensiResponse struct {
	IDAbsen    int     `json:"id_absen"`
	IDSiswa    int     `json:"id_siswa"`
	IDSemester int     `json:"id_semester"`
	IDTemplate int     `json:"id_template"`
	IDGiliran  *int    `json:"id_giliran,omitempty"`
	Tanggal    string  `json:"tanggal"`
	Status     string  `json:"status"`
}

// updateRekapAbsensi updates the rekap_absensi count for the given siswa, semester, jenis, status by delta
func updateRekapAbsensi(tx *gorm.DB, idSiswa, idSemester, idJenis int, status string, delta int) error {
	var rekap models.RekapAbsensi
	err := tx.Where("id_siswa = ? AND id_semester = ? AND id_jenis = ?", idSiswa, idSemester, idJenis).First(&rekap).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new rekap
			rekap = models.RekapAbsensi{
				IDSiswa:    idSiswa,
				IDSemester: idSemester,
				IDJenis:    idJenis,
			}
			if delta > 0 {
				switch status {
				case "hadir":
					rekap.JumlahHadir = delta
				case "izin":
					rekap.JumlahIzin = delta
				case "sakit":
					rekap.JumlahSakit = delta
				case "alpha":
					rekap.JumlahAlpha = delta
				}
			}
			return tx.Create(&rekap).Error
		}
		return err
	}
	// Update existing
	switch status {
	case "hadir":
		rekap.JumlahHadir += delta
	case "izin":
		rekap.JumlahIzin += delta
	case "sakit":
		rekap.JumlahSakit += delta
	case "alpha":
		rekap.JumlahAlpha += delta
	}
	return tx.Save(&rekap).Error
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
		fullPath := c.Request.URL.Path

		logger.Infow("CreateAbsensi called",
			"full_path", fullPath,
			"raw_path_param", c.Param("path"),
		)

		nis := extractStudentNIS(c)
		if nis == "" {
			cleanPath := strings.Trim(fullPath, "/")
			base := path.Base(cleanPath)
			if base == "absensi" || base == "attendances" {
				segments := strings.Split(cleanPath, "/")
				if len(segments) >= 2 {
					nis = segments[len(segments)-2]
				}
			}
		}

		logger.Infow("NIS extracted", "nis", nis)

		if nis == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "NIS tidak valid",
			})
			return
		}

		// Check if student exists
		var siswa models.Siswa
		if err := db.Where("nis = ? AND deleted_at IS NULL", nis).First(&siswa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Student not found for absensi",
					"nis", nis,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code": "NOT_FOUND",
						"message": "Siswa tidak ditemukan",
					},
				})
				return
			}
			logger.Errorw("Failed to check student",
				"nis", nis,
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal memeriksa data siswa",
				},
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
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "Data absensi tidak valid",
					"details": []string{err.Error()},
				},
			})
			return
		}

		// Validate status
		validator := utils.NewValidator()
		validator.AbsensiStatus("status", req.Status)
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "Status absensi tidak valid",
					"details": validator.Errors(),
				},
			})
			return
		}

		// Check if semester exists
		var semester models.SemesterAkademik
		if err := db.First(&semester, "id_semester = ?", req.IDSemester).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Semester not found for absensi",
					"id_semester", req.IDSemester,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code": "NOT_FOUND",
						"message": "Semester akademik tidak ditemukan",
					},
				})
				return
			}
			logger.Errorw("Failed to check semester",
				"id_semester", req.IDSemester,
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal memeriksa semester akademik",
				},
			})
			return
		}

		// Check if template exists
		var template models.JadwalSholatTemplate
		if err := db.Preload("JenisSholat").First(&template, "id_template = ?", req.IDTemplate).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Template not found for absensi",
					"id_template", req.IDTemplate,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code": "NOT_FOUND",
						"message": "Template jadwal sholat tidak ditemukan",
					},
				})
				return
			}
			logger.Errorw("Failed to check template",
				"id_template", req.IDTemplate,
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal memeriksa template jadwal sholat",
				},
			})
			return
		}

		// Check giliran logic
		if template.JenisSholat.ButuhGiliran {
			if req.IDGiliran == nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code": "VALIDATION_ERROR",
						"message": "ID giliran wajib untuk jenis sholat ini",
					},
				})
				return
			}
			// Check if giliran exists and jurusan matches siswa's kelas
			var giliran models.GiliranDhuha
			if err := db.First(&giliran, "id_giliran = ?", *req.IDGiliran).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					logger.Warnw("Giliran not found",
						"id_giliran", *req.IDGiliran,
					)
					c.JSON(http.StatusNotFound, gin.H{
						"error": gin.H{
							"code": "NOT_FOUND",
							"message": "Giliran Dhuha tidak ditemukan",
						},
					})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code": "INTERNAL_ERROR",
						"message": "Gagal memeriksa giliran Dhuha",
					},
				})
				return
			}
			// Check jurusan matches
			if siswa.KelasRef != nil && giliran.Jurusan != siswa.KelasRef.Jurusan {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code": "VALIDATION_ERROR",
						"message": "Jurusan giliran tidak sesuai dengan kelas siswa",
					},
				})
				return
			}
		} else {
			if req.IDGiliran != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code": "VALIDATION_ERROR",
						"message": "ID giliran tidak boleh disertakan untuk jenis sholat ini",
					},
				})
				return
			}
		}

		// Parse tanggal
		tanggal, err := time.Parse("2006-01-02", req.Tanggal)
		if err != nil {
			logger.Warnw("Invalid date format",
				"tanggal", req.Tanggal,
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code": "VALIDATION_ERROR",
					"message": "Format tanggal tidak valid. Gunakan format YYYY-MM-DD",
				},
			})
			return
		}

		// Role-based restrictions for Wali Kelas
		role, _ := c.Get("role")
		userRole := strings.ToLower(strings.TrimSpace(role.(string)))

		if userRole == "wali_kelas" {
			nip, _ := c.Get("nip")
			userNip := nip.(string)

			var wali models.WaliKelas
			if err := db.Where("id_staff = (SELECT id_staff FROM staff WHERE nip = ?) AND is_active = true", userNip).First(&wali).Error; err != nil {
				logger.Errorw("Failed to fetch wali kelas info", "nip", userNip, "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code": "INTERNAL_ERROR",
						"message": "Gagal memverifikasi data wali kelas",
					},
				})
				return
			}

			if wali.IDKelas != *siswa.IDKelas {
				logger.Warnw("Wali kelas attempted to edit student outside their class",
					"guru_nip", userNip,
					"wali_kelas", wali.IDKelas,
					"siswa_nis", nis,
					"siswa_kelas", siswa.IDKelas,
				)
				c.JSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"code": "FORBIDDEN",
						"message": "Anda hanya diperbolehkan mengelola absensi untuk kelas yang ditugaskan",
					},
				})
				return
			}
		}

		// Use transaction for absensi and rekap sync
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Check if an absensi record already exists for this siswa/tanggal/template
		var absensi models.Absensi
		var existingAbsensi models.Absensi
		err = tx.Where("id_siswa = ? AND tanggal = ? AND id_template = ?", siswa.IDSiswa, tanggal, req.IDTemplate).First(&existingAbsensi).Error

		if err == nil {
			// Record exists, update it
			oldStatus := existingAbsensi.Status
			existingAbsensi.Status = req.Status
			existingAbsensi.IDGiliran = req.IDGiliran

			if err := tx.Save(&existingAbsensi).Error; err != nil {
				tx.Rollback()
				logger.Errorw("Failed to update existing absensi", "nis", nis, "error", err.Error())
				if strings.Contains(err.Error(), "duplicate") {
					c.JSON(http.StatusConflict, gin.H{
						"error": gin.H{
							"code": "CONFLICT",
							"message": "Data absensi sudah ada",
						},
					})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": gin.H{
							"code": "INTERNAL_ERROR",
							"message": "Gagal memperbarui data absensi",
						},
					})
				}
				return
			}
			absensi = existingAbsensi
			// Update rekap: decrement old status, increment new status
			if err := updateRekapAbsensi(tx, siswa.IDSiswa, req.IDSemester, template.JenisSholat.IDJenis, oldStatus, -1); err != nil {
				tx.Rollback()
				logger.Errorw("Failed to update rekap on update", "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code": "INTERNAL_ERROR",
						"message": "Gagal memperbarui rekap absensi",
					},
				})
				return
			}
			if err := updateRekapAbsensi(tx, siswa.IDSiswa, req.IDSemester, template.JenisSholat.IDJenis, req.Status, 1); err != nil {
				tx.Rollback()
				logger.Errorw("Failed to update rekap on update", "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code": "INTERNAL_ERROR",
						"message": "Gagal memperbarui rekap absensi",
					},
				})
				return
			}
		} else if err == gorm.ErrRecordNotFound {
			// Record doesn't exist, create new one
			absensi = models.Absensi{
				IDSiswa:    siswa.IDSiswa,
				IDSemester: req.IDSemester,
				IDTemplate: req.IDTemplate,
				IDGiliran:  req.IDGiliran,
				Tanggal:    tanggal,
				Status:     req.Status,
			}

			if err := tx.Create(&absensi).Error; err != nil {
				tx.Rollback()
				logger.Errorw("Failed to create absensi", "nis", nis, "error", err.Error())
				if strings.Contains(err.Error(), "duplicate") {
					c.JSON(http.StatusConflict, gin.H{
						"error": gin.H{
							"code": "CONFLICT",
							"message": "Data absensi sudah ada",
						},
					})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": gin.H{
							"code": "INTERNAL_ERROR",
							"message": "Gagal membuat absensi",
						},
					})
				}
				return
			}
			// Update rekap: increment new status
			if err := updateRekapAbsensi(tx, siswa.IDSiswa, req.IDSemester, template.JenisSholat.IDJenis, req.Status, 1); err != nil {
				tx.Rollback()
				logger.Errorw("Failed to update rekap on create", "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code": "INTERNAL_ERROR",
						"message": "Gagal memperbarui rekap absensi",
					},
				})
				return
			}
		} else {
			tx.Rollback()
			logger.Errorw("Failed to check existing absensi", "nis", nis, "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal mengecek data absensi",
				},
			})
			return
		}

		if err := tx.Commit().Error; err != nil {
			logger.Errorw("Failed to commit transaction", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code": "INTERNAL_ERROR",
					"message": "Gagal menyimpan perubahan",
				},
			})
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

		c.JSON(http.StatusCreated, gin.H{
			"data": AbsensiResponse{
				IDAbsen:    absensi.IDAbsen,
				IDSiswa:    absensi.IDSiswa,
				IDSemester: absensi.IDSemester,
				IDTemplate: absensi.IDTemplate,
				IDGiliran:  absensi.IDGiliran,
				Tanggal:    absensi.Tanggal.Format("2006-01-02"),
				Status:     absensi.Status,
			},
		})
	}
}
