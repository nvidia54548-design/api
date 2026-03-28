-- Migration: Fix Wali Kelas Role
-- Version: 000007
-- Description: Update incorrect 'wali kelas' role to 'wali_kelas'

UPDATE users_staff SET role = 'wali_kelas' WHERE role = 'wali kelas';
UPDATE refresh_tokens SET role = 'wali_kelas' WHERE role = 'wali kelas';
