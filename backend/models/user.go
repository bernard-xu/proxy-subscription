package models

import (
	"crypto/sha256"
	"encoding/hex"
)

// User 用户模型
type User struct {
	BaseModel
	Username     string `json:"username" gorm:"unique;not null"`
	PasswordHash string `json:"-" gorm:"not null"`
	IsAdmin      bool   `json:"is_admin" gorm:"default:false"`
}

// HashPassword 对密码进行哈希处理
func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// VerifyPassword 验证密码是否正确
func (u *User) VerifyPassword(password string) bool {
	return u.PasswordHash == HashPassword(password)
}
