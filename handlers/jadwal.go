package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

// JadwalSholatFilterRequest represents query parameters for filtering jadwal sholat
type JadwalSholatFilterRequest struct {
	Hari        string `form:"hari"`         // Filter by hari
	JenisSholat string `form:"jenis_sholat"` // Filter by jenis sholat
	Jurusan     string `form:"jurusan"`      // Filter by jurusan
	Page        int    `form:"page"`         // Page number (1-based)
	PageSize    int    `form:"page_size"`    // Items per page
	SortBy      string `form:"sort_by"`      // Sort field: id_jadwal, hari, jenis_sholat, waktu_mulai
	SortDir     string `form:"sort_dir"`     // Sort direction: asc, desc
}

type JadwalSholatListResponse struct {
	Message string                `json:"message"`
	Data    []models.JadwalSholat `json:"data"`
}

type JadwalSholatListPaginatedResponse struct {
	Message    string                `json:"message"`
	Data       []models.JadwalSholat `json:"data"`
	Pagination PaginationMeta        `json:"pagination"`
	Filters    JadwalSholatFilters   `json:"filters"`
}

type JadwalSholatFilters struct {
	Hari        string `json:"hari,omitempty"`
	JenisSholat string `json:"jenis_sholat,omitempty"`
	Jurusan     string `json:"jurusan,omitempty"`
}

type JadwalSholatResponse struct {
	Message string              `json:"message"`
	Data    models.JadwalSholat `json:"data"`
}

type JadwalSholatCreateRequest struct {
	Hari         string `json:"hari" binding:"required"`
	JenisSholat  string `json:"jenis_sholat" binding:"required"`
	WaktuMulai   string `json:"waktu_mulai" binding:"required"`
	WaktuSelesai string `json:"waktu_selesai" binding:"required"`
	Jurusan      string `json:"jurusan"`
	Kelas        string `json:"kelas"`
}

type JadwalSholatUpdateRequest struct {
	Hari         string `json:"hari"`
	JenisSholat  string `json:"jenis_sholat"`
	WaktuMulai   string `json:"waktu_mulai"`
	WaktuSelesai string `json:"waktu_selesai"`
	Jurusan      string `json:"jurusan"`
	Kelas        string `json:"kelas"`
}

type JadwalSholatErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type JadwalDhuhaTodayResponse struct {
	Message string                 `json:"message"`
	Data    []JadwalDhuhaByJurusan `json:"data"`
}

type JadwalDhuhaByJurusan struct {
	Jurusan string                `json:"jurusan"`
	Jadwal  []models.JadwalSholat `json:"jadwal"`
}

// getCurrentDayInIndonesian returns the current day in Indonesian
// isDayInRange checks if a specific day is within a range or matches exactly
func isDayInRange(targetDay string, rangeStr string) bool {
	if targetDay == "" || rangeStr == "" {
		return false
	}
	if rangeStr == "Senin-Minggu" || rangeStr == "Semua Hari" || rangeStr == "ALL_DAYS" {
		return true
	}
	if targetDay == rangeStr {
		return true
	}
	// Simple range checks
	if rangeStr == "Senin-Jumat" {
		weekdays := map[string]bool{"Senin": true, "Selasa": true, "Rabu": true, "Kamis": true, "Jumat": true}
		return weekdays[targetDay]
	}
	if rangeStr == "Senin-Kamis" {
		weekdays := map[string]bool{"Senin": true, "Selasa": true, "Rabu": true, "Kamis": true}
		return weekdays[targetDay]
	}
	return false
}

