package models

import (
	"time"
)

// BaseModel 自定义基础模型，替代 gorm.Model
// 使用小写的 JSON 字段名
type BaseModel struct {
	ID        uint       `json:"id" gorm:"primarykey"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}
