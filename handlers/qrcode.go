package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

type QRCodeResponse struct {
	Message string     `json:"message"`
	Data    QRCodeData `json:"data"`
}

type QRCodeData struct {
	QRCode      string `json:"qr_code"`
	Token       string `json:"token"`
	ExpiresAt   string `json:"expires_at"`
	JenisSholat string `json:"jenis_sholat"`
	IDJadwal    int    `json:"id_jadwal"`
}

// GenerateQRCode godoc
// @Summary Generate QR Code dengan format JSON (Base64)
// @Description Menghasilkan QR code untuk sesi absensi sholat aktif dan mengembalikannya sebagai data base64 dalam response JSON. QR code berisi token yang ter-sign dan berlaku selama 5 menit. Digunakan untuk menampilkan QR code di layar atau aplikasi mobile untuk siswa scan. Hanya untuk admin, guru, atau wali_kelas
// @Tags qrcode
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} QRCodeResponse "QR code berhasil dibuat. Berisi base64 PNG, token, waktu kadaluarsa, jenis sholat, dan ID jadwal"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi atau token tidak valid"
// @Failure 403 {object} ErrorResponse "Akses ditolak - hanya admin, guru, atau wali kelas yang dapat mengakses"
// @Failure 404 {object} ErrorResponse "Tidak ada jadwal sholat aktif saat ini"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal saat membuat QR code"
// @Param force query string false "Set to 'true' to force-generate QR for testing (non-production only)"
// @Param jenis_sholat query string false "Specify prayer type (e.g., Maghrib) when forcing"
// @Param id_jadwal query int false "Specify jadwal ID when forcing"
// @Router /qrcode/generate [get]
func GenerateQRCode(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get current day and time in WIB (UTC+7)
		now := utils.GetJakartaTime()
		currentDay := getDayName(now.Weekday())
		currentTime := now.Format("15:04:05")

		// Find active jadwal sholat for current day and time
		var jadwal models.JadwalSholat

		// Support test forcing: allow admins to request a specific jadwal or jenis_sholat when ENVIRONMENT != production
		force := c.Query("force")
		jenis := c.Query("jenis_sholat")
		idJadwalStr := c.Query("id_jadwal")

		var err error
		if force == "true" && os.Getenv("ENVIRONMENT") != "production" {
			if idJadwalStr != "" {
				id, errConv := strconv.Atoi(idJadwalStr)
				if errConv != nil {
					c.JSON(http.StatusBadRequest, gin.H{"message": "id_jadwal tidak valid"})
					return
				}
				err = db.First(&jadwal, "id_jadwal = ?", id).Error
			} else if jenis != "" {
				err = db.Where("hari = ? AND jenis_sholat = ?", currentDay, jenis).First(&jadwal).Error
			} else {
				// fallback to active jadwal behavior
				err = db.Where("hari = ? AND waktu_mulai <= ? AND waktu_selesai >= ?",
					currentDay, currentTime, currentTime).
					First(&jadwal).Error
			}
		} else {
			err = db.Where("hari = ? AND waktu_mulai <= ? AND waktu_selesai >= ?",
				currentDay, currentTime, currentTime).
				First(&jadwal).Error
		}

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"message": "Tidak ada jadwal sholat aktif saat ini",
				})
				return
			}
			logger.Errorw("Failed to fetch jadwal sholat",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil jadwal sholat",
			})
			return
		}

		// Generate expiry time (5 minutes from now)
		expiresAt := now.Add(5 * time.Minute)

		// Create token payload: id_jadwal|date|expires_unix
		tokenPayload := fmt.Sprintf("%d|%s|%d",
			jadwal.IDJadwal,
			now.Format("2006-01-02"),
			expiresAt.Unix(),
		)

		// Sign the token with HMAC
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = "default-secret-key"
		}

		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(tokenPayload))
		signature := hex.EncodeToString(h.Sum(nil))

		// Create the final token
		token := base64.URLEncoding.EncodeToString([]byte(tokenPayload)) + "." + signature

		// Generate QR code as PNG
		qrPNG, err := qrcode.Encode(token, qrcode.Medium, 256)
		if err != nil {
			logger.Errorw("Failed to generate QR code",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal membuat QR code",
			})
			return
		}

		// Convert to base64 for JSON response
		qrBase64 := base64.StdEncoding.EncodeToString(qrPNG)

		logger.Infow("QR code generated successfully",
			"jadwal_id", jadwal.IDJadwal,
			"jenis_sholat", jadwal.JenisSholat,
			"expires_at", expiresAt,
		)

		c.JSON(http.StatusOK, QRCodeResponse{
			Message: "QR code berhasil dibuat",
			Data: QRCodeData{
				QRCode:      "data:image/png;base64," + qrBase64,
				Token:       token,
				ExpiresAt:   expiresAt.Format(time.RFC3339),
				JenisSholat: jadwal.JenisSholat,
				IDJadwal:    jadwal.IDJadwal,
			},
		})
	}
}

