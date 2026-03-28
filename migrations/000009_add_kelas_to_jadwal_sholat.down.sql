-- Migration: Remove kelas from jadwal_sholat
-- Version: 000009
ALTER TABLE jadwal_sholat DROP COLUMN IF EXISTS kelas;
