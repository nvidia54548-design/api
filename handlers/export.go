package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

type ExportAbsensiRequest struct {
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
	Kelas     string `form:"kelas"`
	Jurusan   string `form:"jurusan"`
}

// Header constants
const (
	CSVContentType   = "text/csv"
	ExcelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

var (
	// Headers for CSV exports
	csvAbsensiHeaders = []string{"No", "NIS", "Nama Siswa", "Kelas", "Jurusan", "Tanggal", "Status", "Deskripsi"}
	csvLaporanHeaders = []string{"Status", "Jumlah", "Persentase"}
	csvDetailHeaders  = csvAbsensiHeaders // Same structure for detail section
)

// Helper function to write CSV rows with error handling
func writeCSVRow(writer *csv.Writer, logger *zap.SugaredLogger, row []string, context string) error {
	if err := writer.Write(row); err != nil {
		logger.Errorw("Failed to write CSV row",
			"error", err.Error(),
			"context", context,
			"row_data", row, // Include row data for debugging
		)
		return fmt.Errorf("failed to write CSV %s row: %w", context, err)
	}
	return nil
}

// Helper function to write multiple CSV rows with error handling
func writeCSVRows(writer *csv.Writer, logger *zap.SugaredLogger, rows [][]string, context string) error {
	for _, row := range rows {
		if err := writeCSVRow(writer, logger, row, context); err != nil {
			return err
		}
	}
	return nil
}

// ExportAbsensiCSV godoc
// @Summary Export data absensi ke CSV
// @Description Mengexport data absensi siswa dalam format CSV. Dapat difilter berdasarkan tanggal, kelas, dan jurusan
// @Tags export
// @Accept json
// @Produce text/csv
// @Param start_date query string false "Tanggal mulai (format: YYYY-MM-DD)"
// @Param end_date query string false "Tanggal akhir (format: YYYY-MM-DD)"
// @Param kelas query string false "Filter berdasarkan kelas"
// @Param jurusan query string false "Filter berdasarkan jurusan"
// @Security BearerAuth
// @Success 200 {file} file "File CSV berhasil didownload"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi"
// @Failure 40 ErrorResponse "Akses ditolak"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal"
// @Router /export/absensi [get]
func ExportAbsensiCSV(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ExportAbsensiRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Warnw("Invalid export request",
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Parameter tidak valid",
				"error":   err.Error(),
			})
			return
		}

		// Build query
		query := db.Model(&models.Absensi{}).
			Preload("Siswa").
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		// Apply date filters
		if req.StartDate != "" {
			query = query.Where("DATE(absensi.tanggal) >= ?", req.StartDate)
		}
		if req.EndDate != "" {
			query = query.Where("DATE(absensi.tanggal) <= ?", req.EndDate)
		}

		// Apply class and department filters
		if req.Kelas != "" {
			query = query.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			query = query.Where("siswa.jurusan = ?", req.Jurusan)
		}

		var absensiList []models.Absensi
		if err := query.Order("absensi.tanggal DESC, siswa.kelas, siswa.nis").Find(&absensiList).Error; err != nil {
			logger.Errorw("Failed to fetch absensi for export",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data absensi",
			})
			return
		}

		// Set CSV headers
		filename := fmt.Sprintf("absensi_export_%s.csv", time.Now().Format("20060102_150405"))
		c.Header("Content-Type", CSVContentType)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush() // Ensure buffer is flushed

		// Write CSV header
		if err := writeCSVRow(writer, logger, csvAbsensiHeaders, "header"); err != nil {
			// Error already logged by writeCSVRow
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menulis header CSV",
				"error":   err.Error(),
			})
			return
		}

		// Write data rows
		for i, absensi := range absensiList {
			namaSiswa := ""
			kelas := ""
			jurusan := ""
			if absensi.Siswa != nil {
				namaSiswa = absensi.Siswa.NamaSiswa
				kelas = absensi.Siswa.Kelas
				jurusan = absensi.Siswa.Jurusan
			}

			row := []string{
				fmt.Sprintf("%d", i+1),
				absensi.NIS,
				namaSiswa,
				kelas,
				jurusan,
				absensi.Tanggal.Format("2006-01-02"),
				absensi.Status,
				absensi.Deskripsi,
			}
			if err := writeCSVRow(writer, logger, row, fmt.Sprintf("data_row_%d", i+1)); err != nil {
				// Error already logged by writeCSVRow
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal menulis data CSV",
					"error":   err.Error(),
				})
				return
			}
		}

		logger.Infow("Absensi exported to CSV successfully",
			"total_records", len(absensiList),
			"start_date", req.StartDate,
			"end_date", req.EndDate,
		)
	}
}

type LaporanStatistik struct {
	TotalSiswa        int64   `json:"total_siswa"`
	TotalAbsensi      int64   `json:"total_absensi"`
	TotalHadir        int64   `json:"total_hadir"`
	TotalIzin         int64   `json:"total_izin"`
	TotalSakit        int64   `json:"total_sakit"`
	TotalAlpha        int64   `json:"total_alpha"`
	PersentaseHadir   float64 `json:"persentase_hadir"`
	PersentaseIzin    float64 `json:"persentase_izin"`
	PersentaseSakit   float64 `json:"persentase_sakit"`
	PersentaseAlpha   float64 `json:"persentase_alpha"`
	RataRataKehadiran float64 `json:"rata_rata_kehadiran"`
}

