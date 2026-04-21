package routes

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/handlers"
	"absensholat-api/middleware"
)

func SetupRoutes(router *gin.Engine, db *gorm.DB, logger *zap.SugaredLogger) {
	// API v2 routes (industry-standard naming)
	v2 := router.Group("/api/v2")
	setupAPIV2Routes(v2, db, logger)

	// API v1 routes (current version)
	v1 := router.Group("/api/v1")
	setupAPIRoutes(v1, db, logger)

	// Legacy /api routes (backwards compatibility, mirrors v1)
	api := router.Group("/api")
	setupAPIRoutes(api, db, logger)
}

// setupAPIV2Routes configures standardized resource-oriented API endpoints.
func setupAPIV2Routes(api *gin.RouterGroup, db *gorm.DB, logger *zap.SugaredLogger) {
	// Auth and identity routes
	auth := api.Group("/auth")
	{
		strictRateLimit := middleware.StrictRateLimitMiddleware()

		auth.POST("/registrations", strictRateLimit, handlers.Register(db, logger))
		auth.POST("/sessions", strictRateLimit, handlers.Login(db, logger))
		auth.POST("/tokens/refresh", handlers.RefreshToken(db, logger))
		auth.DELETE("/sessions/current", handlers.Logout(db, logger))
		auth.GET("/profile", middleware.AuthMiddleware(), handlers.Me(db, logger))

		auth.POST("/password-resets", strictRateLimit, handlers.ForgotPassword(db, logger))
		auth.POST("/password-resets/verify-otp", strictRateLimit, handlers.VerifyOTP(db, logger))
		auth.POST("/password-resets/confirm", strictRateLimit, handlers.ResetPassword(db, logger))

		auth.POST("/email-change-requests", middleware.AuthMiddleware(), handlers.RequestChangeEmail(db, logger))
		auth.POST("/email-change-requests/verify", middleware.AuthMiddleware(), handlers.VerifyAndChangeEmail(db, logger))
	}

	// Analytics and notifications
	api.GET("/analytics/attendance", handlers.GetStatistics(db, logger))
	api.GET("/notifications", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetPendingNotifications(db, logger))

	// Attendance automation triggers
	api.GET("/internal/cron/attendance/auto-mark", middleware.CronMiddleware(), handlers.TriggerAutoAbsen(db, logger))
	api.POST("/attendance/auto-mark", middleware.AuthMiddleware("admin"), handlers.TriggerAutoAbsen(db, logger))

	// Reporting exports
	reports := api.Group("/reports")
	reports.Use(middleware.AuthMiddleware("admin", "guru", "wali_kelas"))
	{
		reports.GET("/attendances/csv", handlers.ExportAbsensiCSV(db, logger))
		reports.GET("/attendances/excel", handlers.ExportAbsensiExcel(db, logger))
		reports.GET("/summary/csv", handlers.ExportLaporanCSV(db, logger))
		reports.GET("/summary/excel", handlers.ExportLaporanExcel(db, logger))
		reports.GET("/attendance/excel", handlers.ExportAttendanceReportExcel(db, logger))
		reports.GET("/full/excel", handlers.ExportIntegratedReportExcel(db, logger))
	}

	// Data retention and cleanup workflows
	retention := api.Group("/data-retention/backups")
	{
		retention.GET("/status", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetBackupStatus(db, logger))
		retention.POST("/confirm", middleware.AuthMiddleware("admin", "wali_kelas"), handlers.ConfirmBackup(db, logger))
		retention.DELETE("", middleware.AuthMiddleware("admin"), handlers.CleanupBackedUpData(db, logger))
	}

	// Attendance history
	api.GET("/students/me/attendance-history", middleware.AuthMiddleware("siswa"), handlers.GetHistorySiswa(db, logger))
	api.GET("/attendance/history", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetHistoryStaff(db, logger))

	// QR-based attendance
	qr := api.Group("/attendance/qr-codes")
	{
		qr.GET("/current", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GenerateQRCode(db, logger))
		qr.GET("/current/image", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GenerateQRCodeImage(db, logger))
		qr.POST("/verify", middleware.AuthMiddleware("siswa"), handlers.VerifyQRCode(db, logger))
	}

	// Student resources
	students := api.Group("/students")
	{
		students.GET("", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetSiswa(db, logger))
		students.POST("", middleware.AuthMiddleware("admin"), handlers.CreateSiswa(db, logger))

		students.GET("/:nis", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetSiswaByID(db, logger))
		students.PUT("/:nis", middleware.AuthMiddleware("admin", "wali_kelas"), handlers.UpdateSiswa(db, logger))
		students.DELETE("/:nis", middleware.AuthMiddleware("admin"), handlers.DeleteSiswa(db, logger))
		students.POST("/:nis/attendances", middleware.AuthMiddleware("admin", "wali_kelas"), handlers.CreateAbsensi(db, logger))
	}

	// Prayer schedule resources
	prayerSchedules := api.Group("/prayer-schedules")
	{
		prayerSchedules.GET("", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetJadwalSholat(db, logger))
		prayerSchedules.GET("/:id", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetJadwalSholatByID(db, logger))
		prayerSchedules.GET("/dhuha/today", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetJadwalDhuhaToday(db, logger))

		prayerSchedules.POST("", middleware.AuthMiddleware("admin"), handlers.CreateJadwalSholat(db, logger))
		prayerSchedules.PUT("/:id", middleware.AuthMiddleware("admin"), handlers.UpdateJadwalSholat(db, logger))
		prayerSchedules.DELETE("/:id", middleware.AuthMiddleware("admin"), handlers.DeleteJadwalSholat(db, logger))
	}
}

