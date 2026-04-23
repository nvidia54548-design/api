CREATE SCHEMA IF NOT EXISTS "public";

CREATE TABLE "absensi" (
	"id_absen" serial PRIMARY KEY,
	"id_jadwal" integer NOT NULL,
	"id_siswa" integer NOT NULL,
	"tanggal" date NOT NULL,
	"status" varchar(15) NOT NULL,
	"created_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT "absensi_id_jadwal_id_siswa_tanggal_key" UNIQUE("id_jadwal","id_siswa","tanggal"),
	CONSTRAINT "absensi_status_check" CHECK (((status)::text = ANY ((ARRAY['hadir'::character varying, 'izin'::character varying, 'sakit'::character varying, 'alpha'::character varying])::text[])))
);

CREATE TABLE "accounts" (
	"id" serial PRIMARY KEY,
	"email" varchar(255) NOT NULL,
	"password" varchar(255) NOT NULL,
	"role" varchar(20) NOT NULL,
	"created_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT "accounts_email_key" UNIQUE("email"),
	CONSTRAINT "accounts_role_check" CHECK (((role)::text = ANY ((ARRAY['admin'::character varying, 'guru'::character varying, 'siswa'::character varying])::text[])))
);

CREATE TABLE "jadwal_sholat" (
	"id_jadwal" serial PRIMARY KEY,
	"id_kelas" integer NOT NULL,
	"hari" varchar(10) NOT NULL,
	"jenis_sholat" varchar(30) NOT NULL,
	"waktu_mulai" time NOT NULL,
	"waktu_selesai" time NOT NULL,
	"created_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT "jadwal_sholat_id_kelas_hari_jenis_sholat_key" UNIQUE("id_kelas","hari","jenis_sholat"),
	CONSTRAINT "jadwal_sholat_check" CHECK ((waktu_selesai > waktu_mulai)),
	CONSTRAINT "jadwal_sholat_hari_check" CHECK (((hari)::text = ANY ((ARRAY['Senin'::character varying, 'Selasa'::character varying, 'Rabu'::character varying, 'Kamis'::character varying, 'Jumat'::character varying, 'Sabtu'::character varying, 'Ahad'::character varying])::text[]))),
	CONSTRAINT "jadwal_sholat_jenis_sholat_check" CHECK (((jenis_sholat)::text = ANY ((ARRAY['Subuh'::character varying, 'Dzuhur'::character varying, 'Ashar'::character varying, 'Maghrib'::character varying, 'Isya'::character varying, 'Dhuha'::character varying, 'Jumat'::character varying])::text[])))
);

CREATE TABLE "kelas" (
	"id_kelas" serial PRIMARY KEY,
	"tingkatan" smallint NOT NULL,
	"jurusan" varchar(20) NOT NULL,
	"part" varchar(3) NOT NULL,
	CONSTRAINT "kelas_tingkatan_jurusan_part_key" UNIQUE("tingkatan","jurusan","part"),
	CONSTRAINT "kelas_tingkatan_check" CHECK (((tingkatan >= 10) AND (tingkatan <= 12)))
);

CREATE TABLE "refresh_tokens" (
	"id" serial PRIMARY KEY,
	"account_id" integer NOT NULL,
	"token" text NOT NULL,
	"expires_at" timestamp NOT NULL,
	"created_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	"updated_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT "refresh_tokens_token_key" UNIQUE("token")
);

CREATE TABLE "siswa" (
	"id_siswa" serial PRIMARY KEY,
	"id_account" integer NOT NULL,
	"nis" varchar(20) NOT NULL,
	"nama_siswa" varchar(255) NOT NULL,
	"jk" char(1) NOT NULL,
	"id_kelas" integer,
	"academic_year" char(9),
	"class_status" varchar(20) DEFAULT 'active' NOT NULL,
	"last_promotion_at" timestamp,
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
	CONSTRAINT "staff_id_account_key" UNIQUE("id_account"),
	CONSTRAINT "staff_nip_key" UNIQUE("nip"),
	CONSTRAINT "staff_tipe_staff_check" CHECK (((tipe_staff)::text = ANY ((ARRAY['admin'::character varying, 'guru'::character varying])::text[])))
);

CREATE TABLE "student_class_transitions" (
	"id" serial PRIMARY KEY,
	"id_siswa" integer NOT NULL,
	"action" varchar(30) NOT NULL,
	"from_class" integer,
	"to_class" integer,
	"from_academic_year" char(9),
	"to_academic_year" char(9),
	"from_semester" smallint,
	"to_semester" smallint,
	"from_status" varchar(20),
	"to_status" varchar(20),
	"decided_by" integer,
	"note" text,
	"created_at" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT "student_class_transitions_action_check" CHECK (((action)::text = ANY ((ARRAY['naik_kelas'::character varying, 'tinggal_kelas'::character varying, 'lulus'::character varying, 'pindah'::character varying, 'masuk'::character varying])::text[]))),
	CONSTRAINT "student_class_transitions_from_semester_check" CHECK ((from_semester = ANY (ARRAY[1, 2]))),
	CONSTRAINT "student_class_transitions_to_semester_check" CHECK ((to_semester = ANY (ARRAY[1, 2])))
);

CREATE TABLE "wali_kelas" (
	"id_wali" serial PRIMARY KEY,
	"id_kelas" integer NOT NULL,
	"id_staff" integer NOT NULL,
	CONSTRAINT "wali_kelas_id_kelas_key" UNIQUE("id_kelas")
);

