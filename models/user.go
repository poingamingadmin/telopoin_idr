package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model

	UserCode         string                `gorm:"uniqueIndex;size:32" json:"user_code"`
	AgentCode        string                `gorm:"index;size:32" json:"agent_code"`
	Balance          float64               `json:"balance"`
	Country          string                `gorm:"size:64" json:"country"`
	Currency         string                `gorm:"size:8" json:"currency"`
	IsActive         bool                  `gorm:"default:true" json:"is_active"`
	Transactions     []UserTransaction     `gorm:"foreignKey:UserID"`
	GameTransactions []UserGameTransaction `gorm:"foreignKey:UserID"`
}

type UserTransaction struct {
	gorm.Model

	UserID        uint    `gorm:"index"`
	AgentCode     string  `gorm:"index;size:32"`
	UserCode      string  `gorm:"size:32"`
	TrxType       string  `gorm:"size:16"`
	Amount        int64   `json:"amount"`
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	Currency      string  `gorm:"size:8" json:"currency"`
	Note          string  `gorm:"size:255"`
	RefID         string  `gorm:"size:64"`
}

type UserGameTransaction struct {
	gorm.Model

	UserID    uint   `gorm:"index"`
	UserCode  string `gorm:"size:32;index"`
	AgentCode string `gorm:"size:32;index"`

	GameID     string `gorm:"size:64;index"`
	SubGameID  uint16 `gorm:"index"`
	ProviderTx string `gorm:"size:64;index:idx_provider_tx,unique"`
	Provider   string `gorm:"size:32;index:idx_provider_tx,unique"`

	BetAmount   int64   `json:"bet_amount"`
	WinAmount   int64   `json:"win_amount"`
	BonusAmount int64   `json:"bonus_amount"`
	JPContrib   float64 `json:"jp_contrib"`
	Currency    string  `gorm:"size:8"`

	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`

	Status string `gorm:"size:16;index"`
	Note   string `gorm:"size:255"`
	RefID  string `gorm:"size:64;index"`
}
