package models

import "gorm.io/gorm"

// PlaystarTransaction merepresentasikan transaksi dari Playstar Result API
type PlaystarTransaction struct {
	gorm.Model
	AccessToken string  `json:"access_token" query:"access_token" gorm:"type:varchar(255);index"` // token sesi player
	TxnID       uint64  `json:"txn_id" query:"txn_id" gorm:"uniqueIndex"`                         // Unique transaction ID (idempotency)
	TotalWin    uint64  `json:"total_win" query:"total_win"`                                      // Total win (in cents)
	BonusWin    uint64  `json:"bonus_win" query:"bonus_win"`                                      // Bonus win (in cents)
	GameID      string  `json:"game_id" query:"game_id" gorm:"type:varchar(64);index"`            // PS Game unique ID
	SubGameID   uint16  `json:"subgame_id" query:"subgame_id"`                                    // Sub game ID
	TS          uint64  `json:"ts" query:"ts" gorm:"index"`                                       // UTC timestamp (seconds)
	JPContrib   float64 `json:"jp_contrib" query:"jp_contrib"`                                    // Jackpot contribution
	BetAmt      uint64  `json:"betamt,omitempty" query:"betamt"`                                  // Optional bet (card games)
	WinAmt      uint64  `json:"winamt,omitempty" query:"winamt"`                                  // Optional win (card games)
	MemberID    string  `json:"member_id,omitempty" query:"member_id" gorm:"type:varchar(64)"`    // Optional player ID
}
