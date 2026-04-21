package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	NIS      string `json:"nis" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Identifier string `json:"identifier"` // Primary: 'identifier'
	Username   string `json:"username"`   // Legacy/Alternative: 'username'
	Password   string `json:"password" binding:"required"`
}

// LoginResponse represents the login response
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
	// Staff fields
	Username string `json:"username,omitempty"`
	Role     string `json:"role,omitempty"`
	Name     string `json:"name,omitempty"` // Compatibility with old API
	Nama     string `json:"nama,omitempty"` // Matches Android AkunLoginResponse
	NIP      string `json:"nip,omitempty"`
	IDStaff  int    `json:"id_staff,omitempty"`
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

// isGoogleAccount validates if email is a Google account
func isGoogleAccount(email string) bool {
	return strings.HasSuffix(strings.ToLower(email), "@gmail.com")
}

// isValidEmail validates email format
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// Register godoc
// @Summary Pendaftaran Siswa
// @Description Mendaftarkan akun siswa baru untuk sistem absensi. Siswa harus memiliki NIS yang terdaftar di sistem dan menggunakan email Google
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Data pendaftaran siswa"
// @Success 201 {object} RegisterResponse "Pendaftaran berhasil - Akun siswa berhasil dibuat"
// @Failure 400 {object} ErrorResponse "Permintaan tidak valid - NIS/Password/Email kosong, format email salah, atau bukan email Google"
// @Failure 401 {object} ErrorResponse "NIS tidak terdaftar - NIS tidak ditemukan di database siswa"
// @Failure 409 {object} ErrorResponse "Akun sudah terdaftar - NIS sudah memiliki akun login"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal - Database error atau sistem error"
// @Router /auth/register [post]
func Register(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RegisterRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			logger.Warnw("Invalid JSON format",
				"error", err.Error(),
				"ip", c.ClientIP(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format JSON tidak valid",
				"error":   err.Error(),
			})
			return
		}

		logger.Infow("Register request received",
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

		// Validate Google account
		if !isGoogleAccount(input.Email) {
			logger.Warnw("Non-Google email provided",
				"email", input.Email,
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Email harus menggunakan domain Google (contoh: @gmail.com)",
			})
			return
		}

		// Check if NIS exists in siswa table
		var siswa models.Siswa
		if err := db.First(&siswa, "nis = ?", input.NIS).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("NIS not found",
					"nis", input.NIS,
				)
				c.JSON(http.StatusUnauthorized, gin.H{
					"message": "NIS tidak terdaftar sebagai siswa",
				})
				return
			}
			logger.Errorw("Database error while checking NIS",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memverifikasi NIS",
			})
			return
		}

		// Check if account already exists
		var existingAccount models.AkunLoginSiswa
		if err := db.First(&existingAccount, "nis = ?", input.NIS).Error; err == nil {
			logger.Warnw("Account already registered",
				"nis", input.NIS,
			)
			c.JSON(http.StatusConflict, gin.H{
				"message": "Akun untuk NIS ini sudah terdaftar",
			})
			return
		} else if err != gorm.ErrRecordNotFound {
			logger.Errorw("Database error while checking existing account",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memeriksa status akun",
			})
			return
		}

		// Validate password strength
		if err := utils.ValidatePassword(input.Password); err != nil {
			if pwdErr, ok := err.(*utils.PasswordValidationError); ok {
				logger.Warnw("Password validation failed",
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

		// Create new account with hashed password
		hashedPassword, err := utils.HashPassword(input.Password)
		if err != nil {
			logger.Errorw("Failed to hash password",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal memproses password",
			})
			return
		}

		newAccount := models.AkunLoginSiswa{
			NIS:      input.NIS,
			Password: hashedPassword,
			Email:    input.Email,
		}

		if err := db.Create(&newAccount).Error; err != nil {
			logger.Errorw("Failed to create account",
				"error", err.Error(),
				"nis", input.NIS,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal membuat akun",
			})
			return
		}

		logger.Infow("Account created successfully",
			"nis", input.NIS,
			"email", input.Email,
		)

		c.JSON(http.StatusCreated, gin.H{
			"message":        "Registrasi berhasil",
			"nis":            input.NIS,
			"email":          input.Email,
			"created_at":     newAccount.CreatedAt,
			"is_google_acct": isGoogleAccount(input.Email),
		})
	}
}

// Login godoc
// @Summary Login Pengguna
// @Description Login untuk sistem absensi menggunakan identifier (NIS untuk siswa, username untuk admin/guru) dan password. Mengembalikan data profil pengguna jika login berhasil
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Kredensial login (identifier dan password)"
// @Success 200 {object} LoginResponseData "Login berhasil - Mengembalikan profil lengkap pengguna beserta token"
// @Failure 400 {object} ErrorResponse "Permintaan tidak valid - Identifier atau password kosong, atau format JSON salah"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi - Identifier tidak ditemukan atau password salah"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal - Database error atau sistem error"
// @Router /auth/login [post]
func Login(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input LoginRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			logger.Warnw("Invalid JSON format",
				"error", err.Error(),
				"ip", c.ClientIP(),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Format JSON tidak valid atau field tidak lengkap",
				"error":   err.Error(),
			})
			return
		}

		// Support both 'identifier' and 'username' fields
		loginID := input.Identifier
		if loginID == "" {
			loginID = input.Username
		}

		if loginID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "NIS atau Username harus diisi",
			})
			return
		}

		logger.Infow("Login effort received", "identifier", loginID)

		// 1. Try finding Student (AkunLoginSiswa)
		var studentAccount models.AkunLoginSiswa
		if err := db.Preload("Siswa").First(&studentAccount, "nis = ?", loginID).Error; err == nil {
			// Validate password
			if !utils.CheckPasswordHash(input.Password, studentAccount.Password) {
				logger.Warnw("Invalid password for siswa", "nis", loginID)
				c.JSON(http.StatusUnauthorized, gin.H{"message": "Password salah"})
				return
			}

			// Generate Access Token
			accessToken, err := utils.GenerateToken(studentAccount.NIS, studentAccount.Email, "siswa", studentAccount.Siswa.NamaSiswa)
			if err != nil {
				logger.Errorw("Token generation failed", "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat token akses"})
				return
			}

			// Generate Refresh Token
			refreshToken, terr := utils.GenerateRefreshToken(studentAccount.NIS, "siswa")
			if terr != nil {
				logger.Errorw("Failed to generate refresh token", "error", terr.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
				return
			}

			refreshClaims, err := utils.ValidateToken(refreshToken)
			if err != nil || refreshClaims.ExpiresAt == nil {
				logger.Errorw("Failed to read refresh token expiration", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
				return
			}

			// Store/Update Refresh Token in DB
			if err := db.Create(&models.RefreshToken{
				UserID:    studentAccount.NIS,
				Token:     refreshToken,
				Role:      "siswa",
				ExpiresAt: refreshClaims.ExpiresAt.Time,
			}).Error; err != nil {
				logger.Errorw("Failed to store refresh token", "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
				return
			}

			logger.Infow("Siswa login successful", "nis", loginID)

			c.JSON(http.StatusOK, gin.H{
				"message": "Login berhasil",
				"data": gin.H{
					"token":         accessToken,
					"refresh_token": refreshToken,
					"role":          "siswa",
					"nis":           studentAccount.NIS,
					"nama_siswa":    studentAccount.Siswa.NamaSiswa,
					"jk":            studentAccount.Siswa.JK,
					"jurusan":       studentAccount.Siswa.Jurusan,
					"kelas":         studentAccount.Siswa.Kelas,
					"email":         studentAccount.Email,
				},
			})
			return
		}

		// 2. Try finding Staff
		var staffAccount models.UserStaff
		var loginByUsername bool = true

		// First, try finding by username
		err := db.First(&staffAccount, "username = ?", loginID).Error
		if err != nil {
			// If not found by username, try finding by NIP for teachers/wali kelas
			var guru models.Guru
			if err := db.Where("nip = ?", loginID).First(&guru).Error; err == nil {
				// Found guru/wali kelas by NIP, now get their staff account
				if err := db.First(&staffAccount, "id_staff = ?", guru.IDStaff).Error; err == nil {
					loginByUsername = false
					logger.Infow("Staff login attempt by NIP", "nip", loginID, "id_staff", guru.IDStaff)
				} else {
					// Guru found but no staff account
					logger.Warnw("Guru found but no staff account", "nip", loginID, "id_staff", guru.IDStaff)
					c.JSON(http.StatusUnauthorized, gin.H{"message": "NIS/Username belum terdaftar atau belum membuat akun"})
					return
				}
			} else {
				// Not found by username or NIP
				logger.Warnw("Account not found", "identifier", loginID)
				c.JSON(http.StatusUnauthorized, gin.H{
					"message": "NIS/Username belum terdaftar atau belum membuat akun",
				})
				return
			}
		} else {
			logger.Infow("Staff login attempt by username", "username", loginID)
		}

		// Validate password
		if !utils.CheckPasswordHash(input.Password, staffAccount.Password) {
			logger.Warnw("Invalid password for staff", "username", staffAccount.Username, "login_method", map[bool]string{true: "username", false: "nip"}[loginByUsername])
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Password salah"})
			return
		}

		// Fetch Staff-specific profile details
		var name string
		var nip string
		if staffAccount.Role == "admin" {
			var adm models.Admin
			if err := db.First(&adm, "id_staff = ?", staffAccount.IDStaff).Error; err == nil {
				name = adm.NamaAdmin
			}
		} else if staffAccount.Role == "guru" || staffAccount.Role == "wali_kelas" {
			var gru models.Guru
			if err := db.First(&gru, "id_staff = ?", staffAccount.IDStaff).Error; err == nil {
				name = gru.NamaGuru
				nip = gru.NIP
			}
		}

		// Generate Token
		accessToken, err := utils.GenerateTokenWithNIP(staffAccount.Username, "", staffAccount.Role, name, nip)
		if err != nil {
			logger.Errorw("Staff token generation failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat token akses staff"})
			return
		}

		refreshToken, terr := utils.GenerateRefreshToken(staffAccount.Username, staffAccount.Role)
		if terr != nil {
			logger.Errorw("Failed to generate refresh token", "error", terr.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
			return
		}

		refreshClaims, err := utils.ValidateToken(refreshToken)
		if err != nil || refreshClaims.ExpiresAt == nil {
			logger.Errorw("Failed to read refresh token expiration", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
			return
		}

		// Store/Update Refresh Token
		if err := db.Create(&models.RefreshToken{
			UserID:    staffAccount.Username,
			Token:     refreshToken,
			Role:      staffAccount.Role,
			ExpiresAt: refreshClaims.ExpiresAt.Time,
		}).Error; err != nil {
			logger.Errorw("Failed to store refresh token", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat sesi login"})
			return
		}

		loginMethod := "username"
		if !loginByUsername {
			loginMethod = "nip"
		}
		logger.Infow("Staff login successful", "username", staffAccount.Username, "role", staffAccount.Role, "login_method", loginMethod)

		c.JSON(http.StatusOK, gin.H{
			"message": "Login berhasil",
			"data": gin.H{
				"token":         accessToken,
				"refresh_token": refreshToken,
				"role":          staffAccount.Role,
				"username":      staffAccount.Username,
				"nis":           staffAccount.Username, // Mobile app compatibility for identifier
				"nama":          name,                  // Matches Android 'nama'
				"name":          name,                  // Compatibility
				"nip":           nip,
				"is_staff":      true,
			},
		})
	}
}

// Me godoc
// @Summary Get Current User
// @Description Mendapatkan data profil user yang sedang login (siswa atau staff)
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} LoginResponse "Berhasil mendapatkan data profil"
// @Failure 401 {object} ErrorResponse "Tidak terotentikasi - Token tidak valid atau tidak ada"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal - Database error"
// @Router /auth/me [get]
func Me(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Format role tidak valid"})
			return
		}

		// Handle siswa (student) users
		if roleStr == "siswa" {
			nis, exists := c.Get("nis")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{
					"message": "User not authenticated",
				})
				return
			}

			var account models.AkunLoginSiswa
			if err := db.Preload("Siswa").First(&account, "nis = ?", nis).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					logger.Warnw("Account not found",
						"nis", nis,
					)
					c.JSON(http.StatusUnauthorized, gin.H{
						"message": "Akun tidak ditemukan",
					})
					return
				}
				logger.Errorw("Database error during me request",
					"error", err.Error(),
					"nis", nis,
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal mengambil data profil",
				})
				return
			}

			response := LoginResponse{
				NIS:          account.NIS,
				NamaSiswa:    account.Siswa.NamaSiswa,
				JK:           account.Siswa.JK,
				Jurusan:      account.Siswa.Jurusan,
				Kelas:        account.Siswa.Kelas,
				Email:        account.Email,
				IsGoogleAcct: isGoogleAccount(account.Email),
				Role:         "siswa",
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Berhasil mendapatkan data profil",
				"data":    response,
			})
			return
		}

		// Handle staff users (admin, guru, wali_kelas)
		username, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "User not authenticated",
			})
			return
		}

		var staff models.UserStaff
		if err := db.First(&staff, "username = ?", username).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warnw("Staff account not found",
					"username", username,
				)
				c.JSON(http.StatusUnauthorized, gin.H{
					"message": "Akun tidak ditemukan",
				})
				return
			}
			logger.Errorw("Database error during me request",
				"error", err.Error(),
				"username", username,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mengambil data profil",
			})
			return
		}

		response := LoginResponse{
			Username: staff.Username,
			Role:     staff.Role,
			IDStaff:  staff.IDStaff,
		}

		// Get additional info based on role
		if staff.Role == "admin" {
			var admin models.Admin
			if err := db.First(&admin, "id_staff = ?", staff.IDStaff).Error; err == nil {
				response.Name = admin.NamaAdmin
			}
		} else if staff.Role == "guru" || staff.Role == "wali_kelas" {
			var guru models.Guru
			if err := db.First(&guru, "id_staff = ?", staff.IDStaff).Error; err == nil {
				response.Name = guru.NamaGuru
				response.NIP = guru.NIP
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"data": response,
		})
	}
}

// RefreshToken godoc
// @Summary Perbarui Token Akses
// @Description Memperbarui access token yang sudah expired menggunakan refresh token yang masih valid
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Refresh token"
// @Success 200 {object} object "Token berhasil diperbarui"
// @Failure 401 {object} ErrorResponse "Refresh token tidak valid atau sudah expired"
// @Failure 500 {object} ErrorResponse "Kesalahan server internal"
// @Router /auth/refresh [post]
func RefreshToken(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Refresh token diperlukan",
				"error":   err.Error(),
			})
			return
		}

		// Validate JWT signature + expiry from token claims (source of truth for expiration)
		refreshClaims, tokenErr := utils.ValidateToken(req.RefreshToken)
		if tokenErr != nil {
			logger.Warnw("Invalid or expired refresh token (JWT validation failed)",
				"error", tokenErr.Error(),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.",
				"code":    "REFRESH_TOKEN_INVALID",
			})
			return
		}

		// Validate token existence in DB (for revocation/logout checks)
		var storedToken models.RefreshToken
		if err := db.First(&storedToken, "token = ?", req.RefreshToken).Error; err != nil {
			logger.Warnw("Refresh token not found in DB",
				"token", req.RefreshToken,
				"error", err.Error(),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.",
				"code":    "REFRESH_TOKEN_INVALID",
			})
			return
		}

		// Ensure DB record matches token claims to prevent metadata mismatch
		tokenUserID := refreshClaims.Username
		if tokenUserID == "" {
			tokenUserID = refreshClaims.NIS
		}
		if tokenUserID == "" || refreshClaims.Role == "" || storedToken.UserID != tokenUserID || storedToken.Role != refreshClaims.Role {
			logger.Warnw("Refresh token claims mismatch with DB record",
				"db_user_id", storedToken.UserID,
				"db_role", storedToken.Role,
				"token_user_id", tokenUserID,
				"token_role", refreshClaims.Role,
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Sesi tidak valid atau sudah berakhir. Silakan login kembali.",
				"code":    "REFRESH_TOKEN_INVALID",
			})
			return
		}

		// Generate new access token based on role
		var newAccessToken string
		var err error

		if storedToken.Role == "siswa" {
			var account models.AkunLoginSiswa
			if errSiswa := db.Preload("Siswa").First(&account, "nis = ?", storedToken.UserID).Error; errSiswa != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil data siswa"})
				return
			}
			newAccessToken, err = utils.GenerateToken(account.NIS, account.Email, "siswa", account.Siswa.NamaSiswa)
		} else {
			// Staff (admin, guru, wali_kelas)
			var staffAccount models.UserStaff
			if errStaff := db.First(&staffAccount, "username = ?", storedToken.UserID).Error; errStaff != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil data staff"})
				return
			}
			// Get name and nip for staff
			var name, nip string
			if staffAccount.Role == "admin" {
				var admin models.Admin
				db.Where("id_staff = ?", staffAccount.IDStaff).First(&admin)
				name = admin.NamaAdmin
				nip = "-"
			} else {
				var guru models.Guru
				db.Where("id_staff = ?", staffAccount.IDStaff).First(&guru)
				name = guru.NamaGuru
				nip = guru.NIP
			}
			newAccessToken, err = utils.GenerateTokenWithNIP(staffAccount.Username, "", staffAccount.Role, name, nip)
		}

		if err != nil {
			logger.Errorw("Failed to generate new access token", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat token baru"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      "Token berhasil diperbarui",
			"access_token": newAccessToken,
		})
	}
}

// Logout godoc
// @Summary Logout Pengguna
// @Description Menghapus sesi login dengan menghapus refresh token dari sistem
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Refresh token yang akan dihapus"
// @Success 200 {object} object "Berhasil logout"
// @Router /auth/logout [post]
func Logout(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}

		// If refresh token is provided, delete it
		if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
			db.Delete(&models.RefreshToken{}, "token = ?", req.RefreshToken)
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Berhasil logout",
		})
	}
}
