package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

// TahunMasuk Handlers

type TahunMasukFilterRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

type TahunMasukCreateRequest struct {
	Tahun    string `json:"tahun" binding:"required"`
	IsActive bool   `json:"is_active"`
}

type TahunMasukUpdateRequest struct {
	Tahun    string `json:"tahun"`
	IsActive bool   `json:"is_active"`
}

func GetTahunMasuk(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TahunMasukFilterRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid query parameters",
					"details": []string{err.Error()},
				},
			})
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		if req.SortBy == "" {
			req.SortBy = "id_tahun_masuk"
		}
		if req.SortDir == "" {
			req.SortDir = "desc"
		}

		validSortFields := map[string]bool{
			"id_tahun_masuk": true, "tahun": true, "is_active": true,
		}
		if !validSortFields[req.SortBy] {
			req.SortBy = "id_tahun_masuk"
		}
		if req.SortDir != "asc" && req.SortDir != "desc" {
			req.SortDir = "desc"
		}

		var tahunMasuk []models.TahunMasuk
		var totalItems int64

		query := db.Model(&models.TahunMasuk{})

		if err := query.Count(&totalItems).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to count records",
				},
			})
			return
		}

		offset := (req.Page - 1) * req.PageSize
		order := req.SortBy + " " + req.SortDir
		if err := query.Order(order).Offset(offset).Limit(req.PageSize).Find(&tahunMasuk).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve records",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": tahunMasuk,
			"meta": gin.H{
				"total": totalItems,
				"page":  req.Page,
				"limit": req.PageSize,
			},
		})
	}
}

func CreateTahunMasuk(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TahunMasukCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		validator := utils.NewValidator()
		validator.Tahun("tahun", req.Tahun)
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid data",
					"details": validator.Errors(),
				},
			})
			return
		}

		tahunMasuk := models.TahunMasuk{
			Tahun:    req.Tahun,
			IsActive: req.IsActive,
		}

		if err := db.Create(&tahunMasuk).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code":    "CONFLICT",
						"message": "Tahun sudah ada",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to create record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"data": tahunMasuk,
		})
	}
}

func GetTahunMasukByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var tahunMasuk models.TahunMasuk
		if err := db.First(&tahunMasuk, "id_tahun_masuk = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Tahun masuk not found",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to retrieve record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": tahunMasuk,
		})
	}
}

func UpdateTahunMasuk(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var req TahunMasukUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		validator := utils.NewValidator()
		if req.Tahun != "" {
			validator.Tahun("tahun", req.Tahun)
		}
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid data",
					"details": validator.Errors(),
				},
			})
			return
		}

		result := db.Model(&models.TahunMasuk{}).Where("id_tahun_masuk = ?", id).Updates(req)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Tahun masuk not found",
				},
			})
			return
		}

		var updated models.TahunMasuk
		if err := db.First(&updated, "id_tahun_masuk = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve updated record",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": updated,
		})
	}
}

func DeleteTahunMasuk(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		// Check if referenced by siswa
		var count int64
		if err := db.Model(&models.Siswa{}).Where("id_tahun_masuk = ?", id).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to check references",
				},
			})
			return
		}

		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "CONFLICT",
					"message": "Cannot delete: still referenced by students",
				},
			})
			return
		}

		result := db.Delete(&models.TahunMasuk{}, "id_tahun_masuk = ?", id)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Tahun masuk not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{"id": id},
		})
	}
}

// SemesterAkademik Handlers

type SemesterAkademikFilterRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

type SemesterAkademikCreateRequest struct {
	IDTahun       int    `json:"id_tahun" binding:"required"`
	Semester      int    `json:"semester" binding:"required"`
	TanggalMulai  string `json:"tanggal_mulai" binding:"required"`
	TanggalSelesai string `json:"tanggal_selesai" binding:"required"`
}

type SemesterAkademikUpdateRequest struct {
	IDTahun        int    `json:"id_tahun"`
	Semester       int    `json:"semester"`
	TanggalMulai   string `json:"tanggal_mulai"`
	TanggalSelesai string `json:"tanggal_selesai"`
}

