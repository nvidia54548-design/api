CREATE SCHEMA IF NOT EXISTS "public";

CREATE TABLE "absensi" (
	"id_absen" serial PRIMARY KEY,
	"id_siswa" integer NOT NULL,
	"id_semester" integer NOT NULL,
	"id_template" integer NOT NULL,
	"id_giliran" integer,
	"tanggal" date NOT NULL,
	"status" varchar(15) NOT NULL,
	CONSTRAINT "absensi_id_siswa_tanggal_id_template_key" UNIQUE("id_siswa","tanggal","id_template"),
	CONSTRAINT "absensi_status_check" CHECK (((status)::text = ANY ((ARRAY['hadir'::character varying, 'izin'::character varying, 'sakit'::character varying, 'alpha'::character varying])::text[])))
);

CREATE TABLE "accounts" (
	"id" serial PRIMARY KEY,
	"email" varchar(255) NOT NULL,
	"password" varchar(255) NOT NULL,
	"role" varchar(20) NOT NULL,
	"created_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	"updated_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	"deleted_at" timestamp,
	CONSTRAINT "accounts_email_key" UNIQUE("email"),
	CONSTRAINT "accounts_role_check" CHECK (((role)::text = ANY ((ARRAY['admin'::character varying, 'guru'::character varying, 'siswa'::character varying])::text[])))
);

CREATE TABLE "giliran_dhuha" (
	"id_giliran" serial PRIMARY KEY,
	"jurusan" varchar(20) NOT NULL,
	"hari" varchar(10) NOT NULL,
	CONSTRAINT "giliran_dhuha_hari_jurusan_key" UNIQUE("hari","jurusan"),
	CONSTRAINT "giliran_dhuha_hari_check" CHECK (((hari)::text = ANY ((ARRAY['Senin'::character varying, 'Selasa'::character varying, 'Rabu'::character varying, 'Kamis'::character varying, 'Jumat'::character varying])::text[])))
);

CREATE TABLE "jadwal_sholat_template" (
	"id_template" serial PRIMARY KEY,
	"hari" varchar(10) NOT NULL,
	"id_jenis" integer NOT NULL,
	CONSTRAINT "jadwal_sholat_template_hari_id_jenis_key" UNIQUE("hari","id_jenis"),
	CONSTRAINT "jadwal_sholat_template_hari_check" CHECK (((hari)::text = ANY ((ARRAY['Senin'::character varying, 'Selasa'::character varying, 'Rabu'::character varying, 'Kamis'::character varying, 'Jumat'::character varying, 'Sabtu'::character varying, 'Ahad'::character varying])::text[])))
);

CREATE TABLE "jenis_sholat" (
	"id_jenis" serial PRIMARY KEY,
	"nama_jenis" varchar(30) NOT NULL,
	"butuh_giliran" boolean NOT NULL,
	CONSTRAINT "jenis_sholat_nama_jenis_check" CHECK (((nama_jenis)::text = ANY ((ARRAY['Subuh'::character varying, 'Dzuhur'::character varying, 'Ashar'::character varying, 'Maghrib'::character varying, 'Isya'::character varying, 'Dhuha'::character varying, 'Jumat'::character varying])::text[])))
);

CREATE TABLE "kelas" (
	"id_kelas" serial PRIMARY KEY,
	"tingkatan" smallint NOT NULL,
	"jurusan" varchar(20) NOT NULL,
	"part" varchar(3) NOT NULL,
	"deleted_at" timestamp,
	CONSTRAINT "kelas_tingkatan_jurusan_part_key" UNIQUE("tingkatan","jurusan","part"),
	CONSTRAINT "kelas_tingkatan_check" CHECK (((tingkatan >= 10) AND (tingkatan <= 12)))
);

CREATE TABLE "rekap_absensi" (
	"id_rekap" serial PRIMARY KEY,
	"id_siswa" integer NOT NULL,
	"id_semester" integer NOT NULL,
	"id_jenis" integer NOT NULL,
	"jumlah_hadir" integer DEFAULT 0 NOT NULL,
	"jumlah_izin" integer DEFAULT 0 NOT NULL,
	"jumlah_sakit" integer DEFAULT 0 NOT NULL,
	"jumlah_alpha" integer DEFAULT 0 NOT NULL,
	CONSTRAINT "rekap_absensi_id_siswa_id_semester_id_jenis_key" UNIQUE("id_siswa","id_semester","id_jenis")
);

CREATE TABLE "semester_akademik" (
	"id_semester" serial PRIMARY KEY,
	"id_tahun" integer NOT NULL,
	"semester" smallint NOT NULL,
	"tanggal_mulai" date NOT NULL,
	"tanggal_selesai" date NOT NULL,
	CONSTRAINT "semester_akademik_id_tahun_semester_key" UNIQUE("id_tahun","semester"),
	CONSTRAINT "semester_akademik_semester_check" CHECK ((semester = ANY (ARRAY[1, 2]))),
	CONSTRAINT "semester_akademik_tanggal_selesai_check" CHECK ((tanggal_selesai > tanggal_mulai))
);

