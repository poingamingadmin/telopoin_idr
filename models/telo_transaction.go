package models

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

type FlexibleString string

func (fs *FlexibleString) UnmarshalJSON(data []byte) error {
	var s string
	var i int64
	var f float64

	if err := json.Unmarshal(data, &s); err == nil {
		*fs = FlexibleString(s)
		return nil
	}

	if err := json.Unmarshal(data, &i); err == nil {
		*fs = FlexibleString(fmt.Sprintf("%d", i))
		return nil
	}

	if err := json.Unmarshal(data, &f); err == nil {
		*fs = FlexibleString(fmt.Sprintf("%g", f))
		return nil
	}

	return fmt.Errorf("unable to parse %s as FlexibleString", string(data))
}

func (fs FlexibleString) ToInt64() (int64, error) {
	return strconv.ParseInt(string(fs), 10, 64)
}

type TeloSlotTransaction struct {
	gorm.Model

	AgentCode    string         `json:"agent_code"`
	AgentSecret  string         `json:"agent_secret"`
	AgentBalance FlexibleString `json:"agent_balance"`

	UserCode        string         `json:"user_code"`
	UserBalance     FlexibleString `json:"user_balance"`
	UserTotalCredit FlexibleString `json:"user_total_credit"`
	UserTotalDebit  FlexibleString `json:"user_total_debit"`

	GameType string `json:"game_type"`

	Slot TeloSlotDetail `gorm:"embedded" json:"slot"`
}

type TeloSlotDetail struct {
	ProviderCode    string         `json:"provider_code"`
	GameCode        FlexibleString `json:"game_code"`
	RoundID         FlexibleString `json:"round_id"`
	IsRoundFinished bool           `json:"is_round_finished"`
	Type            string         `json:"type"`

	Bet FlexibleString `json:"bet"`
	Win FlexibleString `json:"win"`

	TxnID   FlexibleString `json:"txn_id"`
	TxnType string         `json:"txn_type"`

	UserBeforeBalance FlexibleString `json:"user_before_balance"`
	UserAfterBalance  FlexibleString `json:"user_after_balance"`

	AgentBeforeBalance FlexibleString `json:"agent_before_balance"`
	AgentAfterBalance  FlexibleString `json:"agent_after_balance"`

	CreatedAtRaw string `json:"created_at"`
}
