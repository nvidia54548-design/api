package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"absensholat-api/models"
	"absensholat-api/utils"
)

type CreateUserStaffRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Name     string `json:"name,omitempty"`
	NIP      string `json:"nip,omitempty"`
}

type UpdateUserStaffRequest struct {
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
	Role     *string `json:"role,omitempty"`
	Name     *string `json:"name,omitempty"`
	NIP      *string `json:"nip,omitempty"`
}

type SQLQueryRequest struct {
	Query string `json:"query" binding:"required"`
}

type GenericDataResponse struct {
	Data interface{} `json:"data"`
}

type StaffAccountDTO struct {
	IDStaff   int    `json:"id_staff"`
	IDAccount int    `json:"id_account"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	NIP       string `json:"nip,omitempty"`
}

func nipPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nipValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func AdminListUsersStaff(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rows []StaffAccountDTO
		err := db.Table("staff s").
			Select("s.id_staff, s.id_account, a.email as username, a.role, s.nama as name, COALESCE(s.nip, '') as nip").
			Joins("JOIN accounts a ON a.id = s.id_account").
			Order("s.id_staff ASC").
			Scan(&rows).Error
		if err != nil {
			logger.Errorw("failed to list staff", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil daftar staff"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}
}

func AdminGetUserStaff(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var row StaffAccountDTO
		err := db.Table("staff s").
			Select("s.id_staff, s.id_account, a.email as username, a.role, s.nama as name, COALESCE(s.nip, '') as nip").
			Joins("JOIN accounts a ON a.id = s.id_account").
			Where("s.id_staff = ?", id).
			Scan(&row).Error
		if err != nil {
			logger.Errorw("failed to get staff", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil staff"})
			return
		}
		if row.IDStaff == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "Staff tidak ditemukan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": row})
	}
}

func AdminCreateUserStaff(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CreateUserStaffRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Payload tidak valid", "error": err.Error()})
			return
		}
		input.Role = strings.ToLower(strings.TrimSpace(input.Role))
		if input.Role != "admin" && input.Role != "guru" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Role tidak valid"})
			return
		}
		if err := utils.ValidatePassword(input.Password); err != nil {
			if pwdErr, ok := err.(*utils.PasswordValidationError); ok {
				c.JSON(http.StatusBadRequest, gin.H{"message": "Password tidak memenuhi syarat", "errors": pwdErr.Errors})
				return
			}
		}
		hashed, err := utils.HashPassword(input.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memproses password"})
			return
		}
		tx := db.Begin()
		account := models.Account{Email: strings.TrimSpace(input.Username), Password: hashed, Role: input.Role}
		if err := tx.Create(&account).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusConflict, gin.H{"message": "Username/email sudah digunakan"})
			return
		}
		staff := models.Staff{IDAccount: account.ID, Nama: strings.TrimSpace(input.Name), NIP: nipPtr(input.NIP), TipeStaff: input.Role}
		if staff.Nama == "" {
			staff.Nama = account.Email
		}
		if err := tx.Create(&staff).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat staff"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan staff"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": StaffAccountDTO{
			IDStaff:   staff.IDStaff,
			IDAccount: account.ID,
			Username:  account.Email,
			Role:      account.Role,
			Name:      staff.Nama,
			NIP:       nipValue(staff.NIP),
		}})
	}
}

func AdminUpdateUserStaff(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var staff models.Staff
		if err := db.First(&staff, "id_staff = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "Staff tidak ditemukan"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil staff"})
			return
		}
		var account models.Account
		if err := db.First(&account, "id = ?", staff.IDAccount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil akun staff"})
			return
		}

		var input UpdateUserStaffRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Payload tidak valid", "error": err.Error()})
			return
		}

		if input.Username != nil {
			account.Email = strings.TrimSpace(*input.Username)
		}
		if input.Password != nil {
			if err := utils.ValidatePassword(*input.Password); err != nil {
				if pwdErr, ok := err.(*utils.PasswordValidationError); ok {
					c.JSON(http.StatusBadRequest, gin.H{"message": "Password tidak memenuhi syarat", "errors": pwdErr.Errors})
					return
				}
			}
			hashed, err := utils.HashPassword(*input.Password)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memproses password"})
				return
			}
			account.Password = hashed
		}
		if input.Role != nil {
			role := strings.ToLower(strings.TrimSpace(*input.Role))
			if role != "admin" && role != "guru" {
				c.JSON(http.StatusBadRequest, gin.H{"message": "Role tidak valid"})
				return
			}
			account.Role = role
			staff.TipeStaff = role
		}
		if input.Name != nil {
			staff.Nama = strings.TrimSpace(*input.Name)
		}
		if input.NIP != nil {
			staff.NIP = nipPtr(*input.NIP)
		}

		tx := db.Begin()
		if err := tx.Save(&account).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui akun staff"})
			return
		}
		if err := tx.Save(&staff).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui staff"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan perubahan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": StaffAccountDTO{
			IDStaff:   staff.IDStaff,
			IDAccount: account.ID,
			Username:  account.Email,
			Role:      account.Role,
			Name:      staff.Nama,
			NIP:       nipValue(staff.NIP),
		}})
	}
}

func AdminDeleteUserStaff(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var staff models.Staff
		if err := db.First(&staff, "id_staff = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "Staff tidak ditemukan"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengambil staff"})
			return
		}
		tx := db.Begin()
		if err := tx.Delete(&staff).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menghapus staff"})
			return
		}
		if err := tx.Delete(&models.Account{}, "id = ?", staff.IDAccount).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menghapus akun staff"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan perubahan"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func AdminRunSelectQuery(db *gorm.DB, logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SQLQueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Payload tidak valid", "error": err.Error()})
			return
		}

		q := strings.TrimSpace(req.Query)
		low := strings.ToLower(q)
		if !strings.HasPrefix(low, "select") {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Hanya query SELECT yang diizinkan"})
			return
		}
		if strings.Contains(q, ";") {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Beberapa pernyataan tidak diizinkan"})
			return
		}

		rows, err := db.Raw(q).Rows()
		if err != nil {
			logger.Errorw("query error", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menjalankan query"})
			return
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membaca hasil"})
			return
		}

		results := make([]map[string]interface{}, 0)
		for rows.Next() {
			colsPtrs := make([]interface{}, len(cols))
			colsVals := make([]sql.NullString, len(cols))
			for i := range colsPtrs {
				colsPtrs[i] = &colsVals[i]
			}
			if err := rows.Scan(colsPtrs...); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membaca baris hasil"})
				return
			}
			rowMap := make(map[string]interface{})
			for i, col := range cols {
				if colsVals[i].Valid {
					rowMap[col] = colsVals[i].String
				} else {
					rowMap[col] = nil
				}
			}
			results = append(results, rowMap)
		}
		c.JSON(http.StatusOK, gin.H{"data": results})
	}
}