// GetJadwalDhuhaToday retrieves Dhuha prayer schedules for today, all jurusan
// @Summary Get Dhuha prayer schedules for today
// @Description Retrieve Dhuha prayer schedules for the current day, all scheduled jurusan
// @Tags jadwal-sholat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} JadwalDhuhaTodayResponse
// @Failure 500 {object} JadwalSholatErrorResponse
// @Router /jadwal-sholat/dhuha-today [get]
func GetJadwalDhuhaToday(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		hari := utils.GetIndonesianDayName(utils.GetJakartaTime())

		var jadwals []models.JadwalSholat
		if err := db.Where("jenis_sholat = ? AND (hari = ? OR hari = 'Senin-Minggu' OR hari = 'Semua Hari' OR (hari = 'Senin-Jumat' AND ? NOT IN ('Sabtu', 'Minggu')) OR (hari = 'Senin-Kamis' AND ? NOT IN ('Jumat', 'Sabtu', 'Minggu')))", "Dhuha", hari, hari, hari).Find(&jadwals).Error; err != nil {
			logger.Error("Failed to get jadwal dhuha", "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to retrieve jadwal dhuha",
			})
			return
		}

		// Collect unique jurusan — return ALL scheduled jurusan (no limit)
		jurusanMap := make(map[string][]models.JadwalSholat)
		jurusanOrder := []string{} // preserve insertion order
		for _, jadwal := range jadwals {
			if _, exists := jurusanMap[jadwal.Jurusan]; !exists {
				jurusanOrder = append(jurusanOrder, jadwal.Jurusan)
			}
			jurusanMap[jadwal.Jurusan] = append(jurusanMap[jadwal.Jurusan], jadwal)
		}

		var data []JadwalDhuhaByJurusan
		for _, jurusan := range jurusanOrder {
			data = append(data, JadwalDhuhaByJurusan{
				Jurusan: jurusan,
				Jadwal:  jurusanMap[jurusan],
			})
		}

		c.JSON(http.StatusOK, JadwalDhuhaTodayResponse{
			Message: "Jadwal Dhuha hari ini berhasil diambil",
			Data:    data,
		})
	}
}

// GetJadwalSholat retrieves all jadwal sholat with optional filtering and pagination
// @Summary Get all jadwal sholat
// @Description Retrieve a list of jadwal sholat with optional filters
// @Tags jadwal-sholat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param hari query string false "Filter by hari"
// @Param jenis_sholat query string false "Filter by jenis sholat"
// @Param jurusan query string false "Filter by jurusan"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Items per page" default(10)
// @Param sort_by query string false "Sort field" default(id_jadwal)
// @Param sort_dir query string false "Sort direction" default(asc)
// @Success 200 {object} JadwalSholatListPaginatedResponse
// @Failure 500 {object} JadwalSholatErrorResponse
// @Router /jadwal-sholat [get]
func GetJadwalSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req JadwalSholatFilterRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Error("Invalid query parameters", "error", err)
			c.JSON(http.StatusBadRequest, JadwalSholatErrorResponse{
				Error:   "INVALID_REQUEST",
				Message: "Invalid query parameters",
			})
			return
		}

		// Set defaults
		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 10
		}
		if req.SortBy == "" {
			req.SortBy = "id_jadwal"
		}
		if req.SortDir == "" {
			req.SortDir = "asc"
		}

		// Validate sort field
		validSortFields := map[string]bool{
			"id_jadwal": true, "hari": true, "jenis_sholat": true, "waktu_mulai": true,
		}
		if !validSortFields[req.SortBy] {
			req.SortBy = "id_jadwal"
		}
		if req.SortDir != "asc" && req.SortDir != "desc" {
			req.SortDir = "asc"
		}

		var jadwalSholat []models.JadwalSholat
		var totalItems int64

		query := db.Model(&models.JadwalSholat{})

		// Apply filters
		if req.Hari != "" {
			// If searching for a specific day, we need to find exact matches
			// AND ranges that include that day
			query = query.Where("hari = ? OR hari = 'Senin-Minggu' OR hari = 'Semua Hari' OR (hari = 'Senin-Jumat' AND ? NOT IN ('Sabtu', 'Minggu')) OR (hari = 'Senin-Kamis' AND ? NOT IN ('Jumat', 'Sabtu', 'Minggu'))", req.Hari, req.Hari, req.Hari)
		}
		if req.JenisSholat != "" {
			query = query.Where("jenis_sholat = ?", req.JenisSholat)
		}
		if req.Jurusan != "" {
			query = query.Where("jurusan = ?", req.Jurusan)
		}

		// Get total count
		if err := query.Count(&totalItems).Error; err != nil {
			logger.Error("Failed to count jadwal sholat", "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to retrieve jadwal sholat count",
			})
			return
		}

		// Calculate pagination
		offset := (req.Page - 1) * req.PageSize
		totalPages := int((totalItems + int64(req.PageSize) - 1) / int64(req.PageSize))

		// Apply sorting and pagination
		order := req.SortBy + " " + req.SortDir
		if err := query.Order(order).Offset(offset).Limit(req.PageSize).Find(&jadwalSholat).Error; err != nil {
			logger.Error("Failed to retrieve jadwal sholat", "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to retrieve jadwal sholat",
			})
			return
		}

		// Build applied filters
		filters := JadwalSholatFilters{
			Hari:        req.Hari,
			JenisSholat: req.JenisSholat,
			Jurusan:     req.Jurusan,
		}

		c.JSON(http.StatusOK, JadwalSholatListPaginatedResponse{
			Message: "Jadwal sholat retrieved successfully",
			Data:    jadwalSholat,
			Pagination: PaginationMeta{
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalItems: totalItems,
				TotalPages: totalPages,
			},
			Filters: filters,
		})
	}
}

