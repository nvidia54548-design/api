package models

import (
	"time"
)

type Account struct {
	ID        int       `gorm:"primaryKey;column:id" json:"id"`
	Email     string    `gorm:"column:email;size:255;not null;unique" json:"email"`
	Password  string    `gorm:"column:password;size:255;not null" json:"-"`
	Role      string    `gorm:"column:role;size:20;not null" json:"role"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (Account) TableName() string {
	return "accounts"
}

type Kelas struct {
	IDKelas    int    `gorm:"primaryKey;column:id_kelas" json:"id_kelas"`
	Tingkatan  int    `gorm:"column:tingkatan;not null" json:"tingkatan"`
	Jurusan    string `gorm:"column:jurusan;size:20;not null" json:"jurusan"`
	Part       string `gorm:"column:part;size:3;not null" json:"part"`
}

func (Kelas) TableName() string {
	return "kelas"
}

type Siswa struct {
	IDSiswa          int        `gorm:"primaryKey;column:id_siswa" json:"id_siswa"`
	IDAccount        int        `gorm:"column:id_account;not null" json:"id_account"`
	NIS              string     `gorm:"column:nis;size:20;not null;unique" json:"nis"`
	NamaSiswa        string     `gorm:"column:nama_siswa;not null" json:"nama_siswa"`
	JK               string     `gorm:"column:jk;type:char(1);not null" json:"jk"`
	IDKelas          int        `gorm:"column:id_kelas;not null" json:"id_kelas"`
	AcademicYear     string     `gorm:"column:academic_year" json:"academic_year"`
	CurrentSemester  int        `gorm:"column:current_semester" json:"current_semester"`
	ClassStatus      string     `gorm:"column:class_status" json:"class_status"`
	LastPromotionAt  *time.Time `gorm:"column:last_promotion_at" json:"last_promotion_at,omitempty"`
	Account          *Account   `gorm:"foreignKey:IDAccount;references:ID" json:"-"`
	KelasRef         *Kelas     `gorm:"foreignKey:IDKelas;references:IDKelas" json:"-"`
	Jurusan          string     `gorm:"column:jurusan" json:"jurusan,omitempty"`
	Kelas            string     `gorm:"column:kelas" json:"kelas,omitempty"`
	Part             string     `gorm:"-" json:"part,omitempty"`
}

func (Siswa) TableName() string {
	return "siswa"
}

type JadwalSholat struct {
	IDJadwal     int       `gorm:"primaryKey;column:id_jadwal" json:"id_jadwal"`
	Hari         string    `gorm:"not null" json:"hari"`
	JenisSholat  string    `gorm:"not null;column:jenis_sholat" json:"jenis_sholat"`
	WaktuMulai   string    `gorm:"column:waktu_mulai;type:time" json:"waktu_mulai"`
	WaktuSelesai string    `gorm:"column:waktu_selesai;type:time" json:"waktu_selesai"`
	IDKelas      int       `gorm:"column:id_kelas;not null" json:"id_kelas"`
	Jurusan      string    `gorm:"column:jurusan" json:"jurusan,omitempty"`
	Kelas        string    `gorm:"column:kelas" json:"kelas,omitempty"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	KelasRef     *Kelas    `gorm:"foreignKey:IDKelas;references:IDKelas" json:"-"`
}

func (JadwalSholat) TableName() string {
	return "jadwal_sholat"
}

