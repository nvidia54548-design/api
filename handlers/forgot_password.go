package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

// ForgotPasswordRequest represents the forgot password request body
type ForgotPasswordRequest struct {
	NIS   string `json:"nis" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

// VerifyOTPRequest represents the OTP verification request body
type VerifyOTPRequest struct {
	NIS string `json:"nis" binding:"required"`
	OTP string `json:"otp" binding:"required"`
}

// ResetPasswordRequest represents the password reset request body
type ResetPasswordRequest struct {
	NIS         string `json:"nis" binding:"required"`
	OTP         string `json:"otp" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ForgotPassword godoc
// @Summary Request Password Reset
// @Description Request OTP untuk reset password. OTP akan dikirim ke email yang terdaftar
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ForgotPasswordRequest true "NIS dan Email untuk verifikasi"
// @Success 200 {object} map[string]interface{} "OTP berhasil dikirim ke email"
// @Failure 400 {object} ErrorResponse "Permintaan tidak valid - NIS/Email kosong atau format salah"
// @Failure 404 {object} ErrorResponse "Akun tidak ditemukan atau email tidak cocok"
// @Failure 500 {object} ErrorResponse "Gagal mengirim email atau error server"
// @Router /auth/forgot-password [post]
func ForgotPassword(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ForgotPasswordRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			logger.Warnw("Invalid JSON format for forgot password",
				"error", err.Error(),
				"ip", c.ClientIP(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format JSON tidak valid",
				"error":   err.Error(),
			})
			return
		}

		logger.Infow("Forgot password request received",
			"nis", input.NIS,
			"email", input.Email,
		)

		// Validate email format
		if !isValidEmail(input.Email) {
			logger.Warnw("Invalid email format",
				"email", input.Email,
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format email tidak valid",
			})
			return
		}

		// Check if account exists and email matches
		var account models.AkunLoginSiswa
		if err := db.Preload("Siswa").First(&account, "nis = ?", input.NIS).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Account not found for forgot password",
					"nis", input.NIS,
				)
				c.JSON(http.StatusNotFound, gin.H{
					"message": "Akun dengan NIS tersebut tidak ditemukan",
				})
				return
			}
			logger.Errorw("Database error during forgot password",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memproses permintaan",
			})
			return
		}

		// Verify email matches
		if account.Email != input.Email {
			logger.Warnw("Email mismatch for forgot password",
				"nis", input.NIS,
				"provided_email", input.Email,
				"registered_email", account.Email,
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Email tidak cocok dengan akun yang terdaftar",
			})
			return
		}

		// Generate OTP
		otpCode, err := utils.GenerateOTP()
		if err != nil {
			logger.Errorw("Failed to generate OTP",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal membuat kode OTP",
			})
			return
		}

		// Store OTP in Firebase with 10 minutes expiration
		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			logger.Errorw("OTP store not initialized",
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Layanan OTP tidak tersedia. Pastikan Firebase sudah dikonfigurasi",
			})
			return
		}

		if err := otpStore.SaveOTP(input.NIS, input.Email, otpCode, 10*time.Minute); err != nil {
			logger.Errorw("Failed to save OTP to Firebase",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menyimpan kode OTP",
			})
			return
		}

		// Get student name for email
		namaSiswa := "Siswa"
		if account.Siswa != nil {
			namaSiswa = account.Siswa.NamaSiswa
		}

		// Send OTP email
		if err := utils.SendOTPEmail(input.Email, otpCode, namaSiswa); err != nil {
			logger.Errorw("Failed to send OTP email",
				"error", err.Error(),
				"nis", input.NIS,
				"email", input.Email,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengirim email OTP",
				"error":   err.Error(),
			})
			return
		}

		logger.Infow("OTP generated and sent successfully",
			"nis", input.NIS,
			"email", input.Email,
		)

		c.JSON(http.StatusOK, gin.H{
			"message":    "Kode OTP telah dikirim ke email Anda",
			"email":      maskEmail(input.Email),
			"expires_in": "10 menit",
		})
	}
}