func GetSemesterAkademik(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SemesterAkademikFilterRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid query parameters",
					"details": []string{err.Error()},
				},
			})
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		if req.SortBy == "" {
			req.SortBy = "id_semester"
		}
		if req.SortDir == "" {
			req.SortDir = "desc"
		}

		validSortFields := map[string]bool{
			"id_semester": true, "id_tahun": true, "semester": true,
		}
		if !validSortFields[req.SortBy] {
			req.SortBy = "id_semester"
		}
		if req.SortDir != "asc" && req.SortDir != "desc" {
			req.SortDir = "desc"
		}

		var semesters []models.SemesterAkademik
		var totalItems int64

		query := db.Model(&models.SemesterAkademik{}).Preload("TahunMasuk")

		if err := query.Count(&totalItems).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to count records",
				},
			})
			return
		}

		offset := (req.Page - 1) * req.PageSize
		order := req.SortBy + " " + req.SortDir
		if err := query.Order(order).Offset(offset).Limit(req.PageSize).Find(&semesters).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve records",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": semesters,
			"meta": gin.H{
				"total": totalItems,
				"page":  req.Page,
				"limit": req.PageSize,
			},
		})
	}
}

func CreateSemesterAkademik(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SemesterAkademikCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		// Parse dates
		tanggalMulai, err := time.Parse("2006-01-02", req.TanggalMulai)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid tanggal_mulai format",
				},
			})
			return
		}

		tanggalSelesai, err := time.Parse("2006-01-02", req.TanggalSelesai)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid tanggal_selesai format",
				},
			})
			return
		}

		validator := utils.NewValidator()
		validator.Semester("semester", req.Semester)
		validator.DateRange("tanggal_mulai", "tanggal_selesai", req.TanggalMulai, req.TanggalSelesai)
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid data",
					"details": validator.Errors(),
				},
			})
			return
		}

		// Check tahun exists
		var tahun models.TahunMasuk
		if err := db.First(&tahun, "id_tahun_masuk = ?", req.IDTahun).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Tahun masuk not found",
				},
			})
			return
		}

		semester := models.SemesterAkademik{
			IDTahun:       req.IDTahun,
			Semester:      req.Semester,
			TanggalMulai:  tanggalMulai,
			TanggalSelesai: tanggalSelesai,
		}

		if err := db.Create(&semester).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code":    "CONFLICT",
						"message": "Semester already exists for this tahun",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to create record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"data": semester,
		})
	}
}

func GetSemesterAkademikByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var semester models.SemesterAkademik
		if err := db.Preload("TahunMasuk").First(&semester, "id_semester = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Semester not found",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to retrieve record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": semester,
		})
	}
}

func UpdateSemesterAkademik(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var req SemesterAkademikUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		updates := make(map[string]interface{})
		if req.IDTahun != 0 {
			updates["id_tahun"] = req.IDTahun
		}
		if req.Semester != 0 {
			validator := utils.NewValidator()
			validator.Semester("semester", req.Semester)
			if validator.HasErrors() {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid semester",
						"details": validator.Errors(),
					},
				})
				return
			}
			updates["semester"] = req.Semester
		}
		if req.TanggalMulai != "" {
			tanggalMulai, err := time.Parse("2006-01-02", req.TanggalMulai)
			if err != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid tanggal_mulai format",
					},
				})
				return
			}
			updates["tanggal_mulai"] = tanggalMulai
		}
		if req.TanggalSelesai != "" {
			tanggalSelesai, err := time.Parse("2006-01-02", req.TanggalSelesai)
			if err != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid tanggal_selesai format",
					},
				})
				return
			}
			updates["tanggal_selesai"] = tanggalSelesai
		}

		// Validate date range if both dates provided
		if req.TanggalMulai != "" && req.TanggalSelesai != "" {
			validator := utils.NewValidator()
			validator.DateRange("tanggal_mulai", "tanggal_selesai", req.TanggalMulai, req.TanggalSelesai)
			if validator.HasErrors() {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid date range",
						"details": validator.Errors(),
					},
				})
				return
			}
		}

		result := db.Model(&models.SemesterAkademik{}).Where("id_semester = ?", id).Updates(updates)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Semester not found",
				},
			})
			return
		}

		var updated models.SemesterAkademik
		if err := db.Preload("TahunMasuk").First(&updated, "id_semester = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve updated record",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": updated,
		})
	}
}

func DeleteSemesterAkademik(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		// Check if referenced by absensi
		var count int64
		if err := db.Model(&models.Absensi{}).Where("id_semester = ?", id).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to check references",
				},
			})
			return
		}

		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "CONFLICT",
					"message": "Cannot delete: still referenced by attendance records",
				},
			})
			return
		}

		result := db.Delete(&models.SemesterAkademik{}, "id_semester = ?", id)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Semester not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{"id": id},
		})
	}
}