type Absensi struct {
	IDAbsen   int       `gorm:"primaryKey;column:id_absen" json:"id_absen"`
	IDSiswa   int       `gorm:"not null;column:id_siswa" json:"id_siswa"`
	IDJadwal  int       `gorm:"not null;column:id_jadwal" json:"id_jadwal"`
	Tanggal   time.Time `gorm:"type:date;not null" json:"tanggal"`
	Status    string    `gorm:"not null" json:"status"`
	Deskripsi string    `json:"deskripsi"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	Siswa     *Siswa    `gorm:"foreignKey:IDSiswa;references:IDSiswa" json:"siswa,omitempty"`
	NIS       string    `gorm:"column:nis" json:"nis,omitempty"`
}

func (Absensi) TableName() string {
	return "absensi"
}

type Staff struct {
	IDStaff   int       `gorm:"primaryKey;column:id_staff" json:"id_staff"`
	IDAccount int       `gorm:"column:id_account;not null;unique" json:"id_account"`
	NIP       string    `gorm:"column:nip;size:20" json:"nip,omitempty"`
	Nama      string    `gorm:"column:nama;size:255;not null" json:"nama"`
	TipeStaff string    `gorm:"column:tipe_staff;size:10;not null" json:"tipe_staff"`
	Account   *Account  `gorm:"foreignKey:IDAccount;references:ID" json:"-"`
}

func (Staff) TableName() string {
	return "staff"
}

type WaliKelas struct {
	IDWali   int    `gorm:"primaryKey;column:id_wali" json:"id_wali"`
	IDKelas  int    `gorm:"column:id_kelas;not null;unique" json:"id_kelas"`
	IDStaff  int    `gorm:"column:id_staff;not null" json:"id_staff"`
	KelasRef *Kelas `gorm:"foreignKey:IDKelas;references:IDKelas" json:"-"`
	StaffRef *Staff `gorm:"foreignKey:IDStaff;references:IDStaff" json:"-"`
}

func (WaliKelas) TableName() string {
	return "wali_kelas"
}

type StudentClassTransition struct {
	ID               int       `gorm:"primaryKey;column:id" json:"id"`
	IDSiswa          int       `gorm:"column:id_siswa;not null" json:"id_siswa"`
	Action           string    `gorm:"column:action;size:30;not null" json:"action"`
	FromClass        *int      `gorm:"column:from_class" json:"from_class,omitempty"`
	ToClass          *int      `gorm:"column:to_class" json:"to_class,omitempty"`
	FromAcademicYear string    `gorm:"column:from_academic_year;size:9" json:"from_academic_year,omitempty"`
	ToAcademicYear   string    `gorm:"column:to_academic_year;size:9" json:"to_academic_year,omitempty"`
	FromSemester     *int      `gorm:"column:from_semester" json:"from_semester,omitempty"`
	ToSemester       *int      `gorm:"column:to_semester" json:"to_semester,omitempty"`
	FromStatus       string    `gorm:"column:from_status;size:30" json:"from_status,omitempty"`
	ToStatus         string    `gorm:"column:to_status;size:30" json:"to_status,omitempty"`
	DecidedBy        *int      `gorm:"column:decided_by" json:"decided_by,omitempty"`
	Note             string    `gorm:"column:note" json:"note,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (StudentClassTransition) TableName() string {
	return "student_class_transitions"
}

type RefreshToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AccountID int       `gorm:"column:account_id;not null" json:"account_id"`
	Token     string    `gorm:"uniqueIndex:idx_token;not null" json:"token"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// Legacy transition models retained to keep old call sites compiling.
type UserStaff struct {
	IDStaff   int       `gorm:"column:id" json:"id_staff"`
	Username  string    `gorm:"column:email" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Role      string    `gorm:"column:role" json:"role"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (UserStaff) TableName() string {
	return "accounts"
}

type Guru struct {
	IDGuru    int    `gorm:"-" json:"id_guru"`
	IDStaff   int    `gorm:"column:id_staff" json:"id_staff"`
	NIP       string `gorm:"column:nip" json:"nip"`
	NamaGuru  string `gorm:"column:nama" json:"nama_guru"`
	KelasWali string `gorm:"-" json:"kelas_wali"`
}

func (Guru) TableName() string {
	return "staff"
}

type Admin struct {
	IDAdmin   int    `gorm:"-" json:"id_admin"`
	IDStaff   int    `gorm:"column:id_staff" json:"id_staff"`
	NamaAdmin string `gorm:"column:nama" json:"nama_admin"`
}

func (Admin) TableName() string {
	return "staff"
}

type AkunLoginSiswa struct {
	NIS       string    `gorm:"-" json:"nis"`
	Password  string    `gorm:"column:password;not null" json:"password"`
	Email     string    `gorm:"column:email;unique;not null" json:"email"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	Siswa     *Siswa    `gorm:"-" json:"-"`
}

func (AkunLoginSiswa) TableName() string {
	return "accounts"
}