// GetJadwalSholatByID retrieves a specific jadwal sholat by ID
// @Summary Get jadwal sholat by ID
// @Description Retrieve a specific jadwal sholat by its ID
// @Tags jadwal-sholat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Jadwal Sholat ID"
// @Success 200 {object} JadwalSholatResponse
// @Failure 400 {object} JadwalSholatErrorResponse
// @Failure 404 {object} JadwalSholatErrorResponse
// @Failure 500 {object} JadwalSholatErrorResponse
// @Router /jadwal-sholat/{id} [get]
func GetJadwalSholatByID(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			logger.Error("Invalid jadwal sholat ID", "id", idStr, "error", err)
			c.JSON(http.StatusBadRequest, JadwalSholatErrorResponse{
				Error:   "INVALID_ID",
				Message: "Invalid jadwal sholat ID",
			})
			return
		}

		var jadwal models.JadwalSholat
		if err := db.First(&jadwal, "id_jadwal = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warn("Jadwal sholat not found", "id", id)
				c.JSON(http.StatusNotFound, JadwalSholatErrorResponse{
					Error:   "NOT_FOUND",
					Message: "Jadwal sholat not found",
				})
				return
			}
			logger.Error("Failed to retrieve jadwal sholat", "id", id, "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to retrieve jadwal sholat",
			})
			return
		}

		c.JSON(http.StatusOK, JadwalSholatResponse{
			Message: "Jadwal sholat retrieved successfully",
			Data:    jadwal,
		})
	}
}

// CreateJadwalSholat creates a new jadwal sholat
// @Summary Create jadwal sholat
// @Description Create a new jadwal sholat entry
// @Tags jadwal-sholat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param jadwal body JadwalSholatCreateRequest true "Jadwal Sholat data"
// @Success 201 {object} JadwalSholatResponse
// @Failure 400 {object} JadwalSholatErrorResponse
// @Failure 500 {object} JadwalSholatErrorResponse
// @Router /jadwal-sholat [post]
func CreateJadwalSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req JadwalSholatCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error("Invalid request body", "error", err)
			c.JSON(http.StatusBadRequest, JadwalSholatErrorResponse{
				Error:   "INVALID_REQUEST",
				Message: "Invalid request body",
			})
			return
		}

		jadwal := models.JadwalSholat{
			Hari:         req.Hari,
			JenisSholat:  req.JenisSholat,
			WaktuMulai:   req.WaktuMulai,
			WaktuSelesai: req.WaktuSelesai,
			Jurusan:      req.Jurusan,
			Kelas:        req.Kelas,
		}

		if err := db.Create(&jadwal).Error; err != nil {
			logger.Error("Failed to create jadwal sholat", "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to create jadwal sholat",
			})
			return
		}

		logger.Info("Jadwal sholat created", "id", jadwal.IDJadwal)
		c.JSON(http.StatusCreated, JadwalSholatResponse{
			Message: "Jadwal sholat created successfully",
			Data:    jadwal,
		})
	}
}