// WaliKelas Handlers

type WaliKelasFilterRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

type WaliKelasCreateRequest struct {
	IDKelas     int    `json:"id_kelas" binding:"required"`
	IDStaff     int    `json:"id_staff" binding:"required"`
	IsActive    bool   `json:"is_active"`
	BerlakuMulai string `json:"berlaku_mulai" binding:"required"`
}

type WaliKelasUpdateRequest struct {
	IDKelas      int    `json:"id_kelas"`
	IDStaff      int    `json:"id_staff"`
	IsActive     bool   `json:"is_active"`
	BerlakuMulai string `json:"berlaku_mulai"`
}

func GetWaliKelas(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req WaliKelasFilterRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid query parameters",
					"details": []string{err.Error()},
				},
			})
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		if req.SortBy == "" {
			req.SortBy = "id_wali"
		}
		if req.SortDir == "" {
			req.SortDir = "desc"
		}

		validSortFields := map[string]bool{
			"id_wali": true, "id_kelas": true, "id_staff": true,
		}
		if !validSortFields[req.SortBy] {
			req.SortBy = "id_wali"
		}
		if req.SortDir != "asc" && req.SortDir != "desc" {
			req.SortDir = "desc"
		}

		var waliKelas []models.WaliKelas
		var totalItems int64

		query := db.Model(&models.WaliKelas{}).Preload("Kelas").Preload("Staff")

		if err := query.Count(&totalItems).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to count records",
				},
			})
			return
		}

		offset := (req.Page - 1) * req.PageSize
		order := req.SortBy + " " + req.SortDir
		if err := query.Order(order).Offset(offset).Limit(req.PageSize).Find(&waliKelas).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve records",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": waliKelas,
			"meta": gin.H{
				"total": totalItems,
				"page":  req.Page,
				"limit": req.PageSize,
			},
		})
	}
}

func CreateWaliKelas(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req WaliKelasCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		berlakuMulai, err := time.Parse("2006-01-02", req.BerlakuMulai)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid berlaku_mulai format",
				},
			})
			return
		}

		// Check kelas exists
		var kelas models.Kelas
		if err := db.First(&kelas, "id_kelas = ?", req.IDKelas).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Kelas not found",
				},
			})
			return
		}

		// Check staff exists
		var staff models.Staff
		if err := db.First(&staff, "id_staff = ?", req.IDStaff).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Staff not found",
				},
			})
			return
		}

		waliKelas := models.WaliKelas{
			IDKelas:      req.IDKelas,
			IDStaff:      req.IDStaff,
			IsActive:     req.IsActive,
			BerlakuMulai: berlakuMulai,
		}

		if err := db.Create(&waliKelas).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code":    "CONFLICT",
						"message": "Kelas already has a wali kelas",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to create record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"data": waliKelas,
		})
	}
}

func GetWaliKelasByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var waliKelas models.WaliKelas
		if err := db.Preload("Kelas").Preload("Staff").First(&waliKelas, "id_wali = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Wali kelas not found",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to retrieve record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": waliKelas,
		})
	}
}

func UpdateWaliKelas(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var req WaliKelasUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		updates := make(map[string]interface{})
		if req.IDKelas != 0 {
			updates["id_kelas"] = req.IDKelas
		}
		if req.IDStaff != 0 {
			updates["id_staff"] = req.IDStaff
		}
		updates["is_active"] = req.IsActive
		if req.BerlakuMulai != "" {
			berlakuMulai, err := time.Parse("2006-01-02", req.BerlakuMulai)
			if err != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid berlaku_mulai format",
					},
				})
				return
			}
			updates["berlaku_mulai"] = berlakuMulai
		}

		result := db.Model(&models.WaliKelas{}).Where("id_wali = ?", id).Updates(updates)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Wali kelas not found",
				},
			})
			return
		}

		var updated models.WaliKelas
		if err := db.Preload("Kelas").Preload("Staff").First(&updated, "id_wali = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve updated record",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": updated,
		})
	}
}

func DeleteWaliKelas(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		result := db.Delete(&models.WaliKelas{}, "id_wali = ?", id)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Wali kelas not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{"id": id},
		})
	}
}

// JenisSholat Handlers

type JenisSholatFilterRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

