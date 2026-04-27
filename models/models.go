package models

import (
	"time"
)

type Account struct {
	ID        int        `gorm:"primaryKey;column:id" json:"id"`
	Email     string     `gorm:"column:email;size:255;not null;unique" json:"email"`
	Password  string     `gorm:"column:password;size:255;not null" json:"-"`
	Role      string     `gorm:"column:role;size:20;not null" json:"role"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (Account) TableName() string {
	return "accounts"
}

type Staff struct {
	IDStaff   int        `gorm:"primaryKey;column:id_staff" json:"id_staff"`
	IDAccount int        `gorm:"column:id_account;not null;unique" json:"id_account"`
	NIP       *string    `gorm:"column:nip;size:20;unique" json:"nip,omitempty"`
	Nama      string     `gorm:"column:nama;size:255;not null" json:"nama"`
	TipeStaff string     `gorm:"column:tipe_staff;size:10;not null" json:"tipe_staff"`
	DeletedAt *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
	Account   *Account   `gorm:"foreignKey:IDAccount;references:ID" json:"-"`
}

func (Staff) TableName() string {
	return "staff"
}

type Siswa struct {
	IDSiswa         int        `gorm:"primaryKey;column:id_siswa" json:"id_siswa"`
	IDAccount       int        `gorm:"column:id_account;not null;unique" json:"id_account"`
	NIS             string     `gorm:"column:nis;size:20;not null;unique" json:"nis"`
	NamaSiswa       string     `gorm:"column:nama_siswa;not null" json:"nama_siswa"`
	JK              string     `gorm:"column:jk;type:char(1);not null" json:"jk"`
	Kelas           string     `gorm:"column:kelas;size:50" json:"kelas,omitempty"`
	Jurusan         string     `gorm:"column:jurusan;size:20" json:"jurusan,omitempty"`
	Part            string     `gorm:"-" json:"part,omitempty"`
	AcademicYear    string     `gorm:"column:academic_year;size:9" json:"academic_year,omitempty"`
	CurrentSemester int        `gorm:"column:current_semester" json:"current_semester,omitempty"`
	IDKelas         *int       `gorm:"column:id_kelas" json:"id_kelas,omitempty"`
	IDTahunMasuk    *int       `gorm:"column:id_tahun_masuk" json:"id_tahun_masuk,omitempty"`
	ClassStatus     string     `gorm:"column:class_status;default:'active'" json:"class_status"`
	LastPromotionAt *time.Time `gorm:"column:last_promotion_at" json:"last_promotion_at,omitempty"`
	IsRegistered    bool       `gorm:"column:is_registered;default:false" json:"is_registered"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
	Account         *Account   `gorm:"foreignKey:IDAccount;references:ID" json:"-"`
	KelasRef        *Kelas     `gorm:"foreignKey:IDKelas;references:IDKelas" json:"-"`
	TahunMasuk      *TahunMasuk `gorm:"foreignKey:IDTahunMasuk;references:IDTahunMasuk" json:"-"`
}

func (Siswa) TableName() string {
	return "siswa"
}

