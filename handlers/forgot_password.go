package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/utils"
)

type ForgotPasswordRequest struct {
	NIS   string `json:"nis" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

type VerifyOTPRequest struct {
	NIS string `json:"nis" binding:"required"`
	OTP string `json:"otp" binding:"required"`
}

type ResetPasswordRequest struct {
	NIS         string `json:"nis" binding:"required"`
	OTP         string `json:"otp" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type forgotAccountInfo struct {
	AccountID int
	Email     string
	NamaSiswa string
}

func findForgotAccount(db *gorm.DB, nis string) (forgotAccountInfo, error) {
	var info forgotAccountInfo
	err := db.Table("siswa s").
		Select("a.id as account_id, a.email as email, s.nama_siswa").
		Joins("JOIN accounts a ON a.id = s.id_account").
		Where("s.nis = ?", nis).
		Scan(&info).Error
	if err != nil {
		return forgotAccountInfo{}, err
	}
	if info.AccountID == 0 {
		return forgotAccountInfo{}, gorm.ErrRecordNotFound
	}
	return info, nil
}

func ForgotPassword(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ForgotPasswordRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format JSON tidak valid", "error": err.Error()})
			return
		}
		if !isValidEmail(input.Email) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format email tidak valid"})
			return
		}

		account, err := findForgotAccount(db, input.NIS)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "Akun dengan NIS tersebut tidak ditemukan"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memproses permintaan"})
			return
		}
		if account.Email != input.Email {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Email tidak cocok dengan akun yang terdaftar"})
			return
		}

		otpCode, err := utils.GenerateOTP()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat kode OTP"})
			return
		}
		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Layanan OTP tidak tersedia. Pastikan Firebase sudah dikonfigurasi"})
			return
		}
		if err := otpStore.SaveOTP(input.NIS, input.Email, otpCode, 10*time.Minute); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan kode OTP"})
			return
		}
		if err := utils.SendOTPEmail(input.Email, otpCode, account.NamaSiswa); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengirim email OTP", "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Kode OTP telah dikirim ke email Anda", "email": maskEmail(input.Email), "expires_in": "10 menit"})
	}
}

func VerifyOTP(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input VerifyOTPRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format JSON tidak valid", "error": err.Error()})
			return
		}

		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Layanan OTP tidak tersedia. Pastikan Firebase sudah dikonfigurasi"})
			return
		}
		valid, err := otpStore.VerifyOTP(input.NIS, input.OTP)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Kode OTP tidak valid"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Kode OTP valid. Silakan reset password Anda", "verified": true})
	}
}

func ResetPassword(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ResetPasswordRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format JSON tidak valid", "error": err.Error()})
			return
		}
		if err := utils.ValidatePassword(input.NewPassword); err != nil {
			if pwdErr, ok := err.(*utils.PasswordValidationError); ok {
				c.JSON(http.StatusBadRequest, gin.H{"message": "Password tidak memenuhi syarat", "code": "WEAK_PASSWORD", "errors": pwdErr.Errors})
				return
			}
		}

		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Layanan OTP tidak tersedia. Pastikan Firebase sudah dikonfigurasi"})
			return
		}
		valid, err := otpStore.VerifyOTP(input.NIS, input.OTP)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Kode OTP tidak valid"})
			return
		}

		account, err := findForgotAccount(db, input.NIS)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Akun tidak ditemukan"})
			return
		}
		hashedPassword, err := utils.HashPassword(input.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memproses password baru"})
			return
		}
		if err := db.Table("accounts").Where("id = ?", account.AccountID).Update("password", hashedPassword).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan password baru"})
			return
		}
		_ = otpStore.ClearOTP(input.NIS)
		c.JSON(http.StatusOK, gin.H{"message": "Password berhasil direset. Silakan login dengan password baru"})
	}
}

func maskEmail(email string) string {
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}
	if atIndex <= 1 {
		return email
	}
	masked := email[:2]
	for i := 2; i < atIndex; i++ {
		masked += "*"
	}
	masked += email[atIndex:]
	return masked
}
