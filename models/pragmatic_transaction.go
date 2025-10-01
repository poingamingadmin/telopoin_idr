package models

import (
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PragmaticTransaction struct {
	gorm.Model

	UserID       string  `gorm:"size:100;not null;index;column:user_id"`
	Currency     string  `gorm:"size:3;not null;index"`
	Country      *string `gorm:"size:2;index"`
	Jurisdiction *string `gorm:"size:2;index"`
	DataType     *string `gorm:"size:3;index"`
	Platform     *string `gorm:"size:10;index"`
	Language     *string `gorm:"size:2;index"`

	Cash                decimal.Decimal `gorm:"type:numeric(10,2);default:0"`
	Bonus               decimal.Decimal `gorm:"type:numeric(10,2);default:0"`
	Amount              decimal.Decimal `gorm:"type:numeric(10,2);default:0"`
	TotalBalance        decimal.Decimal `gorm:"type:numeric(10,2);default:0"`
	ChosenBalance       decimal.Decimal `gorm:"type:numeric(10,2);default:0"`
	Win                 decimal.Decimal `gorm:"type:numeric(10,2);default:0"`
	UsedPromo           decimal.Decimal `gorm:"type:numeric(10,2);default:0"`
	JackpotContribution decimal.Decimal `gorm:"type:numeric(10,6);default:0"`
	PromoWinAmount      decimal.Decimal `gorm:"type:numeric(10,2);default:0"`

	GameID        *string `gorm:"size:20;index"`
	RoundID       *int64  `gorm:"index"`
	JackpotID     *int64  `gorm:"index"`
	SessionID     *string `gorm:"size:100;index"`
	ProviderID    *string `gorm:"size:32;index"`
	LaunchingType *string `gorm:"size:1"`
	PreviousToken *string `gorm:"size:100"`

	Reference     string  `gorm:"size:32;uniqueIndex"`
	TransactionID string  `gorm:"size:32;uniqueIndex"`
	Token         string  `gorm:"size:100;index"`
	RequestID     *string `gorm:"size:252;index"`
	BonusCode     *string `gorm:"size:252;index"`

	ExtraInfo      datatypes.JSON `gorm:"type:jsonb"`
	JackpotDetails datatypes.JSON `gorm:"type:jsonb"`
	RoundDetails   *string        `gorm:"type:text"`

	IPAddress         *string `gorm:"size:32;index"`
	CampaignID        *string `gorm:"size:100;index"`
	CampaignType      *string `gorm:"size:3"`
	PromoWinReference *string `gorm:"size:100"`
	PromoCampaignID   *int64  `gorm:"index"`

	ErrorCode   *int32  `gorm:"column:error"`
	Description *string `gorm:"size:100"`

	ProviderTSMS *int64 `gorm:"column:timestamp;index"`
}