type VerifyQRRequest struct {
	Token string `json:"token" binding:"required"`
}

type VerifyQRResponse struct {
	Message string       `json:"message"`
	Data    VerifyQRData `json:"data"`
}

type VerifyQRData struct {
	Valid       bool   `json:"valid"`
	NIS         string `json:"nis"`
	NamaSiswa   string `json:"nama_siswa"`
	Kelas       string `json:"kelas"`
	Jurusan     string `json:"jurusan"`
	JenisSholat string `json:"jenis_sholat"`
	Tanggal     string `json:"tanggal"`
	Status      string `json:"status"`
}

// VerifyQRCode godoc
// @Summary Scan dan verifikasi QR Code untuk mencatat absensi
// @Description Scan QR code yang sudah di-generate untuk mencatat kehadiran siswa dalam sesi absensi sholat. Endpoint ini memvalidasi token, memeriksa waktu kadaluarsa, dan mencatat absensi siswa dalam database. Token harus dalam format: Base64Payload.HexSignature. Hanya untuk siswa
// @Tags qrcode
// @Accept json
// @Produce json
// @Param request body VerifyQRRequest true "Token QR code hasil scan (dari endpoint generate atau image)"
// @Security BearerAuth
// @Success 200 {object} VerifyQRResponse "Absensi berhasil dicatat. Menampilkan data siswa, jenis sholat, status kehadiran (hadir)"
// @Failure 400 {object} ErrorResponse "Token format tidak valid, format payload tidak valid, signature tidak valid, token sudah kadaluarsa (lebih dari 5 menit), atau QR code tidak valid untuk hari ini"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi atau NIS tidak ditemukan di token JWT"
// @Failure 403 {object} ErrorResponse "Akses ditolak - hanya siswa yang dapat mengakses"
// @Failure 404 {object} ErrorResponse "Siswa atau jadwal sholat tidak ditemukan"
// @Failure 409 {object} ErrorResponse "Konflik - siswa sudah absen untuk jadwal sholat ini pada hari yang sama"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal saat mencatat absensi"
// @Router /qrcode/verify [post]
func VerifyQRCode(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get NIS from JWT context (siswa yang scan)
		nis, exists := c.Get("nis")
		if !exists || nis == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "NIS tidak ditemukan di token",
			})
			return
		}

		nisStr, ok := nis.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Terjadi kesalahan tipe data JWT",
			})
			return
		}

		var req VerifyQRRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Token tidak boleh kosong",
			})
			return
		}

		// Split token and signature
		parts := splitToken(req.Token)
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format token tidak valid",
			})
			return
		}

		// Decode payload
		payloadBytes, err := base64.URLEncoding.DecodeString(parts[0])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Token tidak valid",
			})
			return
		}
		payload := string(payloadBytes)

		// Verify signature
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = "default-secret-key"
		}

		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(payload))
		expectedSignature := hex.EncodeToString(h.Sum(nil))

		if !hmac.Equal([]byte(parts[1]), []byte(expectedSignature)) {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Tanda tangan token tidak valid",
			})
			return
		}

		// Parse payload: id_jadwal|date|expires_unix
		payloadParts := splitPayload(payload)
		if len(payloadParts) != 3 {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format payload tidak valid",
			})
			return
		}

		var idJadwal int
		var dateStr string
		var expiresUnix int64

		if _, parseErr1 := fmt.Sscanf(payloadParts[0], "%d", &idJadwal); parseErr1 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format payload tidak valid: id_jadwal"})
			return
		}
		dateStr = payloadParts[1]
		if _, parseErr2 := fmt.Sscanf(payloadParts[2], "%d", &expiresUnix); parseErr2 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format payload tidak valid: expires_unix"})
			return
		}

		// Check expiry
		if utils.GetJakartaTime().Unix() > expiresUnix {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "QR code sudah kadaluarsa",
			})
			return
		}

		// Verify scan is for today
		today := utils.GetJakartaDateString()
		if dateStr != today {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "QR code tidak valid untuk hari ini",
			})
			return
		}

		// Get siswa info
		var siswa models.Siswa
		if errSiswa := db.First(&siswa, "nis = ?", nisStr).Error; errSiswa != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"message": "Siswa tidak ditemukan",
			})
			return
		}

		// Get jadwal info
		var jadwal models.JadwalSholat
		if errJadwal := db.First(&jadwal, "id_jadwal = ?", idJadwal).Error; errJadwal != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"message": "Jadwal sholat tidak ditemukan",
			})
			return
		}

		// Check if already absent today for this jadwal
		var existingAbsensi models.Absensi
		err = db.Where("id_siswa = ? AND id_jadwal = ? AND DATE(tanggal) = ?", siswa.IDSiswa, idJadwal, today).
			First(&existingAbsensi).Error

		if err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"message": fmt.Sprintf("Anda sudah absen untuk %s hari ini", jadwal.JenisSholat),
				"data": VerifyQRData{
					Valid:       false,
					NIS:         siswa.NIS,
					NamaSiswa:   siswa.NamaSiswa,
					Kelas:       siswa.Kelas,
					Jurusan:     siswa.Jurusan,
					JenisSholat: jadwal.JenisSholat,
					Tanggal:     today,
					Status:      existingAbsensi.Status,
				},
			})
			return
		}

		// Create absensi record
		tanggal, err := time.Parse("2006-01-02", today)
		if err != nil {
			logger.Errorw("Failed to parse today's date", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Kesalahan internal (tanggal)"})
			return
		}
		absensi := models.Absensi{
			IDSiswa:  siswa.IDSiswa,
			IDJadwal: idJadwal,
			Tanggal:  tanggal,
			Status:   "hadir",
		}

		if err := db.Create(&absensi).Error; err != nil {
			logger.Errorw("Failed to create absensi",
				"error", err.Error(),
				"nis", nisStr,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mencatat absensi",
			})
			return
		}

		// Invalidate statistics cache
		if utils.CacheEnabled() {
			if err := utils.GetCache().DeletePattern(context.Background(), "stats:*"); err != nil {
				logger.Warnw("Failed to invalidate statistics cache", "error", err.Error())
			}
		}

		logger.Infow("Absensi recorded via QR code",
			"nis", nisStr,
			"nama_siswa", siswa.NamaSiswa,
			"jadwal_id", idJadwal,
			"jenis_sholat", jadwal.JenisSholat,
		)

		c.JSON(http.StatusOK, VerifyQRResponse{
			Message: "Absensi berhasil dicatat",
			Data: VerifyQRData{
				Valid:       true,
				NIS:         siswa.NIS,
				NamaSiswa:   siswa.NamaSiswa,
				Kelas:       siswa.Kelas,
				Jurusan:     siswa.Jurusan,
				JenisSholat: jadwal.JenisSholat,
				Tanggal:     today,
				Status:      "hadir",
			},
		})
	}
}

