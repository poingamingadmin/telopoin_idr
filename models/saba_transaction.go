package models

import "gorm.io/gorm"

type SabaTransaction struct {
	gorm.Model

	UserID    uint   `gorm:"index"`         // relasi ke tabel users
	UserCode  string `gorm:"size:32;index"` // kode user unik
	AgentCode string `gorm:"size:32;index"` // kode agent

	OperationID string `gorm:"size:64;index"` // unik dari SABA (idempotency check)
	GameID      string `gorm:"size:64;index"` // pertandingan / event id
	BetType     string `gorm:"size:32"`       // Single, Parlay, etc
	Market      string `gorm:"size:32"`       // e.g. FT/HT
	OddsType    string `gorm:"size:16"`
	Currency    string `gorm:"size:8"`

	BetAmount    float64 `json:"bet_amount"`    // jumlah bet
	WinAmount    float64 `json:"win_amount"`    // jumlah kemenangan
	RefundAmount float64 `json:"refund_amount"` // jumlah refund (jika cancel/unsettle)

	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`

	Status string `gorm:"size:16;index"` // BET, CONFIRM, CANCEL, SETTLE, RESETTLE, UNSETTLE
	Note   string `gorm:"size:255"`
	RefID  string `gorm:"size:64;index"` // internal ref (misal SABABET-xxxx)
}
