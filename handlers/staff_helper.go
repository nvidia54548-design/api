package handlers

import (
	"fmt"

	"gorm.io/gorm"
)

type waliKelasInfo struct {
	IDKelas   int
	Tingkatan int
	Jurusan   string
	Part      string
}

func resolveWaliKelasInfo(db *gorm.DB, nip string) (waliKelasInfo, error) {
	var info waliKelasInfo
	err := db.Table("staff s").
		Select("wk.id_kelas, k.tingkatan, k.jurusan, k.part").
		Joins("JOIN wali_kelas wk ON wk.id_staff = s.id_staff").
		Joins("JOIN kelas k ON k.id_kelas = wk.id_kelas").
		Where("s.nip = ?", nip).
		Scan(&info).Error
	if err != nil {
		return waliKelasInfo{}, err
	}
	if info.IDKelas == 0 {
		return waliKelasInfo{}, gorm.ErrRecordNotFound
	}
	return info, nil
}

func waliKelasLabel(info waliKelasInfo) string {
	if info.IDKelas == 0 {
		return ""
	}
	return fmt.Sprintf("%d%s", info.Tingkatan, info.Part)
}
