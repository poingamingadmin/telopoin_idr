package models

import "gorm.io/gorm"

type Agent struct {
	gorm.Model

	Username  string  `gorm:"uniqueIndex;size:32" json:"username"`
	AgentCode string  `gorm:"uniqueIndex;size:32" json:"agent_code"`
	SecretKey string  `gorm:"size:128" json:"secret_key"`
	Balance   int64   `json:"balance"`
	Currency  string  `gorm:"size:8" json:"currency"`
	GGR       float64 `json:"ggr"`
	IsActive  bool    `gorm:"default:true" json:"isactive"`

	Users        []User             `gorm:"foreignKey:AgentCode;references:AgentCode"`
	Transactions []AgentTransaction `gorm:"foreignKey:AgentID"`
}

type AgentTransaction struct {
	gorm.Model

	AgentID       uint   `gorm:"index"`
	AgentCode     string `gorm:"index;size:32"`
	TrxType       string `gorm:"size:16"`
	Amount        int64  `json:"amount"`
	BalanceBefore int64  `json:"balance_before"`
	BalanceAfter  int64  `json:"balance_after"`
	Currency      string `gorm:"size:8"`
	Note          string `gorm:"size:255"`
	RefID         string `gorm:"size:64"`
}
