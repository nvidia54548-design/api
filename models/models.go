package models

import (
	"time"
)

type Siswa struct {
	NIS       string `gorm:"primaryKey" json:"nis"`
	NamaSiswa string `gorm:"not null" json:"nama_siswa"`
	JK        string `gorm:"not null" json:"jk"`
	Jurusan   string `json:"jurusan"`
	Kelas     string `json:"kelas"`
	Part      string `json:"part"`
}

func (Siswa) TableName() string {
	return "siswa"
}

type AkunLoginSiswa struct {
	NIS       string    `gorm:"primaryKey" json:"nis"`
	Password  string    `gorm:"not null" json:"password"`
	Email     string    `gorm:"unique;not null" json:"email"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	Siswa     *Siswa    `gorm:"foreignKey:NIS;references:NIS" json:"-"`
}

func (AkunLoginSiswa) TableName() string {
	return "akun_login_siswa"
}

type JadwalSholat struct {
	IDJadwal     int       `gorm:"primaryKey;column:id_jadwal" json:"id_jadwal"`
	Hari         string    `gorm:"not null" json:"hari"`
	JenisSholat  string    `gorm:"not null;column:jenis_sholat" json:"jenis_sholat"`
	WaktuMulai   string    `gorm:"column:waktu_mulai;type:time" json:"waktu_mulai"`
	WaktuSelesai string    `gorm:"column:waktu_selesai;type:time" json:"waktu_selesai"`
	Jurusan      string    `json:"jurusan"`
	Kelas        string    `json:"kelas"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (JadwalSholat) TableName() string {
	return "jadwal_sholat"
}

type Absensi struct {
	IDAbsen   int       `gorm:"primaryKey;column:id_absen" json:"id_absen"`
	NIS       string    `gorm:"not null;column:nis" json:"nis"`
	IDJadwal  int       `gorm:"not null;column:id_jadwal" json:"id_jadwal"`
	Tanggal   time.Time `gorm:"type:date;not null" json:"tanggal"`
	Status    string    `gorm:"not null" json:"status"`
	Deskripsi string    `json:"deskripsi"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	Siswa     *Siswa    `gorm:"foreignKey:NIS;references:NIS" json:"siswa,omitempty"`
}

func (Absensi) TableName() string {
	return "absensi"
}

type UserStaff struct {
	IDStaff   int       `gorm:"primaryKey;column:id_staff" json:"id_staff"`
	Username  string    `gorm:"unique;not null" json:"username"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `gorm:"not null" json:"role"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (UserStaff) TableName() string {
	return "users_staff"
}

type Guru struct {
	IDGuru    int        `gorm:"primaryKey;column:id_guru" json:"id_guru"`
	IDStaff   int        `gorm:"not null;unique" json:"id_staff"`
	NIP       string     `gorm:"not null;unique" json:"nip"`
	NamaGuru  string     `gorm:"not null" json:"nama_guru"`
	KelasWali string     `json:"kelas_wali"`
	Staff     *UserStaff `gorm:"foreignKey:IDStaff;references:IDStaff" json:"staff,omitempty"`
}

func (Guru) TableName() string {
	return "guru"
}

type Admin struct {
	IDAdmin   int        `gorm:"primaryKey;column:id_admin" json:"id_admin"`
	IDStaff   int        `gorm:"not null;unique" json:"id_staff"`
	NamaAdmin string     `gorm:"not null" json:"nama_admin"`
	Staff     *UserStaff `gorm:"foreignKey:IDStaff;references:IDStaff" json:"staff,omitempty"`
}

func (Admin) TableName() string {
	return "admin"
}

type RefreshToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"not null" json:"user_id"` // NIS for siswa, or IDStaff for staff
	Token     string    `gorm:"uniqueIndex:idx_token;not null" json:"token"`
	Role      string    `gorm:"not null" json:"role"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