// ExportLaporanCSV godoc
// @Summary Export laporan absensi dengan statistik ke CSV
// @Description Mengexport laporan absensi dengan statistik kehadiran dalam format CSV. Termasuk persentase dan rata-rata kehadiran
// @Tags export
// @Accept json
// @Produce text/csv
// @Param start_date query string false "Tanggal mulai (format: YYYY-MM-DD)"
// @Param end_date query string false "Tanggal akhir (format: YYYY-MM-DD)"
// @Param kelas query string false "Filter berdasarkan kelas"
// @Param jurusan query string false "Filter berdasarkan jurusan"
// @Security BearerAuth
// @Success 200 {file} file "File CSV laporan berhasil didownload"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi"
// @Failure 403 {object} ErrorResponse "Akses ditolak"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal"
// @Router /export/laporan [get]
func ExportLaporanCSV(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ExportAbsensiRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Warnw("Invalid export request",
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Parameter tidak valid",
				"error":   err.Error(),
			})
			return
		}

		// Build base query for statistics
		baseQuery := db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		if req.StartDate != "" {
			baseQuery = baseQuery.Where("DATE(absensi.tanggal) >= ?", req.StartDate)
		}
		if req.EndDate != "" {
			baseQuery = baseQuery.Where("DATE(absensi.tanggal) <= ?", req.EndDate)
		}
		if req.Kelas != "" {
			baseQuery = baseQuery.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			baseQuery = baseQuery.Where("siswa.jurusan = ?", req.Jurusan)
		}

		// Get statistics
		var stats LaporanStatistik

		// Total siswa in filter
		siswaQuery := db.Model(&models.Siswa{})
		if req.Kelas != "" {
			siswaQuery = siswaQuery.Where("kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			siswaQuery = siswaQuery.Where("jurusan = ?", req.Jurusan)
		}
		siswaQuery.Count(&stats.TotalSiswa)

		// Count by status
		baseQuery.Count(&stats.TotalAbsensi)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "hadir").
			Count(&stats.TotalHadir)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "izin").
			Count(&stats.TotalIzin)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "sakit").
			Count(&stats.TotalSakit)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "alpha").
			Count(&stats.TotalAlpha)

		// Calculate percentages
		if stats.TotalAbsensi > 0 {
			stats.PersentaseHadir = float64(stats.TotalHadir) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseIzin = float64(stats.TotalIzin) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseSakit = float64(stats.TotalSakit) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseAlpha = float64(stats.TotalAlpha) / float64(stats.TotalAbsensi) * 100
		}

		// Calculate average attendance per student
		if stats.TotalSiswa > 0 && stats.TotalAbsensi > 0 {
			stats.RataRataKehadiran = float64(stats.TotalHadir) / float64(stats.TotalAbsensi) * 100
		}

		// Get detailed absensi data
		var absensiList []models.Absensi
		detailQuery := db.Model(&models.Absensi{}).
			Preload("Siswa").
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		if req.StartDate != "" {
			detailQuery = detailQuery.Where("DATE(absensi.tanggal) >= ?", req.StartDate)
		}
		if req.EndDate != "" {
			detailQuery = detailQuery.Where("DATE(absensi.tanggal) <= ?", req.EndDate)
		}
		if req.Kelas != "" {
			detailQuery = detailQuery.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			detailQuery = detailQuery.Where("siswa.jurusan = ?", req.Jurusan)
		}

		if err := detailQuery.Order("absensi.tanggal DESC, siswa.kelas, siswa.nis").Find(&absensiList).Error; err != nil {
			logger.Errorw("Failed to fetch absensi for laporan",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data absensi",
			})
			return
		}

		// Set CSV headers
		filename := fmt.Sprintf("laporan_absensi_%s.csv", time.Now().Format("20060102_150405"))
		c.Header("Content-Type", CSVContentType)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// Prepare and write sections
		// 1. Statistics Summary Section
		statSectionTitle := []string{"LAPORAN STATISTIK ABSENSI SHOLAT"}
		emptyRow := []string{""}

		// Filters
		var filterRows [][]string
		if req.StartDate != "" || req.EndDate != "" {
			periode := "Semua Waktu"
			if req.StartDate != "" && req.EndDate != "" {
				periode = fmt.Sprintf("%s s/d %s", req.StartDate, req.EndDate)
			} else if req.StartDate != "" {
				periode = fmt.Sprintf("Dari %s", req.StartDate)
			} else if req.EndDate != "" {
				periode = fmt.Sprintf("Sampai %s", req.EndDate)
			}
			filterRows = append(filterRows, []string{"Periode", periode})
		}
		if req.Kelas != "" {
			filterRows = append(filterRows, []string{"Kelas", req.Kelas})
		}
		if req.Jurusan != "" {
			filterRows = append(filterRows, []string{"Jurusan", req.Jurusan})
		}

		// Summary Stats
		summaryRows := [][]string{
			{"RINGKASAN STATISTIK"},
			{"Total Siswa", fmt.Sprintf("%d", stats.TotalSiswa)},
			{"Total Absensi", fmt.Sprintf("%d", stats.TotalAbsensi)},
			emptyRow,          // Empty row before status breakdown
			csvLaporanHeaders, // Header for status table
			{"Hadir", fmt.Sprintf("%d", stats.TotalHadir), fmt.Sprintf("%.2f%%", stats.PersentaseHadir)},
			{"Izin", fmt.Sprintf("%d", stats.TotalIzin), fmt.Sprintf("%.2f%%", stats.PersentaseIzin)},
			{"Sakit", fmt.Sprintf("%d", stats.TotalSakit), fmt.Sprintf("%.2f%%", stats.PersentaseSakit)},
			{"Alpha", fmt.Sprintf("%d", stats.TotalAlpha), fmt.Sprintf("%.2f%%", stats.PersentaseAlpha)},
			emptyRow,           // Empty row before{"Rata-rata Kehadiran", fmt.Sprintf("%.2f%%", stats.RataRataKehadiran)},
			emptyRow, emptyRow, // Two empty rows before detail section
			{"DETAIL ABSENSI"},
			csvDetailHeaders, // Header for detail table
		}

		// Combine all rows for the statistics section
		allStatRows := [][]string{statSectionTitle, emptyRow}
		allStatRows = append(allStatRows, filterRows...)
		allStatRows = append(allStatRows, emptyRow) // Empty row after filters
		allStatRows = append(allStatRows, summaryRows...)

		if err := writeCSVRows(writer, logger, allStatRows, "statistics_section"); err != nil {
			// Error already logged by writeCSVRows/writeCSVRow
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menulis bagian statistik CSV",
				"error":   err.Error(),
			})
			return
		}

		// 2. Detail Data Rows Section
		for i, absensi := range absensiList {
			namaSiswa := ""
			kelas := ""
			jurusan := ""
			if absensi.Siswa != nil {
				namaSiswa = absensi.Siswa.NamaSiswa
				kelas = absensi.Siswa.Kelas
				jurusan = absensi.Siswa.Jurusan
			}

			row := []string{
				fmt.Sprintf("%d", i+1),
				absensi.NIS,
				namaSiswa,
				kelas,
				jurusan,
				absensi.Tanggal.Format("2006-01-02"),
				absensi.Status,
				absensi.Deskripsi,
			}
			if err := writeCSVRow(writer, logger, row, fmt.Sprintf("detail_row_%d", i+1)); err != nil {
				// Error already logged by writeCSVRow
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal menulis bagian detail CSV",
					"error":   err.Error(),
				})
				return
			}
		}

		logger.Infow("Laporan exported to CSV successfully",
			"total_records", len(absensiList),
			"stats", stats,
		)
	}
}

