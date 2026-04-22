package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/utils"
)

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

type VerifyEmailOTPRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
	OTP      string `json:"otp" binding:"required"`
}

type siswaAccountInfo struct {
	AccountID int
	Email     string
	NamaSiswa string
}

func findSiswaAccountByNIS(db *gorm.DB, nis string) (siswaAccountInfo, error) {
	var info siswaAccountInfo
	err := db.Table("siswa s").
		Select("a.id as account_id, a.email as email, s.nama_siswa").
		Joins("JOIN accounts a ON a.id = s.id_account").
		Where("s.nis = ?", nis).
		Scan(&info).Error
	if err != nil {
		return siswaAccountInfo{}, err
	}
	if info.AccountID == 0 {
		return siswaAccountInfo{}, gorm.ErrRecordNotFound
	}
	return info, nil
}

func RequestChangeEmail(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		nisVal, exists := c.Get("nis")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "User not authenticated"})
			return
		}
		nis, ok := nisVal.(string)
		if !ok || nis == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Format identitas user tidak valid"})
			return
		}

		var input ChangeEmailRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format JSON tidak valid", "error": err.Error()})
			return
		}
		if !isValidEmail(input.NewEmail) || !isGoogleAccount(input.NewEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Email harus valid dan menggunakan domain Google"})
			return
		}

		account, err := findSiswaAccountByNIS(db, nis)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memproses permintaan"})
			return
		}
		if account.Email == input.NewEmail {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Email baru sama dengan email saat ini"})
			return
		}

		var existingCount int64
		if err := db.Table("accounts").Where("email = ?", input.NewEmail).Count(&existingCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memverifikasi email"})
			return
		}
		if existingCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"message": "Email sudah digunakan oleh akun lain"})
			return
		}

		otpCode, err := utils.GenerateOTP()
		if err != nil {
			logger.Errorw("failed to generate OTP", "error", err.Error(), "nis", nis)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat kode OTP"})
			return
		}
		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Layanan OTP tidak tersedia. Pastikan Firebase sudah dikonfigurasi"})
			return
		}
		emailChangeKey := nis + "_email_change"
		if err := otpStore.SaveOTP(emailChangeKey, input.NewEmail, otpCode, 10*time.Minute); err != nil {
			logger.Errorw("failed to save OTP", "error", err.Error(), "nis", nis)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan kode OTP"})
			return
		}
		if err := utils.SendOTPEmail(input.NewEmail, otpCode, account.NamaSiswa); err != nil {
			logger.Errorw("failed to send OTP email", "error", err.Error(), "nis", nis)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengirim email OTP", "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Kode OTP telah dikirim ke email baru Anda", "email": maskEmail(input.NewEmail), "expires_in": "10 menit"})
	}
}

func VerifyAndChangeEmail(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		nisVal, exists := c.Get("nis")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "User not authenticated"})
			return
		}
		nis, ok := nisVal.(string)
		if !ok || nis == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Format identitas user tidak valid"})
			return
		}

		var input VerifyEmailOTPRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format JSON tidak valid", "error": err.Error()})
			return
		}
		if !isValidEmail(input.NewEmail) || !isGoogleAccount(input.NewEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Email harus valid dan menggunakan domain Google"})
			return
		}

		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Layanan OTP tidak tersedia"})
			return
		}
		emailChangeKey := nis + "_email_change"
		valid, err := otpStore.VerifyOTP(emailChangeKey, input.OTP)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Kode OTP tidak valid"})
			return
		}

		account, err := findSiswaAccountByNIS(db, nis)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Akun tidak ditemukan"})
			return
		}

		var existingCount int64
		if err := db.Table("accounts").Where("email = ?", input.NewEmail).Count(&existingCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memverifikasi email"})
			return
		}
		if existingCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"message": "Email sudah digunakan oleh akun lain"})
			return
		}

		if err := db.Table("accounts").Where("id = ?", account.AccountID).Update("email", input.NewEmail).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan email baru"})
			return
		}

		_ = otpStore.ClearOTP(emailChangeKey)
		c.JSON(http.StatusOK, gin.H{"message": "Email berhasil diubah", "new_email": input.NewEmail})
	}
}
