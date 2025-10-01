package models

import (
	"gorm.io/gorm"
)

type EvolutionTransaction struct {
	gorm.Model
	UserID   uint   `gorm:"index"`
	SID      string `gorm:"size:128"`
	TxID     string `gorm:"size:64;uniqueIndex"`
	RefID    string `gorm:"size:64;index"`
	Amount   float64
	Currency string `gorm:"size:8"`
	Type     string `gorm:"size:16"`
	GameID   string `gorm:"size:64"`
	GameType string `gorm:"size:32"`
	TableID  string `gorm:"size:64"`
	TableVID string `gorm:"size:64"`
	UUID     string `gorm:"size:64"`
	Status   string `gorm:"size:16"`
	Provider string `gorm:"size:32"`
}
