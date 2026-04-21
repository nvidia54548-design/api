package handlers

import (
	"github.com/stretchr/testify/require"

	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"absensholat-api/models"
	"absensholat-api/utils"

	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Auto-migrate models
	err = db.AutoMigrate(&models.Siswa{}, &models.AkunLoginSiswa{}, &models.UserStaff{}, &models.Admin{}, &models.Guru{}, &models.RefreshToken{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

// setupTestLogger creates a no-op logger for testing
func setupTestLogger() *zap.SugaredLogger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return logger.Sugar()
}

// Helper to create JSON request body
func jsonBody(data interface{}) *bytes.Buffer {
	body, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(body)
}

// ============================================
// Registration Tests
// ============================================

func TestRegister_Success(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create a student first (required for registration)
	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
		Jurusan:   "IPA",
		Kelas:     "XII",
	})

	router := gin.New()
	router.POST("/register", Register(db, logger))

	body := jsonBody(RegisterRequest{
		NIS:      "12345",
		Password: "SecurePass123!",
		Email:    "student@gmail.com",
	})

	req, err := http.NewRequest("POST", "/register", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	// Verify account was created
	var account models.AkunLoginSiswa
	if err := db.First(&account, "nis = ?", "12345").Error; err != nil {
		t.Errorf("Account not created in database: %v", err)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	router := gin.New()
	router.POST("/register", Register(db, logger))

	req, err := http.NewRequest("POST", "/register", bytes.NewBufferString("invalid json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
	})

	router := gin.New()
	router.POST("/register", Register(db, logger))

	body := jsonBody(RegisterRequest{
		NIS:      "12345",
		Password: "SecurePass123!",
		Email:    "invalid-email",
	})

	req, err := http.NewRequest("POST", "/register", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRegister_NonGoogleEmail(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
	})

	router := gin.New()
	router.POST("/register", Register(db, logger))

	body := jsonBody(RegisterRequest{
		NIS:      "12345",
		Password: "SecurePass123!",
		Email:    "student@yahoo.com", // Not Gmail
	})

	req, err := http.NewRequest("POST", "/register", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for non-Google email, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRegister_NISNotFound(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	router := gin.New()
	router.POST("/register", Register(db, logger))

	body := jsonBody(RegisterRequest{
		NIS:      "nonexistent",
		Password: "SecurePass123!",
		Email:    "student@gmail.com",
	})

	req, err := http.NewRequest("POST", "/register", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRegister_DuplicateAccount(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create student
	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
	})

	// Create existing account
	hashedPwd, err := utils.HashPassword("existingPass123!")
	require.NoError(t, err)
	db.Create(&models.AkunLoginSiswa{
		NIS:      "12345",
		Password: hashedPwd,
		Email:    "existing@gmail.com",
	})

	router := gin.New()
	router.POST("/register", Register(db, logger))

	body := jsonBody(RegisterRequest{
		NIS:      "12345",
		Password: "SecurePass123!",
		Email:    "new@gmail.com",
	})

	req, err := http.NewRequest("POST", "/register", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d for duplicate account, got %d", http.StatusConflict, w.Code)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
	})

	router := gin.New()
	router.POST("/register", Register(db, logger))

	body := jsonBody(RegisterRequest{
		NIS:      "12345",
		Password: "weak", // Too short, missing requirements
		Email:    "student@gmail.com",
	})

	req, err := http.NewRequest("POST", "/register", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for weak password, got %d", http.StatusBadRequest, w.Code)
	}

	// Check that WEAK_PASSWORD code is in response
	if !bytes.Contains(w.Body.Bytes(), []byte("WEAK_PASSWORD")) {
		t.Errorf("Expected WEAK_PASSWORD error code in response: %s", w.Body.String())
	}
}

// ============================================
// Login Tests
// ============================================

func TestLogin_Success_Siswa(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create student
	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
		Jurusan:   "IPA",
		Kelas:     "XII",
	})

	// Create account
	hashedPwd, err := utils.HashPassword("SecurePass123!")
	require.NoError(t, err)
	db.Create(&models.AkunLoginSiswa{
		NIS:      "12345",
		Password: hashedPwd,
		Email:    "student@gmail.com",
	})

	router := gin.New()
	router.POST("/login", Login(db, logger))

	body := jsonBody(LoginRequest{
		Identifier: "12345",
		Password:   "SecurePass123!",
	})

	req, err := http.NewRequest("POST", "/login", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify token is returned
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err) // Gunakan 't' untuk fungsi test
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'data' field")
	}
	if data["token"] == nil || data["token"] == "" {
		t.Error("Response missing token")
	}
}

