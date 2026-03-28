-- Migration: Add kelas to jadwal_sholat
-- Version: 000009
ALTER TABLE jadwal_sholat ADD COLUMN IF NOT EXISTS kelas VARCHAR(50);