// setupAPIRoutes configures all API endpoints for a given router group
func setupAPIRoutes(api *gin.RouterGroup, db *gorm.DB, logger *zap.SugaredLogger) {
	// Auth routes with strict rate limiting for security
	auth := api.Group("/auth")
	{
		// Apply strict rate limiting to sensitive auth endpoints
		strictRateLimit := middleware.StrictRateLimitMiddleware()

		auth.POST("/register", strictRateLimit, handlers.Register(db, logger))
		auth.POST("/login", strictRateLimit, handlers.Login(db, logger))
		auth.POST("/refresh", handlers.RefreshToken(db, logger))
		auth.POST("/logout", handlers.Logout(db, logger))
		auth.GET("/me", middleware.AuthMiddleware(), handlers.Me(db, logger))
		auth.POST("/forgot-password", strictRateLimit, handlers.ForgotPassword(db, logger))
		auth.POST("/verify-otp", strictRateLimit, handlers.VerifyOTP(db, logger))
		auth.POST("/reset-password", strictRateLimit, handlers.ResetPassword(db, logger))
		auth.POST("/change-email", middleware.AuthMiddleware(), handlers.RequestChangeEmail(db, logger))
		auth.POST("/verify-change-email", middleware.AuthMiddleware(), handlers.VerifyAndChangeEmail(db, logger))
	}

	// Statistics route
	api.GET("/statistics", handlers.GetStatistics(db, logger))

	// Auto-absen trigger routes
	// 1. For Automated Cron (Uses GET and CRON_SECRET)
	api.GET("/auto-absen/trigger", middleware.CronMiddleware(), handlers.TriggerAutoAbsen(db, logger))
	// 2. For Admin (Manual trigger, uses POST and Admin JWT)
	api.POST("/auto-absen/trigger", middleware.AuthMiddleware("admin"), handlers.TriggerAutoAbsen(db, logger))

	// Notification route - for admin, guru, wali_kelas
	api.GET("/notifications", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetPendingNotifications(db, logger))

	// Export routes (for admin, guru, wali_kelas)
	export := api.Group("/export")
	export.Use(middleware.AuthMiddleware("admin", "guru", "wali_kelas"))
	{
		export.GET("/absensi", handlers.ExportAbsensiCSV(db, logger))
		export.GET("/absensi/excel", handlers.ExportAbsensiExcel(db, logger))
		export.GET("/laporan", handlers.ExportLaporanCSV(db, logger))
		export.GET("/laporan/excel", handlers.ExportLaporanExcel(db, logger))
		export.GET("/attendance-report", handlers.ExportAttendanceReportExcel(db, logger))
		export.GET("/full-report", handlers.ExportIntegratedReportExcel(db, logger))
	}

	// Backup routes
	backup := api.Group("/backup")
	{
		// Status and confirm: admin, guru, wali_kelas can view/confirm
		backup.GET("/status", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetBackupStatus(db, logger))
		backup.POST("/confirm", middleware.AuthMiddleware("admin", "wali_kelas"), handlers.ConfirmBackup(db, logger))
		// Cleanup (delete): admin only
		backup.DELETE("/cleanup", middleware.AuthMiddleware("admin"), handlers.CleanupBackedUpData(db, logger))
	}

	// History routes
	history := api.Group("/history")
	{
		// Siswa history (per week) - requires siswa role
		history.GET("/siswa", middleware.AuthMiddleware("siswa"), handlers.GetHistorySiswa(db, logger))

		// Staff history with filters - for admin, guru, wali_kelas
		history.GET("/staff", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetHistoryStaff(db, logger))
	}

	// QR Code routes
	qrcode := api.Group("/qrcode")
	{
		// Generate QR code - only for admin, guru, wali_kelas
		qrcode.GET("/generate", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GenerateQRCode(db, logger))

		// Download QR code as PNG image - only for admin, guru, wali_kelas
		qrcode.GET("/image", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GenerateQRCodeImage(db, logger))

		// Verify QR code - only for siswa
		qrcode.POST("/verify", middleware.AuthMiddleware("siswa"), handlers.VerifyQRCode(db, logger))
	}

	siswa := api.Group("/siswa")
	{
		// GET: accessible to admin, guru, wali_kelas (guru = read-only)
		siswa.GET("", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetSiswa(db, logger))
		// POST: admin only (guru cannot create)
		siswa.POST("", middleware.AuthMiddleware("admin"), handlers.CreateSiswa(db, logger))

		// Match all paths under /siswa to handle both regular paths and absensi
		siswa.GET("/*nis", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetSiswaByID(db, logger))
		// PUT/DELETE: admin and wali_kelas only (guru cannot edit/delete)
		siswa.PUT("/*nis", middleware.AuthMiddleware("admin", "wali_kelas"), handlers.UpdateSiswa(db, logger))
		siswa.DELETE("/*nis", middleware.AuthMiddleware("admin"), handlers.DeleteSiswa(db, logger))
		siswa.POST("/*path", middleware.AuthMiddleware("admin", "wali_kelas"), handlers.HandleSiswaPath(db, logger))
	}

	// Jadwal Sholat routes
	jadwal := api.Group("/jadwal-sholat")
	{
		// GET: accessible to admin, guru, wali_kelas
		jadwal.GET("", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetJadwalSholat(db, logger))
		jadwal.GET("/:id", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetJadwalSholatByID(db, logger))
		jadwal.GET("/dhuha-today", middleware.AuthMiddleware("admin", "guru", "wali_kelas"), handlers.GetJadwalDhuhaToday(db, logger))
		// POST/PUT/DELETE: admin only (guru cannot modify)
		jadwal.POST("", middleware.AuthMiddleware("admin"), handlers.CreateJadwalSholat(db, logger))
		jadwal.PUT("/:id", middleware.AuthMiddleware("admin"), handlers.UpdateJadwalSholat(db, logger))
		jadwal.DELETE("/:id", middleware.AuthMiddleware("admin"), handlers.DeleteJadwalSholat(db, logger))
	}
}
