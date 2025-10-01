package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Session struct {
	gorm.Model
	SID       string    `gorm:"size:36;uniqueIndex;not null"`
	UserID    uint      `gorm:"index"`
	User      User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ExpiresAt time.Time `gorm:"index"`
}

func (s *Session) BeforeCreate(tx *gorm.DB) (err error) {
	s.SID = strings.ToLower(uuid.New().String())
	return nil
}
