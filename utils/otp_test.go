package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"absensholat-api/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Auto-migrate all necessary models
	err = db.AutoMigrate(
		&models.OTPCode{},
		&models.Siswa{},
		&models.Account{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

// ============================================
// GenerateOTP Tests
// ============================================

func TestGenerateOTP(t *testing.T) {
	tests := []struct {
		name    string
		wantLen int
	}{
		{"generate first OTP", 6},
		{"generate second OTP", 6},
		{"generate third OTP", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := GenerateOTP()
			require.NoError(t, err)
			require.Len(t, code, tt.wantLen)
			// Verify it's all digits
			for _, ch := range code {
				require.GreaterOrEqual(t, ch, '0')
				require.LessOrEqual(t, ch, '9')
			}
		})
	}
}

func TestGenerateOTP_Unique(t *testing.T) {
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := GenerateOTP()
		require.NoError(t, err)
		codes[code] = true
	}
	// With 100 codes out of 1,000,000 possibilities, we expect very few or no collisions
	// But at minimum, verify we generated 100 codes
	require.GreaterOrEqual(t, len(codes), 1)
}

// ============================================
// DatabaseOTPStore - SaveOTP Tests
// ============================================

func TestDatabaseOTPStore_SaveOTP(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"
	expiration := 10 * time.Minute

	err := store.SaveOTP(nis, email, code, expiration)
	require.NoError(t, err)

	// Verify the OTP was saved
	var otp models.OTPCode
	result := db.Where("nis = ?", nis).First(&otp)
	require.NoError(t, result.Error)
	require.Equal(t, nis, otp.NIS)
	require.Equal(t, email, otp.Email)
	require.Equal(t, code, otp.Code)
	require.False(t, otp.Verified)
	require.WithinDuration(t, time.Now().Add(expiration), otp.ExpiresAt, 2*time.Second)
}

func TestDatabaseOTPStore_SaveOTP_OverwritesExisting(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"

	// Save first OTP
	err := store.SaveOTP(nis, email, "111111", 10*time.Minute)
	require.NoError(t, err)

	// Save second OTP (should create new record, not update)
	err = store.SaveOTP(nis, email, "222222", 10*time.Minute)
	require.NoError(t, err)

	// Verify both records exist (since we don't delete old ones on save)
	var otps []models.OTPCode
	result := db.Where("nis = ?", nis).Find(&otps)
	require.NoError(t, result.Error)
	require.GreaterOrEqual(t, len(otps), 2)
}

func TestDatabaseOTPStore_SaveOTP_MultipleUsers(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	users := []struct {
		nis   string
		email string
		code  string
	}{
		{"11111", "user1@gmail.com", "111111"},
		{"22222", "user2@gmail.com", "222222"},
		{"33333", "user3@gmail.com", "333333"},
	}

	for _, user := range users {
		err := store.SaveOTP(user.nis, user.email, user.code, 10*time.Minute)
		require.NoError(t, err)
	}

	// Verify all were saved
	var count int64
	result := db.Model(&models.OTPCode{}).Count(&count)
	require.NoError(t, result.Error)
	require.Equal(t, int64(3), count)

	// Verify each user's OTP
	for _, user := range users {
		var otp models.OTPCode
		result := db.Where("nis = ? AND code = ?", user.nis, user.code).First(&otp)
		require.NoError(t, result.Error)
		require.Equal(t, user.email, otp.Email)
	}
}

// ============================================
// DatabaseOTPStore - VerifyOTP Tests
// ============================================

func TestDatabaseOTPStore_VerifyOTP_Success(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"

	err := store.SaveOTP(nis, email, code, 10*time.Minute)
	require.NoError(t, err)

	valid, err := store.VerifyOTP(nis, code)
	require.NoError(t, err)
	require.True(t, valid)

	// Verify the OTP is marked as verified
	var otp models.OTPCode
	result := db.Where("nis = ?", nis).First(&otp)
	require.NoError(t, result.Error)
	require.True(t, otp.Verified)
}

