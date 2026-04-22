package docs

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	openAPIVersion = "3.1.0"
	specVersion    = "2.0.0"
)

type operationSpec struct {
	tags          []string
	summary       string
	description   string
	security      bool
	requestSchema string
	queryParams   []map[string]any
	headerParams  []map[string]any
	responses     map[string]any
}

// OpenAPIJSONHandler serves a fully explicit OpenAPI document for development.
func OpenAPIJSONHandler(router *gin.Engine, isProduction bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		spec := buildOpenAPISpec(router, isProduction)
		payload, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to generate OpenAPI document"})
			return
		}

		c.Data(200, "application/json; charset=utf-8", payload)
	}
}

func buildOpenAPISpec(router *gin.Engine, isProduction bool) map[string]any {
	routes := router.Routes()
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	explicit := explicitOperations()
	paths := map[string]any{}

	for _, route := range routes {
		if shouldSkipPath(route.Path) {
			continue
		}

		normalizedPath := normalizeGinPath(route.Path)
		canonical := canonicalPath(normalizedPath)
		key := route.Method + " " + canonical

		spec, ok := explicit[key]
		if !ok {
			continue
		}

		operation := map[string]any{
			"tags":        spec.tags,
			"summary":     spec.summary,
			"description": spec.description,
			"operationId": routeOperationID(route.Method, normalizedPath),
			"responses":   spec.responses,
		}

		params := make([]map[string]any, 0)
		params = append(params, routeParameters(normalizedPath)...)
		params = append(params, spec.queryParams...)
		params = append(params, spec.headerParams...)
		params = uniqueParams(params)
		if len(params) > 0 {
			operation["parameters"] = params
		}

		if spec.requestSchema != "" {
			operation["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": schemaRef(spec.requestSchema),
					},
				},
			}
		}

		if spec.security {
			operation["security"] = []map[string]any{{"BearerAuth": []string{}}}
		}

		if _, ok := paths[normalizedPath]; !ok {
			paths[normalizedPath] = map[string]any{}
		}
		paths[normalizedPath].(map[string]any)[strings.ToLower(route.Method)] = operation
	}

	servers := []map[string]string{{"url": "http://localhost:8080"}}
	if isProduction {
		servers = []map[string]string{{"url": "/"}}
	}

	return map[string]any{
		"openapi": openAPIVersion,
		"info": map[string]any{
			"title":       "Absensholat API",
			"version":     specVersion,
			"description": "Explicit, endpoint-complete API reference for development in Scalar.",
		},
		"servers": servers,
		"tags": []map[string]string{
			{"name": "auth", "description": "Authentication, sessions, token refresh, and account recovery."},
			{"name": "statistics", "description": "Daily attendance analytics and dashboard summaries."},
			{"name": "notifications", "description": "Operational notifications for staff."},
			{"name": "attendance", "description": "Attendance automation, QR attendance, and attendance history."},
			{"name": "reports", "description": "CSV and Excel export endpoints."},
			{"name": "backups", "description": "Backup state, confirmation, and cleanup workflows."},
			{"name": "students", "description": "Student CRUD and student attendance write operations."},
			{"name": "student-control", "description": "Admin-only bulk progression and rollover operations."},
			{"name": "prayer-schedules", "description": "Prayer schedule listing and management."},
			{"name": "system", "description": "Runtime health and metrics endpoints."},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"BearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
			"schemas": schemaComponents(),
		},
		"paths": paths,
	}
}