// Helper to get Indonesian day name
func getDayName(weekday time.Weekday) string {
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

// Helper to split token by last dot
func splitToken(token string) []string {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return []string{token}
}

// Helper to split payload by pipe
func splitPayload(payload string) []string {
	var parts []string
	current := ""
	for _, c := range payload {
		if c == '|' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}

// GenerateQRCodeImage godoc
// @Summary Download QR Code sebagai file PNG
// @Description Menghasilkan dan mendownload QR code sebagai file PNG untuk sesi absensi sholat aktif. Mengembalikan file PNG 256x256 pixel dengan error correction level Medium. QR code berisi token yang ter-sign dan berlaku selama 5 menit. Gunakan endpoint ini untuk mencetak atau menampilkan QR code di layar. Hanya untuk admin, guru, atau wali_kelas
// @Tags qrcode
// @Accept json
// @Produce image/png
// @Security BearerAuth
// @Success 200 {file} binary "QR code PNG image (256x256 pixels). Filename: qrcode_[JenisSholat]_[IDJadwal].png"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi atau token tidak valid"
// @Failure 403 {object} ErrorResponse "Akses ditolak - hanya admin, guru, atau wali kelas yang dapat mengakses"
// @Failure 404 {object} ErrorResponse "Tidak ada jadwal sholat aktif saat ini"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal saat membuat QR code image"
// @Param force query string false "Set to 'true' to force-generate QR for testing (non-production only)"
// @Param jenis_sholat query string false "Specify prayer type (e.g., Maghrib) when forcing"
// @Param id_jadwal query int false "Specify jadwal ID when forcing"
// @Router /qrcode/image [get]
func GenerateQRCodeImage(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get current day and time in WIB (UTC+7)
		now := utils.GetJakartaTime()
		currentDay := getDayName(now.Weekday())
		currentTime := now.Format("15:04:05")

		// Find active jadwal sholat for current day and time
		var jadwal models.JadwalSholat

		// Support test forcing: allow admins to request a specific jadwal or jenis_sholat when ENVIRONMENT != production
		force := c.Query("force")
		jenis := c.Query("jenis_sholat")
		idJadwalStr := c.Query("id_jadwal")

		var err error
		if force == "true" && os.Getenv("ENVIRONMENT") != "production" {
			if idJadwalStr != "" {
				id, errConv := strconv.Atoi(idJadwalStr)
				if errConv != nil {
					c.JSON(http.StatusBadRequest, gin.H{"message": "id_jadwal tidak valid"})
					return
				}
				err = db.First(&jadwal, "id_jadwal = ?", id).Error
			} else if jenis != "" {
				err = db.Where("hari = ? AND jenis_sholat = ?", currentDay, jenis).First(&jadwal).Error
			} else {
				// fallback to active jadwal behavior
				err = db.Where("hari = ? AND waktu_mulai <= ? AND waktu_selesai >= ?",
					currentDay, currentTime, currentTime).
					First(&jadwal).Error
			}
		} else {
			err = db.Where("hari = ? AND waktu_mulai <= ? AND waktu_selesai >= ?",
				currentDay, currentTime, currentTime).
				First(&jadwal).Error
		}

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"message": "Tidak ada jadwal sholat aktif saat ini",
				})
				return
			}
			logger.Errorw("Failed to fetch jadwal sholat",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil jadwal sholat",
			})
			return
		}

		// Generate expiry time (5 minutes from now)
		expiresAt := now.Add(5 * time.Minute)

		// Create token payload: id_jadwal|date|expires_unix
		tokenPayload := fmt.Sprintf("%d|%s|%d",
			jadwal.IDJadwal,
			now.Format("2006-01-02"),
			expiresAt.Unix(),
		)

		// Sign the token with HMAC
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = "default-secret-key"
		}

		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(tokenPayload))
		signature := hex.EncodeToString(h.Sum(nil))

		// Create the final token
		token := base64.URLEncoding.EncodeToString([]byte(tokenPayload)) + "." + signature

		// Generate QR code as PNG
		qrPNG, err := qrcode.Encode(token, qrcode.Medium, 256)
		if err != nil {
			logger.Errorw("Failed to generate QR code",
				"error", err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal membuat QR code",
			})
			return
		}

		logger.Infow("QR code image generated successfully",
			"jadwal_id", jadwal.IDJadwal,
			"jenis_sholat", jadwal.JenisSholat,
			"expires_at", expiresAt,
		)

		// Set response headers for PNG image
		c.Header("Content-Type", "image/png")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"qrcode_%s_%d.png\"",
			jadwal.JenisSholat, jadwal.IDJadwal))
		c.Data(http.StatusOK, "image/png", qrPNG)
	}
}
