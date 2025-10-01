package models

import "gorm.io/gorm"

type SpadeGamingTransaction struct {
	gorm.Model
	TransferID   string  `gorm:"uniqueIndex;size:50;not null" json:"transferId"` // unique transfer ID dari provider
	MerchantCode string  `gorm:"size:50;not null" json:"merchantCode"`
	MerchantTxID string  `gorm:"size:50" json:"merchantTxId"` // optional: ID internal merchant
	AcctID       string  `gorm:"index;size:50;not null" json:"acctId"`
	Currency     string  `gorm:"size:10;not null" json:"currency"`
	Amount       float64 `gorm:"type:decimal(20,9);not null" json:"amount"`
	Type         int     `gorm:"not null" json:"type"` // 1=bet, 2=cancel, 4=payout
	TicketID     string  `gorm:"size:50" json:"ticketId"`
	Channel      string  `gorm:"size:20" json:"channel"`
	GameCode     string  `gorm:"size:20" json:"gameCode"`
	ReferenceID  string  `gorm:"size:50" json:"referenceId"`
	PlayerIP     string  `gorm:"size:50" json:"playerIp"`
	GameFeature  string  `gorm:"size:50" json:"gameFeature"`
	TransferTime string  `gorm:"size:20" json:"transferTime"`

	// Extra fields for payout / special game
	SpecialType  string `gorm:"size:20" json:"specialType"`
	SpecialCount int    `json:"specialCount"`
	SpecialSeq   int    `json:"specialSeq"`
	RefTicketIds string `gorm:"type:text" json:"refTicketIds"` // bisa simpan JSON array string

	// Status & balance info
	BalanceBefore float64 `gorm:"type:decimal(20,4)" json:"balanceBefore"`
	BalanceAfter  float64 `gorm:"type:decimal(20,4)" json:"balanceAfter"`
	Status        string  `gorm:"size:20;default:'Success'" json:"status"` // Success, Failed, Pending
	Msg           string  `gorm:"size:255" json:"msg"`
	Code          int     `json:"code"`
	SerialNo      string  `gorm:"size:50" json:"serialNo"`
}