func explicitOperations() map[string]operationSpec {
	qStudentList := []map[string]any{
		queryParam("search", "Search by NIS or student name.", "string"),
		queryParam("kelas", "Exact class filter.", "string"),
		queryParam("jurusan", "Exact major/department filter.", "string"),
		queryParam("jk", "Gender filter.", "string"),
		queryParam("page", "Page number (1-based).", "integer"),
		queryParam("page_size", "Items per page (max 100).", "integer"),
		queryParam("sort_by", "Sort field: nis, nama_siswa, kelas, jurusan.", "string"),
		queryParam("sort_dir", "Sort direction: asc, desc.", "string"),
	}
	qHistory := []map[string]any{
		queryParam("week", "Week offset for siswa history (0 current week, 1 last week).", "integer"),
		queryParam("start_date", "Start date YYYY-MM-DD.", "string"),
		queryParam("end_date", "End date YYYY-MM-DD.", "string"),
		queryParam("tanggal", "Single date YYYY-MM-DD (overrides range when provided).", "string"),
		queryParam("kelas", "Class filter.", "string"),
		queryParam("jurusan", "Major filter.", "string"),
		queryParam("nis", "Filter by one student NIS.", "string"),
		queryParam("status", "Status filter: hadir, izin, sakit, alpha.", "string"),
		queryParam("page", "Page number.", "integer"),
		queryParam("limit", "Items per page (max 100).", "integer"),
	}
	qExport := []map[string]any{
		queryParam("start_date", "Optional export range start YYYY-MM-DD.", "string"),
		queryParam("end_date", "Optional export range end YYYY-MM-DD.", "string"),
		queryParam("kelas", "Optional class filter.", "string"),
		queryParam("jurusan", "Optional major filter.", "string"),
	}
	hCron := []map[string]any{
		headerParam("Authorization", "Cron secret in plain form or Bearer token. Required in production."),
	}

	okMessage := jsonResponse("Operation successful.", "MessageResponse", map[string]any{"message": "Operasi berhasil"})
	badRequest := errorResponse("Request payload or parameters are invalid.", "VALIDATION_ERROR", map[string]any{"message": "Format JSON tidak valid"})
	unauthorized := errorResponse("Authentication failed or token missing/invalid.", "UNAUTHORIZED", map[string]any{"message": "Unauthorized"})
	forbidden := errorResponse("Authenticated but not allowed for this role.", "FORBIDDEN", map[string]any{"message": "Akses ditolak"})
	notFound := errorResponse("Requested entity is not found.", "NOT_FOUND", map[string]any{"message": "Data tidak ditemukan"})
	conflict := errorResponse("Resource state conflicts with request.", "CONFLICT", map[string]any{"message": "Data sudah ada"})
	rateLimited := errorResponse("Rate limit exceeded.", "RATE_LIMITED", map[string]any{"message": "Too many requests"})
	serverError := errorResponse("Unexpected server-side error.", "INTERNAL_ERROR", map[string]any{"message": "Terjadi kesalahan server"})

	ops := map[string]operationSpec{
		"GET /health": {
			tags:        []string{"system"},
			summary:     "Service health check",
			description: "Checks service liveness and database connectivity used by monitors and deployment probes.",
			responses: map[string]any{
				"200": jsonResponse("Service healthy.", "HealthResponse", map[string]any{"status": "ok", "message": "API is running", "database": "connected", "timestamp": "2026-04-21T09:32:10Z"}),
				"503": jsonResponse("Service degraded (for example, database unreachable).", "HealthResponse", map[string]any{"status": "degraded", "message": "API is running", "database": "disconnected", "timestamp": "2026-04-21T09:32:10Z"}),
			},
		},
		"GET /metrics": {
			tags:        []string{"system"},
			summary:     "Prometheus metrics",
			description: "Returns Prometheus-compatible plain text metrics for scraping.",
			responses: map[string]any{
				"200": map[string]any{"description": "Prometheus text format metrics."},
			},
		},
		"POST /auth/register": {
			tags:          []string{"auth"},
			summary:       "Register student account",
			description:   "Registers login account for an existing student NIS. Enforces Google email and password rules.",
			requestSchema: "RegisterRequest",
			responses: map[string]any{
				"201": jsonResponse("Account created.", "RegisterResponse", map[string]any{"message": "Registrasi berhasil", "nis": "12345678", "email": "siswa@gmail.com", "created_at": "2026-04-21T09:32:10Z", "is_google_acct": true}),
				"400": badRequest,
				"401": errorResponse("NIS not registered in student master data.", "NIS_NOT_REGISTERED", map[string]any{"message": "NIS tidak terdaftar sebagai siswa"}),
				"409": conflict,
				"429": rateLimited,
				"500": serverError,
			},
		},
		"POST /auth/login": {
			tags:          []string{"auth"},
			summary:       "Authenticate user",
			description:   "Logs in siswa or staff using identifier/username and password, then returns access and refresh tokens.",
			requestSchema: "LoginRequest",
			responses: map[string]any{
				"200": jsonResponse("Login successful.", "AuthResponse", map[string]any{"message": "Login berhasil", "data": map[string]any{"role": "siswa", "email": "siswa@gmail.com", "token": "jwt-access", "refresh_token": "jwt-refresh"}}),
				"400": badRequest,
				"401": unauthorized,
				"429": rateLimited,
				"500": serverError,
			},
		},
		"POST /auth/refresh": {
			tags:          []string{"auth"},
			summary:       "Refresh token pair",
			description:   "Generates a new access token (and optionally refresh token) from a valid refresh token.",
			requestSchema: "RefreshRequest",
			responses: map[string]any{
				"200": jsonResponse("Token refreshed.", "AuthResponse", map[string]any{"message": "Token berhasil diperbarui", "data": map[string]any{"token": "jwt-access-new", "refresh_token": "jwt-refresh-new"}}),
				"400": badRequest,
				"401": unauthorized,
				"500": serverError,
			},
		},
		"POST /auth/logout": {
			tags:          []string{"auth"},
			summary:       "Logout session",
			description:   "Invalidates the refresh token/session provided in request body.",
			requestSchema: "RefreshRequest",
			responses: map[string]any{
				"200": okMessage,
				"400": badRequest,
				"401": unauthorized,
				"500": serverError,
			},
		},
		"DELETE /auth/logout": {
			tags:        []string{"auth"},
			summary:     "Logout current session (v2)",
			description: "Deletes the currently authenticated session.",
			responses: map[string]any{
				"200": okMessage,
				"401": unauthorized,
				"500": serverError,
			},
		},
		"GET /auth/me": {
			tags:        []string{"auth"},
			summary:     "Current profile",
			description: "Returns current authenticated identity and role profile data.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Profile data.", "AuthResponse", map[string]any{"message": "Data profil", "data": map[string]any{"role": "admin", "username": "admin1"}}),
				"401": unauthorized,
				"500": serverError,
			},
		},
		"POST /auth/forgot-password": {
			tags:          []string{"auth"},
			summary:       "Request password reset OTP",
			description:   "Sends OTP to registered email for password reset.",
			requestSchema: "ForgotPasswordRequest",
			responses: map[string]any{
				"200": jsonResponse("OTP sent.", "MessageResponse", map[string]any{"message": "Kode OTP telah dikirim ke email Anda", "email": "s***@gmail.com", "expires_in": "10 menit"}),
				"400": badRequest,
				"404": notFound,
				"429": rateLimited,
				"500": serverError,
			},
		},
		"POST /auth/verify-otp": {
			tags:          []string{"auth"},
			summary:       "Verify OTP",
			description:   "Validates OTP for reset password flow.",
			requestSchema: "VerifyOTPRequest",
			responses: map[string]any{
				"200": jsonResponse("OTP valid.", "MessageResponse", map[string]any{"message": "Kode OTP valid. Silakan reset password Anda", "verified": true}),
				"400": errorResponse("OTP invalid or expired.", "OTP_INVALID", map[string]any{"message": "Kode OTP tidak valid"}),
				"429": rateLimited,
				"500": serverError,
			},
		},
		"POST /auth/reset-password": {
			tags:          []string{"auth"},
			summary:       "Reset password",
			description:   "Resets password using OTP and new password.",
			requestSchema: "ResetPasswordRequest",
			responses: map[string]any{
				"200": okMessage,
				"400": badRequest,
				"429": rateLimited,
				"500": serverError,
			},
		},
		"POST /auth/change-email": {
			tags:          []string{"auth"},
			summary:       "Request email change",
			description:   "Requests OTP to validate and apply a new email address.",
			security:      true,
			requestSchema: "ChangeEmailRequest",
			responses: map[string]any{
				"200": jsonResponse("OTP sent to new email.", "MessageResponse", map[string]any{"message": "Kode OTP telah dikirim ke email baru Anda", "email": "n***@gmail.com", "expires_in": "10 menit"}),
				"400": badRequest,
				"401": unauthorized,
				"409": conflict,
				"500": serverError,
			},
		},
		"POST /auth/verify-change-email": {
			tags:          []string{"auth"},
			summary:       "Verify email change",
			description:   "Verifies email-change OTP and finalizes account email update.",
			security:      true,
			requestSchema: "VerifyChangeEmailRequest",
			responses: map[string]any{
				"200": okMessage,
				"400": badRequest,
				"401": unauthorized,
				"500": serverError,
			},
		},
		"GET /statistics": {
			tags:        []string{"statistics"},
			summary:     "Daily attendance statistics",
			description: "Returns current day attendance aggregates and percentages for dashboard widgets.",
			responses: map[string]any{
				"200": jsonResponse("Statistics payload.", "StatisticsResponse", map[string]any{"message": "Statistik berhasil diambil", "data": map[string]any{"tanggal": "2026-04-21", "total_siswa": 450, "total_absen_hari_ini": 390, "total_kehadiran_hari_ini": 360, "persentase_kehadiran": 92.3}}),
				"500": serverError,
			},
		},
		"GET /notifications": {
			tags:        []string{"notifications"},
			summary:     "Pending notifications",
			description: "Returns pending notifications for admin, guru, and wali_kelas roles.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Notifications returned.", "NotificationListResponse", map[string]any{"message": "Notifikasi berhasil diambil", "data": []map[string]any{{"type": "backup", "message": "Data siap di-backup"}}}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /auto-absen/trigger": {
			tags:         []string{"attendance"},
			summary:      "Cron trigger auto attendance",
			description:  "Internal cron endpoint to run auto-mark attendance flow (requires CRON_SECRET in production).",
			headerParams: hCron,
			responses: map[string]any{
				"200": okMessage,
				"401": errorResponse("Invalid CRON_SECRET.", "CRON_UNAUTHORIZED", map[string]any{"message": "Unauthorized cron trigger"}),
				"500": serverError,
			},
		},
		"POST /auto-absen/trigger": {
			tags:        []string{"attendance"},
			summary:     "Manual trigger auto attendance",
			description: "Admin-only endpoint to manually run auto attendance marking.",
			security:    true,
			responses: map[string]any{
				"200": okMessage,
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /export/absensi":           exportCSV("Export attendance CSV", "Exports attendance rows in CSV format for selected filters.", qExport),
		"GET /export/absensi/excel":     exportFile("Export attendance Excel", "Exports attendance rows in Excel format.", qExport),
		"GET /export/laporan":           exportCSV("Export summary CSV", "Exports summary report in CSV format.", qExport),
		"GET /export/laporan/excel":     exportFile("Export summary Excel", "Exports summary report in Excel format.", qExport),
		"GET /export/attendance-report": exportFile("Export attendance report", "Exports attendance report workbook for management analysis.", qExport),
		"GET /export/full-report":       exportFile("Export integrated report", "Exports integrated multi-sheet report workbook.", qExport),
		"GET /backup/status": {
			tags:        []string{"backups"},
			summary:     "Backup status",
			description: "Returns backup readiness status and retention hints.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Backup status payload.", "MessageResponse", map[string]any{"message": "Status backup berhasil diambil", "ready": true}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"POST /backup/confirm": {
			tags:        []string{"backups"},
			summary:     "Confirm backup",
			description: "Confirms that data backup has been completed before cleanup step.",
			security:    true,
			responses: map[string]any{
				"200": okMessage,
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"DELETE /backup/cleanup": {
			tags:        []string{"backups"},
			summary:     "Cleanup backed-up data",
			description: "Deletes data that has been marked as safely backed up.",
			security:    true,
			responses: map[string]any{
				"200": okMessage,
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /history/siswa": {
			tags:        []string{"attendance"},
			summary:     "Student attendance history",
			description: "Returns weekly attendance history and statistics for the authenticated student.",
			security:    true,
			queryParams: []map[string]any{queryParam("week", "Week offset (0 this week).", "integer")},
			responses: map[string]any{
				"200": jsonResponse("History payload.", "HistorySiswaResponse", map[string]any{"message": "Riwayat absensi berhasil diambil", "data": map[string]any{"periode": "Minggu Ini", "statistik": map[string]any{"total_absensi": 5, "total_hadir": 4}}}),
				"401": unauthorized,
				"404": notFound,
				"500": serverError,
			},
		},
		"GET /history/staff": {
			tags:        []string{"attendance"},
			summary:     "Staff attendance history",
			description: "Returns attendance history with filters and pagination for admin/guru/wali_kelas.",
			security:    true,
			queryParams: qHistory,
			responses: map[string]any{
				"200": jsonResponse("History payload.", "HistoryStaffResponse", map[string]any{"message": "Riwayat absensi berhasil diambil", "data": map[string]any{"pagination": map[string]any{"page": 1, "limit": 20, "total_items": 120}}}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /qrcode/generate": {
			tags:        []string{"attendance"},
			summary:     "Generate attendance QR",
			description: "Generates active QR token payload used for student attendance check-in.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("QR payload generated.", "QRCodeResponse", map[string]any{"message": "QR code berhasil dibuat", "data": map[string]any{"token": "qr-token", "expires_at": "2026-04-21T10:00:00Z"}}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /qrcode/image": {
			tags:        []string{"attendance"},
			summary:     "Generate QR image",
			description: "Returns PNG image representation of current attendance QR code.",
			security:    true,
			responses: map[string]any{
				"200": map[string]any{"description": "PNG image bytes of QR code."},
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"POST /qrcode/verify": {
			tags:          []string{"attendance"},
			summary:       "Verify scanned QR",
			description:   "Verifies scanned QR token and records attendance for authenticated student.",
			security:      true,
			requestSchema: "QRCodeVerifyRequest",
			responses: map[string]any{
				"200": jsonResponse("Attendance recorded.", "MessageResponse", map[string]any{"message": "Absensi berhasil"}),
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /siswa": {
			tags:        []string{"students"},
			summary:     "List students",
			description: "Returns paginated list of students with search, filter, and sorting support.",
			security:    true,
			queryParams: qStudentList,
			responses: map[string]any{
				"200": jsonResponse("Student list returned.", "SiswaListPaginatedResponse", map[string]any{"message": "Data siswa berhasil diambil", "pagination": map[string]any{"page": 1, "page_size": 20, "total_items": 450, "total_pages": 23}}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"POST /siswa": {
			tags:          []string{"students"},
			summary:       "Create student",
			description:   "Creates new student master-data row.",
			security:      true,
			requestSchema: "StudentCreateRequest",
			responses: map[string]any{
				"201": jsonResponse("Student created.", "SiswaDetailResponse", map[string]any{"message": "Siswa berhasil ditambahkan", "data": map[string]any{"nis": "12345678", "nama_siswa": "Budi"}}),
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"409": conflict,
				"500": serverError,
			},
		},
		"GET /siswa/{nis}": {
			tags:        []string{"students"},
			summary:     "Get student by NIS",
			description: "Returns single student detail by NIS.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Student detail returned.", "SiswaDetailResponse", map[string]any{"message": "Data siswa berhasil diambil", "data": map[string]any{"nis": "12345678", "nama_siswa": "Budi"}}),
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"PUT /siswa/{nis}": {
			tags:          []string{"students"},
			summary:       "Update student",
			description:   "Updates student identity and class attributes by NIS.",
			security:      true,
			requestSchema: "StudentUpdateRequest",
			responses: map[string]any{
				"200": jsonResponse("Student updated.", "SiswaDetailResponse", map[string]any{"message": "Data siswa berhasil diperbarui", "data": map[string]any{"nis": "12345678", "kelas": "XII-A"}}),
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"DELETE /siswa/{nis}": {
			tags:        []string{"students"},
			summary:     "Delete student",
			description: "Deletes student record by NIS.",
			security:    true,
			responses: map[string]any{
				"200": okMessage,
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"POST /siswa/{nis}/attendances": {
			tags:          []string{"students"},
			summary:       "Create attendance for student",
			description:   "Creates one attendance row for selected student NIS.",
			security:      true,
			requestSchema: "AttendanceCreateRequest",
			responses: map[string]any{
				"201": jsonResponse("Attendance row created.", "MessageResponse", map[string]any{"message": "Absensi berhasil ditambahkan"}),
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"POST /siswa/{path}": {
			tags:          []string{"students"},
			summary:       "Legacy siswa wildcard action",
			description:   "Compatibility endpoint used by legacy clients that post to wildcard siswa path.",
			security:      true,
			requestSchema: "LegacySiswaPathRequest",
			responses: map[string]any{
				"200": okMessage,
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"GET /admin/student-control/overview": {
			tags:        []string{"student-control"},
			summary:     "Student control overview",
			description: "Returns dashboard overview for admin student progression controls.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Overview returned.", "MessageResponse", map[string]any{"message": "Overview berhasil diambil"}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /admin/student-control/transitions": {
			tags:        []string{"student-control"},
			summary:     "Student transitions",
			description: "Returns transition candidates and progression states.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Transitions returned.", "MessageResponse", map[string]any{"message": "Transisi berhasil diambil"}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"POST /admin/student-control/bulk-progression": {
			tags:          []string{"student-control"},
			summary:       "Bulk progression",
			description:   "Runs bulk class progression according to academic rules.",
			security:      true,
			requestSchema: "StudentControlBulkRequest",
			responses: map[string]any{
				"200": okMessage,
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"POST /admin/student-control/annual-rollover": {
			tags:          []string{"student-control"},
			summary:       "Annual rollover",
			description:   "Performs annual academic rollover for all managed students.",
			security:      true,
			requestSchema: "StudentControlRolloverRequest",
			responses: map[string]any{
				"200": okMessage,
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /jadwal-sholat": {
			tags:        []string{"prayer-schedules"},
			summary:     "List prayer schedules",
			description: "Returns prayer schedules filtered by role permissions.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Schedule list returned.", "PrayerScheduleListResponse", map[string]any{"message": "Jadwal sholat berhasil diambil", "data": []map[string]any{{"id_jadwal": 1, "hari": "Senin", "jenis_sholat": "Dzuhur"}}}),
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"GET /jadwal-sholat/{id}": {
			tags:        []string{"prayer-schedules"},
			summary:     "Get prayer schedule by ID",
			description: "Returns one prayer schedule row by id_jadwal.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Schedule returned.", "PrayerScheduleDetailResponse", map[string]any{"message": "Jadwal sholat berhasil diambil", "data": map[string]any{"id_jadwal": 1, "jenis_sholat": "Dzuhur"}}),
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"GET /jadwal-sholat/dhuha-today": {
			tags:        []string{"prayer-schedules"},
			summary:     "Get today's Dhuha schedule",
			description: "Returns active Dhuha schedule for current day.",
			security:    true,
			responses: map[string]any{
				"200": jsonResponse("Today Dhuha schedule returned.", "PrayerScheduleDetailResponse", map[string]any{"message": "Jadwal dhuha hari ini", "data": map[string]any{"jenis_sholat": "Dhuha"}}),
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"POST /jadwal-sholat": {
			tags:          []string{"prayer-schedules"},
			summary:       "Create prayer schedule",
			description:   "Creates a new prayer schedule definition.",
			security:      true,
			requestSchema: "PrayerScheduleCreateRequest",
			responses: map[string]any{
				"201": jsonResponse("Schedule created.", "PrayerScheduleDetailResponse", map[string]any{"message": "Jadwal sholat berhasil ditambahkan", "data": map[string]any{"id_jadwal": 12}}),
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"500": serverError,
			},
		},
		"PUT /jadwal-sholat/{id}": {
			tags:          []string{"prayer-schedules"},
			summary:       "Update prayer schedule",
			description:   "Updates existing prayer schedule by id_jadwal.",
			security:      true,
			requestSchema: "PrayerScheduleUpdateRequest",
			responses: map[string]any{
				"200": jsonResponse("Schedule updated.", "PrayerScheduleDetailResponse", map[string]any{"message": "Jadwal sholat berhasil diperbarui", "data": map[string]any{"id_jadwal": 12}}),
				"400": badRequest,
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
		"DELETE /jadwal-sholat/{id}": {
			tags:        []string{"prayer-schedules"},
			summary:     "Delete prayer schedule",
			description: "Deletes prayer schedule by id_jadwal.",
			security:    true,
			responses: map[string]any{
				"200": okMessage,
				"401": unauthorized,
				"403": forbidden,
				"404": notFound,
				"500": serverError,
			},
		},
	}

	return ops
}

func exportCSV(summary, description string, query []map[string]any) operationSpec {
	return operationSpec{
		tags:        []string{"reports"},
		summary:     summary,
		description: description,
		security:    true,
		queryParams: query,
		responses: map[string]any{
			"200": map[string]any{"description": "CSV file stream."},
			"401": errorResponse("Authentication required.", "UNAUTHORIZED", map[string]any{"message": "Unauthorized"}),
			"403": errorResponse("Forbidden for current role.", "FORBIDDEN", map[string]any{"message": "Akses ditolak"}),
			"500": errorResponse("Export generation failed.", "EXPORT_ERROR", map[string]any{"message": "Gagal mengekspor data"}),
		},
	}
}

func exportFile(summary, description string, query []map[string]any) operationSpec {
	return operationSpec{
		tags:        []string{"reports"},
		summary:     summary,
		description: description,
		security:    true,
		queryParams: query,
		responses: map[string]any{
			"200": map[string]any{"description": "Binary XLSX file stream."},
			"401": errorResponse("Authentication required.", "UNAUTHORIZED", map[string]any{"message": "Unauthorized"}),
			"403": errorResponse("Forbidden for current role.", "FORBIDDEN", map[string]any{"message": "Akses ditolak"}),
			"500": errorResponse("Export generation failed.", "EXPORT_ERROR", map[string]any{"message": "Gagal mengekspor data"}),
		},
	}
}

func schemaComponents() map[string]any {
	return map[string]any{
		"ErrorResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"code":    map[string]any{"type": "string"},
				"error":   map[string]any{"description": "Optional raw error details."},
				"errors":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
			"required": []string{"message"},
		},
		"MessageResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
			"required": []string{"message"},
		},
		"HealthResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":    map[string]any{"type": "string"},
				"message":   map[string]any{"type": "string"},
				"database":  map[string]any{"type": "string"},
				"timestamp": map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"RegisterRequest": map[string]any{
			"type":     "object",
			"required": []string{"nis", "email", "password"},
			"properties": map[string]any{
				"nis":      map[string]any{"type": "string", "example": "12345678"},
				"email":    map[string]any{"type": "string", "format": "email", "example": "siswa@gmail.com"},
				"password": map[string]any{"type": "string", "minLength": 6, "example": "StrongPass123!"},
			},
		},
		"RegisterResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message":        map[string]any{"type": "string"},
				"nis":            map[string]any{"type": "string"},
				"email":          map[string]any{"type": "string", "format": "email"},
				"created_at":     map[string]any{"type": "string", "format": "date-time"},
				"is_google_acct": map[string]any{"type": "boolean"},
			},
		},
		"LoginRequest": map[string]any{
			"type":     "object",
			"required": []string{"password"},
			"properties": map[string]any{
				"identifier": map[string]any{"type": "string"},
				"username":   map[string]any{"type": "string"},
				"password":   map[string]any{"type": "string"},
			},
		},
		"RefreshRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"refresh_token": map[string]any{"type": "string"},
			},
		},
		"ForgotPasswordRequest": map[string]any{
			"type":     "object",
			"required": []string{"nis", "email"},
			"properties": map[string]any{
				"nis":   map[string]any{"type": "string"},
				"email": map[string]any{"type": "string", "format": "email"},
			},
		},
		"VerifyOTPRequest": map[string]any{
			"type":     "object",
			"required": []string{"nis", "otp"},
			"properties": map[string]any{
				"nis": map[string]any{"type": "string"},
				"otp": map[string]any{"type": "string"},
			},
		},
		"ResetPasswordRequest": map[string]any{
			"type":     "object",
			"required": []string{"nis", "otp", "new_password"},
			"properties": map[string]any{
				"nis":          map[string]any{"type": "string"},
				"otp":          map[string]any{"type": "string"},
				"new_password": map[string]any{"type": "string", "minLength": 6},
			},
		},
		"ChangeEmailRequest": map[string]any{
			"type":     "object",
			"required": []string{"new_email"},
			"properties": map[string]any{
				"new_email": map[string]any{"type": "string", "format": "email"},
			},
		},
		"VerifyChangeEmailRequest": map[string]any{
			"type":     "object",
			"required": []string{"new_email", "otp"},
			"properties": map[string]any{
				"new_email": map[string]any{"type": "string", "format": "email"},
				"otp":       map[string]any{"type": "string"},
			},
		},
		"AuthResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
		},
		"StatisticsResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
		},
		"NotificationListResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
		},
		"StudentCreateRequest": map[string]any{
			"type":     "object",
			"required": []string{"nis", "nama_siswa", "jk"},
			"properties": map[string]any{
				"nis":        map[string]any{"type": "string"},
				"nama_siswa": map[string]any{"type": "string"},
				"jk":         map[string]any{"type": "string"},
				"jurusan":    map[string]any{"type": "string"},
				"kelas":      map[string]any{"type": "string"},
			},
		},
		"StudentUpdateRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"nama_siswa": map[string]any{"type": "string"},
				"jk":         map[string]any{"type": "string"},
				"jurusan":    map[string]any{"type": "string"},
				"kelas":      map[string]any{"type": "string"},
			},
		},
		"SiswaDetailResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
		},
		"SiswaListPaginatedResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message":    map[string]any{"type": "string"},
				"data":       map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"pagination": map[string]any{"type": "object"},
				"filters":    map[string]any{"type": "object"},
			},
		},
		"AttendanceCreateRequest": map[string]any{
			"type":     "object",
			"required": []string{"id_jadwal", "tanggal", "status"},
			"properties": map[string]any{
				"id_jadwal": map[string]any{"type": "integer"},
				"tanggal":   map[string]any{"type": "string", "format": "date"},
				"status":    map[string]any{"type": "string"},
				"deskripsi": map[string]any{"type": "string"},
			},
		},
		"LegacySiswaPathRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"payload": map[string]any{"type": "object"},
			},
		},
		"HistorySiswaResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
		},
		"HistoryStaffResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
		},
		"QRCodeVerifyRequest": map[string]any{
			"type":     "object",
			"required": []string{"token"},
			"properties": map[string]any{
				"token": map[string]any{"type": "string"},
			},
		},
		"QRCodeResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
		},
		"StudentControlBulkRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"academic_year": map[string]any{"type": "string"},
				"target_class":  map[string]any{"type": "string"},
			},
		},
		"StudentControlRolloverRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"academic_year": map[string]any{"type": "string"},
				"confirm":       map[string]any{"type": "boolean"},
			},
		},
		"PrayerScheduleCreateRequest": map[string]any{
			"type":     "object",
			"required": []string{"hari", "jenis_sholat", "waktu_mulai", "waktu_selesai"},
			"properties": map[string]any{
				"hari":          map[string]any{"type": "string"},
				"jenis_sholat":  map[string]any{"type": "string"},
				"waktu_mulai":   map[string]any{"type": "string"},
				"waktu_selesai": map[string]any{"type": "string"},
				"jurusan":       map[string]any{"type": "string"},
				"kelas":         map[string]any{"type": "string"},
			},
		},
		"PrayerScheduleUpdateRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hari":          map[string]any{"type": "string"},
				"jenis_sholat":  map[string]any{"type": "string"},
				"waktu_mulai":   map[string]any{"type": "string"},
				"waktu_selesai": map[string]any{"type": "string"},
				"jurusan":       map[string]any{"type": "string"},
				"kelas":         map[string]any{"type": "string"},
			},
		},
		"PrayerScheduleListResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
		},
		"PrayerScheduleDetailResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"type": "object"},
			},
		},
	}
}

func shouldSkipPath(path string) bool {
	switch path {
	case "/docs", "/openapi.json":
		return true
	default:
		return strings.HasPrefix(path, "/debug/pprof")
	}
}

func normalizeGinPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
			continue
		}
		if strings.HasPrefix(part, "*") {
			name := strings.TrimPrefix(part, "*")
			if name == "" {
				name = "wildcard"
			}
			parts[i] = "{" + name + "}"
		}
	}
	return strings.Join(parts, "/")
}