func buildWhereClause(req ExportAbsensiRequest) string {
	where := "1=1"
	if req.StartDate != "" {
		where += fmt.Sprintf(" AND DATE(absensi.tanggal) >= '%s'", req.StartDate)
	}
	if req.EndDate != "" {
		where += fmt.Sprintf(" AND DATE(absensi.tanggal) <= '%s'", req.EndDate)
	}
	if req.Kelas != "" {
		where += fmt.Sprintf(" AND siswa.kelas = '%s'", req.Kelas)
	}
	if req.Jurusan != "" {
		where += fmt.Sprintf(" AND siswa.jurusan = '%s'", req.Jurusan)
	}
	return where
}

// ExportAbsensiExcel godoc
// @Summary Export data absensi ke Excel
// @Description Mengexport data absensi siswa dalam format Excel. Dapat difilter berdasarkan tanggal, kelas, dan jurusan
// @Tags export
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param start_date query string false "Tanggal mulai (format: YYYY-MM-DD)"
// @Param end_date query string false "Tanggal akhir (format: YYYY-MM-DD)"
// @Param kelas query string false "Filter berdasarkan kelas"
// @Param jurusan query string false "Filter berdasarkan jurusan"
// @Security BearerAuth
// @Success 200 {file} file "File Excel berhasil didownload"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi"
// @Failure 403 {object} ErrorResponse "Akses ditolak"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal"
// @Router /export/absensi/excel [get]
func ExportAbsensiExcel(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ExportAbsensiRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Warnw("Invalid export request",
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Parameter tidak valid",
				"error":   err.Error(),
			})
			return
		}

		// Build query
		query := db.Model(&models.Absensi{}).
			Preload("Siswa").
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		// Apply date filters
		if req.StartDate != "" {
			query = query.Where("DATE(absensi.tanggal) >= ?", req.StartDate)
		}
		if req.EndDate != "" {
			query = query.Where("DATE(absensi.tanggal) <= ?", req.EndDate)
		}

		// Apply class and department filters
		if req.Kelas != "" {
			query = query.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			query = query.Where("siswa.jurusan = ?", req.Jurusan)
		}

		var absensiList []models.Absensi
		if err := query.Order("absensi.tanggal DESC, siswa.kelas, siswa.nis").Find(&absensiList).Error; err != nil {
			logger.Errorw("Failed to fetch absensi for export",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data absensi",
			})
			return
		}

		// Create Excel file
		f := excelize.NewFile()
		defer func() {
			if err := f.Close(); err != nil {
				logger.Errorw("Failed to close Excel file",
					"error", err.Error(),
				)
			}
		}()

		// Set sheet name
		sheetName := "Absensi"
		if err := f.SetSheetName("Sheet1", sheetName); err != nil {
			logger.Errorw("Failed to set sheet name", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menyiapkan file Excel",
				"error":   err.Error(),
			})
			return
		}

		setCellValue := func(sheet, cell string, value interface{}) error {
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				logger.Errorw("Failed to set cell value", "error", err.Error(), "cell", cell)
				return err
			}
			return nil
		}

		// Define styles
		headerStyle, err := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		if err != nil {
			logger.Errorw("Failed to create header style", "error", err.Error())
			// Continue without style, but log the error
		}

		// Write header with styling
		headers := []string{"No", "NIS", "Nama Siswa", "Kelas", "Jurusan", "Tanggal", "Status", "Deskripsi"}
		for colIndex, header := range headers {
			cellName, err := excelize.CoordinatesToCellName(colIndex+1, 1) // Row 1
			if err != nil {
				logger.Errorw("Failed to calculate cell name for header", "error", err.Error(), "column", colIndex+1)
				// Consider aborting if this is critical
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal menyiapkan file Excel",
					"error":   "Internal error calculating cell coordinates",
				})
				return
			}
			if err := setCellValue(sheetName, cellName, header); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal menulis file Excel",
				})
				return
			}
			if headerStyle != 0 { // Only apply style if it was created successfully
				if err := f.SetCellStyle(sheetName, cellName, cellName, headerStyle); err != nil {
					logger.Warnw("Failed to set cell style", "error", err.Error())
				}
			}
		}

		// Write data rows
		for rowIndex, absensi := range absensiList {
			rowNum := rowIndex + 2 // Start from row 2 (after header)
			namaSiswa := ""
			kelas := ""
			jurusan := ""
			if absensi.Siswa != nil {
				namaSiswa = absensi.Siswa.NamaSiswa
				kelas = absensi.Siswa.Kelas
				jurusan = absensi.Siswa.Jurusan
			}

			// Calculate cell names for this row
			if err := setCellValue(sheetName, fmt.Sprintf("A%d", rowNum), rowIndex+1); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("B%d", rowNum), absensi.NIS); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("C%d", rowNum), namaSiswa); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("D%d", rowNum), kelas); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("E%d", rowNum), jurusan); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("F%d", rowNum), absensi.Tanggal.Format("2006-01-02")); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("G%d", rowNum), absensi.Status); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("H%d", rowNum), absensi.Deskripsi); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
		}

		// Auto-adjust column widths
		colLetters := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
		widths := []float64{5, 12, 20, 12, 15, 12, 12, 20}
		for i, colLetter := range colLetters {
			if err := f.SetColWidth(sheetName, colLetter, colLetter, widths[i]); err != nil {
				logger.Warnw("Failed to set column width", "error", err.Error(), "col", colLetter)
			}
		}

		// Set response headers
		filename := fmt.Sprintf("absensi_export_%s.xlsx", time.Now().Format("20060102_150405"))
		c.Header("Content-Type", ExcelContentType)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		// Write to response
		if err := f.Write(c.Writer); err != nil {
			logger.Errorw("Failed to write Excel file to response",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengirim file Excel",
				"error":   err.Error(),
			})
			return
		}

		logger.Infow("Absensi exported to Excel successfully",
			"total_records", len(absensiList),
			"start_date", req.StartDate,
			"end_date", req.EndDate,
		)
	}
}

