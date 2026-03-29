# Dokumentasi Teknis SASMobile (Student Attendance System)

Dokumentasi ini memberikan panduan menyeluruh mengenai arsitektur, keamanan, dan logika bisnis sistem absensi sholat di SMK Negeri 2 Singosari.

---

## I. Arsitektur Folder & File

### 1. Android Native (Kotlin) - `com.xirpl2.SASMobile`
Aplikasi mobile menggunakan pendekatan Activity-based dengan pemisahan komponen UI, Model, dan Adapter.

```text
com.xirpl2.SASMobile
├── adapter/
│   ├── NotifikasiAdapter.kt       # Menangani list notifikasi
│   ├── RiwayatAbsensiAdapter.kt   # List sejarah absensi siswa
│   └── PrayerDataAdapter.kt       # Data sholat pada pop-up detail
├── model/
│   ├── Siswa.kt                   # Model data profil siswa
│   ├── Absensi.kt                 # Model transaksi absensi
│   └── JadwalSholat.kt            # Model jadwal harian
├── PresenceDetailPopUpFragment.kt # DialogFragment untuk detail absensi (High-Fidelity)
├── MasukActivity.kt               # Entry point untuk login & manajemen session
├── BerandaActivity.kt             # Dashboard utama (Siswa)
├── DataSiswaAdminActivity.kt     # Manajemen data siswa (Admin)
└── JadwalSholatHelper.kt          # Utilitas logika waktu & jenis sholat
```

### 2. Backend (Golang - Gin)
Backend dirancang dengan modularitas tinggi berbasis fungsionalitas.

```text
absensisholat-api-agent
├── api/
│   └── index.go                   # Integrasi Serverless/Entry point
├── handlers/
│   ├── auth.go                    # Registrasi, Login, & Refresh Token
│   ├── absensi.go                 # Pencatatan & Riwayat history
│   └── export.go                  # Engine Export Excel (Excelize)
├── middleware/
│   ├── auth.go                    # Validasi JWT & Role Check
│   └── security.go                # Header keamanan & CORS
├── models/
│   └── models.go                  # Definisi schema database (GORM)
├── scheduler/
│   └── missed_prayer.go           # Background task untuk Auto-Alpha
├── routes/
│   └── Routes.go                  # Registrasi seluruh endpoint API
└── utils/
    ├── jwt.go                     # Sign & Verify JWT
    └── time_utils.go              # Manajemen timezone Asia/Jakarta
```

---

## II. Alur Kerja Autentikasi (JWT Security)

Sistem menggunakan standar **JSON Web Token (JWT)** dengan mekanisme rotasi untuk menjaga keamanan sesi tanpa mengorbankan kenyamanan pengguna.

1.  **Login Awal**: Saat user login, sistem menghasilkan sepasang token:
    *   **Access Token**: Berlaku selama 15 menit (untuk transaksi API).
    *   **Refresh Token**: Berlaku selama 7 hari (untuk mendapatkan access token baru).
2.  **Penyimpanan**: Token disimpan di Android menggunakan **SharedPreferences** (`UserData` dan `user_session`). Sesuai standar keamanan, data NIS dan Role dienkripsi dalam payload JWT menggunakan HS256.
3.  **Proses Refresh**:
    *   Setiap kali Access Token expired (Error 401), apliaksi mobile secara otomatis mengirimkan Refresh Token ke endpoint `/auth/refresh`.
    *   Backend memvalidasi Refresh Token di database (`refresh_tokens` table). Jika valid, Access Token baru diberikan tanpa memaksa user login ulang.

---

## III. Logika Bisnis Spesifik

### 1. Filter Jadwal Dhuha (Student Major Based)
Jadwal Dhuha tidak muncul secara pukul rata untuk semua siswa. Sistem melakukan pengecekan berdasarkan kolom `jurusan`:
-   Saat fetching jadwal, backend melakukan query: `WHERE (jurusan = ? OR jurusan = 'Semua Jurusan')`.
-   Jika jadwal Dhuha untuk jurusan "PPLG" belum waktunya atau tidak tersedia, maka item Dhuha tidak akan muncul di Beranda siswa tersebut.

### 2. Logika Hari Jumat (Dynamic Substitution)
Sistem secara otomatis menyesuaikan jenis sholat di hari Jumat berdasarkan jenis kelamin:
-   **Laki-laki**: Sholat **Dzuhur** digantikan oleh **Jumat**.
-   **Perempuan**: Tetap melaksanakan Sholat **Dzuhur**.
-   Logika ini diterapkan secara konsisten baik di Mobile (`JadwalSholatHelper.kt`) maupun di Backend (`RecordMissedPrayers`).

---

## IV. Dokumentasi API (Endpoint List)

| Method | Endpoint | Deskripsi | Status |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/auth/login` | Autentikasi user & pemberian token | Public |
| `POST` | `/api/v1/auth/refresh` | Pembaruan access token (Refresh Token rot) | Public |
| `GET` | `/api/v1/siswa/history/{nis}` | Mengambil detail sejarah absensi siswa | Protected |
| `GET` | `/api/v1/export/full-report` | Download laporan Excel multi-sheet | Admin/Guru |

### Contoh Request Login
```json
{
  "email": "siswa@smkn2.id",
  "password": "Password123!"
}
```

### Contoh Response Sukses
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "f7b8c9d0...",
  "role": "siswa",
  "name": "Budi Santoso"
}
```

---

## V. Panduan Pemeliharaan (Maintenance)

### Mekanisme Auto-Alpha (Cron Job)
Untuk memastikan data kehadiran akurat, sistem menjalankan **Background Worker** di Go yang dipicu setiap hari pada pukul **13:30 WIB**:

1.  **Scanning**: Sistem mencari seluruh siswa yang terdaftar di database namun belum memiliki record di tabel `absensi` untuk tanggal hari ini.
2.  **Conditional Check**: Mengecek apakah hari ini Jumat untuk menentukan record "ALPHA" yang akan dimasukkan (Dzuhur vs Jumat).
3.  **Bulk Insert**: Sistem memasukkan record dengan status **"ALPHA"** secara otomatis.
4.  **Logging**: Proses ini dicatat dalam log server (`utils.RecordMissedPrayers`) untuk keperluan audit jika ada siswa yang merasa sudah hadir namun terhitung alpha.
