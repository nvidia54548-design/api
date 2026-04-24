package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return ""
	}
	var msgs []string
	for _, e := range v {
		msgs = append(msgs, e.Field+": "+e.Message)
	}
	return strings.Join(msgs, "; ")
}

// Validator provides validation methods
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new Validator
func NewValidator() *Validator {
	return &Validator{errors: make(ValidationErrors, 0)}
}

// AddError adds a validation error
func (v *Validator) AddError(field, message string) {
	v.errors = append(v.errors, ValidationError{Field: field, Message: message})
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors returns all validation errors
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

// Required validates that a string is not empty
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "is required")
	}
	return v
}

// MinLength validates minimum string length
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.AddError(field, "must be at least "+string(rune('0'+min))+" characters")
	}
	return v
}

// MaxLength validates maximum string length
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.AddError(field, "must be at most "+string(rune('0'+max))+" characters")
	}
	return v
}

// Email validates email format
func (v *Validator) Email(field, value string) *Validator {
	if value == "" {
		return v // Skip if empty, use Required for mandatory check
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		v.AddError(field, "must be a valid email address")
	}
	return v
}

// NIS validates NIS (Nomor Induk Siswa) format
func (v *Validator) NIS(field, value string) *Validator {
	if value == "" {
		return v
	}
	// NIS should be numeric and between 5-20 characters
	nisRegex := regexp.MustCompile(`^[0-9]{5,20}$`)
	if !nisRegex.MatchString(value) {
		v.AddError(field, "must be a valid NIS (5-20 digits)")
	}
	return v
}

// NIP validates NIP (Nomor Induk Pegawai) format
func (v *Validator) NIP(field, value string) *Validator {
	if value == "" {
		return v
	}
	// NIP format: 18 digits (yyyymmdd yyyymm x xxx)
	nipRegex := regexp.MustCompile(`^[0-9]{18}$`)
	if !nipRegex.MatchString(value) {
		v.AddError(field, "must be a valid NIP (18 digits)")
	}
	return v
}

// AlphaNumeric validates that a string contains only alphanumeric characters
func (v *Validator) AlphaNumeric(field, value string) *Validator {
	if value == "" {
		return v
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			v.AddError(field, "must contain only letters and numbers")
			return v
		}
	}
	return v
}

// InList validates that a value is in a list of allowed values
func (v *Validator) InList(field, value string, allowed []string) *Validator {
	if value == "" {
		return v
	}
	for _, a := range allowed {
		if value == a {
			return v
		}
	}
	v.AddError(field, "must be one of: "+strings.Join(allowed, ", "))
	return v
}

// Gender validates gender value (L/P)
func (v *Validator) Gender(field, value string) *Validator {
	if value == "" {
		return v
	}
	allowed := []string{"L", "P"}
	for _, a := range allowed {
		if value == a {
			return v
		}
	}
	v.AddError(field, "must be L or P")
	return v
}

// Role validates account role
func (v *Validator) Role(field, value string) *Validator {
	if value == "" {
		return v
	}
	allowed := []string{"admin", "guru", "siswa"}
	return v.InList(field, value, allowed)
}

// TipeStaff validates staff type
func (v *Validator) TipeStaff(field, value string) *Validator {
	if value == "" {
		return v
	}
	allowed := []string{"admin", "guru"}
	return v.InList(field, value, allowed)
}

// ClassStatus validates student class status
func (v *Validator) ClassStatus(field, value string) *Validator {
	if value == "" {
		return v
	}
	allowed := []string{"active", "inactive", "graduated", "transferred"}
	return v.InList(field, value, allowed)
}

// Tingkatan validates class level
func (v *Validator) Tingkatan(field string, value int) *Validator {
	if value < 10 || value > 12 {
		v.AddError(field, "must be 10, 11, or 12")
	}
	return v
}