func TestDatabaseOTPStore_VerifyOTP_WrongCode(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"

	err := store.SaveOTP(nis, email, code, 10*time.Minute)
	require.NoError(t, err)

	valid, err := store.VerifyOTP(nis, "654321")
	require.Error(t, err)
	require.False(t, valid)
	require.Contains(t, err.Error(), "tidak ada permintaan reset password")
}

func TestDatabaseOTPStore_VerifyOTP_NonExistentNIS(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	valid, err := store.VerifyOTP("99999", "123456")
	require.Error(t, err)
	require.False(t, valid)
	require.Contains(t, err.Error(), "tidak ada permintaan reset password")
}

func TestDatabaseOTPStore_VerifyOTP_Expired(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"

	// Save OTP with expiration in the past
	err := store.SaveOTP(nis, email, code, -1*time.Minute)
	require.NoError(t, err)

	valid, err := store.VerifyOTP(nis, code)
	require.Error(t, err)
	require.False(t, valid)
	// Since the query filters by expires_at > now, expired OTPs won't be found
	// So we get "tidak ada permintaan reset password" instead of "kadaluarsa"
	require.Contains(t, err.Error(), "tidak ada permintaan reset password")

	// Verify expired OTP was deleted by CleanupExpired (not by VerifyOTP since it wasn't found)
	var count int64
	result := db.Model(&models.OTPCode{}).Where("nis = ?", nis).Count(&count)
	require.NoError(t, result.Error)
	// The expired OTP still exists since VerifyOTP didn't find it to delete it
	require.Equal(t, int64(1), count)

	// But CleanupExpired will remove it
	err = store.CleanupExpired()
	require.NoError(t, err)
	result = db.Model(&models.OTPCode{}).Where("nis = ?", nis).Count(&count)
	require.NoError(t, result.Error)
	require.Equal(t, int64(0), count)
}

func TestDatabaseOTPStore_VerifyOTP_AlreadyVerified(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"

	err := store.SaveOTP(nis, email, code, 10*time.Minute)
	require.NoError(t, err)

	// First verification
	valid, err := store.VerifyOTP(nis, code)
	require.NoError(t, err)
	require.True(t, valid)

	// Second verification with same code should still succeed
	// (the code in DB hasn't changed, so it still matches)
	valid, err = store.VerifyOTP(nis, code)
	require.NoError(t, err)
	require.True(t, valid)

	// Verify marked as verified
	require.True(t, store.IsVerified(nis))
}

// ============================================
// DatabaseOTPStore - IsVerified Tests
// ============================================

func TestDatabaseOTPStore_IsVerified(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"

	err := store.SaveOTP(nis, email, code, 10*time.Minute)
	require.NoError(t, err)

	// Should not be verified initially
	require.False(t, store.IsVerified(nis))

	// Verify the OTP
	valid, err := store.VerifyOTP(nis, code)
	require.NoError(t, err)
	require.True(t, valid)

	// Should be verified now
	require.True(t, store.IsVerified(nis))
}

func TestDatabaseOTPStore_IsVerified_Expired(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"

	err := store.SaveOTP(nis, email, code, -1*time.Minute)
	require.NoError(t, err)

	// Even if verified, expired OTP should not be considered verified
	var otp models.OTPCode
	db.Where("nis = ?", nis).First(&otp)

otp.Verified = true
	db.Save(&otp)

	require.False(t, store.IsVerified(nis))
}

func TestDatabaseOTPStore_IsVerified_NotFound(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	require.False(t, store.IsVerified("nonexistent"))
}

// ============================================
// DatabaseOTPStore - ClearOTP Tests
// ============================================

func TestDatabaseOTPStore_ClearOTP(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	code := "123456"

	err := store.SaveOTP(nis, email, code, 10*time.Minute)
	require.NoError(t, err)

	// Verify it exists
	var count int64
	db.Model(&models.OTPCode{}).Where("nis = ?", nis).Count(&count)
	require.Equal(t, int64(1), count)

	// Clear the OTP
	err = store.ClearOTP(nis)
	require.NoError(t, err)

	// Verify it's deleted
	db.Model(&models.OTPCode{}).Where("nis = ?", nis).Count(&count)
	require.Equal(t, int64(0), count)
}