// ExportLaporanExcel godoc
// @Summary Export laporan absensi dengan statistik ke Excel
// @Description Mengexport laporan absensi dengan statistik kehadiran dalam format Excel. Termasuk persentase dan rata-rata kehadiran
// @Tags export
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param start_date query string false "Tanggal mulai (format: YYYY-MM-DD)"
// @Param end_date query string false "Tanggal akhir (format: YYYY-MM-DD)"
// @Param kelas query string false "Filter berdasarkan kelas"
// @Param jurusan query string false "Filter berdasarkan jurusan"
// @Security BearerAuth
// @Success 200 {file} file "File Excel laporan berhasil didownload"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi"
// @Failure 403 {object} ErrorResponse "Akses ditolak"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal"
// @Router /export/laporan/excel [get]
func ExportLaporanExcel(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ExportAbsensiRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Warnw("Invalid export request",
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Parameter tidak valid",
				"error":   err.Error(),
			})
			return
		}

		// Build base query for statistics
		baseQuery := db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		if req.StartDate != "" {
			baseQuery = baseQuery.Where("DATE(absensi.tanggal) >= ?", req.StartDate)
		}
		if req.EndDate != "" {
			baseQuery = baseQuery.Where("DATE(absensi.tanggal) <= ?", req.EndDate)
		}
		if req.Kelas != "" {
			baseQuery = baseQuery.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			baseQuery = baseQuery.Where("siswa.jurusan = ?", req.Jurusan)
		}

		// Get statistics
		var stats LaporanStatistik

		// Total siswa in filter
		siswaQuery := db.Model(&models.Siswa{})
		if req.Kelas != "" {
			siswaQuery = siswaQuery.Where("kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			siswaQuery = siswaQuery.Where("jurusan = ?", req.Jurusan)
		}
		siswaQuery.Count(&stats.TotalSiswa)

		// Count by status
		baseQuery.Count(&stats.TotalAbsensi)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "hadir").
			Count(&stats.TotalHadir)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "izin").
			Count(&stats.TotalIzin)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "sakit").
			Count(&stats.TotalSakit)

		db.Model(&models.Absensi{}).
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis").
			Where(buildWhereClause(req)).
			Where("status = ?", "alpha").
			Count(&stats.TotalAlpha)

		// Calculate percentages
		if stats.TotalAbsensi > 0 {
			stats.PersentaseHadir = float64(stats.TotalHadir) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseIzin = float64(stats.TotalIzin) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseSakit = float64(stats.TotalSakit) / float64(stats.TotalAbsensi) * 100
			stats.PersentaseAlpha = float64(stats.TotalAlpha) / float64(stats.TotalAbsensi) * 100
		}

		// Calculate average attendance per student
		if stats.TotalSiswa > 0 && stats.TotalAbsensi > 0 {
			stats.RataRataKehadiran = float64(stats.TotalHadir) / float64(stats.TotalAbsensi) * 100
		}

		// Get detailed absensi data
		var absensiList []models.Absensi
		detailQuery := db.Model(&models.Absensi{}).
			Preload("Siswa").
			Joins("LEFT JOIN siswa ON absensi.nis = siswa.nis")

		if req.StartDate != "" {
			detailQuery = detailQuery.Where("DATE(absensi.tanggal) >= ?", req.StartDate)
		}
		if req.EndDate != "" {
			detailQuery = detailQuery.Where("DATE(absensi.tanggal) <= ?", req.EndDate)
		}
		if req.Kelas != "" {
			detailQuery = detailQuery.Where("siswa.kelas = ?", req.Kelas)
		}
		if req.Jurusan != "" {
			detailQuery = detailQuery.Where("siswa.jurusan = ?", req.Jurusan)
		}

		if err := detailQuery.Order("absensi.tanggal DESC, siswa.kelas, siswa.nis").Find(&absensiList).Error; err != nil {
			logger.Errorw("Failed to fetch absensi for laporan",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data absensi",
			})
			return
		}

		// Create Excel file
		f := excelize.NewFile()
		defer func() {
			if err := f.Close(); err != nil {
				logger.Errorw("Failed to close Excel file",
					"error", err.Error(),
				)
			}
		}()

		// Rename default sheet
		sheetName := "Laporan"
		if err := f.SetSheetName("Sheet1", sheetName); err != nil {
			logger.Errorw("Failed to set sheet name", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menyiapkan file Excel",
				"error":   err.Error(),
			})
			return
		}

		setCellValue := func(sheet, cell string, value interface{}) error {
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				logger.Errorw("Failed to set cell value", "error", err.Error(), "cell", cell)
				return err
			}
			return nil
		}

		// Define styles
		titleStyle, err := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 14},
		})
		if err != nil {
			logger.Errorw("Failed to create title style", "error", err.Error())
		}

		subHeaderStyle, err := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true},
		})
		if err != nil {
			logger.Errorw("Failed to create sub header style", "error", err.Error())
		}

		tableHeaderStyle, err := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		if err != nil {
			logger.Errorw("Failed to create table header style", "error", err.Error())
		}

		// Write title
		if err := setCellValue(sheetName, "A1", "LAPORAN STATISTIK ABSENSI SHOLAT"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyiapkan file Excel"})
			return
		}
		if titleStyle != 0 {
			if err := f.SetCellStyle(sheetName, "A1", "A1", titleStyle); err != nil {
				logger.Warnw("Failed to set cell style", "error", err.Error())
			}
		}

		row := 3

		// Write filters
		if req.StartDate != "" || req.EndDate != "" {
			periode := "Semua Waktu"
			if req.StartDate != "" && req.EndDate != "" {
				periode = fmt.Sprintf("%s s/d %s", req.StartDate, req.EndDate)
			} else if req.StartDate != "" {
				periode = fmt.Sprintf("Dari %s", req.StartDate)
			} else if req.EndDate != "" {
				periode = fmt.Sprintf("Sampai %s", req.EndDate)
			}
			if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "Periode"); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyiapkan file Excel"})
				return
			}
			if subHeaderStyle != 0 {
				if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), subHeaderStyle); err != nil {
					logger.Warnw("Failed to set style", "error", err.Error())
				}
			}
			if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), periode); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyiapkan file Excel"})
				return
			}
			row++
		}

		if req.Kelas != "" {
			if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "Kelas"); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyiapkan file Excel"})
				return
			}
			if subHeaderStyle != 0 {
				if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), subHeaderStyle); err != nil {
					logger.Warnw("Failed to set style", "error", err.Error())
				}
			}
			if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), req.Kelas); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyiapkan file Excel"})
				return
			}
			row++
		}

		if req.Jurusan != "" {
			if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "Jurusan"); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyiapkan file Excel"})
				return
			}
			if subHeaderStyle != 0 {
				if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), subHeaderStyle); err != nil {
					logger.Warnw("Failed to set style", "error", err.Error())
				}
			}
			if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), req.Jurusan); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyiapkan file Excel"})
				return
			}
			row++
		}

		row++ // Blank row after filters

		// Write statistics summary
		if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "RINGKASAN STATISTIK"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		if subHeaderStyle != 0 {
			if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), subHeaderStyle); err != nil {
				logger.Warnw("Failed to set style", "error", err.Error())
			}
		}
		row++

		if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "Total Siswa"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), stats.TotalSiswa); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		row++

		if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "Total Absensi"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), stats.TotalAbsensi); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		row += 2 // Blank row before status table

		// Write status breakdown table header
		if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "Status"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		if tableHeaderStyle != 0 {
			if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), tableHeaderStyle); err != nil {
				logger.Warnw("Failed to set style", "error", err.Error())
			}
		}
		if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), "Jumlah"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		if err := setCellValue(sheetName, fmt.Sprintf("C%d", row), "Persentase"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		row++

		// Write status breakdown table data
		statusData := [][]interface{}{
			{"Hadir", stats.TotalHadir, fmt.Sprintf("%.2f%%", stats.PersentaseHadir)},
			{"Izin", stats.TotalIzin, fmt.Sprintf("%.2f%%", stats.PersentaseIzin)},
			{"Sakit", stats.TotalSakit, fmt.Sprintf("%.2f%%", stats.PersentaseSakit)},
			{"Alpha", stats.TotalAlpha, fmt.Sprintf("%.2f%%", stats.PersentaseAlpha)},
		}
		for _, dataRow := range statusData {
			if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), dataRow[0]); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), dataRow[1]); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("C%d", row), dataRow[2]); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			row++
		}
		row++ // Blank row after status table

		// Write average attendance
		if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "Rata-rata Kehadiran"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		if subHeaderStyle != 0 {
			if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), subHeaderStyle); err != nil {
				logger.Warnw("Failed to set style", "error", err.Error())
			}
		}
		if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("%.2f%%", stats.RataRataKehadiran)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		row += 3 // Blank rows before detail section

		// Write detail section header
		if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), "DETAIL ABSENSI"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
			return
		}
		if subHeaderStyle != 0 {
			if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), subHeaderStyle); err != nil {
				logger.Warnw("Failed to set style", "error", err.Error())
			}
		}
		row++

		// Write detail table headers
		detailHeaders := []string{"No", "NIS", "Nama Siswa", "Kelas", "Jurusan", "Tanggal", "Status", "Deskripsi"}
		for colIndex, header := range detailHeaders {
			cellName, err := excelize.CoordinatesToCellName(colIndex+1, row)
			if err != nil {
				logger.Errorw("Failed to calculate cell name for detail header", "error", err.Error(), "column", colIndex+1)
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal menyiapkan file Excel",
					"error":   "Internal error calculating cell coordinates",
				})
				return
			}
			if err := setCellValue(sheetName, cellName, header); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if tableHeaderStyle != 0 {
				if err := f.SetCellStyle(sheetName, cellName, cellName, tableHeaderStyle); err != nil {
					logger.Warnw("Failed to set style", "error", err.Error())
				}
			}
		}
		row++ // Move to next row for data

		// Write detail data rows
		for _, absensi := range absensiList {
			namaSiswa := ""
			kelas := ""
			jurusan := ""
			if absensi.Siswa != nil {
				namaSiswa = absensi.Siswa.NamaSiswa
				kelas = absensi.Siswa.Kelas
				jurusan = absensi.Siswa.Jurusan
			}

			if err := setCellValue(sheetName, fmt.Sprintf("A%d", row), row-1); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			} // No (row index starting from 1)
			if err := setCellValue(sheetName, fmt.Sprintf("B%d", row), absensi.NIS); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("C%d", row), namaSiswa); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("D%d", row), kelas); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("E%d", row), jurusan); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("F%d", row), absensi.Tanggal.Format("2006-01-02")); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("G%d", row), absensi.Status); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			if err := setCellValue(sheetName, fmt.Sprintf("H%d", row), absensi.Deskripsi); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menulis file Excel"})
				return
			}
			row++
		}

		// Auto-adjust column widths
		colLetters := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
		widths := []float64{5, 12, 20, 12, 15, 12, 12, 20}
		for i, colLetter := range colLetters {
			if err := f.SetColWidth(sheetName, colLetter, colLetter, widths[i]); err != nil {
				logger.Warnw("Failed to set column width", "error", err.Error())
			}
		}

		// Set response headers
		filename := fmt.Sprintf("laporan_absensi_%s.xlsx", time.Now().Format("20060102_150405"))
		c.Header("Content-Type", ExcelContentType)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		// Write to response
		if err := f.Write(c.Writer); err != nil {
			logger.Errorw("Failed to write Excel file to response",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengirim file Excel",
				"error":   err.Error(),
			})
			return
		}

		logger.Infow("Laporan exported to Excel successfully",
			"total_records", len(absensiList),
			"stats", stats,
		)
	}
}