// NamaJenis validates prayer type name
func (v *Validator) NamaJenis(field, value string) *Validator {
	if value == "" {
		return v
	}
	allowed := []string{"Subuh", "Dzuhur", "Ashar", "Maghrib", "Isya", "Dhuha", "Jumat"}
	return v.InList(field, value, allowed)
}

// AbsensiStatus validates attendance status
func (v *Validator) AbsensiStatus(field, value string) *Validator {
	if value == "" {
		return v
	}
	allowed := []string{"hadir", "izin", "sakit", "alpha"}
	return v.InList(field, value, allowed)
}

// Semester validates semester number
func (v *Validator) Semester(field string, value int) *Validator {
	if value != 1 && value != 2 {
		v.AddError(field, "must be 1 or 2")
	}
	return v
}

// Hari validates day of week
func (v *Validator) Hari(field, value string) *Validator {
	if value == "" {
		return v
	}
	allowed := []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Ahad"}
	return v.InList(field, value, allowed)
}

// Tahun validates academic year format
func (v *Validator) Tahun(field, value string) *Validator {
	if value == "" {
		return v
	}
	tahunRegex := regexp.MustCompile(`^\d{4}/\d{4}$`)
	if !tahunRegex.MatchString(value) {
		v.AddError(field, "must match YYYY/YYYY format")
	}
	return v
}

// DateRange validates that start date is before end date
func (v *Validator) DateRange(startField, endField, startDate, endDate string) *Validator {
	if startDate == "" || endDate == "" {
		return v
	}
	if startDate >= endDate {
		v.AddError(endField, "must be after "+startField)
	}
	return v
}

// TimeRange validates that start time is before end time
func (v *Validator) TimeRange(startField, endField, startTime, endTime string) *Validator {
	if startTime == "" || endTime == "" {
		return v
	}
	if startTime >= endTime {
		v.AddError(endField, "must be after "+startField)
	}
	return v
}

// Date validates date format (YYYY-MM-DD)
func (v *Validator) Date(field, value string) *Validator {
	if value == "" {
		return v
	}
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if !dateRegex.MatchString(value) {
		v.AddError(field, "must be in YYYY-MM-DD format")
	}
	return v
}

// Time validates time format (HH:MM or HH:MM:SS)
func (v *Validator) Time(field, value string) *Validator {
	if value == "" {
		return v
	}
	timeRegex := regexp.MustCompile(`^([01]?[0-9]|2[0-3]):[0-5][0-9](:[0-5][0-9])?$`)
	if !timeRegex.MatchString(value) {
		v.AddError(field, "must be in HH:MM or HH:MM:SS format")
	}
	return v
}

// PositiveInt validates that an integer is positive
func (v *Validator) PositiveInt(field string, value int) *Validator {
	if value <= 0 {
		v.AddError(field, "must be a positive number")
	}
	return v
}

// Range validates that an integer is within a range
func (v *Validator) Range(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.AddError(field, "must be between specified range")
	}
	return v
}

// Username validates username format
func (v *Validator) Username(field, value string) *Validator {
	if value == "" {
		return v
	}
	// Username: 3-50 chars, alphanumeric and underscore
	usernameRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,49}$`)
	if !usernameRegex.MatchString(value) {
		v.AddError(field, "must start with a letter and contain only letters, numbers, and underscores (3-50 chars)")
	}
	return v
}

// Password validates password using the existing ValidatePassword function
func (v *Validator) Password(field, value string) *Validator {
	if err := ValidatePassword(value); err != nil {
		v.AddError(field, err.Error())
	}
	return v
}

// ValidateNIS is a standalone validation function for NIS
func ValidateNIS(nis string) error {
	v := NewValidator()
	v.Required("nis", nis).NIS("nis", nis)
	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// ValidateEmail is a standalone validation function for email
func ValidateEmail(email string) error {
	v := NewValidator()
	v.Required("email", email).Email("email", email)
	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// ValidateUsername is a standalone validation function for username
func ValidateUsername(username string) error {
	v := NewValidator()
	v.Required("username", username).Username("username", username)
	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}