CREATE TABLE "siswa" (
	"id_siswa" serial PRIMARY KEY,
	"id_account" integer NOT NULL,
	"nis" varchar(20) NOT NULL,
	"nama_siswa" varchar(255) NOT NULL,
	"jk" char(1) NOT NULL,
	"id_kelas" integer,
	"id_tahun_masuk" integer,
	"class_status" varchar(20) DEFAULT 'active' NOT NULL,
	"last_promotion_at" timestamp,
	"deleted_at" timestamp,
	CONSTRAINT "siswa_id_account_key" UNIQUE("id_account"),
	CONSTRAINT "siswa_nis_key" UNIQUE("nis"),
	CONSTRAINT "siswa_class_status_check" CHECK (((class_status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying, 'graduated'::character varying, 'transferred'::character varying])::text[]))),
	CONSTRAINT "siswa_jk_check" CHECK ((jk = ANY (ARRAY['L'::bpchar, 'P'::bpchar])))
);

CREATE TABLE "staff" (
	"id_staff" serial PRIMARY KEY,
	"id_account" integer NOT NULL,
	"nip" varchar(20),
	"nama" varchar(255) NOT NULL,
	"tipe_staff" varchar(10) NOT NULL,
	"deleted_at" timestamp,
	CONSTRAINT "staff_id_account_key" UNIQUE("id_account"),
	CONSTRAINT "staff_nip_key" UNIQUE("nip"),
	CONSTRAINT "staff_tipe_staff_check" CHECK (((tipe_staff)::text = ANY ((ARRAY['admin'::character varying, 'guru'::character varying])::text[])))
);

CREATE TABLE "tahun_masuk" (
	"id_tahun_masuk" serial PRIMARY KEY,
	"tahun" varchar(9) NOT NULL,
	"is_active" boolean NOT NULL,
	"created_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT "tahun_masuk_tahun_key" UNIQUE("tahun")
);

CREATE TABLE "wali_kelas" (
	"id_wali" serial PRIMARY KEY,
	"id_kelas" integer NOT NULL,
	"id_staff" integer NOT NULL,
	"is_active" boolean NOT NULL,
	"berlaku_mulai" date NOT NULL,
	CONSTRAINT "wali_kelas_id_kelas_key" UNIQUE("id_kelas")
);

CREATE TABLE "waktu_sholat" (
	"id_waktu" serial PRIMARY KEY,
	"id_jenis" integer NOT NULL,
	"waktu_mulai" time NOT NULL,
	"waktu_selesai" time NOT NULL,
	"berlaku_mulai" date NOT NULL,
	"berlaku_sampai" date,
	CONSTRAINT "waktu_sholat_waktu_selesai_check" CHECK ((waktu_selesai > waktu_mulai))
);

CREATE UNIQUE INDEX "absensi_id_siswa_tanggal_id_template_key" ON "absensi" ("id_siswa","tanggal","id_template");
CREATE INDEX "absensi_id_giliran_idx" ON "absensi" ("id_giliran");
CREATE INDEX "absensi_id_semester_idx" ON "absensi" ("id_semester");
CREATE INDEX "absensi_id_siswa_idx" ON "absensi" ("id_siswa");
CREATE INDEX "absensi_id_template_idx" ON "absensi" ("id_template");
CREATE UNIQUE INDEX "absensi_pkey" ON "absensi" ("id_absen");
CREATE INDEX "absensi_tanggal_idx" ON "absensi" ("tanggal");
CREATE UNIQUE INDEX "accounts_email_key" ON "accounts" ("email");
CREATE UNIQUE INDEX "accounts_pkey" ON "accounts" ("id");
CREATE UNIQUE INDEX "giliran_dhuha_hari_jurusan_key" ON "giliran_dhuha" ("hari","jurusan");
CREATE UNIQUE INDEX "giliran_dhuha_pkey" ON "giliran_dhuha" ("id_giliran");
CREATE UNIQUE INDEX "jadwal_sholat_template_hari_id_jenis_key" ON "jadwal_sholat_template" ("hari","id_jenis");
CREATE UNIQUE INDEX "jadwal_sholat_template_pkey" ON "jadwal_sholat_template" ("id_template");
CREATE UNIQUE INDEX "jenis_sholat_pkey" ON "jenis_sholat" ("id_jenis");
CREATE UNIQUE INDEX "kelas_pkey" ON "kelas" ("id_kelas");
CREATE UNIQUE INDEX "kelas_tingkatan_jurusan_part_key" ON "kelas" ("tingkatan","jurusan","part");
CREATE UNIQUE INDEX "rekap_absensi_id_siswa_id_semester_id_jenis_key" ON "rekap_absensi" ("id_siswa","id_semester","id_jenis");
CREATE UNIQUE INDEX "rekap_absensi_pkey" ON "rekap_absensi" ("id_rekap");
CREATE UNIQUE INDEX "semester_akademik_id_tahun_semester_key" ON "semester_akademik" ("id_tahun","semester");
CREATE UNIQUE INDEX "semester_akademik_pkey" ON "semester_akademik" ("id_semester");
CREATE UNIQUE INDEX "siswa_id_account_key" ON "siswa" ("id_account");
CREATE UNIQUE INDEX "siswa_nis_key" ON "siswa" ("nis");
CREATE UNIQUE INDEX "siswa_pkey" ON "siswa" ("id_siswa");
CREATE UNIQUE INDEX "staff_id_account_key" ON "staff" ("id_account");
CREATE INDEX "staff_nip_idx" ON "staff" ("nip");
CREATE UNIQUE INDEX "staff_pkey" ON "staff" ("id_staff");
CREATE UNIQUE INDEX "tahun_masuk_pkey" ON "tahun_masuk" ("id_tahun_masuk");
CREATE UNIQUE INDEX "tahun_masuk_tahun_key" ON "tahun_masuk" ("tahun");
CREATE UNIQUE INDEX "wali_kelas_id_kelas_key" ON "wali_kelas" ("id_kelas");
CREATE UNIQUE INDEX "wali_kelas_pkey" ON "wali_kelas" ("id_wali");
CREATE UNIQUE INDEX "waktu_sholat_pkey" ON "waktu_sholat" ("id_waktu");

