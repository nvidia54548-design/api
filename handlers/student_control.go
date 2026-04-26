package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
)

var academicYearPattern = regexp.MustCompile(`^\d{4}-\d{4}$`)

type StudentControlAction string

const (
	ActionPromote     StudentControlAction = "promote"
	ActionRetain      StudentControlAction = "retain"
	ActionDemote      StudentControlAction = "demote"
	ActionSetClass    StudentControlAction = "set_class"
	ActionSetNoClass  StudentControlAction = "set_no_class"
	ActionMarkDropout StudentControlAction = "mark_dropout"
)

type BulkStudentControlRequest struct {
	NISList         []string `json:"nis_list" binding:"required,min=1"`
	Action          string   `json:"action" binding:"required"`
	TargetClass     string   `json:"target_class"`
	AcademicYear    string   `json:"academic_year"`
	CurrentSemester *int     `json:"current_semester,omitempty"`
	Note            string   `json:"note"`
}

type AnnualRolloverStudentMove struct {
	NIS         string `json:"nis" binding:"required"`
	TargetClass string `json:"target_class" binding:"required"`
}

type AnnualRolloverRequest struct {
	FromAcademicYear string                      `json:"from_academic_year" binding:"required"`
	ToAcademicYear   string                      `json:"to_academic_year" binding:"required"`
	Promoted         []AnnualRolloverStudentMove `json:"promoted"`
	RetainedNIS      []string                    `json:"retained_nis"`
	Demoted          []AnnualRolloverStudentMove `json:"demoted"`
	DropoutNIS       []string                    `json:"dropout_nis"`
	NoClassNIS       []string                    `json:"no_class_nis"`
	NextSemester     *int                        `json:"next_semester,omitempty"`
	Note             string                      `json:"note"`
}

type StudentTransitionListResponse struct {
	Message string                 `json:"message"`
	Data    []map[string]interface{} `json:"data"`
}

type studentTransitionInput struct {
	Action           StudentControlAction
	TargetClass      string
	TargetStatus     string
	TargetAcademicYr string
	TargetSemester   *int
	Note             string
	Actor            string
}

func isValidAcademicYear(v string) bool {
	if v == "" {
		return true
	}
	if !academicYearPattern.MatchString(v) {
		return false
	}
	var start, end int
	_, err := fmt.Sscanf(v, "%d-%d", &start, &end)
	if err != nil {
		return false
	}
	return end == start+1
}

func normalizeNISList(list []string) []string {
	set := make(map[string]struct{}, len(list))
	for _, nis := range list {
		n := strings.TrimSpace(nis)
		if n == "" {
			continue
		}
		set[n] = struct{}{}
	}

	result := make([]string, 0, len(set))
	for nis := range set {
		result = append(result, nis)
	}
	sort.Strings(result)
	return result
}

func getActor(c *gin.Context) string {
	if username, ok := c.Get("username"); ok {
		if u, ok := username.(string); ok && strings.TrimSpace(u) != "" {
			return u
		}
	}
	if name, ok := c.Get("name"); ok {
		if n, ok := name.(string); ok && strings.TrimSpace(n) != "" {
			return n
		}
	}
	return "admin"
}

func resolveClassID(tx *gorm.DB, classLabel string) (*int, error) {
	label := strings.TrimSpace(classLabel)
	if label == "" {
		return nil, nil
	}
	if asID, err := strconv.Atoi(label); err == nil {
		return &asID, nil
	}
	var id int
	err := tx.Table("kelas").
		Select("id_kelas").
		Where("CONCAT(CAST(tingkatan AS TEXT), ' ', jurusan, ' ', part) = ?", label).
		Scan(&id).Error
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, fmt.Errorf("kelas tujuan tidak ditemukan: %s", label)
	}
	return &id, nil
}

func resolveActorStaffID(tx *gorm.DB, actor string) *int {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil
	}
	var id int
	if err := tx.Table("staff").Select("id_staff").Where("nip = ?", actor).Scan(&id).Error; err == nil && id > 0 {
		return &id
	}
	id = 0
	if err := tx.Table("staff").Select("id_staff").Where("nama = ?", actor).Scan(&id).Error; err == nil && id > 0 {
		return &id
	}
	return nil
}