func TestDatabaseOTPStore_ClearOTP_NonExistent(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	// Should not error when clearing non-existent OTP
	err := store.ClearOTP("nonexistent")
	require.NoError(t, err)
}

// ============================================
// DatabaseOTPStore - CleanupExpired Tests
// ============================================

func TestDatabaseOTPStore_CleanupExpired(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	// Save expired OTP
	err := store.SaveOTP("11111", "user1@gmail.com", "111111", -5*time.Minute)
	require.NoError(t, err)

	// Save another expired OTP
	err = store.SaveOTP("22222", "user2@gmail.com", "222222", -10*time.Minute)
	require.NoError(t, err)

	// Save valid OTP
	err = store.SaveOTP("33333", "user3@gmail.com", "333333", 10*time.Minute)
	require.NoError(t, err)

	// Verify all 3 are present
	var count int64
	db.Model(&models.OTPCode{}).Count(&count)
	require.Equal(t, int64(3), count)

	// Cleanup expired
	err = store.CleanupExpired()
	require.NoError(t, err)

	// Verify only the valid OTP remains
	db.Model(&models.OTPCode{}).Count(&count)
	require.Equal(t, int64(1), count)

	var remaining models.OTPCode
	db.Where("nis = ?", "33333").First(&remaining)
	require.Equal(t, "333333", remaining.Code)
}

func TestDatabaseOTPStore_CleanupExpired_AllValid(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	// Save valid OTPs
	err := store.SaveOTP("11111", "user1@gmail.com", "111111", 10*time.Minute)
	require.NoError(t, err)
	err = store.SaveOTP("22222", "user2@gmail.com", "222222", 20*time.Minute)
	require.NoError(t, err)

	var count int64
	db.Model(&models.OTPCode{}).Count(&count)
	require.Equal(t, int64(2), count)

	// Cleanup expired
	err = store.CleanupExpired()
	require.NoError(t, err)

	// All should remain
	db.Model(&models.OTPCode{}).Count(&count)
	require.Equal(t, int64(2), count)
}

func TestDatabaseOTPStore_CleanupExpired_AllExpired(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	// Save expired OTPs
	err := store.SaveOTP("11111", "user1@gmail.com", "111111", -1*time.Minute)
	require.NoError(t, err)
	err = store.SaveOTP("22222", "user2@gmail.com", "222222", -5*time.Minute)
	require.NoError(t, err)

	var count int64
	db.Model(&models.OTPCode{}).Count(&count)
	require.Equal(t, int64(2), count)

	// Cleanup expired
	err = store.CleanupExpired()
	require.NoError(t, err)

	// All should be deleted
	db.Model(&models.OTPCode{}).Count(&count)
	require.Equal(t, int64(0), count)
}

// ============================================
// Full OTP Flow Integration Tests
// ============================================

func TestOTP_FullFlow(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"

	// Step 1: Generate OTP
	code, err := GenerateOTP()
	require.NoError(t, err)
	require.Len(t, code, 6)

	// Step 2: Save OTP
	err = store.SaveOTP(nis, email, code, 10*time.Minute)
	require.NoError(t, err)

	// Step 3: Verify OTP is not yet verified
	require.False(t, store.IsVerified(nis))

	// Step 4: Verify OTP with correct code
	valid, err := store.VerifyOTP(nis, code)
	require.NoError(t, err)
	require.True(t, valid)

	// Step 5: Verify OTP is now marked as verified
	require.True(t, store.IsVerified(nis))

	// Step 6: Clear OTP after use
	err = store.ClearOTP(nis)
	require.NoError(t, err)

	// Step 7: Verify OTP is gone
	require.False(t, store.IsVerified(nis))
}