ALTER TABLE "absensi" ADD CONSTRAINT "absensi_id_giliran_fkey" FOREIGN KEY ("id_giliran") REFERENCES "giliran_dhuha"("id_giliran");
ALTER TABLE "absensi" ADD CONSTRAINT "absensi_id_semester_fkey" FOREIGN KEY ("id_semester") REFERENCES "semester_akademik"("id_semester");
ALTER TABLE "absensi" ADD CONSTRAINT "absensi_id_siswa_fkey" FOREIGN KEY ("id_siswa") REFERENCES "siswa"("id_siswa") ON DELETE CASCADE;
ALTER TABLE "absensi" ADD CONSTRAINT "absensi_id_template_fkey" FOREIGN KEY ("id_template") REFERENCES "jadwal_sholat_template"("id_template");
ALTER TABLE "jadwal_sholat_template" ADD CONSTRAINT "jadwal_sholat_template_id_jenis_fkey" FOREIGN KEY ("id_jenis") REFERENCES "jenis_sholat"("id_jenis");
ALTER TABLE "rekap_absensi" ADD CONSTRAINT "rekap_absensi_id_jenis_fkey" FOREIGN KEY ("id_jenis") REFERENCES "jenis_sholat"("id_jenis");
ALTER TABLE "rekap_absensi" ADD CONSTRAINT "rekap_absensi_id_semester_fkey" FOREIGN KEY ("id_semester") REFERENCES "semester_akademik"("id_semester");
ALTER TABLE "rekap_absensi" ADD CONSTRAINT "rekap_absensi_id_siswa_fkey" FOREIGN KEY ("id_siswa") REFERENCES "siswa"("id_siswa") ON DELETE CASCADE;
ALTER TABLE "semester_akademik" ADD CONSTRAINT "semester_akademik_id_tahun_fkey" FOREIGN KEY ("id_tahun") REFERENCES "tahun_masuk"("id_tahun_masuk");
ALTER TABLE "siswa" ADD CONSTRAINT "siswa_id_account_fkey" FOREIGN KEY ("id_account") REFERENCES "accounts"("id") ON DELETE CASCADE;
ALTER TABLE "siswa" ADD CONSTRAINT "siswa_id_kelas_fkey" FOREIGN KEY ("id_kelas") REFERENCES "kelas"("id_kelas");
ALTER TABLE "siswa" ADD CONSTRAINT "siswa_id_tahun_masuk_fkey" FOREIGN KEY ("id_tahun_masuk") REFERENCES "tahun_masuk"("id_tahun_masuk");
ALTER TABLE "staff" ADD CONSTRAINT "staff_id_account_fkey" FOREIGN KEY ("id_account") REFERENCES "accounts"("id") ON DELETE CASCADE;
ALTER TABLE "wali_kelas" ADD CONSTRAINT "wali_kelas_id_kelas_fkey" FOREIGN KEY ("id_kelas") REFERENCES "kelas"("id_kelas");
ALTER TABLE "wali_kelas" ADD CONSTRAINT "wali_kelas_id_staff_fkey" FOREIGN KEY ("id_staff") REFERENCES "staff"("id_staff");
ALTER TABLE "waktu_sholat" ADD CONSTRAINT "waktu_sholat_id_jenis_fkey" FOREIGN KEY ("id_jenis") REFERENCES "jenis_sholat"("id_jenis");