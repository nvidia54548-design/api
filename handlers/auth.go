package handlers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

type RegisterRequest struct {
	NIS      string `json:"nis" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Username   string `json:"username"`
	Password   string `json:"password" binding:"required"`
}

type LoginResponse struct {
	NIS          string `json:"nis,omitempty"`
	NamaSiswa    string `json:"nama_siswa,omitempty"`
	JK           string `json:"jk,omitempty"`
	Jurusan      string `json:"jurusan,omitempty"`
	Kelas        string `json:"kelas,omitempty"`
	Email        string `json:"email"`
	IsGoogleAcct bool   `json:"is_google_acct"`
	Token        string `json:"token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Username     string `json:"username,omitempty"`
	Role         string `json:"role,omitempty"`
	Name         string `json:"name,omitempty"`
	Nama         string `json:"nama,omitempty"`
	NIP          string `json:"nip,omitempty"`
	IDStaff      int    `json:"id_staff,omitempty"`
}

type RegisterResponse struct {
	Message      string `json:"message"`
	NIS          string `json:"nis"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
	IsGoogleAcct bool   `json:"is_google_acct"`
}

type LoginResponseData struct {
	Message string        `json:"message"`
	Data    LoginResponse `json:"data"`
}

type ErrorResponse struct {
	Message string      `json:"message"`
	Error   interface{} `json:"error,omitempty"`
}

func isGoogleAccount(email string) bool {
	return strings.HasSuffix(strings.ToLower(email), "@gmail.com")
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func Register(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RegisterRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format JSON tidak valid", "error": err.Error()})
			return
		}
		if !isValidEmail(input.Email) || !isGoogleAccount(input.Email) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Email harus valid dan menggunakan domain Google"})
			return
		}
		if err := utils.ValidatePassword(input.Password); err != nil {
			if pwdErr, ok := err.(*utils.PasswordValidationError); ok {
				c.JSON(http.StatusBadRequest, gin.H{"message": "Password tidak memenuhi syarat", "errors": pwdErr.Errors})
				return
			}
		}

		var siswa models.Siswa
		if err := db.First(&siswa, "nis = ?", input.NIS).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "NIS tidak terdaftar sebagai siswa"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memverifikasi NIS"})
			return
		}
		if siswa.IDAccount != 0 {
			c.JSON(http.StatusConflict, gin.H{"message": "Akun untuk NIS ini sudah terdaftar"})
			return
		}

		var existing models.Account
		if err := db.First(&existing, "email = ?", input.Email).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"message": "Email sudah digunakan"})
			return
		} else if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memeriksa email"})
			return
		}

		hashed, err := utils.HashPassword(input.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memproses password"})
			return
		}

		tx := db.Begin()
		account := models.Account{Email: input.Email, Password: hashed, Role: "siswa"}
		if err := tx.Create(&account).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat akun"})
			return
		}
		if err := tx.Model(&models.Siswa{}).Where("id_siswa = ?", siswa.IDSiswa).Update("id_account", account.ID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menautkan akun siswa"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan akun"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":        "Registrasi berhasil",
			"nis":            input.NIS,
			"email":          input.Email,
			"created_at":     account.CreatedAt,
			"is_google_acct": isGoogleAccount(input.Email),
		})
	}
}

func Login(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input LoginRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format JSON tidak valid atau field tidak lengkap", "error": err.Error()})
			return
		}

		loginID := strings.TrimSpace(input.Identifier)
		if loginID == "" {
			loginID = strings.TrimSpace(input.Username)
		}
		if loginID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "NIS atau Username harus diisi"})
			return
		}

		var siswa models.Siswa
		err := db.Preload("Account").Preload("KelasRef").Where("nis = ?", loginID).First(&siswa).Error
		if err != nil {
			err = db.Preload("Account").Preload("KelasRef").Joins("JOIN accounts ON accounts.id = siswa.id_account").Where("accounts.email = ?", loginID).First(&siswa).Error
		}
		if err == nil && siswa.Account != nil {
			if !utils.CheckPasswordHash(input.Password, siswa.Account.Password) {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "Password salah"})
				return
			}

			accessToken, tokenErr := utils.GenerateToken(siswa.NIS, siswa.Account.Email, "siswa", siswa.NamaSiswa)
			if tokenErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat token akses"})
				return
			}
			refreshToken, refreshErr := utils.GenerateRefreshToken(strconv.Itoa(siswa.IDAccount), "siswa")
			if refreshErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
				return
			}
			claims, claimErr := utils.ValidateToken(refreshToken)
			if claimErr != nil || claims.ExpiresAt == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
				return
			}
			if err := db.Create(&models.RefreshToken{AccountID: siswa.IDAccount, Token: refreshToken, ExpiresAt: claims.ExpiresAt.Time}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
				return
			}

			kelas := ""
			jurusan := ""
			if siswa.KelasRef != nil {
				kelas = strconv.Itoa(siswa.KelasRef.Tingkatan) + siswa.KelasRef.Part
				jurusan = siswa.KelasRef.Jurusan
			}
			c.JSON(http.StatusOK, gin.H{"message": "Login berhasil", "data": gin.H{
				"token":         accessToken,
				"refresh_token": refreshToken,
				"role":          "siswa",
				"nis":           siswa.NIS,
				"nama_siswa":    siswa.NamaSiswa,
				"jk":            siswa.JK,
				"jurusan":       jurusan,
				"kelas":         kelas,
				"email":         siswa.Account.Email,
			}})
			return
		}

		var staff models.Staff
		loginByEmail := true
		err = db.Preload("Account").Joins("JOIN accounts ON accounts.id = staff.id_account").Where("accounts.email = ?", loginID).First(&staff).Error
		if err != nil {
			err = db.Preload("Account").Where("nip = ?", loginID).First(&staff).Error
			loginByEmail = false
		}
		if err != nil || staff.Account == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "NIS/Username belum terdaftar atau belum membuat akun"})
			return
		}
		if !utils.CheckPasswordHash(input.Password, staff.Account.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Password salah"})
			return
		}

		accessToken, tokenErr := utils.GenerateTokenWithNIP(staff.Account.Email, staff.Account.Email, staff.Account.Role, staff.Nama, staff.NIP)
		if tokenErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat token akses staff"})
			return
		}
		refreshToken, refreshErr := utils.GenerateRefreshToken(strconv.Itoa(staff.IDAccount), staff.Account.Role)
		if refreshErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
			return
		}
		claims, claimErr := utils.ValidateToken(refreshToken)
		if claimErr != nil || claims.ExpiresAt == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
			return
		}
		if err := db.Create(&models.RefreshToken{AccountID: staff.IDAccount, Token: refreshToken, ExpiresAt: claims.ExpiresAt.Time}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
			return
		}

		_ = loginByEmail
		c.JSON(http.StatusOK, gin.H{"message": "Login berhasil", "data": gin.H{
			"token":         accessToken,
			"refresh_token": refreshToken,
			"role":          staff.Account.Role,
			"username":      staff.Account.Email,
			"nis":           staff.Account.Email,
			"nama":          staff.Nama,
			"name":          staff.Nama,
			"nip":           staff.NIP,
			"is_staff":      true,
		}})
	}
}

func Me(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, ok := role.(string)
		if !ok || strings.TrimSpace(roleStr) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "User not authenticated"})
			return
		}

		if roleStr == "siswa" {
			nis, exists := c.Get("nis")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "User not authenticated"})
				return
			}
			var siswa models.Siswa
			if err := db.Preload("Account").Preload("KelasRef").First(&siswa, "nis = ?", nis).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "Akun tidak ditemukan"})
				return
			}
			if siswa.Account == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "Akun tidak ditemukan"})
				return
			}
			kelas := ""
			jurusan := ""
			if siswa.KelasRef != nil {
				kelas = strconv.Itoa(siswa.KelasRef.Tingkatan) + siswa.KelasRef.Part
				jurusan = siswa.KelasRef.Jurusan
			}
			c.JSON(http.StatusOK, gin.H{"message": "Berhasil mendapatkan data profil", "data": LoginResponse{
				NIS:          siswa.NIS,
				NamaSiswa:    siswa.NamaSiswa,
				JK:           siswa.JK,
				Jurusan:      jurusan,
				Kelas:        kelas,
				Email:        siswa.Account.Email,
				IsGoogleAcct: isGoogleAccount(siswa.Account.Email),
				Role:         "siswa",
			}})
			return
		}

		username, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "User not authenticated"})
			return
		}
		var staff models.Staff
		if err := db.Preload("Account").Joins("JOIN accounts ON accounts.id = staff.id_account").Where("accounts.email = ?", username).First(&staff).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Akun tidak ditemukan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": LoginResponse{
			Username: staff.Account.Email,
			Role:     staff.Account.Role,
			IDStaff:  staff.IDStaff,
			Name:     staff.Nama,
			Nama:     staff.Nama,
			NIP:      staff.NIP,
		}})
	}
}

func RefreshToken(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Refresh token diperlukan", "error": err.Error()})
			return
		}

		refreshClaims, tokenErr := utils.ValidateToken(req.RefreshToken)
		if tokenErr != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.", "code": "REFRESH_TOKEN_INVALID"})
			return
		}

		var stored models.RefreshToken
		if err := db.First(&stored, "token = ?", req.RefreshToken).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.", "code": "REFRESH_TOKEN_INVALID"})
			return
		}

		tokenAccountID, convErr := strconv.Atoi(refreshClaims.Username)
		if convErr != nil {
			tokenAccountID, convErr = strconv.Atoi(refreshClaims.NIS)
		}
		if convErr != nil || tokenAccountID == 0 || stored.AccountID != tokenAccountID {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.", "code": "REFRESH_TOKEN_INVALID"})
			return
		}

		var account models.Account
		if err := db.First(&account, "id = ?", stored.AccountID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.", "code": "REFRESH_TOKEN_INVALID"})
			return
		}
		if refreshClaims.Role != "" && refreshClaims.Role != account.Role {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.", "code": "REFRESH_TOKEN_INVALID"})
			return
		}

		var newAccessToken string
		var err error
		if account.Role == "siswa" {
			var siswa models.Siswa
			if errSiswa := db.First(&siswa, "id_account = ?", stored.AccountID).Error; errSiswa != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil data siswa"})
				return
			}
			newAccessToken, err = utils.GenerateToken(siswa.NIS, account.Email, "siswa", siswa.NamaSiswa)
		} else {
			var staff models.Staff
			if errStaff := db.First(&staff, "id_account = ?", stored.AccountID).Error; errStaff != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil data staff"})
				return
			}
			newAccessToken, err = utils.GenerateTokenWithNIP(account.Email, account.Email, account.Role, staff.Nama, staff.NIP)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat token baru"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Token berhasil diperbarui", "access_token": newAccessToken})
	}
}

func Logout(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
			db.Delete(&models.RefreshToken{}, "token = ?", req.RefreshToken)
		}
		c.JSON(http.StatusOK, gin.H{"message": "Berhasil logout"})
	}
}