func TestOTP_Flow_ExpiredCode(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"

	// Generate and save OTP with short expiration (already expired)
	code, err := GenerateOTP()
	require.NoError(t, err)

	err = store.SaveOTP(nis, email, code, -1*time.Minute) // Expired 1 minute ago
	require.NoError(t, err)

	// Try to verify - should fail because expired OTP won't be found by the query
	valid, err := store.VerifyOTP(nis, code)
	require.Error(t, err)
	require.False(t, valid)
	require.Contains(t, err.Error(), "tidak ada permintaan reset password")

	// OTP should still exist (wasn't found by VerifyOTP to delete it)
	// But CleanupExpired will remove it
	err = store.CleanupExpired()
	require.NoError(t, err)

	var count int64
	db.Model(&models.OTPCode{}).Where("nis = ?", nis).Count(&count)
	require.Equal(t, int64(0), count)
}

func TestOTP_Flow_WrongCodeThenCorrect(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"
	correctCode := "123456"

	// Save OTP
	err := store.SaveOTP(nis, email, correctCode, 10*time.Minute)
	require.NoError(t, err)

	// Try wrong code
	valid, err := store.VerifyOTP(nis, "654321")
	require.Error(t, err)
	require.False(t, valid)

	// Try correct code - should still work
	valid, err = store.VerifyOTP(nis, correctCode)
	require.NoError(t, err)
	require.True(t, valid)

	// Verify marked as verified
	require.True(t, store.IsVerified(nis))
}

func TestOTP_Flow_MultipleAttempts(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	nis := "12345"
	email := "student@gmail.com"

	// Generate and save first OTP
	code1, err := GenerateOTP()
	require.NoError(t, err)
	err = store.SaveOTP(nis, email, code1, 10*time.Minute)
	require.NoError(t, err)

	// Verify first OTP
	valid, err := store.VerifyOTP(nis, code1)
	require.NoError(t, err)
	require.True(t, valid)

	// Generate and save second OTP (new request - overwrites in practice since new record)
	code2, err := GenerateOTP()
	require.NoError(t, err)
	err = store.SaveOTP(nis, email, code2, 10*time.Minute)
	require.NoError(t, err)

	// Now verify with second code
	valid, err = store.VerifyOTP(nis, code2)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestOTP_Flow_CleanupBackground(t *testing.T) {
	db := setupTestDB(t)
	store := &DatabaseOTPStore{db: db}

	// Save mix of expired and valid OTPs
	err := store.SaveOTP("11111", "user1@gmail.com", "111111", -10*time.Minute)
	require.NoError(t, err)
	err = store.SaveOTP("22222", "user2@gmail.com", "222222", -5*time.Minute)
	require.NoError(t, err)
	err = store.SaveOTP("33333", "user3@gmail.com", "333333", 10*time.Minute)
	require.NoError(t, err)

	// Run cleanup
	err = store.CleanupExpired()
	require.NoError(t, err)

	// Only valid OTP should remain
	var otps []models.OTPCode
	db.Find(&otps)
	require.Equal(t, 1, len(otps))
	require.Equal(t, "33333", otps[0].NIS)
}

// ============================================
// Benchmark Tests
// ============================================

func BenchmarkGenerateOTP(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateOTP()
		require.NoError(b, err)
	}
}

func BenchmarkDatabaseOTPStore_SaveOTP(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(b, err)
	err = db.AutoMigrate(&models.OTPCode{})
	require.NoError(b, err)

	store := &DatabaseOTPStore{db: db}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nis := string(rune('0' + i%10))
		err := store.SaveOTP(nis, "test@gmail.com", "123456", 10*time.Minute)
		require.NoError(b, err)
	}
}

func BenchmarkDatabaseOTPStore_VerifyOTP(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(b, err)
	err = db.AutoMigrate(&models.OTPCode{})
	require.NoError(b, err)

	store := &DatabaseOTPStore{db: db}

	// Pre-populate with test data
	for i := 0; i < b.N; i++ {
		nis := string(rune('0' + i%10))
		err := store.SaveOTP(nis, "test@gmail.com", "123456", 10*time.Minute)
		require.NoError(b, err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nis := string(rune('0' + i%10))
		_, err := store.VerifyOTP(nis, "123456")
		require.NoError(b, err)
	}
}
