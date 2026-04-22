# SASMobile - Student Attendance System

SASMobile is a high-fidelity digital ecosystem designed to track student prayer attendance (Dhuha, Dzuhur, and Jumat) at SMK Negeri 2 Singosari. The system consists of a Kotlin-based Android application and a Go-based REST API, ensuring real-time synchronization and professional reporting.

## 1. Project Overview

### Purpose
To modernize the manual attendance process during congregational prayers, providing accuracy for Teachers (Guru) and transparency for Students (Siswa).

### User Roles
- **Admin**: Full system management, including student data, schedules, and global reports.
- **Guru / Wali Kelas**: Monitor class-specific attendance and export monthly reports.
- **Student (Siswa)**: View personal attendance history and scan QR codes for attendance (if applicable).

---

## 2. File Structure & Architecture

### Mobile Architecture (Android Native - Kotlin)
The mobile app follows a traditional Android architecture geared toward high-fidelity UI rendering and robust data handling.

```text
com.xirpl2.SASMobile
├── adapter/                      # RecyclerView adapters (Notifikasi, Riwayat)
├── model/                        # Data Transfer Objects (DTOs) & Presence models
├── ui/                           # (Merged in root) Activity-based UI
│   ├── BerandaAdminActivity.kt   # Admin Dashboard
│   ├── DataSiswaAdminActivity.kt # Student List Management
│   ├── DetailAbsensiActivity.kt  # Attendance Detail View
│   └── MasukActivity.kt          # Authentication Entry Point
├── PresenceDetailPopUpFragment.kt# High-fidelity Modal Dialogs
├── JadwalSholatHelper.kt         # Logic for prayer time calculations
└── SASMobileApp.kt               # Application entry & DI setup
```

### Backend Architecture (Golang - Gin)
The backend uses a clean, folder-based structure for scalability and clear separation of concerns.

```text
absensisholat-api-agent
├── cmd/                          # Application entry points
├── handlers/                     # API Handlers (logic for Auth, Absensi, Export)
├── middleware/                   # JWT Auth, Rate Limiting, & Security
├── migrations/                   # SQL Migration files for schema versioning
├── models/                       # GORM/Database model definitions
├── routes/                       # Route registration (v1, Auth, Export)
├── scheduler/                    # Automated tasks (Auto-Alpha Dzuhur)
└── utils/                        # Helpers (JWT, Timezone, Missed Prayer logic)
```

---

## 3. Application Workflow

### Login & JWT Persistence
1.  **Authentication**: User provides credentials via `MasukActivity`.
2.  **Token Issuance**: Backend issues an **Access Token** (15 mins) and a **Refresh Token** (7 days).
3.  **Persistence**: Tokens are stored securely using `EncryptedSharedPreferences` on Android.
4.  **Auto-Refresh**: Mobile client uses interceptors to refresh the Access Token using the Refresh Token when 401 Unauthorized is detected.

### Attendance Dashboard & Detail Pop-up
-   **Dashboard**: Shows real-time summaries for the current day.
-   **Pop-up Logic**: Clicking a student opens a `PresenceDetailPopUpFragment`.
    -   **Real-time Fetch**: Fetches monthly statistics directly from the backend.
    -   **Pill Cards**: Displays "Hadir", "Izin", "Sakit" as vertical pill capsules (80x130dp).

### Auto-Alpha Dzuhur Cron Job
1.  **Trigger**: Runs automatically at 13:30 (after the Dzuhur prayer window).
2.  **Logic**: Identifies students with no attendance record for "Dzuhur" on the current day.
3.  **Friday Exception**: For male students, it checks for "Jumat" attendance instead.
4.  **Update**: Bulk inserts "ALPHA" status for all missing records.

---

## 4. Database Schema

### Entity Relationship
- **users_staff** (1:1) **admin** / **guru**
- **siswa** (1:1) **akun_login_siswa**
- **siswa** (1:N) **absensi**
- **jadwal_sholat** (1:N) **absensi**

### Table Definitions
| Table | Description | Key Fields |
| :--- | :--- | :--- |
| `siswa` | Core student data | `nis` (PK), `nama_siswa`, `jk`, `jurusan` |
| `users_staff` | Credentials for Staff | `username`, `password`, `role` |
| `jadwal_sholat` | Weekly prayer schedule | `hari`, `jenis_sholat`, `waktu_mulai`, `jurusan` |
| `absensi` | Transactional attendance | `id_absen`, `nis`, `status` (HADIR/ALPHA/IZIN), `tanggal` |

---

## 5. API Documentation

### Scalar API Reference (Development)
- Scalar docs are enabled automatically when `ENVIRONMENT` is not `production`.
- Start the API locally, then open:
    - `http://localhost:8080/docs` for the Scalar UI.
    - `http://localhost:8080/openapi.json` for the generated OpenAPI document.
- The OpenAPI spec is generated from all registered Gin routes (`/api/v1`, `/api/v2`, and legacy `/api`) so route coverage stays in sync during development.

### Authentication Endpoints
#### `POST /api/v1/siswa/login`
- **Body**: `{"email": "...", "password": "..."}`
- **Response**: `200 OK` with `token` and `refresh_token`.

#### `POST /api/v1/auth/refresh`
- **Header**: `Authorization: <refresh_token>`
- **Response**: New access token pair.

### Attendance & Reporting
#### `GET /api/v1/absensi/history/{nis}`
- **Response**: List of attendance records filtered by student ID.

#### `GET /api/v1/export/full-report`
- **Params**: `month=YYYY-MM`, `jurusan=...`
- **Response**: `.xlsx` file with two sheets (Laporan_Absensi, Jadwal_Sholat_Referensi).

---

## 6. Security Implementation

-   **JWT Signing**: All tokens are signed with `HMAC-SHA256` using a 32-character secret.
-   **Refresh Token Rotation**: Refresh tokens expire after 7 days, forcing re-authentication for inactive sessions.
-   **Encryption**: Student passwords are hashed using `bcrypt` (cost: 10).
-   **Android Security**:
    -   All API calls require **HTTPS** (Cleartext Traffic disabled in Manifest).
    -   JWT stored in **EncryptedSharedPreferences** to prevent extraction from rooted devices.
-   **Rate Limiting**: Backend implements middleware to prevent brute-force attacks on login endpoints.