// ExportAttendanceReportExcel godoc
// @Summary Export high-fidelity report absensi ke Excel
// @Description Mengexport laporan absensi sholat siswa dalam format Excel dengan pengelompokan per hari dan pewarnaan status kustom.
// @Tags export
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param start_date query string true "Tanggal mulai (format: YYYY-MM-DD)"
// @Param end_date query string true "Tanggal akhir (format: YYYY-MM-DD)"
// @Param jurusan query string true "Nama Jurusan (e.g., PPLG)"
// @Security BearerAuth
// @Success 200 {file} file "File Excel laporan berhasil didownload"
// @Router /export/attendance-report [get]
func ExportAttendanceReportExcel(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startDate := c.Query("start_date")
		endDate := c.Query("end_date")
		jurusan := c.Query("jurusan")

		if startDate == "" || endDate == "" || jurusan == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "start_date, end_date, dan jurusan wajib diisi"})
			return
		}

		// Data structures for aggregation
		type RawData struct {
			NIS         string
			NamaSiswa   string
			Jurusan     string
			Tanggal     time.Time
			JenisSholat string
			Status      string
		}

		var rawResults []RawData
		err := db.Table("siswa s").
			Select("s.nis, s.nama_siswa, s.jurusan, a.tanggal, j.jenis_sholat, a.status").
			Joins("LEFT JOIN absensi a ON s.nis = a.nis").
			Joins("LEFT JOIN jadwal_sholat j ON a.id_jadwal = j.id_jadwal").
			Where("s.jurusan = ?", jurusan).
			Where("a.tanggal BETWEEN ? AND ?", startDate, endDate).
			Scan(&rawResults).Error

		if err != nil {
			logger.Errorw("Failed to fetch raw data for aggregated report", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil data laporan"})
			return
		}

		type RowKey struct {
			NIS     string
			Tanggal string
		}
		type ReportRow struct {
			NIS          string
			Nama         string
			Jurusan      string
			Tanggal      string
			Dhuha        string
			DzuhurJumat  string
			HadirCount   int
			IzinSakitCnt int
			AlphaCount   int
		}

		// Grouping logic
		rowsMap := make(map[RowKey]*ReportRow)
		var sortedKeys []RowKey

		for _, item := range rawResults {
			dateStr := item.Tanggal.Format("02-01-2006")
			key := RowKey{NIS: item.NIS, Tanggal: dateStr}

			if _, exists := rowsMap[key]; !exists {
				rowsMap[key] = &ReportRow{
					NIS:         item.NIS,
					Nama:        item.NamaSiswa,
					Jurusan:     item.Jurusan,
					Tanggal:     dateStr,
					Dhuha:       "-",
					DzuhurJumat: "-",
				}
				sortedKeys = append(sortedKeys, key)
			}

			row := rowsMap[key]
			status := item.Status

			// Map to columns
			if item.JenisSholat == "Dhuha" {
				row.Dhuha = status
			} else if item.JenisSholat == "Dzuhur" || item.JenisSholat == "Jumat" {
				row.DzuhurJumat = status
			}

			// Aggregate totals
			switch status {
			case "HADIR":
				row.HadirCount++
			case "IZIN", "SAKIT":
				row.IzinSakitCnt++
			case "ALPHA":
				row.AlphaCount++
			}
		}

		// Create Excel
		f := excelize.NewFile()
		sheetName := "Data Absensi"
		f.SetSheetName("Sheet1", sheetName)

		// Styles
		titleStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 14},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		subtitleStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 11},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F5597"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
		})
		dataStyle, _ := f.NewStyle(&excelize.Style{
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		leftAlignStyle, _ := f.NewStyle(&excelize.Style{
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "left"},
		})

		// Status Colors
		hadirStyle, _ := f.NewStyle(&excelize.Style{
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"C6EFCE"}, Pattern: 1},
			Font:      &excelize.Font{Color: "006100"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		alphaStyle, _ := f.NewStyle(&excelize.Style{
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFC7CE"}, Pattern: 1},
			Font:      &excelize.Font{Color: "9C0006"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		izinStyle, _ := f.NewStyle(&excelize.Style{
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFEB9C"}, Pattern: 1},
			Font:      &excelize.Font{Color: "9C6500"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})

		// 1. Titles
		f.SetCellValue(sheetName, "A1", "LAPORAN ABSENSI SHOLAT SISWA - SMK NEGERI 2 SINGOSARI")
		f.MergeCell(sheetName, "A1", "J1")
		f.SetCellStyle(sheetName, "A1", "A1", titleStyle)

		subtitle := fmt.Sprintf("Periode: %s s/d %s | Jurusan: %s", startDate, endDate, jurusan)
		f.SetCellValue(sheetName, "A2", subtitle)
		f.MergeCell(sheetName, "A2", "J2")
		f.SetCellStyle(sheetName, "A2", "A2", subtitleStyle)

		// 2. Table Headers
		headers := []string{"No", "NIS", "Nama Siswa", "Jurusan", "Tanggal", "Dhuha", "Dzuhur/Jumat", "Hadir", "Izin/Sakit", "Alpha"}
		colWidths := []float64{5, 12, 30, 15, 15, 12, 12, 10, 12, 10}
		for i, h := range headers {
			colName, _ := excelize.CoordinatesToCellName(i+1, 4)
			f.SetCellValue(sheetName, colName, h)
			f.SetCellStyle(sheetName, colName, colName, headerStyle)
			f.SetColWidth(sheetName, string('A'+rune(i)), string('A'+rune(i)), colWidths[i])
		}

		// 3. Data Rows
		rowIdx := 5
		for i, key := range sortedKeys {
			rowData := rowsMap[key]

			f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowIdx), i+1)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowIdx), rowData.NIS)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowIdx), rowData.Nama)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowIdx), rowData.Jurusan)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowIdx), rowData.Tanggal)
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowIdx), rowData.Dhuha)
			f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowIdx), rowData.DzuhurJumat)
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowIdx), rowData.HadirCount)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowIdx), rowData.IzinSakitCnt)
			f.SetCellValue(sheetName, fmt.Sprintf("J%d", rowIdx), rowData.AlphaCount)

			// Simple Alignment
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", rowIdx), fmt.Sprintf("B%d", rowIdx), dataStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("C%d", rowIdx), fmt.Sprintf("C%d", rowIdx), leftAlignStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("D%d", rowIdx), fmt.Sprintf("E%d", rowIdx), dataStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("H%d", rowIdx), fmt.Sprintf("J%d", rowIdx), dataStyle)

			// Conditional Formatting for Dhuha
			switch rowData.Dhuha {
			case "HADIR":
				f.SetCellStyle(sheetName, fmt.Sprintf("F%d", rowIdx), fmt.Sprintf("F%d", rowIdx), hadirStyle)
			case "ALPHA":
				f.SetCellStyle(sheetName, fmt.Sprintf("F%d", rowIdx), fmt.Sprintf("F%d", rowIdx), alphaStyle)
			case "IZIN", "SAKIT":
				f.SetCellStyle(sheetName, fmt.Sprintf("F%d", rowIdx), fmt.Sprintf("F%d", rowIdx), izinStyle)
			default:
				f.SetCellStyle(sheetName, fmt.Sprintf("F%d", rowIdx), fmt.Sprintf("F%d", rowIdx), dataStyle)
			}

			// Conditional Formatting for Dzuhur/Jumat
			switch rowData.DzuhurJumat {
			case "HADIR":
				f.SetCellStyle(sheetName, fmt.Sprintf("G%d", rowIdx), fmt.Sprintf("G%d", rowIdx), hadirStyle)
			case "ALPHA":
				f.SetCellStyle(sheetName, fmt.Sprintf("G%d", rowIdx), fmt.Sprintf("G%d", rowIdx), alphaStyle)
			case "IZIN", "SAKIT":
				f.SetCellStyle(sheetName, fmt.Sprintf("G%d", rowIdx), fmt.Sprintf("G%d", rowIdx), izinStyle)
			default:
				f.SetCellStyle(sheetName, fmt.Sprintf("G%d", rowIdx), fmt.Sprintf("G%d", rowIdx), dataStyle)
			}

			rowIdx++
		}

		// 4. Response
		fileName := fmt.Sprintf("Laporan_Absensi_%s_%s.xlsx", jurusan, time.Now().Format("Jan_2006"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		f.Write(c.Writer)
	}
}