func canonicalPath(path string) string {
	if strings.HasPrefix(path, "/api/v2") {
		path = strings.TrimPrefix(path, "/api/v2")
	} else if strings.HasPrefix(path, "/api/v1") {
		path = strings.TrimPrefix(path, "/api/v1")
	} else if strings.HasPrefix(path, "/api") {
		path = strings.TrimPrefix(path, "/api")
	}
	if path == "" {
		path = "/"
	}

	rewrites := map[string]string{
		"/auth/registrations":                "/auth/register",
		"/auth/sessions":                     "/auth/login",
		"/auth/tokens/refresh":               "/auth/refresh",
		"/auth/sessions/current":             "/auth/logout",
		"/auth/profile":                      "/auth/me",
		"/auth/password-resets":              "/auth/forgot-password",
		"/auth/password-resets/verify-otp":   "/auth/verify-otp",
		"/auth/password-resets/confirm":      "/auth/reset-password",
		"/auth/email-change-requests":        "/auth/change-email",
		"/auth/email-change-requests/verify": "/auth/verify-change-email",
		"/analytics/attendance":              "/statistics",
		"/reports/attendances/csv":           "/export/absensi",
		"/reports/attendances/excel":         "/export/absensi/excel",
		"/reports/summary/csv":               "/export/laporan",
		"/reports/summary/excel":             "/export/laporan/excel",
		"/reports/attendance/excel":          "/export/attendance-report",
		"/reports/full/excel":                "/export/full-report",
		"/data-retention/backups/status":     "/backup/status",
		"/data-retention/backups/confirm":    "/backup/confirm",
		"/data-retention/backups":            "/backup/cleanup",
		"/students/me/attendance-history":    "/history/siswa",
		"/attendance/history":                "/history/staff",
		"/attendance/qr-codes/current":       "/qrcode/generate",
		"/attendance/qr-codes/current/image": "/qrcode/image",
		"/attendance/qr-codes/verify":        "/qrcode/verify",
		"/students":                          "/siswa",
		"/prayer-schedules":                  "/jadwal-sholat",
		"/prayer-schedules/dhuha/today":      "/jadwal-sholat/dhuha-today",
	}

	if strings.HasPrefix(path, "/students/{nis}") {
		path = strings.Replace(path, "/students", "/siswa", 1)
	}
	if strings.HasPrefix(path, "/prayer-schedules/") {
		path = strings.Replace(path, "/prayer-schedules", "/jadwal-sholat", 1)
	}
	if strings.HasPrefix(path, "/attendance/auto-mark") || strings.HasPrefix(path, "/internal/cron/attendance/auto-mark") {
		path = "/auto-absen/trigger"
	}

	if rewritten, ok := rewrites[path]; ok {
		return rewritten
	}
	return path
}