CREATE UNIQUE INDEX "absensi_id_jadwal_id_siswa_tanggal_key" ON "absensi" ("id_jadwal","id_siswa","tanggal");
CREATE INDEX "absensi_id_jadwal_idx" ON "absensi" ("id_jadwal");
CREATE INDEX "absensi_id_siswa_idx" ON "absensi" ("id_siswa");
CREATE UNIQUE INDEX "absensi_pkey" ON "absensi" ("id_absen");
CREATE INDEX "absensi_tanggal_idx" ON "absensi" ("tanggal");
CREATE UNIQUE INDEX "accounts_email_key" ON "accounts" ("email");
CREATE UNIQUE INDEX "accounts_pkey" ON "accounts" ("id");
CREATE INDEX "jadwal_sholat_hari_idx" ON "jadwal_sholat" ("hari");
CREATE UNIQUE INDEX "jadwal_sholat_id_kelas_hari_jenis_sholat_key" ON "jadwal_sholat" ("id_kelas","hari","jenis_sholat");
CREATE INDEX "jadwal_sholat_jenis_sholat_idx" ON "jadwal_sholat" ("jenis_sholat");
CREATE UNIQUE INDEX "jadwal_sholat_pkey" ON "jadwal_sholat" ("id_jadwal");
CREATE UNIQUE INDEX "kelas_pkey" ON "kelas" ("id_kelas");
CREATE UNIQUE INDEX "kelas_tingkatan_jurusan_part_key" ON "kelas" ("tingkatan","jurusan","part");
CREATE INDEX "refresh_tokens_account_id_idx" ON "refresh_tokens" ("account_id");
CREATE UNIQUE INDEX "refresh_tokens_pkey" ON "refresh_tokens" ("id");
CREATE UNIQUE INDEX "refresh_tokens_token_key" ON "refresh_tokens" ("token");
CREATE UNIQUE INDEX "siswa_id_account_key" ON "siswa" ("id_account");
CREATE INDEX "siswa_nis_idx" ON "siswa" ("nis");
CREATE UNIQUE INDEX "siswa_nis_key" ON "siswa" ("nis");
CREATE UNIQUE INDEX "siswa_pkey" ON "siswa" ("id_siswa");
CREATE UNIQUE INDEX "staff_id_account_key" ON "staff" ("id_account");
CREATE INDEX "staff_nip_idx" ON "staff" ("nip");
CREATE UNIQUE INDEX "staff_nip_key" ON "staff" ("nip");
CREATE UNIQUE INDEX "staff_pkey" ON "staff" ("id_staff");
CREATE INDEX "student_class_transitions_created_at_idx" ON "student_class_transitions" ("created_at");
CREATE INDEX "student_class_transitions_id_siswa_idx" ON "student_class_transitions" ("id_siswa");
CREATE UNIQUE INDEX "student_class_transitions_pkey" ON "student_class_transitions" ("id");
CREATE UNIQUE INDEX "wali_kelas_id_kelas_key" ON "wali_kelas" ("id_kelas");
CREATE UNIQUE INDEX "wali_kelas_pkey" ON "wali_kelas" ("id_wali");

ALTER TABLE "absensi" ADD CONSTRAINT "absensi_id_jadwal_fkey" FOREIGN KEY ("id_jadwal") REFERENCES "jadwal_sholat"("id_jadwal") ON DELETE CASCADE;
ALTER TABLE "absensi" ADD CONSTRAINT "absensi_id_siswa_fkey" FOREIGN KEY ("id_siswa") REFERENCES "siswa"("id_siswa") ON DELETE CASCADE;
ALTER TABLE "jadwal_sholat" ADD CONSTRAINT "jadwal_sholat_id_kelas_fkey" FOREIGN KEY ("id_kelas") REFERENCES "kelas"("id_kelas") ON DELETE CASCADE;
ALTER TABLE "refresh_tokens" ADD CONSTRAINT "refresh_tokens_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "accounts"("id") ON DELETE CASCADE;
ALTER TABLE "siswa" ADD CONSTRAINT "siswa_id_account_fkey" FOREIGN KEY ("id_account") REFERENCES "accounts"("id") ON DELETE CASCADE;
ALTER TABLE "siswa" ADD CONSTRAINT "siswa_id_kelas_fkey" FOREIGN KEY ("id_kelas") REFERENCES "kelas"("id_kelas");
ALTER TABLE "staff" ADD CONSTRAINT "staff_id_account_fkey" FOREIGN KEY ("id_account") REFERENCES "accounts"("id") ON DELETE CASCADE;
ALTER TABLE "student_class_transitions" ADD CONSTRAINT "student_class_transitions_decided_by_fkey" FOREIGN KEY ("decided_by") REFERENCES "staff"("id_staff");
ALTER TABLE "student_class_transitions" ADD CONSTRAINT "student_class_transitions_from_class_fkey" FOREIGN KEY ("from_class") REFERENCES "kelas"("id_kelas");
ALTER TABLE "student_class_transitions" ADD CONSTRAINT "student_class_transitions_id_siswa_fkey" FOREIGN KEY ("id_siswa") REFERENCES "siswa"("id_siswa") ON DELETE CASCADE;
ALTER TABLE "student_class_transitions" ADD CONSTRAINT "student_class_transitions_to_class_fkey" FOREIGN KEY ("to_class") REFERENCES "kelas"("id_kelas");
ALTER TABLE "wali_kelas" ADD CONSTRAINT "wali_kelas_id_kelas_fkey" FOREIGN KEY ("id_kelas") REFERENCES "kelas"("id_kelas");
ALTER TABLE "wali_kelas" ADD CONSTRAINT "wali_kelas_id_staff_fkey" FOREIGN KEY ("id_staff") REFERENCES "staff"("id_staff");
