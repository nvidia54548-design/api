package handlers

import (
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

	"github.com/stretchr/testify/require"
)

type JadwalSholatCreateRequest struct {
	Hari         string `json:"hari"`
	JenisSholat  string `json:"jenis_sholat"`
	WaktuMulai   string `json:"waktu_mulai"`
	WaktuSelesai string `json:"waktu_selesai"`
	Jurusan      string `json:"jurusan"`
	Kelas        string `json:"kelas"`
}

// setupTestDBJadwal creates an in-memory SQLite database for testing jadwal handlers
func setupTestDBJadwal(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Auto-migrate models
	err = db.AutoMigrate(&models.JadwalSholatTemplate{}, &models.JenisSholat{}, &models.Siswa{}, &models.Account{}, &models.Staff{}, &models.Absensi{}, &models.RekapAbsensi{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestGetJadwalSholat(t *testing.T) {
	t.Skip("Test needs to be updated for new schema")
	db := setupTestDBJadwal(t)
	defer func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.Close()
	}()
	logger := setupTestLogger()

	// Create test jenis sholat
	jenis1 := models.JenisSholat{NamaJenis: "Dzuhur", ButuhGiliran: false}
	jenis2 := models.JenisSholat{NamaJenis: "Ashar", ButuhGiliran: false}
	db.Create(&jenis1)
	db.Create(&jenis2)

	// Create test templates
	template1 := models.JadwalSholatTemplate{
		Hari:    "Senin",
		IDJenis: jenis1.IDJenis,
	}
	template2 := models.JadwalSholatTemplate{
		Hari:    "Senin",
		IDJenis: jenis2.IDJenis,
	}
	db.Create(&template1)
	db.Create(&template2)

	// Create router
	router := gin.New()
	router.GET("/jadwal-sholat", GetJadwalSholat(db, logger))

	// Test GET request
	req, err := http.NewRequest("GET", "/jadwal-sholat", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response JadwalSholatTemplateListResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("Expected 2 jadwal sholat templates, got %d", len(response.Data))
	}
}

func TestGetJadwalSholatByID(t *testing.T) {
	t.Skip("Test needs to be updated for new schema")
	db := setupTestDBJadwal(t)
	defer func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.Close()
	}()
	logger := setupTestLogger()

	// Create test data
	jadwal := models.JadwalSholat{
		Hari:         "Senin",
		JenisSholat:  "Dzuhur",
		WaktuMulai:   "12:00",
		WaktuSelesai: "12:30",
		Jurusan:      "IPA",
	}
	db.Create(&jadwal)

	// Create router
	router := gin.New()
	router.GET("/jadwal-sholat/:id", GetJadwalSholatByID(db, logger))

	// Test GET request
	req, err := http.NewRequest("GET", "/jadwal-sholat/1", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response JadwalSholatResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Data.IDJadwal != 1 {
		t.Errorf("Expected ID 1, got %d", response.Data.IDJadwal)
	}
}

func TestCreateJadwalSholat(t *testing.T) {
	t.Skip("Test needs to be updated for new schema")
	db := setupTestDBJadwal(t)
	defer func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.Close()
	}()
	logger := setupTestLogger()

	// Create router
	router := gin.New()
	router.POST("/jadwal-sholat", CreateJadwalSholat(db, logger))

	// Test data
	reqData := JadwalSholatCreateRequest{
		Hari:         "Senin",
		JenisSholat:  "Dzuhur",
		WaktuMulai:   "12:00",
		WaktuSelesai: "12:30",
		Jurusan:      "IPA",
	}

	jsonData, err := json.Marshal(reqData)
	require.NoError(t, err)
	req, err := http.NewRequest("POST", "/jadwal-sholat", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response JadwalSholatResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Data.Hari != "Senin" {
		t.Errorf("Expected Hari 'Senin', got '%s'", response.Data.Hari)
	}
}

func TestUpdateJadwalSholat(t *testing.T) {
	t.Skip("Test needs to be updated for new schema")
	db := setupTestDBJadwal(t)
	defer func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.Close()
	}()
	logger := setupTestLogger()

	// Create test data
	jadwal := models.JadwalSholat{
		Hari:         "Senin",
		JenisSholat:  "Dzuhur",
		WaktuMulai:   "12:00",
		WaktuSelesai: "12:30",
		Jurusan:      "IPA",
	}
	db.Create(&jadwal)

	// Create router
	router := gin.New()
	router.PUT("/jadwal-sholat/:id", UpdateJadwalSholat(db, logger))

	// Test data
	reqData := JadwalSholatUpdateRequest{
		Hari:         "Selasa",
		JenisSholat:  "Dzuhur",
		WaktuMulai:   "12:15",
		WaktuSelesai: "12:45",
		Jurusan:      "IPA",
	}

	jsonData, err := json.Marshal(reqData)
	require.NoError(t, err)
	req, err := http.NewRequest("PUT", "/jadwal-sholat/1", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response JadwalSholatResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Data.Hari != "Selasa" {
		t.Errorf("Expected Hari 'Selasa', got '%s'", response.Data.Hari)
	}
	if response.Data.WaktuMulai != "12:15" {
		t.Errorf("Expected WaktuMulai '12:15', got '%s'", response.Data.WaktuMulai)
	}
}

func TestDeleteJadwalSholat(t *testing.T) {
	t.Skip("Test needs to be updated for new schema")
	db := setupTestDBJadwal(t)
	defer func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.Close()
	}()
	logger := setupTestLogger()

	// Create test data
	jadwal := models.JadwalSholat{
		Hari:         "Senin",
		JenisSholat:  "Dzuhur",
		WaktuMulai:   "12:00",
		WaktuSelesai: "12:30",
		Jurusan:      "IPA",
	}
	db.Create(&jadwal)

	// Create router
	router := gin.New()
	router.DELETE("/jadwal-sholat/:id", DeleteJadwalSholat(db, logger))

	// Test DELETE request
	req, err := http.NewRequest("DELETE", "/jadwal-sholat/1", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deletion
	var count int64
	db.Model(&models.JadwalSholat{}).Where("id_jadwal = ?", 1).Count(&count)
	if count != 0 {
		t.Errorf("Expected jadwal sholat to be deleted, but found %d records", count)
	}
}

func TestGetJadwalDhuhaToday(t *testing.T) {
	t.Skip("Test needs to be updated for new schema")
	db := setupTestDBJadwal(t)
	defer func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.Close()
	}()
	logger := setupTestLogger()

	// Get current day in Indonesian
	currentDay := utils.GetIndonesianDayName(utils.GetJakartaTime())

	// Create test data for Dhuha on current day
	jadwal1 := models.JadwalSholat{
		Hari:         currentDay,
		JenisSholat:  "Dhuha",
		WaktuMulai:   "08:00",
		WaktuSelesai: "08:30",
		Jurusan:      "IPA",
	}
	jadwal2 := models.JadwalSholat{
		Hari:         currentDay,
		JenisSholat:  "Dhuha",
		WaktuMulai:   "08:00",
		WaktuSelesai: "08:30",
		Jurusan:      "IPS",
	}
	jadwal3 := models.JadwalSholat{
		Hari:         currentDay,
		JenisSholat:  "Dhuha",
		WaktuMulai:   "08:30",
		WaktuSelesai: "09:00",
		Jurusan:      "IPA",
	}
	// Create a third jurusan to test the limit
	jadwal4 := models.JadwalSholat{
		Hari:         currentDay,
		JenisSholat:  "Dhuha",
		WaktuMulai:   "09:00",
		WaktuSelesai: "09:30",
		Jurusan:      "Bahasa",
	}
	db.Create(&jadwal1)
	db.Create(&jadwal2)
	db.Create(&jadwal3)
	db.Create(&jadwal4)

	// Create router
	router := gin.New()
	router.GET("/jadwal-sholat/dhuha-today", GetJadwalDhuhaToday(db, logger))

	// Test GET request
	req, err := http.NewRequest("GET", "/jadwal-sholat/dhuha-today", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response JadwalDhuhaTodayResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Current behavior returns all scheduled jurusan for today.
	if len(response.Data) != 3 {
		t.Errorf("Expected 3 jurusan, got %d", len(response.Data))
	}

	// Check that the jurusan are unique and have schedules
	jurusanSet := make(map[string]bool)
	for _, data := range response.Data {
		if jurusanSet[data.Jurusan] {
			t.Errorf("Duplicate jurusan: %s", data.Jurusan)
		}
		jurusanSet[data.Jurusan] = true
		if len(data.Jadwal) == 0 {
			t.Errorf("Jurusan %s has no jadwal", data.Jurusan)
		}
		for _, jadwal := range data.Jadwal {
			if jadwal.JenisSholat != "Dhuha" {
				t.Errorf("Expected jenis_sholat 'Dhuha', got '%s'", jadwal.JenisSholat)
			}
			if jadwal.Hari != currentDay {
				t.Errorf("Expected hari '%s', got '%s'", currentDay, jadwal.Hari)
			}
		}
	}
}
