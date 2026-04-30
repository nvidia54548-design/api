package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log" // Added for logging
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"absensholat-api/models"
	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/iterator" // <--- DITAMBAHKAN
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// OTPEntry stores OTP data with expiration
type OTPEntry struct {
	Code      string    `firestore:"code"`
	Email     string    `firestore:"email"`
	NIS       string    `firestore:"nis"`
	ExpiresAt time.Time `firestore:"expires_at"`
	Verified  bool      `firestore:"verified"`
	CreatedAt time.Time `firestore:"created_at"`
}

// OTPStore interface for OTP operations
type OTPStore interface {
	SaveOTP(nis, email, code string, expiration time.Duration) error
	VerifyOTP(nis, code string) (bool, error)
	IsVerified(nis string) bool
	ClearOTP(nis string) error
	CleanupExpired() error
}

// DatabaseOTPStore manages OTPs using database
type DatabaseOTPStore struct {
	db *gorm.DB
}

// FirebaseOTPStore manages OTPs using Firebase Firestore
type FirebaseOTPStore struct {
	client     *firestore.Client
	collection string
}

var (
	firestoreClient  *firestore.Client
	otpStore         OTPStore
	firebaseInitOnce sync.Once
	firebaseInitErr  error
	otpCleanupOnce   sync.Once
	// firebaseApp      *firebase.App // REMOVED: Variable not used anywhere

	// Database OTP store
	dbOTPStore       *DatabaseOTPStore
)

func envBool(key string, defaultValue bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return defaultValue
	}

	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func envDurationMS(key string, defaultValue time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}

	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		return defaultValue
	}

	return time.Duration(ms) * time.Millisecond
}

// FirebaseLazyInitEnabled controls whether Firebase initialization is deferred
// until the OTP store is requested.
func FirebaseLazyInitEnabled() bool {
	return envBool("FIREBASE_LAZY_INIT", false)
}

// EnsureOTPCleanupStarted makes OTP cleanup scheduler idempotent.
func EnsureOTPCleanupStarted(interval time.Duration) {
	otpCleanupOnce.Do(func() {
		StartOTPCleanup(interval)
	})
}

func lazyInitFirebaseIfNeeded() {
	if otpStore != nil || !FirebaseLazyInitEnabled() {
		return
	}

	start := time.Now()
	timeout := envDurationMS("FIREBASE_LAZY_INIT_TIMEOUT_MS", 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := InitFirebase(ctx)
	if err != nil {
		log.Printf("Lazy Firebase init failed after %dms: %v", time.Since(start).Milliseconds(), err)
		return
	}

	EnsureOTPCleanupStarted(5 * time.Minute)
	log.Printf("Lazy Firebase init completed in %dms", time.Since(start).Milliseconds())
}

// isURL checks if the given path is a URL
func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// downloadCredentials downloads credentials from a URL
func downloadCredentials(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// FIXED: Handle error from http.NewRequestWithContext
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download credentials: HTTP %d", resp.StatusCode)
	}

	credBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return credBytes, nil
}

// InitFirebase initializes the Firebase app and Firestore client
func InitFirebase(ctx context.Context) error {
	firebaseInitOnce.Do(func() {
		var opt option.ClientOption

		// Check for credentials file path or use default
		credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
		if credPath == "" {
			credPath = "serviceAccountKey.json" // Default path
		}

		var credBytes []byte
		var err error

		// Check if credPath is a URL or local file
		if isURL(credPath) {
			// Download from URL
			credBytes, err = downloadCredentials(credPath)
			if err != nil {
				// FIXED: Corrected error message format
				firebaseInitErr = fmt.Errorf("failed to download Firebase credentials from %s: %w", credPath, err)
				return
			}
		} else {
			// Read from local file
			credBytes, err = os.ReadFile(credPath)
			if err != nil {
				firebaseInitErr = fmt.Errorf("failed to read Firebase credentials from %s: %w", credPath, err)
				return
			}
		}

		// Check if it's a service account key or google-services.json
		if !isServiceAccountKey(credBytes) {
			// FIXED: Corrected error message capitalization
			firebaseInitErr = fmt.Errorf("Firebase credentials file appears to be google-services.json (Android) instead of a service account key. Get the service account key from Firebase Console > Project Settings > Service Accounts > Generate New Private Key")
			return
		}

		// SA1019: option.WithCredentialsJSON is deprecated.
		// For now, we'll keep it as is unless there's a clear alternative.
		// Consider updating the Firebase SDK usage pattern if possible.
		opt = option.WithCredentialsJSON(credBytes)

		app, err := firebase.NewApp(ctx, nil, opt)
		if err != nil {
			firebaseInitErr = fmt.Errorf("failed to initialize Firebase app: %w", err)
			return
		}

		client, err := app.Firestore(ctx)
		if err != nil {
			firebaseInitErr = fmt.Errorf("failed to initialize Firestore client: %w", err)
			return
		}

		firestoreClient = client
		otpStore = &FirebaseOTPStore{
			client:     client,
			collection: "otp_codes",
		}
	})

	return firebaseInitErr
}

