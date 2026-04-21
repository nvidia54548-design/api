package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"absensholat-api/models"
)

func setupStudentControlTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	err := db.Exec(`
		CREATE TABLE IF NOT EXISTS student_class_transitions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nis TEXT NOT NULL,
			action TEXT NOT NULL,
			from_class TEXT,
			to_class TEXT,
			from_academic_year TEXT,
			to_academic_year TEXT,
			from_semester INTEGER,
			to_semester INTEGER,
			from_status TEXT,
			to_status TEXT,
			decided_by TEXT,
			note TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	require.NoError(t, err)
}

func performJSONRequest(router *gin.Engine, method, target string, payload interface{}) *httptest.ResponseRecorder {
	var body bytes.Buffer
	if payload != nil {
		_ = json.NewEncoder(&body).Encode(payload)
	}

	req, _ := http.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestIsValidAcademicYear(t *testing.T) {
	require.True(t, isValidAcademicYear("2025-2026"))
	require.False(t, isValidAcademicYear("2025-2027"))
	require.False(t, isValidAcademicYear("25-26"))
	require.True(t, isValidAcademicYear(""))
}

func TestNormalizeNISList(t *testing.T) {
	input := []string{" 123 ", "", "123", "999", "999", "  "}
	got := normalizeNISList(input)
	require.Equal(t, []string{"123", "999"}, got)
}

func TestAdminBulkStudentControl_InvalidAction(t *testing.T) {
	db := setupTestDB(t)
	setupStudentControlTables(t, db)
	logger := setupTestLogger()

	router := gin.New()
	router.POST("/admin/student-control/bulk-progression", AdminBulkStudentControl(db, logger))

	payload := BulkStudentControlRequest{
		NISList:      []string{"12345"},
		Action:       "invalid_action",
		AcademicYear: "2025-2026",
	}
	w := performJSONRequest(router, http.MethodPost, "/admin/student-control/bulk-progression", payload)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminBulkStudentControl_PromoteSuccess(t *testing.T) {
	db := setupTestDB(t)
	setupStudentControlTables(t, db)
	logger := setupTestLogger()

	student := models.Siswa{
		NIS:             "12345",
		NamaSiswa:       "Siswa A",
		JK:              "L",
		Kelas:           "X-A",
		AcademicYear:    "2025-2026",
		CurrentSemester: 2,
		ClassStatus:     "active",
	}
	require.NoError(t, db.Create(&student).Error)

	router := gin.New()
	router.POST("/admin/student-control/bulk-progression", func(c *gin.Context) {
		c.Set("username", "admin_1")
		AdminBulkStudentControl(db, logger)(c)
	})

	nextSemester := 1
	payload := BulkStudentControlRequest{
		NISList:         []string{"12345"},
		Action:          string(ActionPromote),
		TargetClass:     "XI-A",
		AcademicYear:    "2026-2027",
		CurrentSemester: &nextSemester,
		Note:            "Naik kelas tahunan",
	}
	w := performJSONRequest(router, http.MethodPost, "/admin/student-control/bulk-progression", payload)
	require.Equal(t, http.StatusOK, w.Code)

	var updated models.Siswa
	require.NoError(t, db.First(&updated, "nis = ?", "12345").Error)
	require.Equal(t, "XI-A", updated.Kelas)
	require.Equal(t, "2026-2027", updated.AcademicYear)
	require.Equal(t, 1, updated.CurrentSemester)
	require.Equal(t, "promoted", updated.ClassStatus)
	require.NotNil(t, updated.LastPromotionAt)

	var count int64
	require.NoError(t, db.Table("student_class_transitions").Where("nis = ? AND action = ?", "12345", "promote").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestAdminAnnualRollover_DuplicateNISInDecisions(t *testing.T) {
	db := setupTestDB(t)
	setupStudentControlTables(t, db)
	logger := setupTestLogger()

	require.NoError(t, db.Create(&models.Siswa{NIS: "12345", NamaSiswa: "Siswa", JK: "L", Kelas: "X-A"}).Error)

	router := gin.New()
	router.POST("/admin/student-control/annual-rollover", AdminAnnualRollover(db, logger))

	payload := AnnualRolloverRequest{
		FromAcademicYear: "2025-2026",
		ToAcademicYear:   "2026-2027",
		Promoted: []AnnualRolloverStudentMove{
			{NIS: "12345", TargetClass: "XI-A"},
		},
		RetainedNIS: []string{"12345"},
	}
	w := performJSONRequest(router, http.MethodPost, "/admin/student-control/annual-rollover", payload)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminAnnualRollover_SuccessMixedDecisions(t *testing.T) {
	db := setupTestDB(t)
	setupStudentControlTables(t, db)
	logger := setupTestLogger()

	seed := []models.Siswa{
		{NIS: "1001", NamaSiswa: "A", JK: "L", Kelas: "X-A", AcademicYear: "2025-2026", CurrentSemester: 2, ClassStatus: "active"},
		{NIS: "1002", NamaSiswa: "B", JK: "P", Kelas: "X-B", AcademicYear: "2025-2026", CurrentSemester: 2, ClassStatus: "active"},
		{NIS: "1003", NamaSiswa: "C", JK: "L", Kelas: "XI-A", AcademicYear: "2025-2026", CurrentSemester: 2, ClassStatus: "active"},
	}
	require.NoError(t, db.Create(&seed).Error)

	router := gin.New()
	router.POST("/admin/student-control/annual-rollover", func(c *gin.Context) {
		c.Set("username", "admin_2")
		AdminAnnualRollover(db, logger)(c)
	})

	nextSemester := 1
	payload := AnnualRolloverRequest{
		FromAcademicYear: "2025-2026",
		ToAcademicYear:   "2026-2027",
		Promoted:         []AnnualRolloverStudentMove{{NIS: "1001", TargetClass: "XI-A"}},
		RetainedNIS:      []string{"1002"},
		DropoutNIS:       []string{"1003"},
		NextSemester:     &nextSemester,
		Note:             "Rollover semester ganjil",
	}
	w := performJSONRequest(router, http.MethodPost, "/admin/student-control/annual-rollover", payload)
	require.Equal(t, http.StatusOK, w.Code)

	var s1, s2, s3 models.Siswa
	require.NoError(t, db.First(&s1, "nis = ?", "1001").Error)
	require.NoError(t, db.First(&s2, "nis = ?", "1002").Error)
	require.NoError(t, db.First(&s3, "nis = ?", "1003").Error)

	require.Equal(t, "XI-A", s1.Kelas)
	require.Equal(t, "promoted", s1.ClassStatus)
	require.Equal(t, "2026-2027", s1.AcademicYear)
	require.Equal(t, 1, s1.CurrentSemester)

	require.Equal(t, "X-B", s2.Kelas)
	require.Equal(t, "retained", s2.ClassStatus)
	require.Equal(t, "2026-2027", s2.AcademicYear)

	require.Equal(t, "", s3.Kelas)
	require.Equal(t, "dropped_out", s3.ClassStatus)
	require.Equal(t, "2026-2027", s3.AcademicYear)

	var count int64
	require.NoError(t, db.Table("student_class_transitions").Count(&count).Error)
	require.Equal(t, int64(3), count)
}

func TestAdminStudentControlOverview_AndTransitions(t *testing.T) {
	db := setupTestDB(t)
	setupStudentControlTables(t, db)
	logger := setupTestLogger()

	seed := []models.Siswa{
		{NIS: "2001", NamaSiswa: "D", JK: "L", Kelas: "X-A", AcademicYear: "2026-2027", ClassStatus: "active"},
		{NIS: "2002", NamaSiswa: "E", JK: "P", Kelas: "", AcademicYear: "2026-2027", ClassStatus: "no_class"},
	}
	require.NoError(t, db.Create(&seed).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO student_class_transitions
		(nis, action, from_class, to_class, from_academic_year, to_academic_year, from_semester, to_semester, from_status, to_status, decided_by, note)
		VALUES ('2001', 'promote', 'IX-A', 'X-A', '2025-2026', '2026-2027', 2, 1, 'active', 'promoted', 'admin', 'ok')
	`).Error)

	router := gin.New()
	router.GET("/admin/student-control/overview", AdminStudentControlOverview(db, logger))
	router.GET("/admin/student-control/transitions", AdminStudentTransitions(db, logger))

	overviewW := performJSONRequest(router, http.MethodGet, "/admin/student-control/overview", nil)
	require.Equal(t, http.StatusOK, overviewW.Code)

	transitionsW := performJSONRequest(router, http.MethodGet, "/admin/student-control/transitions?nis=2001&limit=5", nil)
	require.Equal(t, http.StatusOK, transitionsW.Code)

	var transitionsResp StudentTransitionListResponse
	require.NoError(t, json.Unmarshal(transitionsW.Body.Bytes(), &transitionsResp))
	require.Len(t, transitionsResp.Data, 1)
}