// VerifyOTP godoc
// @Summary Verify OTP Code
// @Description Verifikasi kode OTP yang dikirim ke email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body VerifyOTPRequest true "NIS dan kode OTP"
// @Success 200 {object} map[string]interface{} "OTP valid, dapat melanjutkan reset password"
// @Failure 400 {object} ErrorResponse "OTP tidak valid atau sudah kadaluarsa"
// @Failure 500 {object} ErrorResponse "Error server"
// @Router /auth/verify-otp [post]
func VerifyOTP(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input VerifyOTPRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			logger.Warnw("Invalid JSON format for verify OTP",
				"error", err.Error(),
				"ip", c.ClientIP(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format JSON tidak valid",
				"error":   err.Error(),
			})
			return
		}

		logger.Infow("OTP verification request received",
			"nis", input.NIS,
		)

		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			logger.Errorw("OTP store not initialized",
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Layanan OTP tidak tersedia. Pastikan Firebase sudah dikonfigurasi",
			})
			return
		}
		valid, err := otpStore.VerifyOTP(input.NIS, input.OTP)

		if err != nil {
			logger.Warnw("OTP verification failed",
				"nis", input.NIS,
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": err.Error(),
			})
			return
		}

		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Kode OTP tidak valid",
			})
			return
		}

		logger.Infow("OTP verified successfully",
			"nis", input.NIS,
		)

		c.JSON(http.StatusOK, gin.H{
			"message":  "Kode OTP valid. Silakan reset password Anda",
			"verified": true,
		})
	}
}

// ResetPassword godoc
// @Summary Reset Password
// @Description Reset password dengan OTP yang sudah diverifikasi
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "NIS, OTP, dan password baru"
// @Success 200 {object} map[string]interface{} "Password berhasil direset"
// @Failure 400 {object} ErrorResponse "OTP tidak valid atau belum diverifikasi"
// @Failure 500 {object} ErrorResponse "Gagal reset password"
// @Router /auth/reset-password [post]
func ResetPassword(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ResetPasswordRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			logger.Warnw("Invalid JSON format for reset password",
				"error", err.Error(),
				"ip", c.ClientIP(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format JSON tidak valid",
				"error":   err.Error(),
			})
			return
		}

		logger.Infow("Reset password request received",
			"nis", input.NIS,
		)

		// Validate password strength
		if err := utils.ValidatePassword(input.NewPassword); err != nil {
			if pwdErr, ok := err.(*utils.PasswordValidationError); ok {
				logger.Warnw("Password validation failed for reset",
					"nis", input.NIS,
					"errors", pwdErr.Errors,
				)
				c.JSON(http.StatusBadRequest, gin.H{
					"message": "Password tidak memenuhi syarat",
					"code":    "WEAK_PASSWORD",
					"errors":  pwdErr.Errors,
				})
				return
			}
		}

		otpStore := utils.GetOTPStore()
		if otpStore == nil {
			logger.Errorw("OTP store not initialized",
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Layanan OTP tidak tersedia. Pastikan Firebase sudah dikonfigurasi",
			})
			return
		}

		// Verify OTP again (in case user skipped verification step)
		valid, err := otpStore.VerifyOTP(input.NIS, input.OTP)
		if err != nil {
			logger.Warnw("OTP verification failed during password reset",
				"nis", input.NIS,
				"error", err.Error(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": err.Error(),
			})
			return
		}

		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Kode OTP tidak valid",
			})
			return
		}

		// Hash new password
		hashedPassword, err := utils.HashPassword(input.NewPassword)
		if err != nil {
			logger.Errorw("Failed to hash new password",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memproses password baru",
			})
			return
		}

		// Update password in database
		result := db.Model(&models.AkunLoginSiswa{}).
			Where("nis = ?", input.NIS).
			Update("password", hashedPassword)

		if result.Error != nil {
			logger.Errorw("Failed to update password",
				"error", result.Error.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal menyimpan password baru",
			})
			return
		}

		if result.RowsAffected == 0 {
			logger.Warnw("No account found to update password",
				"nis", input.NIS,
			)
			c.JSON(http.StatusNotFound, gin.H{
				"message": "Akun tidak ditemukan",
			})
			return
		}

		// Clear OTP after successful password reset
		if err := otpStore.ClearOTP(input.NIS); err != nil {
			logger.Warnw("Failed to clear OTP from Firebase",
				"error", err.Error(),
				"nis", input.NIS,
			)
			// Don't fail the request, password was already reset
		}

		logger.Infow("Password reset successful",
			"nis", input.NIS,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "Password berhasil direset. Silakan login dengan password baru",
		})
	}
}

// maskEmail masks the email address for privacy
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

	// Show first 2 characters and mask the rest before @
	masked := email[:2]
	for i := 2; i < atIndex; i++ {
		masked += "*"
	}
	masked += email[atIndex:]

	return masked
}