// isServiceAccountKey checks if the credentials JSON is a service account key
func isServiceAccountKey(credBytes []byte) bool {
	credStr := string(credBytes)
	// Service account keys have "type": "service_account"
	// google-services.json has "project_info" and "client"
	return strings.Contains(credStr, `"type"`) && strings.Contains(credStr, `"service_account"`)
}

// IsFirebaseInitialized returns true if Firebase was successfully initialized
func IsFirebaseInitialized() bool {
	return firestoreClient != nil && firebaseInitErr == nil
}

// InitDatabaseOTPStore initializes the database OTP store
func InitDatabaseOTPStore(db *gorm.DB) {
	dbOTPStore = &DatabaseOTPStore{db: db}
}

// GetOTPStore returns the database OTP store (preferred over Firebase)
func GetOTPStore() OTPStore {
	if dbOTPStore != nil {
		return dbOTPStore
	}
	// Fallback to Firebase if database store not initialized
	lazyInitFirebaseIfNeeded()
	return otpStore
}

// GetFirestoreClient returns the Firestore client
func GetFirestoreClient() *firestore.Client {
	return firestoreClient
}

// CloseFirebase closes the Firestore client connection
func CloseFirebase() error {
	if firestoreClient != nil {
		return firestoreClient.Close()
	}
	return nil
}

// GenerateOTP creates a 6-digit OTP code
func GenerateOTP() (string, error) {
	// SA1019: math/big.Max is not deprecated. The error might be a misinterpretation or old rule.
	// However, `big.NewInt(1000000)` itself is not deprecated.
	// If `math.Max` was used elsewhere, it would be flagged, but here `big.NewInt` is fine.
	// Let's assume the error was about `max` variable name shadowing `math.Max` (builtinShadow rule).
	// FIXED: Rename variable to avoid potential shadowing
	maxLimit := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, maxLimit)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// sanitizeNIS sanitizes NIS to be used as a Firestore document ID
// Firestore document IDs cannot contain forward slashes
func sanitizeNIS(nis string) string {
	// Replace forward slashes with underscores
	sanitized := strings.ReplaceAll(nis, "/", "_")
	// Replace dots with hyphens to avoid any potential issues
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	return sanitized
}

// SaveOTP stores an OTP entry in Firestore
func (s *FirebaseOTPStore) SaveOTP(nis, email, code string, expiration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sanitize NIS for Firestore document ID
	docID := sanitizeNIS(nis)

	entry := OTPEntry{
		Code:      code,
		Email:     email,
		NIS:       nis,
		ExpiresAt: time.Now().Add(expiration),
		Verified:  false,
		CreatedAt: time.Now(),
	}

	_, err := s.client.Collection(s.collection).Doc(docID).Set(ctx, entry)
	if err != nil {
		return fmt.Errorf("failed to save OTP to Firestore: %w", err)
	}

	return nil
}

// VerifyOTP checks if the OTP is valid for the given NIS
func (s *FirebaseOTPStore) VerifyOTP(nis, code string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sanitize NIS for Firestore document ID
	docID := sanitizeNIS(nis)

	doc, err := s.client.Collection(s.collection).Doc(docID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, fmt.Errorf("tidak ada permintaan reset password untuk NIS ini")
		}
		return false, fmt.Errorf("failed to get OTP from Firestore: %w", err)
	}

	var entry OTPEntry
	// FIXED: Avoid shadowing 'err'. Use 'dataErr' or similar.
	dataErr := doc.DataTo(&entry)
	if dataErr != nil {
		return false, fmt.Errorf("failed to parse OTP data: %w", dataErr)
	}

	if time.Now().After(entry.ExpiresAt) {
		// Delete expired OTP - FIXED: Handle error correctly, ambil kedua nilai
		_, deleteErr := s.client.Collection(s.collection).Doc(docID).Delete(ctx) // <--- FIXED
		if deleteErr != nil {
			log.Printf("Failed to delete expired OTP doc: %v", deleteErr)
		}
		return false, fmt.Errorf("kode OTP sudah kadaluarsa")
	}

	if entry.Code != code {
		return false, fmt.Errorf("kode OTP tidak valid")
	}

	// Mark as verified
	_, err = s.client.Collection(s.collection).Doc(docID).Update(ctx, []firestore.Update{
		{Path: "verified", Value: true},
	})
	if err != nil {
		return false, fmt.Errorf("failed to update OTP verification status: %w", err)
	}

	return true, nil
}