type JenisSholatCreateRequest struct {
	NamaJenis    string `json:"nama_jenis" binding:"required"`
	ButuhGiliran bool   `json:"butuh_giliran"`
}

type JenisSholatUpdateRequest struct {
	NamaJenis    string `json:"nama_jenis"`
	ButuhGiliran bool   `json:"butuh_giliran"`
}

func GetJenisSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req JenisSholatFilterRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid query parameters",
					"details": []string{err.Error()},
				},
			})
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		if req.SortBy == "" {
			req.SortBy = "id_jenis"
		}
		if req.SortDir == "" {
			req.SortDir = "asc"
		}

		validSortFields := map[string]bool{
			"id_jenis": true, "nama_jenis": true,
		}
		if !validSortFields[req.SortBy] {
			req.SortBy = "id_jenis"
		}
		if req.SortDir != "asc" && req.SortDir != "desc" {
			req.SortDir = "asc"
		}

		var jenisSholat []models.JenisSholat
		var totalItems int64

		query := db.Model(&models.JenisSholat{})

		if err := query.Count(&totalItems).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to count records",
				},
			})
			return
		}

		offset := (req.Page - 1) * req.PageSize
		order := req.SortBy + " " + req.SortDir
		if err := query.Order(order).Offset(offset).Limit(req.PageSize).Find(&jenisSholat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve records",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": jenisSholat,
			"meta": gin.H{
				"total": totalItems,
				"page":  req.Page,
				"limit": req.PageSize,
			},
		})
	}
}

func CreateJenisSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req JenisSholatCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		validator := utils.NewValidator()
		validator.NamaJenis("nama_jenis", req.NamaJenis)
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid data",
					"details": validator.Errors(),
				},
			})
			return
		}

		jenisSholat := models.JenisSholat{
			NamaJenis:    req.NamaJenis,
			ButuhGiliran: req.ButuhGiliran,
		}

		if err := db.Create(&jenisSholat).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code":    "CONFLICT",
						"message": "Jenis sholat already exists",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to create record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"data": jenisSholat,
		})
	}
}

func GetJenisSholatByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var jenisSholat models.JenisSholat
		if err := db.First(&jenisSholat, "id_jenis = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Jenis sholat not found",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to retrieve record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": jenisSholat,
		})
	}
}

func UpdateJenisSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var req JenisSholatUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		updates := make(map[string]interface{})
		if req.NamaJenis != "" {
			validator := utils.NewValidator()
			validator.NamaJenis("nama_jenis", req.NamaJenis)
			if validator.HasErrors() {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid nama_jenis",
						"details": validator.Errors(),
					},
				})
				return
			}
			updates["nama_jenis"] = req.NamaJenis
		}
		updates["butuh_giliran"] = req.ButuhGiliran

		result := db.Model(&models.JenisSholat{}).Where("id_jenis = ?", id).Updates(updates)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Jenis sholat not found",
				},
			})
			return
		}

		var updated models.JenisSholat
		if err := db.First(&updated, "id_jenis = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve updated record",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": updated,
		})
	}
}

func DeleteJenisSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		// Check if referenced
		var count int64
		if err := db.Model(&models.JadwalSholatTemplate{}).Where("id_jenis = ?", id).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to check references",
				},
			})
			return
		}

		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "CONFLICT",
					"message": "Cannot delete: still referenced by templates",
				},
			})
			return
		}

		result := db.Delete(&models.JenisSholat{}, "id_jenis = ?", id)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Jenis sholat not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{"id": id},
		})
	}
}

// WaktuSholat Handlers

type WaktuSholatFilterRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

type WaktuSholatCreateRequest struct {
	IDJenis       int    `json:"id_jenis" binding:"required"`
	WaktuMulai    string `json:"waktu_mulai" binding:"required"`
	WaktuSelesai  string `json:"waktu_selesai" binding:"required"`
	BerlakuMulai  string `json:"berlaku_mulai" binding:"required"`
	BerlakuSampai string `json:"berlaku_sampai"`
}

type WaktuSholatUpdateRequest struct {
	IDJenis       int    `json:"id_jenis"`
	WaktuMulai    string `json:"waktu_mulai"`
	WaktuSelesai  string `json:"waktu_selesai"`
	BerlakuMulai  string `json:"berlaku_mulai"`
	BerlakuSampai string `json:"berlaku_sampai"`
}

func GetWaktuSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req WaktuSholatFilterRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid query parameters",
					"details": []string{err.Error()},
				},
			})
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		if req.SortBy == "" {
			req.SortBy = "id_waktu"
		}
		if req.SortDir == "" {
			req.SortDir = "desc"
		}

		validSortFields := map[string]bool{
			"id_waktu": true, "id_jenis": true, "berlaku_mulai": true,
		}
		if !validSortFields[req.SortBy] {
			req.SortBy = "id_waktu"
		}
		if req.SortDir != "asc" && req.SortDir != "desc" {
			req.SortDir = "desc"
		}

		var waktuSholat []models.WaktuSholat
		var totalItems int64

		query := db.Model(&models.WaktuSholat{}).Preload("JenisSholat")

		if err := query.Count(&totalItems).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to count records",
				},
			})
			return
		}

		offset := (req.Page - 1) * req.PageSize
		order := req.SortBy + " " + req.SortDir
		if err := query.Order(order).Offset(offset).Limit(req.PageSize).Find(&waktuSholat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve records",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": waktuSholat,
			"meta": gin.H{
				"total": totalItems,
				"page":  req.Page,
				"limit": req.PageSize,
			},
		})
	}
}

func CreateWaktuSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req WaktuSholatCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		berlakuMulai, err := time.Parse("2006-01-02", req.BerlakuMulai)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid berlaku_mulai format",
				},
			})
			return
		}

		var berlakuSampai *time.Time
		if req.BerlakuSampai != "" {
			parsed, err := time.Parse("2006-01-02", req.BerlakuSampai)
			if err != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid berlaku_sampai format",
					},
				})
				return
			}
			berlakuSampai = &parsed
		}

		validator := utils.NewValidator()
		validator.Time("waktu_mulai", req.WaktuMulai)
		validator.Time("waktu_selesai", req.WaktuSelesai)
		validator.TimeRange("waktu_mulai", "waktu_selesai", req.WaktuMulai, req.WaktuSelesai)
		if berlakuSampai != nil {
			validator.DateRange("berlaku_mulai", "berlaku_sampai", req.BerlakuMulai, req.BerlakuSampai)
		}
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid data",
					"details": validator.Errors(),
				},
			})
			return
		}

		// Check jenis exists
		var jenis models.JenisSholat
		if err := db.First(&jenis, "id_jenis = ?", req.IDJenis).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Jenis sholat not found",
				},
			})
			return
		}

		waktuSholat := models.WaktuSholat{
			IDJenis:       req.IDJenis,
			WaktuMulai:    req.WaktuMulai,
			WaktuSelesai:  req.WaktuSelesai,
			BerlakuMulai:  berlakuMulai,
			BerlakuSampai: berlakuSampai,
		}

		if err := db.Create(&waktuSholat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create record",
				},
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"data": waktuSholat,
		})
	}
}

func GetWaktuSholatByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var waktuSholat models.WaktuSholat
		if err := db.Preload("JenisSholat").First(&waktuSholat, "id_waktu = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Waktu sholat not found",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to retrieve record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": waktuSholat,
		})
	}
}

func UpdateWaktuSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var req WaktuSholatUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		updates := make(map[string]interface{})
		if req.IDJenis != 0 {
			updates["id_jenis"] = req.IDJenis
		}
		if req.WaktuMulai != "" {
			validator := utils.NewValidator()
			validator.Time("waktu_mulai", req.WaktuMulai)
			if validator.HasErrors() {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid waktu_mulai",
						"details": validator.Errors(),
					},
				})
				return
			}
			updates["waktu_mulai"] = req.WaktuMulai
		}
		if req.WaktuSelesai != "" {
			validator := utils.NewValidator()
			validator.Time("waktu_selesai", req.WaktuSelesai)
			if validator.HasErrors() {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid waktu_selesai",
						"details": validator.Errors(),
					},
				})
				return
			}
			updates["waktu_selesai"] = req.WaktuSelesai
		}
		if req.BerlakuMulai != "" {
			berlakuMulai, err := time.Parse("2006-01-02", req.BerlakuMulai)
			if err != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid berlaku_mulai format",
					},
				})
				return
			}
			updates["berlaku_mulai"] = berlakuMulai
		}
		if req.BerlakuSampai != "" {
			berlakuSampai, err := time.Parse("2006-01-02", req.BerlakuSampai)
			if err != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid berlaku_sampai format",
					},
				})
				return
			}
			updates["berlaku_sampai"] = &berlakuSampai
		}

		result := db.Model(&models.WaktuSholat{}).Where("id_waktu = ?", id).Updates(updates)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Waktu sholat not found",
				},
			})
			return
		}

		var updated models.WaktuSholat
		if err := db.Preload("JenisSholat").First(&updated, "id_waktu = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve updated record",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": updated,
		})
	}
}

func DeleteWaktuSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		result := db.Delete(&models.WaktuSholat{}, "id_waktu = ?", id)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Waktu sholat not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{"id": id},
		})
	}
}

// GiliranDhuha Handlers

type GiliranDhuhaFilterRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

type GiliranDhuhaCreateRequest struct {
	Jurusan string `json:"jurusan" binding:"required"`
	Hari    string `json:"hari" binding:"required"`
}

type GiliranDhuhaUpdateRequest struct {
	Jurusan string `json:"jurusan"`
	Hari    string `json:"hari"`
}

func GetGiliranDhuha(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GiliranDhuhaFilterRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid query parameters",
					"details": []string{err.Error()},
				},
			})
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		if req.SortBy == "" {
			req.SortBy = "id_giliran"
		}
		if req.SortDir == "" {
			req.SortDir = "asc"
		}

		validSortFields := map[string]bool{
			"id_giliran": true, "jurusan": true, "hari": true,
		}
		if !validSortFields[req.SortBy] {
			req.SortBy = "id_giliran"
		}
		if req.SortDir != "asc" && req.SortDir != "desc" {
			req.SortDir = "asc"
		}

		var giliranDhuha []models.GiliranDhuha
		var totalItems int64

		query := db.Model(&models.GiliranDhuha{})

		if err := query.Count(&totalItems).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to count records",
				},
			})
			return
		}

		offset := (req.Page - 1) * req.PageSize
		order := req.SortBy + " " + req.SortDir
		if err := query.Order(order).Offset(offset).Limit(req.PageSize).Find(&giliranDhuha).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve records",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": giliranDhuha,
			"meta": gin.H{
				"total": totalItems,
				"page":  req.Page,
				"limit": req.PageSize,
			},
		})
	}
}

func CreateGiliranDhuha(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GiliranDhuhaCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		validator := utils.NewValidator()
		validator.Hari("hari", req.Hari)
		if validator.HasErrors() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid data",
					"details": validator.Errors(),
				},
			})
			return
		}

		giliranDhuha := models.GiliranDhuha{
			Jurusan: req.Jurusan,
			Hari:    req.Hari,
		}

		if err := db.Create(&giliranDhuha).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code":    "CONFLICT",
						"message": "Giliran already exists for this hari and jurusan",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to create record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"data": giliranDhuha,
		})
	}
}

func GetGiliranDhuhaByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var giliranDhuha models.GiliranDhuha
		if err := db.First(&giliranDhuha, "id_giliran = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "Giliran Dhuha not found",
					},
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to retrieve record",
					},
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": giliranDhuha,
		})
	}
}

func UpdateGiliranDhuha(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		var req GiliranDhuhaUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request body",
					"details": []string{err.Error()},
				},
			})
			return
		}

		updates := make(map[string]interface{})
		if req.Jurusan != "" {
			updates["jurusan"] = req.Jurusan
		}
		if req.Hari != "" {
			validator := utils.NewValidator()
			validator.Hari("hari", req.Hari)
			if validator.HasErrors() {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "Invalid hari",
						"details": validator.Errors(),
					},
				})
				return
			}
			updates["hari"] = req.Hari
		}

		result := db.Model(&models.GiliranDhuha{}).Where("id_giliran = ?", id).Updates(updates)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to update record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Giliran Dhuha not found",
				},
			})
			return
		}

		var updated models.GiliranDhuha
		if err := db.First(&updated, "id_giliran = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to retrieve updated record",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": updated,
		})
	}
}

func DeleteGiliranDhuha(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid ID",
				},
			})
			return
		}

		// Check if referenced by absensi
		var count int64
		if err := db.Model(&models.Absensi{}).Where("id_giliran = ?", id).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to check references",
				},
			})
			return
		}

		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "CONFLICT",
					"message": "Cannot delete: still referenced by attendance records",
				},
			})
			return
		}

		result := db.Delete(&models.GiliranDhuha{}, "id_giliran = ?", id)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to delete record",
				},
			})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Giliran Dhuha not found",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{"id": id},
		})
	}
}