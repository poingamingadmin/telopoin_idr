package models

import (
	"gorm.io/gorm"
)

type WmSubBet struct {
	gorm.Model
	UserCode      string  `gorm:"size:64;index" json:"user_code"`
	Username      string  `gorm:"size:64;index" json:"username"`
	Balance       float64 `gorm:"-" json:"balance"`
	TransferCode  string  `gorm:"size:255;index" json:"transfer_code"`
	TransactionId string  `gorm:"size:255;index" json:"transaction_id"`
	GameType      int     `gorm:"index" json:"game_type"`
	GameId        int     `gorm:"index" json:"game_id"`
	Amount        float64 `json:"amount"`
	Status        string  `gorm:"size:16;index" json:"status"`
	WinLoss       float64 `json:"win_loss"`
	BetTime       string  `json:"bet_time"`
	OrderDetail   string  `gorm:"-" json:"order_detail"`
	ResultType    int     `gorm:"-" json:"result_type"`
}