// IsVerified checks if the OTP has been verified for password reset
func (s *FirebaseOTPStore) IsVerified(nis string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sanitize NIS for Firestore document ID
	docID := sanitizeNIS(nis)

	doc, err := s.client.Collection(s.collection).Doc(docID).Get(ctx)
	if err != nil {
		return false
	}

	var entry OTPEntry
	if err := doc.DataTo(&entry); err != nil {
		return false
	}

	if time.Now().After(entry.ExpiresAt) {
		return false
	}

	return entry.Verified
}

// ClearOTP removes the OTP entry for a given NIS from Firestore
func (s *FirebaseOTPStore) ClearOTP(nis string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sanitize NIS for Firestore document ID
	docID := sanitizeNIS(nis)

	// FIXED: Handle error from Delete - ambil kedua nilai
	_, err := s.client.Collection(s.collection).Doc(docID).Delete(ctx) // <--- FIXED
	if err != nil {
		return fmt.Errorf("failed to delete OTP from Firestore: %w", err)
	}
	return nil
}

// GetEntry returns the OTP entry for a given NIS
func (s *FirebaseOTPStore) GetEntry(nis string) (*OTPEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sanitize NIS for Firestore document ID
	docID := sanitizeNIS(nis)

	doc, err := s.client.Collection(s.collection).Doc(docID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get OTP from Firestore: %w", err)
	}

	var entry OTPEntry
	// FIXED: Avoid shadowing 'err'
	dataErr := doc.DataTo(&entry)
	if dataErr != nil {
		return nil, fmt.Errorf("failed to parse OTP data: %w", dataErr)
	}

	return &entry, nil
}

// CleanupExpired removes all expired OTP entries from Firestore
func (s *FirebaseOTPStore) CleanupExpired() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query for expired OTPs
	iter := s.client.Collection(s.collection).Where("expires_at", "<", time.Now()).Documents(ctx)
	defer iter.Stop()

	// SA1019: s.client.Batch is deprecated.
	// FIXED: We will still use Batch here for simplicity as the direct replacement (BulkWriter) is more complex
	// and often overkill for simple batch operations like deletion.
	// However, for new projects or heavy usage, consider migrating to BulkWriter.
	// Example of BulkWriter migration would involve replacing batch.Commit() logic.
	batch := s.client.Batch() // Using deprecated method for now
	count := 0

	for {
		doc, err := iter.Next()
		if err != nil {
			if err == iterator.Done { // <--- FIXED: Gunakan iterator.Done
				break
			}
			// Handle other errors from iter.Next()
			// For simplicity, break on any error other than Done.
			// In a more robust implementation, you'd want to handle specific errors.
			log.Printf("Error iterating expired OTPs: %v", err)
			break // Atau return err jika ingin gagalkan seluruh operasi
		}
		batch.Delete(doc.Ref)
		count++

		// Firestore batch limit is 500
		if count >= 500 {
			// FIXED: Handle error from batch.Commit - ambil kedua nilai
			_, commitErr := batch.Commit(ctx) // <--- FIXED
			if commitErr != nil {
				return fmt.Errorf("failed to delete expired OTPs: %w", commitErr)
			}
			batch = s.client.Batch() // Reset batch
			count = 0
		}
	}

	if count > 0 {
		// FIXED: Handle error from final batch.Commit - ambil kedua nilai
		_, commitErr := batch.Commit(ctx) // <--- FIXED
		if commitErr != nil {
			return fmt.Errorf("failed to delete remaining expired OTPs: %w", commitErr)
		}
	}

	return nil
}

// DatabaseOTPStore methods

// SaveOTP stores an OTP entry in database
func (s *DatabaseOTPStore) SaveOTP(nis, email, code string, expiration time.Duration) error {
	entry := models.OTPCode{
		NIS:       nis,
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(expiration),
		Verified:  false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result := s.db.Create(&entry)
	return result.Error
}

// VerifyOTP checks if the OTP is valid for the given NIS
func (s *DatabaseOTPStore) VerifyOTP(nis, code string) (bool, error) {
	var entry models.OTPCode
	result := s.db.Where("nis = ? AND code = ? AND expires_at > ?", nis, code, time.Now()).First(&entry)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return false, fmt.Errorf("tidak ada permintaan reset password untuk NIS ini")
		}
		return false, fmt.Errorf("failed to verify OTP: %w", result.Error)
	}

	if time.Now().After(entry.ExpiresAt) {
		// Delete expired OTP
		s.db.Delete(&entry)
		return false, fmt.Errorf("kode OTP sudah kadaluarsa")
	}

	if entry.Code != code {
		return false, fmt.Errorf("kode OTP tidak valid")
	}

	// Mark as verified
	entry.Verified = true
	entry.UpdatedAt = time.Now()
	s.db.Save(&entry)

	return true, nil
}