func TestLogin_Success_Staff(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create staff user
	hashedPwd, err := utils.HashPassword("AdminPass123!")
	require.NoError(t, err)
	db.Create(&models.UserStaff{
		IDStaff:  1,
		Username: "admin001",
		Password: hashedPwd,
		Role:     "admin",
	})

	// Create admin
	db.Create(&models.Admin{
		IDAdmin:   1,
		IDStaff:   1,
		NamaAdmin: "Admin User",
	})

	router := gin.New()
	router.POST("/login", Login(db, logger))

	body := jsonBody(LoginRequest{
		Identifier: "admin001",
		Password:   "AdminPass123!",
	})

	req, err := http.NewRequest("POST", "/login", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create student and account
	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
	})
	hashedPwd, err := utils.HashPassword("SecurePass123!")
	require.NoError(t, err)
	db.Create(&models.AkunLoginSiswa{
		NIS:      "12345",
		Password: hashedPwd,
		Email:    "student@gmail.com",
	})

	router := gin.New()
	router.POST("/login", Login(db, logger))

	body := jsonBody(LoginRequest{
		Identifier: "12345",
		Password:   "WrongPassword!",
	})

	req, err := http.NewRequest("POST", "/login", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestLogin_AccountNotFound(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	router := gin.New()
	router.POST("/login", Login(db, logger))

	body := jsonBody(LoginRequest{
		Identifier: "nonexistent",
		Password:   "password",
	})

	req, err := http.NewRequest("POST", "/login", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	router := gin.New()
	router.POST("/login", Login(db, logger))

	req, err := http.NewRequest("POST", "/login", bytes.NewBufferString("{invalid}"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// ============================================
// Me Endpoint Tests
// ============================================

func TestMe_Siswa(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create student and account
	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
		Jurusan:   "IPA",
		Kelas:     "XII",
	})
	hashedPwd, err := utils.HashPassword("SecurePass123!")
	require.NoError(t, err)
	db.Create(&models.AkunLoginSiswa{
		NIS:      "12345",
		Password: hashedPwd,
		Email:    "student@gmail.com",
	})

	router := gin.New()
	router.GET("/me", func(c *gin.Context) {
		// Simulate middleware setting context values
		c.Set("nis", "12345")
		c.Set("role", "siswa")
		c.Next()
	}, Me(db, logger))

	req, err := http.NewRequest("GET", "/me", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err) // Gunakan 't' untuk fungsi test
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'data' field")
	}
	if data["nis"] != "12345" {
		t.Errorf("Expected NIS '12345', got '%v'", data["nis"])
	}
}

func TestMe_Staff(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create staff user
	hashedPwd, err := utils.HashPassword("AdminPass123!")
	require.NoError(t, err)
	db.Create(&models.UserStaff{
		IDStaff:  1,
		Username: "admin001",
		Password: hashedPwd,
		Role:     "admin",
	})
	db.Create(&models.Admin{
		IDAdmin:   1,
		IDStaff:   1,
		NamaAdmin: "Admin User",
	})

	router := gin.New()
	router.GET("/me", func(c *gin.Context) {
		c.Set("username", "admin001")
		c.Set("role", "admin")
		c.Next()
	}, Me(db, logger))

	req, err := http.NewRequest("GET", "/me", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify response contains staff data
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err) // Gunakan 't' untuk fungsi test
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'data' field")
	}
	if data["username"] != "admin001" {
		t.Errorf("Expected username 'admin001', got '%v'", data["username"])
	}
	if data["role"] != "admin" {
		t.Errorf("Expected role 'admin', got '%v'", data["role"])
	}
	if data["name"] != "Admin User" {
		t.Errorf("Expected name 'Admin User', got '%v'", data["name"])
	}
}

func TestMe_Guru(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	// Create guru staff user
	hashedPwd, err := utils.HashPassword("GuruPass123!")
	require.NoError(t, err)
	db.Create(&models.UserStaff{
		IDStaff:  1,
		Username: "guru001",
		Password: hashedPwd,
		Role:     "guru",
	})
	db.Create(&models.Guru{
		IDGuru:   1,
		IDStaff:  1,
		NIP:      "198501012010011001",
		NamaGuru: "Budi Santoso",
	})

	router := gin.New()
	router.GET("/me", func(c *gin.Context) {
		c.Set("username", "guru001")
		c.Set("role", "guru")
		c.Next()
	}, Me(db, logger))

	req, err := http.NewRequest("GET", "/me", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify response contains guru data with NIP
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err) // Gunakan 't' untuk fungsi test
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'data' field")
	}
	if data["username"] != "guru001" {
		t.Errorf("Expected username 'guru001', got '%v'", data["username"])
	}
	if data["role"] != "guru" {
		t.Errorf("Expected role 'guru', got '%v'", data["role"])
	}
	if data["name"] != "Budi Santoso" {
		t.Errorf("Expected name 'Budi Santoso', got '%v'", data["name"])
	}
	if data["nip"] != "198501012010011001" {
		t.Errorf("Expected NIP '198501012010011001', got '%v'", data["nip"])
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	logger := setupTestLogger()

	router := gin.New()
	router.GET("/me", Me(db, logger))

	req, err := http.NewRequest("GET", "/me", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// ============================================
// Helper Function Tests
// ============================================

func TestIsGoogleAccount(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"test@gmail.com", true},
		{"test@GMAIL.COM", true},
		{"test@Gmail.Com", true},
		{"test@yahoo.com", false},
		{"test@outlook.com", false},
		{"test@googlemail.com", false}, // Different from gmail.com
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := isGoogleAccount(tt.email); got != tt.want {
				t.Errorf("isGoogleAccount(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"test@gmail.com", true},
		{"test.user@example.co.uk", true},
		{"test+tag@gmail.com", true},
		{"invalid", false},
		{"@gmail.com", false},
		{"test@", false},
		{"", false},
		{"test@.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := isValidEmail(tt.email); got != tt.want {
				t.Errorf("isValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

// ============================================
// Benchmark Tests
// ============================================

func BenchmarkLogin(b *testing.B) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{ // <--- FIXED: Ditambahkan penanganan error
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(b, err)
	err = db.AutoMigrate(&models.Siswa{}, &models.AkunLoginSiswa{}) // <--- FIXED: Ditambahkan penanganan error
	require.NoError(b, err)

	db.Create(&models.Siswa{
		NIS:       "12345",
		NamaSiswa: "Test Student",
		JK:        "L",
	})
	hashedPwd, err := utils.HashPassword("SecurePass123!")
	require.NoError(b, err)
	db.Create(&models.AkunLoginSiswa{
		NIS:      "12345",
		Password: hashedPwd,
		Email:    "student@gmail.com",
	})

	testLogger, err := zap.NewDevelopment() // <--- FIXED: Ditambahkan penanganan error
	require.NoError(b, err)
	sugar := testLogger.Sugar()

	router := gin.New()
	router.POST("/login", Login(db, sugar))

	body := jsonBody(LoginRequest{
		Identifier: "12345",
		Password:   "SecurePass123!",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, err := http.NewRequest("POST", "/login", bytes.NewBuffer(body.Bytes()))
		require.NoError(b, err)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