func applyStudentTransition(tx *gorm.DB, student models.Siswa, input studentTransitionInput) error {
	updateData := map[string]interface{}{
		"class_status": input.TargetStatus,
	}

	if input.TargetAcademicYr != "" {
		updateData["academic_year"] = input.TargetAcademicYr
	}
	if input.TargetSemester != nil {
		updateData["current_semester"] = *input.TargetSemester
	}

	var fromClassValue interface{}
	if student.IDKelas != nil {
		fromClassValue = *student.IDKelas
	}
	if student.IDKelas == nil && strings.TrimSpace(student.Kelas) != "" {
		fromClassValue = strings.TrimSpace(student.Kelas)
	}
	var toClassValue interface{}
	legacyClassMode := false
	switch input.Action {
	case ActionPromote, ActionDemote, ActionSetClass:
		resolvedClassID, err := resolveClassID(tx, input.TargetClass)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such table") || strings.Contains(strings.ToLower(err.Error()), "does not exist") {
				legacyClassMode = true
				targetClass := strings.TrimSpace(input.TargetClass)
				if targetClass == "" {
					return fmt.Errorf("target_class wajib untuk action %s", input.Action)
				}
				updateData["kelas"] = targetClass
				toClassValue = targetClass
			} else {
				return err
			}
		}
		if !legacyClassMode && resolvedClassID == nil {
			return fmt.Errorf("target_class wajib untuk action %s", input.Action)
		}
		if !legacyClassMode {
			updateData["id_kelas"] = *resolvedClassID
			toClassValue = *resolvedClassID
		}
	case ActionSetNoClass, ActionMarkDropout:
		toClassValue = nil
	}

	if input.Action == ActionPromote {
		now := time.Now()
		updateData["last_promotion_at"] = &now
	}

	if err := tx.Model(&models.Siswa{}).Where("nis = ?", student.NIS).Updates(updateData).Error; err != nil {
		return err
	}

	fromSemester := student.CurrentSemester
	toSemester := student.CurrentSemester
	if input.TargetSemester != nil {
		toSemester = *input.TargetSemester
	}

	fromStatus := student.ClassStatus
	if fromStatus == "" {
		fromStatus = "active"
	}

	if err := tx.Exec(`
		INSERT INTO student_class_transitions
		(id_siswa, action, from_class, to_class, from_academic_year, to_academic_year,
		 from_semester, to_semester, from_status, to_status, decided_by, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		student.IDSiswa,
		string(input.Action),
		fromClassValue,
		toClassValue,
		student.AcademicYear,
		input.TargetAcademicYr,
		fromSemester,
		toSemester,
		fromStatus,
		input.TargetStatus,
		resolveActorStaffID(tx, input.Actor),
		input.Note,
	).Error; err != nil {
		return err
	}

	return nil
}

// AdminBulkStudentControl performs bulk class progression actions for students.
func AdminBulkStudentControl(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req BulkStudentControlRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Payload tidak valid", "error": err.Error()})
			return
		}

		nisList := normalizeNISList(req.NISList)
		if len(nisList) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "nis_list tidak boleh kosong"})
			return
		}

		action := StudentControlAction(strings.TrimSpace(req.Action))
		targetStatus := "active"
		switch action {
		case ActionPromote:
			targetStatus = "promoted"
		case ActionRetain:
			targetStatus = "retained"
		case ActionDemote:
			targetStatus = "demoted"
		case ActionSetClass:
			targetStatus = "active"
		case ActionSetNoClass:
			targetStatus = "no_class"
		case ActionMarkDropout:
			targetStatus = "dropped_out"
		default:
			c.JSON(http.StatusBadRequest, gin.H{"message": "Action tidak valid"})
			return
		}

		if (action == ActionPromote || action == ActionDemote || action == ActionSetClass) && strings.TrimSpace(req.TargetClass) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "target_class wajib untuk action ini"})
			return
		}

		if req.CurrentSemester != nil && (*req.CurrentSemester < 1 || *req.CurrentSemester > 2) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "current_semester harus 1 atau 2"})
			return
		}

		if !isValidAcademicYear(req.AcademicYear) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "academic_year harus format YYYY-YYYY dan berurutan"})
			return
		}

		var students []models.Siswa
		if err := db.Where("nis IN ?", nisList).Find(&students).Error; err != nil {
			logger.Errorw("Failed to fetch students for bulk control", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil data siswa"})
			return
		}

		if len(students) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "Tidak ada siswa ditemukan"})
			return
		}

		found := make(map[string]struct{}, len(students))
		for _, s := range students {
			found[s.NIS] = struct{}{}
		}
		missing := make([]string, 0)
		for _, nis := range nisList {
			if _, ok := found[nis]; !ok {
				missing = append(missing, nis)
			}
		}

		actor := getActor(c)
		updatedCount := 0
		txErr := db.Transaction(func(tx *gorm.DB) error {
			for _, s := range students {
				input := studentTransitionInput{
					Action:           action,
					TargetClass:      req.TargetClass,
					TargetStatus:     targetStatus,
					TargetAcademicYr: strings.TrimSpace(req.AcademicYear),
					TargetSemester:   req.CurrentSemester,
					Note:             strings.TrimSpace(req.Note),
					Actor:            actor,
				}
				if err := applyStudentTransition(tx, s, input); err != nil {
					return err
				}
				updatedCount++
			}
			return nil
		})

		if txErr != nil {
			logger.Errorw("Failed bulk student control transaction", "error", txErr.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui progres siswa"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Kontrol siswa berhasil diproses",
			"action":        action,
			"updated_count": updatedCount,
			"missing_nis":   missing,
		})
	}
}

// AdminAnnualRollover applies yearly progression decisions per student.
func AdminAnnualRollover(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AnnualRolloverRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Payload tidak valid", "error": err.Error()})
			return
		}

		req.FromAcademicYear = strings.TrimSpace(req.FromAcademicYear)
		req.ToAcademicYear = strings.TrimSpace(req.ToAcademicYear)
		if !isValidAcademicYear(req.FromAcademicYear) || !isValidAcademicYear(req.ToAcademicYear) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Format tahun ajaran harus YYYY-YYYY"})
			return
		}

		if req.NextSemester != nil && (*req.NextSemester < 1 || *req.NextSemester > 2) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "next_semester harus 1 atau 2"})
			return
		}

		seen := map[string]string{}
		register := func(nis, action string) error {
			if prev, exists := seen[nis]; exists {
				return fmt.Errorf("nis %s muncul di lebih dari satu keputusan (%s dan %s)", nis, prev, action)
			}
			seen[nis] = action
			return nil
		}

		for _, move := range req.Promoted {
			if err := register(strings.TrimSpace(move.NIS), "promoted"); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
		}
		for _, nis := range req.RetainedNIS {
			if err := register(strings.TrimSpace(nis), "retained"); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
		}
		for _, move := range req.Demoted {
			if err := register(strings.TrimSpace(move.NIS), "demoted"); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
		}
		for _, nis := range req.DropoutNIS {
			if err := register(strings.TrimSpace(nis), "dropout"); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
		}
		for _, nis := range req.NoClassNIS {
			if err := register(strings.TrimSpace(nis), "no_class"); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
		}

		if len(seen) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Tidak ada keputusan siswa untuk diproses"})
			return
		}

		allNIS := make([]string, 0, len(seen))
		for nis := range seen {
			allNIS = append(allNIS, nis)
		}

		var students []models.Siswa
		if err := db.Where("nis IN ?", allNIS).Find(&students).Error; err != nil {
			logger.Errorw("Failed to fetch students for annual rollover", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil data siswa"})
			return
		}

		studentByNIS := map[string]models.Siswa{}
		for _, s := range students {
			studentByNIS[s.NIS] = s
		}

		missing := make([]string, 0)
		for _, nis := range allNIS {
			if _, ok := studentByNIS[nis]; !ok {
				missing = append(missing, nis)
			}
		}
		if len(studentByNIS) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "Tidak ada siswa valid untuk diproses", "missing_nis": missing})
			return
		}

		promotedMap := map[string]string{}
		for _, move := range req.Promoted {
			promotedMap[strings.TrimSpace(move.NIS)] = strings.TrimSpace(move.TargetClass)
		}
		demotedMap := map[string]string{}
		for _, move := range req.Demoted {
			demotedMap[strings.TrimSpace(move.NIS)] = strings.TrimSpace(move.TargetClass)
		}
		retainedSet := map[string]struct{}{}
		for _, nis := range req.RetainedNIS {
			retainedSet[strings.TrimSpace(nis)] = struct{}{}
		}
		dropoutSet := map[string]struct{}{}
		for _, nis := range req.DropoutNIS {
			dropoutSet[strings.TrimSpace(nis)] = struct{}{}
		}
		noClassSet := map[string]struct{}{}
		for _, nis := range req.NoClassNIS {
			noClassSet[strings.TrimSpace(nis)] = struct{}{}
		}

		actor := getActor(c)
		updatedCount := 0
		txErr := db.Transaction(func(tx *gorm.DB) error {
			for nis, s := range studentByNIS {
				input := studentTransitionInput{
					TargetAcademicYr: req.ToAcademicYear,
					TargetSemester:   req.NextSemester,
					Note:             strings.TrimSpace(req.Note),
					Actor:            actor,
				}

				if targetClass, ok := promotedMap[nis]; ok {
					input.Action = ActionPromote
					input.TargetClass = targetClass
					input.TargetStatus = "promoted"
				} else if targetClass, ok := demotedMap[nis]; ok {
					input.Action = ActionDemote
					input.TargetClass = targetClass
					input.TargetStatus = "demoted"
				} else if _, ok := retainedSet[nis]; ok {
					input.Action = ActionRetain
					input.TargetClass = s.Kelas
					input.TargetStatus = "retained"
				} else if _, ok := dropoutSet[nis]; ok {
					input.Action = ActionMarkDropout
					input.TargetStatus = "dropped_out"
				} else if _, ok := noClassSet[nis]; ok {
					input.Action = ActionSetNoClass
					input.TargetStatus = "no_class"
				} else {
					continue
				}

				if err := applyStudentTransition(tx, s, input); err != nil {
					return err
				}
				updatedCount++
			}
			return nil
		})

		if txErr != nil {
			logger.Errorw("Failed annual rollover transaction", "error", txErr.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memproses annual rollover"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":          "Annual rollover berhasil diproses",
			"from_academic_year": req.FromAcademicYear,
			"to_academic_year":   req.ToAcademicYear,
			"updated_count":    updatedCount,
			"missing_nis":      missing,
		})
	}
}

// AdminStudentControlOverview returns summary data for admin control panel.
func AdminStudentControlOverview(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		type statusAgg struct {
			ClassStatus string `json:"class_status"`
			Count       int64  `json:"count"`
		}
		type classAgg struct {
			Kelas string `json:"kelas"`
			Count int64  `json:"count"`
		}
		type yearAgg struct {
			AcademicYear string `json:"academic_year"`
			Count        int64  `json:"count"`
		}

		var total int64
		if err := db.Model(&models.Siswa{}).Count(&total).Error; err != nil {
			logger.Errorw("Failed to count students", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil ringkasan siswa"})
			return
		}

		var byStatus []statusAgg
		db.Model(&models.Siswa{}).
			Select("COALESCE(class_status, 'active') as class_status, COUNT(*) as count").
			Group("COALESCE(class_status, 'active')").
			Scan(&byStatus)

		var byClass []classAgg
		db.Model(&models.Siswa{}).
			Select("COALESCE(NULLIF(kelas, ''), 'NO_CLASS') as kelas, COUNT(*) as count").
			Group("COALESCE(NULLIF(kelas, ''), 'NO_CLASS')").
			Order("count DESC").
			Limit(20).
			Scan(&byClass)

		var byYear []yearAgg
		db.Model(&models.Siswa{}).
			Select("COALESCE(NULLIF(academic_year, ''), 'UNSET') as academic_year, COUNT(*) as count").
			Group("COALESCE(NULLIF(academic_year, ''), 'UNSET')").
			Order("academic_year DESC").
			Limit(10).
			Scan(&byYear)

		c.JSON(http.StatusOK, gin.H{
			"message": "Ringkasan student control panel berhasil diambil",
			"data": gin.H{
				"total_students": total,
				"by_status":      byStatus,
				"top_classes":    byClass,
				"by_academic_year": byYear,
			},
		})
	}
}

// AdminStudentTransitions returns history of class/status decisions.
func AdminStudentTransitions(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		nis := strings.TrimSpace(c.Query("nis"))
		limit := 50
		if q := strings.TrimSpace(c.Query("limit")); q != "" {
			var parsed int
			if _, err := fmt.Sscanf(q, "%d", &parsed); err == nil && parsed > 0 && parsed <= 500 {
				limit = parsed
			}
		}

		query := db.Table("student_class_transitions").Order("created_at DESC").Limit(limit)
		if nis != "" {
			query = query.Where("nis = ?", nis)
		}

		var rows []map[string]interface{}
		if err := query.Find(&rows).Error; err != nil {
			logger.Errorw("Failed to fetch student transition history", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil riwayat transisi siswa"})
			return
		}

		c.JSON(http.StatusOK, StudentTransitionListResponse{
			Message: "Riwayat transisi siswa berhasil diambil",
			Data:    rows,
		})
	}
}