// IsVerified checks if the OTP has been verified for password reset
func (s *DatabaseOTPStore) IsVerified(nis string) bool {
	var entry models.OTPCode
	result := s.db.Where("nis = ? AND expires_at > ? AND verified = ?", nis, time.Now(), true).First(&entry)

	return result.Error == nil
}

// ClearOTP removes the OTP entry for a given NIS from database
func (s *DatabaseOTPStore) ClearOTP(nis string) error {
	result := s.db.Where("nis = ?", nis).Delete(&models.OTPCode{})
	return result.Error
}

// CleanupExpired removes all expired OTP entries from database
func (s *DatabaseOTPStore) CleanupExpired() error {
	result := s.db.Where("expires_at < ?", time.Now()).Delete(&models.OTPCode{})
	return result.Error
}

// StartOTPCleanup starts a background goroutine to clean up expired OTPs
func StartOTPCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if otpStore != nil {
				// FIXED: Handle error from CleanupExpired
				if cleanupErr := otpStore.CleanupExpired(); cleanupErr != nil {
					log.Printf("Failed to cleanup expired OTPs: %v", cleanupErr)
				}
			}
			if dbOTPStore != nil {
				// Cleanup database OTPs too
				if cleanupErr := dbOTPStore.CleanupExpired(); cleanupErr != nil {
					log.Printf("Failed to cleanup expired database OTPs: %v", cleanupErr)
				}
			}
		}
	}()
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// GetSMTPConfig returns SMTP configuration from environment variables
func GetSMTPConfig() *SMTPConfig {
	return &SMTPConfig{
		Host:     getEnvOrDefault("SMTP_HOST", "smtp.gmail.com"),
		Port:     getEnvOrDefault("SMTP_PORT", "587"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     getEnvOrDefault("SMTP_FROM", os.Getenv("SMTP_USERNAME")),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SendOTPEmail sends an OTP code to the specified email address using Mailjet API v3.1
func SendOTPEmail(toEmail, otpCode, namaSiswa string) error {
	mailjetAPIKey := os.Getenv("MAILJET_API_KEY")
	mailjetAPISecret := os.Getenv("MAILJET_API_SECRET")
	if mailjetAPIKey == "" || mailjetAPISecret == "" {
		return fmt.Errorf("MAILJET_API_KEY and MAILJET_API_SECRET not configured in environment variables")
	}

	fromEmail := os.Getenv("MAILJET_FROM_EMAIL")
	if fromEmail == "" {
		return fmt.Errorf("MAILJET_FROM_EMAIL not configured in environment variables")
	}

	fromName := os.Getenv("MAILJET_FROM_NAME")
	if fromName == "" {
		fromName = "Sistem Absensi Sholat"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	subject := "Kode OTP Reset Password - Sistem Absensi Sholat"
	htmlBody := fmt.Sprintf(`<html>
<body>
<h2>Halo %s,</h2>
<p>Anda telah meminta reset password untuk akun Sistem Absensi Sholat.</p>
<p><strong>Kode OTP Anda adalah: <span style="font-size: 24px; color: #007bff;">%s</span></strong></p>
<p>Kode ini berlaku selama <strong>10 menit</strong>. Jangan bagikan kode ini kepada siapapun.</p>
<p>Jika Anda tidak meminta reset password, abaikan email ini.</p>
<p>Salam,<br/>Tim Sistem Absensi Sholat</p>
</body>
</html>`, namaSiswa, otpCode)

	textBody := fmt.Sprintf(`Halo %s,

Anda telah meminta reset password untuk akun Sistem Absensi Sholat.

Kode OTP Anda adalah: %s

Kode ini berlaku selama 10 menit. Jangan bagikan kode ini kepada siapapun.

Jika Anda tidak meminta reset password, abaikan email ini.

Salam,
Tim Sistem Absensi Sholat`, namaSiswa, otpCode)

	payload := map[string]interface{}{
		"Messages": []map[string]interface{}{
			{
				"From": map[string]string{
					"Email": fromEmail,
					"Name":  fromName,
				},
				"To": []map[string]string{
					{
						"Email": toEmail,
					},
				},
				"Subject":  subject,
				"TextPart": textBody,
				"HTMLPart": htmlBody,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.mailjet.com/v3.1/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(mailjetAPIKey, mailjetAPISecret)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mailjet API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