func routeOperationID(method, path string) string {
	replacer := strings.NewReplacer("/", "_", "{", "", "}", "", "-", "_", ".", "_")
	clean := replacer.Replace(strings.Trim(path, "/"))
	if clean == "" {
		clean = "root"
	}
	return strings.ToLower(method) + "_" + clean
}

func routeParameters(path string) []map[string]any {
	parts := strings.Split(path, "/")
	params := make([]map[string]any, 0)
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if name == "" {
				continue
			}
			params = append(params, map[string]any{
				"name":        name,
				"in":          "path",
				"required":    true,
				"description": pathParamDescription(name),
				"schema": map[string]any{
					"type": "string",
				},
			})
		}
	}
	return params
}

func pathParamDescription(name string) string {
	switch name {
	case "nis":
		return "Student NIS identifier."
	case "id":
		return "Numeric resource id."
	case "path":
		return "Legacy wildcard segment."
	default:
		return "Path parameter."
	}
}

func queryParam(name, description, typ string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"required":    false,
		"description": description,
		"schema": map[string]any{
			"type": typ,
		},
	}
}

func headerParam(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "header",
		"required":    false,
		"description": description,
		"schema": map[string]any{
			"type": "string",
		},
	}
}

func schemaRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func jsonResponse(description, schema string, example any) map[string]any {
	content := map[string]any{
		"schema": schemaRef(schema),
	}
	if example != nil {
		content["example"] = example
	}
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": content,
		},
	}
}

func errorResponse(description, code string, example map[string]any) map[string]any {
	if example == nil {
		example = map[string]any{"message": "Error"}
	}
	example["code"] = code
	return jsonResponse(description, "ErrorResponse", example)
}

func uniqueParams(in []map[string]any) []map[string]any {
	seen := map[string]bool{}
	out := make([]map[string]any, 0, len(in))
	for _, p := range in {
		name, _ := p["name"].(string)
		location, _ := p["in"].(string)
		key := location + ":" + name
		if key == ":" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}