// UpdateJadwalSholat updates an existing jadwal sholat
// @Summary Update jadwal sholat
// @Description Update an existing jadwal sholat by ID
// @Tags jadwal-sholat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Jadwal Sholat ID"
// @Param jadwal body JadwalSholatUpdateRequest true "Updated Jadwal Sholat data"
// @Success 200 {object} JadwalSholatResponse
// @Failure 400 {object} JadwalSholatErrorResponse
// @Failure 404 {object} JadwalSholatErrorResponse
// @Failure 500 {object} JadwalSholatErrorResponse
// @Router /jadwal-sholat/{id} [put]
func UpdateJadwalSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			logger.Error("Invalid jadwal sholat ID", "id", idStr, "error", err)
			c.JSON(http.StatusBadRequest, JadwalSholatErrorResponse{
				Error:   "INVALID_ID",
				Message: "Invalid jadwal sholat ID",
			})
			return
		}

		var req JadwalSholatUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error("Invalid request body", "error", err)
			c.JSON(http.StatusBadRequest, JadwalSholatErrorResponse{
				Error:   "INVALID_REQUEST",
				Message: "Invalid request body",
			})
			return
		}

		// Check if jadwal exists
		var jadwal models.JadwalSholat
		if err := db.First(&jadwal, "id_jadwal = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warn("Jadwal sholat not found", "id", id)
				c.JSON(http.StatusNotFound, JadwalSholatErrorResponse{
					Error:   "NOT_FOUND",
					Message: "Jadwal sholat not found",
				})
				return
			}
			logger.Error("Failed to find jadwal sholat", "id", id, "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to find jadwal sholat",
			})
			return
		}

		// Update fields if provided
		updates := make(map[string]interface{})
		if req.Hari != "" {
			updates["hari"] = req.Hari
		}
		if req.JenisSholat != "" {
			updates["jenis_sholat"] = req.JenisSholat
		}
		if req.WaktuMulai != "" {
			updates["waktu_mulai"] = req.WaktuMulai
		}
		if req.WaktuSelesai != "" {
			updates["waktu_selesai"] = req.WaktuSelesai
		}
		if req.Jurusan != "" {
			updates["jurusan"] = req.Jurusan
		}
		if req.Kelas != "" {
			updates["kelas"] = req.Kelas
		}

		if err := db.Model(&jadwal).Updates(updates).Error; err != nil {
			logger.Error("Failed to update jadwal sholat", "id", id, "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to update jadwal sholat",
			})
			return
		}

		// Retrieve updated jadwal
		if err := db.First(&jadwal, "id_jadwal = ?", id).Error; err != nil {
			logger.Error("Failed to retrieve updated jadwal sholat", "id", id, "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to retrieve updated jadwal sholat",
			})
			return
		}

		logger.Info("Jadwal sholat updated", "id", id)
		c.JSON(http.StatusOK, JadwalSholatResponse{
			Message: "Jadwal sholat updated successfully",
			Data:    jadwal,
		})
	}
}

// DeleteJadwalSholat deletes a jadwal sholat by ID
// @Summary Delete jadwal sholat
// @Description Delete a jadwal sholat by its ID
// @Tags jadwal-sholat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Jadwal Sholat ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} JadwalSholatErrorResponse
// @Failure 404 {object} JadwalSholatErrorResponse
// @Failure 500 {object} JadwalSholatErrorResponse
// @Router /jadwal-sholat/{id} [delete]
func DeleteJadwalSholat(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			logger.Error("Invalid jadwal sholat ID", "id", idStr, "error", err)
			c.JSON(http.StatusBadRequest, JadwalSholatErrorResponse{
				Error:   "INVALID_ID",
				Message: "Invalid jadwal sholat ID",
			})
			return
		}

		// Check if jadwal exists
		var jadwal models.JadwalSholat
		if err := db.First(&jadwal, "id_jadwal = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warn("Jadwal sholat not found", "id", id)
				c.JSON(http.StatusNotFound, JadwalSholatErrorResponse{
					Error:   "NOT_FOUND",
					Message: "Jadwal sholat not found",
				})
				return
			}
			logger.Error("Failed to find jadwal sholat", "id", id, "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to find jadwal sholat",
			})
			return
		}

		// Delete the jadwal
		if err := db.Delete(&jadwal).Error; err != nil {
			logger.Error("Failed to delete jadwal sholat", "id", id, "error", err)
			c.JSON(http.StatusInternalServerError, JadwalSholatErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Failed to delete jadwal sholat",
			})
			return
		}

		logger.Info("Jadwal sholat deleted", "id", id)
		c.JSON(http.StatusOK, gin.H{
			"message": "Jadwal sholat deleted successfully",
		})
	}
}
