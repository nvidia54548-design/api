package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type UserStaff struct {
	IDStaff  int    `gorm:"primaryKey;column:id_staff" json:"id_staff"`
	Username string `gorm:"unique;not null" json:"username"`
	Role     string `gorm:"not null" json:"role"`
}

func (UserStaff) TableName() string {
	return "users_staff"
}

func main() {
	dsn := "postgresql://neondb_owner:npg_4PNEReq0jcTz@ep-soft-moon-a4zcp1t3-pooler.us-east-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	var users []UserStaff
	db.Find(&users)

	fmt.Println("Staff Roles in DB:")
	for _, u := range users {
		fmt.Printf("- User: %s, Role: '%s'\n", u.Username, u.Role)
	}
}