// ExportIntegratedReportExcel godoc
// @Summary Export laporan lengkap (Absensi & Jadwal) ke Excel
// @Description Mengexport file Excel terintegrasi dengan dua sheet: Laporan Absensi dan Jadwal Sholat Referensi.
// @Tags export
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param month query string true "Bulan laporan (format: YYYY-MM, e.g., 2026-03)"
// @Param jurusan query string true "Nama Jurusan (e.g., PPLG)"
// @Security BearerAuth
// @Router /export/full-report [get]
func ExportIntegratedReportExcel(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		monthStr := c.Query("month") // YYYY-MM
		jurusan := c.Query("jurusan")

		if monthStr == "" || jurusan == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "month dan jurusan wajib diisi"})
			return
		}

		// Calculate date range
		targetMonth, err := time.Parse("2006-01", monthStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format month tidak valid (harus YYYY-MM)"})
			return
		}
		startDate := targetMonth.Format("2006-01-01")
		endDate := targetMonth.AddDate(0, 1, -1).Format("2006-01-02")

		f := excelize.NewFile()
		defer f.Close()

		// --- STYLES ---
		titleStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 14},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F5597"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
		})
		dataStyle, _ := f.NewStyle(&excelize.Style{
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		leftDataStyle, _ := f.NewStyle(&excelize.Style{
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "left"},
		})

		hadirFill, _ := f.NewStyle(&excelize.Style{
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"C6EFCE"}, Pattern: 1},
			Font:      &excelize.Font{Color: "006100"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		alphaFill, _ := f.NewStyle(&excelize.Style{
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFC7CE"}, Pattern: 1},
			Font:      &excelize.Font{Color: "9C0006"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		izinFill, _ := f.NewStyle(&excelize.Style{
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFEB9C"}, Pattern: 1},
			Font:      &excelize.Font{Color: "9C6500"},
			Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})

		// --- SHEET 1: Laporan_Absensi ---
		sheet1 := "Laporan_Absensi"
		f.SetSheetName("Sheet1", sheet1)
		f.SetCellValue(sheet1, "A1", "LAPORAN KEHADIRAN SHOLAT SISWA - SMK NEGERI 2 SINGOSARI")
		f.MergeCell(sheet1, "A1", "H1")
		f.SetCellStyle(sheet1, "A1", "A1", titleStyle)
		f.SetCellValue(sheet1, "A2", fmt.Sprintf("Periode: %s | Jurusan: %s", monthStr, jurusan))
		f.MergeCell(sheet1, "A2", "H2")
		f.SetCellStyle(sheet1, "A2", "A2", titleStyle)

		headers1 := []string{"No", "NIS", "Nama Siswa", "Tanggal", "Dhuha", "Dzuhur/Jumat", "Total Hadir", "Total Alpha"}
		widths1 := []float64{5, 15, 30, 15, 12, 15, 12, 12}
		for i, h := range headers1 {
			cell, _ := excelize.CoordinatesToCellName(i+1, 4)
			f.SetCellValue(sheet1, cell, h)
			f.SetCellStyle(sheet1, cell, cell, headerStyle)
			f.SetColWidth(sheet1, string('A'+rune(i)), string('A'+rune(i)), widths1[i])
		}
		f.AutoFilter(sheet1, "A4:H4", nil)
		f.SetPanes(sheet1, &excelize.Panes{
			Freeze:      true,
			Split:       false,
			XSplit:      0,
			YSplit:      4,
			TopLeftCell: "A5",
			ActivePane:  "bottomLeft",
		})

		type RawData struct {
			NIS         string
			NamaSiswa   string
			Jurusan     string
			Tanggal     time.Time
			JenisSholat string
			Status      string
		}
		var rawResults []RawData
		db.Table("siswa s").
			Select("s.nis, s.nama_siswa, s.jurusan, a.tanggal, j.jenis_sholat, a.status").
			Joins("LEFT JOIN absensi a ON s.nis = a.nis AND a.tanggal BETWEEN ? AND ?", startDate, endDate).
			Joins("LEFT JOIN jadwal_sholat j ON a.id_jadwal = j.id_jadwal").
			Where("s.jurusan = ?", jurusan).Scan(&rawResults)

		type RowKey struct {
			NIS     string
			Tanggal string
		}
		type ReportRow struct {
			NIS         string
			Nama        string
			Tanggal     string
			Dhuha       string
			DzuhurJumat string
			HadirCount  int
			AlphaCount  int
		}
		rowsMap := make(map[RowKey]*ReportRow)
		var sortedKeys []RowKey
		for _, item := range rawResults {
			if item.NIS == "" || item.Tanggal.IsZero() {
				continue
			}
			key := RowKey{NIS: item.NIS, Tanggal: item.Tanggal.Format("02-01-2006")}
			if _, exists := rowsMap[key]; !exists {
				rowsMap[key] = &ReportRow{NIS: item.NIS, Nama: item.NamaSiswa, Tanggal: key.Tanggal, Dhuha: "-", DzuhurJumat: "-"}
				sortedKeys = append(sortedKeys, key)
			}
			r := rowsMap[key]
			if item.JenisSholat == "Dhuha" {
				r.Dhuha = item.Status
			} else if item.JenisSholat == "Dzuhur" || item.JenisSholat == "Jumat" {
				r.DzuhurJumat = item.Status
			}
			if item.Status == "HADIR" {
				r.HadirCount++
			} else if item.Status == "ALPHA" {
				r.AlphaCount++
			}
		}

		rIdx := 5
		for i, k := range sortedKeys {
			d := rowsMap[k]
			f.SetCellValue(sheet1, fmt.Sprintf("A%d", rIdx), i+1)
			f.SetCellValue(sheet1, fmt.Sprintf("B%d", rIdx), d.NIS)
			f.SetCellValue(sheet1, fmt.Sprintf("C%d", rIdx), d.Nama)
			f.SetCellValue(sheet1, fmt.Sprintf("D%d", rIdx), d.Tanggal)
			f.SetCellValue(sheet1, fmt.Sprintf("E%d", rIdx), d.Dhuha)
			f.SetCellValue(sheet1, fmt.Sprintf("F%d", rIdx), d.DzuhurJumat)
			f.SetCellValue(sheet1, fmt.Sprintf("G%d", rIdx), d.HadirCount)
			f.SetCellValue(sheet1, fmt.Sprintf("H%d", rIdx), d.AlphaCount)
			f.SetCellStyle(sheet1, fmt.Sprintf("A%d", rIdx), fmt.Sprintf("B%d", rIdx), dataStyle)
			f.SetCellStyle(sheet1, fmt.Sprintf("C%d", rIdx), fmt.Sprintf("C%d", rIdx), leftDataStyle)
			f.SetCellStyle(sheet1, fmt.Sprintf("D%d", rIdx), fmt.Sprintf("D%d", rIdx), dataStyle)
			f.SetCellStyle(sheet1, fmt.Sprintf("G%d", rIdx), fmt.Sprintf("H%d", rIdx), dataStyle)

			apply := func(cell, status string) {
				st := dataStyle
				switch status {
				case "HADIR":
					st = hadirFill
				case "ALPHA":
					st = alphaFill
				case "IZIN", "SAKIT":
					st = izinFill
				}
				f.SetCellStyle(sheet1, cell, cell, st)
			}
			apply(fmt.Sprintf("E%d", rIdx), d.Dhuha)
			apply(fmt.Sprintf("F%d", rIdx), d.DzuhurJumat)
			rIdx++
		}

		// --- SHEET 2: Jadwal_Sholat_Referensi ---
		sheet2 := "Jadwal_Sholat_Referensi"
		f.NewSheet(sheet2)
		f.SetCellValue(sheet2, "A1", "JADWAL PELAKSANAAN SHOLAT BERJAMAAH")
		f.MergeCell(sheet2, "A1", "F1")
		f.SetCellStyle(sheet2, "A1", "A1", titleStyle)
		f.SetCellValue(sheet2, "A2", fmt.Sprintf("Jurusan: %s | Bulan: %s", jurusan, monthStr))
		f.MergeCell(sheet2, "A2", "F2")
		f.SetCellStyle(sheet2, "A2", "A2", titleStyle)

		headers2 := []string{"Hari", "Tanggal", "Jenis Sholat", "Waktu Mulai", "Waktu Selesai", "Keterangan"}
		widths2 := []float64{10, 15, 15, 12, 12, 25}
		for i, h := range headers2 {
			cell, _ := excelize.CoordinatesToCellName(i+1, 4)
			f.SetCellValue(sheet2, cell, h)
			f.SetCellStyle(sheet2, cell, cell, headerStyle)
			f.SetColWidth(sheet2, string('A'+rune(i)), string('A'+rune(i)), widths2[i])
		}
		f.AutoFilter(sheet2, "A4:F4", nil)
		f.SetPanes(sheet2, &excelize.Panes{
			Freeze:      true,
			Split:       false,
			XSplit:      0,
			YSplit:      4,
			TopLeftCell: "A5",
			ActivePane:  "bottomLeft",
		})

		var weeklyJadwal []models.JadwalSholat
		db.Where("jurusan = ? OR jurusan = 'Semua Jurusan' OR jurusan = ''", jurusan).Find(&weeklyJadwal)
		jadwalMap := make(map[string][]models.JadwalSholat)
		for _, j := range weeklyJadwal {
			jadwalMap[j.Hari] = append(jadwalMap[j.Hari], j)
		}

		currDate := targetMonth
		rIdx2 := 5
		for currDate.Month() == targetMonth.Month() {
			dayName := utils.GetIndonesianDayName(currDate)
			dayJadwal := jadwalMap[dayName]
			dateStr := currDate.Format("02-01-2006")
			for _, j := range dayJadwal {
				f.SetCellValue(sheet2, fmt.Sprintf("A%d", rIdx2), dayName)
				f.SetCellValue(sheet2, fmt.Sprintf("B%d", rIdx2), dateStr)
				f.SetCellValue(sheet2, fmt.Sprintf("C%d", rIdx2), j.JenisSholat)
				f.SetCellValue(sheet2, fmt.Sprintf("D%d", rIdx2), j.WaktuMulai)
				f.SetCellValue(sheet2, fmt.Sprintf("E%d", rIdx2), j.WaktuSelesai)
				f.SetCellValue(sheet2, fmt.Sprintf("F%d", rIdx2), "-")
				f.SetCellStyle(sheet2, fmt.Sprintf("A%d", rIdx2), fmt.Sprintf("F%d", rIdx2), dataStyle)
				rIdx2++
			}
			currDate = currDate.AddDate(0, 0, 1)
		}
		fileName := fmt.Sprintf("Laporan_Lengkap_Absensi_%s_%s.xlsx", jurusan, targetMonth.Format("Jan_2006"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		f.Write(c.Writer)
	}
}