type Kelas struct {
	IDKelas   int        `gorm:"primaryKey;column:id_kelas" json:"id_kelas"`
	Tingkatan int        `gorm:"column:tingkatan;not null" json:"tingkatan"`
	Jurusan   string     `gorm:"column:jurusan;size:20;not null" json:"jurusan"`
	Part      string     `gorm:"column:part;size:3;not null" json:"part"`
	DeletedAt *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (Kelas) TableName() string {
	return "kelas"
}

type WaliKelas struct {
	IDWali      int       `gorm:"primaryKey;column:id_wali" json:"id_wali"`
	IDKelas     int       `gorm:"column:id_kelas;not null;unique" json:"id_kelas"`
	IDStaff     int       `gorm:"column:id_staff;not null" json:"id_staff"`
	IsActive    bool      `gorm:"column:is_active;not null" json:"is_active"`
	BerlakuMulai time.Time `gorm:"column:berlaku_mulai;type:date;not null" json:"berlaku_mulai"`
	Kelas       *Kelas    `gorm:"foreignKey:IDKelas;references:IDKelas" json:"-"`
	Staff       *Staff    `gorm:"foreignKey:IDStaff;references:IDStaff" json:"-"`
}

func (WaliKelas) TableName() string {
	return "wali_kelas"
}

type TahunMasuk struct {
	IDTahunMasuk int       `gorm:"primaryKey;column:id_tahun_masuk" json:"id_tahun_masuk"`
	Tahun        string    `gorm:"column:tahun;size:9;not null;unique" json:"tahun"`
	IsActive     bool      `gorm:"column:is_active;not null" json:"is_active"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (TahunMasuk) TableName() string {
	return "tahun_masuk"
}

type SemesterAkademik struct {
	IDSemester    int       `gorm:"primaryKey;column:id_semester" json:"id_semester"`
	IDTahun       int       `gorm:"column:id_tahun;not null" json:"id_tahun"`
	Semester      int       `gorm:"column:semester;not null" json:"semester"`
	TanggalMulai  time.Time `gorm:"column:tanggal_mulai;type:date;not null" json:"tanggal_mulai"`
	TanggalSelesai time.Time `gorm:"column:tanggal_selesai;type:date;not null" json:"tanggal_selesai"`
	TahunMasuk    *TahunMasuk `gorm:"foreignKey:IDTahun;references:IDTahunMasuk" json:"-"`
}

func (SemesterAkademik) TableName() string {
	return "semester_akademik"
}

type JenisSholat struct {
	IDJenis      int    `gorm:"primaryKey;column:id_jenis" json:"id_jenis"`
	NamaJenis    string `gorm:"column:nama_jenis;size:30;not null" json:"nama_jenis"`
	ButuhGiliran bool   `gorm:"column:butuh_giliran;not null" json:"butuh_giliran"`
}

func (JenisSholat) TableName() string {
	return "jenis_sholat"
}

type JadwalSholat struct {
	IDJadwal     int        `gorm:"primaryKey;column:id_jadwal" json:"id_jadwal"`
	Hari         string     `gorm:"column:hari;size:10;not null" json:"hari"`
	JenisSholat  string     `gorm:"column:jenis_sholat;size:30;not null" json:"jenis_sholat"`
	WaktuMulai   string     `gorm:"column:waktu_mulai;type:time;not null" json:"waktu_mulai"`
	WaktuSelesai string     `gorm:"column:waktu_selesai;type:time;not null" json:"waktu_selesai"`
	Jurusan      string     `gorm:"column:jurusan;size:20" json:"jurusan,omitempty"`
	Kelas        string     `gorm:"column:kelas;size:50" json:"kelas,omitempty"`
	DeletedAt    *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (JadwalSholat) TableName() string {
	return "jadwal_sholat"
}

type JadwalSholatTemplate struct {
	IDTemplate int          `gorm:"primaryKey;column:id_template" json:"id_template"`
	Hari       string       `gorm:"column:hari;size:10;not null" json:"hari"`
	IDJenis    int          `gorm:"column:id_jenis;not null" json:"id_jenis"`
	JenisSholat *JenisSholat `gorm:"foreignKey:IDJenis;references:IDJenis" json:"-"`
}

func (JadwalSholatTemplate) TableName() string {
	return "jadwal_sholat_template"
}

type GiliranDhuha struct {
	IDGiliran int    `gorm:"primaryKey;column:id_giliran" json:"id_giliran"`
	Jurusan   string `gorm:"column:jurusan;size:20;not null" json:"jurusan"`
	Hari      string `gorm:"column:hari;size:10;not null" json:"hari"`
}

func (GiliranDhuha) TableName() string {
	return "giliran_dhuha"
}

type WaktuSholat struct {
	IDWaktu       int        `gorm:"primaryKey;column:id_waktu" json:"id_waktu"`
	IDJenis       int        `gorm:"column:id_jenis;not null" json:"id_jenis"`
	WaktuMulai    string     `gorm:"column:waktu_mulai;type:time;not null" json:"waktu_mulai"`
	WaktuSelesai  string     `gorm:"column:waktu_selesai;type:time;not null" json:"waktu_selesai"`
	BerlakuMulai  time.Time  `gorm:"column:berlaku_mulai;type:date;not null" json:"berlaku_mulai"`
	BerlakuSampai *time.Time `gorm:"column:berlaku_sampai;type:date" json:"berlaku_sampai,omitempty"`
	JenisSholat   *JenisSholat `gorm:"foreignKey:IDJenis;references:IDJenis" json:"-"`
}

func (WaktuSholat) TableName() string {
	return "waktu_sholat"
}

type Absensi struct {
	IDAbsen    int                   `gorm:"primaryKey;column:id_absen" json:"id_absen"`
	IDSiswa    int                   `gorm:"column:id_siswa;not null" json:"id_siswa"`
	IDSemester int                   `gorm:"column:id_semester;not null" json:"id_semester"`
	IDTemplate int                   `gorm:"column:id_template;not null" json:"id_template"`
	IDGiliran  *int                  `gorm:"column:id_giliran" json:"id_giliran,omitempty"`
	Tanggal    time.Time             `gorm:"column:tanggal;type:date;not null" json:"tanggal"`
	Status     string                `gorm:"column:status;not null" json:"status"`
	Siswa      *Siswa                `gorm:"foreignKey:IDSiswa;references:IDSiswa" json:"siswa,omitempty"`
	Semester   *SemesterAkademik     `gorm:"foreignKey:IDSemester;references:IDSemester" json:"semester,omitempty"`
	Template   *JadwalSholatTemplate `gorm:"foreignKey:IDTemplate;references:IDTemplate" json:"template,omitempty"`
	Giliran    *GiliranDhuha         `gorm:"foreignKey:IDGiliran;references:IDGiliran" json:"giliran,omitempty"`
}

func (Absensi) TableName() string {
	return "absensi"
}

type RekapAbsensi struct {
	IDRekap      int             `gorm:"primaryKey;column:id_rekap" json:"id_rekap"`
	IDSiswa      int             `gorm:"column:id_siswa;not null" json:"id_siswa"`
	IDSemester   int             `gorm:"column:id_semester;not null" json:"id_semester"`
	IDJenis      int             `gorm:"column:id_jenis;not null" json:"id_jenis"`
	JumlahHadir  int             `gorm:"column:jumlah_hadir;default:0;not null" json:"jumlah_hadir"`
	JumlahIzin   int             `gorm:"column:jumlah_izin;default:0;not null" json:"jumlah_izin"`
	JumlahSakit  int             `gorm:"column:jumlah_sakit;default:0;not null" json:"jumlah_sakit"`
	JumlahAlpha  int             `gorm:"column:jumlah_alpha;default:0;not null" json:"jumlah_alpha"`
	Siswa        *Siswa          `gorm:"foreignKey:IDSiswa;references:IDSiswa" json:"siswa,omitempty"`
	Semester     *SemesterAkademik `gorm:"foreignKey:IDSemester;references:IDSemester" json:"semester,omitempty"`
	JenisSholat  *JenisSholat    `gorm:"foreignKey:IDJenis;references:IDJenis" json:"jenis_sholat,omitempty"`
}

func (RekapAbsensi) TableName() string {
	return "rekap_absensi"
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