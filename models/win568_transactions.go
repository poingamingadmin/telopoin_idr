package models

import (
	"database/sql/driver"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ===== Custom Time Parser untuk 568Win =====
type WinTime struct {
	time.Time
}

// Parse JSON dari API (format 2025-09-10T07:54:09.487)
func (wt *WinTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "0001-01-01T00:00:00" {
		return nil
	}
	// Format waktu dari API (millis tanpa timezone)
	t, err := time.Parse("2006-01-02T15:04:05.000", s)
	if err != nil {
		return err
	}
	wt.Time = t
	return nil
}

// Scan implementasi biar bisa dibaca dari DB
func (wt *WinTime) Scan(value interface{}) error {
	if value == nil {
		wt.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		wt.Time = v
		return nil
	case []byte:
		t, err := time.Parse("2006-01-02 15:04:05", string(v))
		if err != nil {
			return err
		}
		wt.Time = t
		return nil
	case string:
		t, err := time.Parse("2006-01-02 15:04:05", v)
		if err != nil {
			return err
		}
		wt.Time = t
		return nil
	default:
		return nil
	}
}

// Value implementasi biar bisa disimpan ke DB
func (wt WinTime) Value() (driver.Value, error) {
	if wt.Time.IsZero() {
		return nil, nil
	}
	return wt.Time, nil
}

// ===== Model Transaksi =====
type X568WinTransaction struct {
	gorm.Model

	CompanyKey    string `gorm:"size:100;index"`
	Username      string `gorm:"size:100;index;index:uk_sbo_transfer_user,unique"`
	Amount        float64
	TransferCode  string `gorm:"size:100;index;index:uk_sbo_transfer_user"`
	TransactionId string `gorm:"size:100;index"`
	BetTime       time.Time
	ProductType   int
	GameType      int
	GameRoundId   *string
	GamePeriodId  *string
	OrderDetail   *string
	PlayerIp      *string
	GameTypeName  *string
	Gpid          int `gorm:"default:-1"`
	GameId        int `gorm:"default:0"`

	ExtraInfo datatypes.JSON `gorm:"type:jsonb"`
	Status    string         `gorm:"size:50;index"`
	WinLoss   float64        `gorm:"default:0"`
	Rollback  bool           `gorm:"default:false"`
	IsCashOut bool           `gorm:"default:false"`

	ResultType int `gorm:"default:0"`
	ResultTime *time.Time
	GameResult *string
}

// ===== Parent Bet =====
type Win568Bet struct {
	gorm.Model
	RefNo                    string  `gorm:"uniqueIndex;size:50" json:"refNo"`
	Username                 string  `gorm:"index;size:50" json:"username"`
	SportsType               string  `gorm:"size:50" json:"sportsType"`
	OrderTime                WinTime `json:"orderTime"`
	WinLostDate              WinTime `json:"winLostDate"`
	SettleTime               WinTime `json:"settleTime"`
	ModifyDate               WinTime `json:"modifyDate"`
	Odds                     float64 `json:"odds"`
	OddsStyle                string  `gorm:"size:5" json:"oddsStyle"`
	Stake                    float64 `json:"stake"`
	ActualStake              float64 `json:"actualStake"`
	Currency                 string  `gorm:"size:10" json:"currency"`
	Status                   string  `gorm:"size:20" json:"status"`
	WinLost                  float64 `json:"winLost"`
	Turnover                 float64 `json:"turnover"`
	TurnoverByStake          float64 `json:"turnoverByStake"`
	TurnoverByActualStake    float64 `json:"turnoverByActualStake"`
	NetTurnoverByStake       float64 `json:"netTurnoverByStake"`
	NetTurnoverByActualStake float64 `json:"netTurnoverByActualStake"`
	IsHalfWonLose            bool    `json:"isHalfWonLose"`
	IsCashOut                bool    `json:"isCashOut"`
	IsLive                   bool    `json:"isLive"`
	MaxWinWithoutActualStake float64 `json:"maxWinWithoutActualStake"`
	IP                       string  `gorm:"size:45" json:"ip"`
	VoidReason               string  `gorm:"size:100" json:"voidReason"`
	NewGameType              int     `json:"newGameType"`
	IsResend                 bool    `gorm:"default:false" json:"-"`
	ResendCount              int     `gorm:"default:0" json:"-"`

	// Relasi One-to-Many
	SubBets []Win568SubBet `json:"subBet" gorm:"foreignKey:BetID;constraint:OnDelete:CASCADE"`
}

// ===== Child SubBet =====
type Win568SubBet struct {
	gorm.Model
	BetID              uint    `json:"-"` // FK ke Win568Bet
	BetOption          string  `gorm:"size:100" json:"betOption"`
	MarketType         string  `gorm:"size:50" json:"marketType"`
	Hdp                float64 `json:"hdp"`
	Odds               float64 `json:"odds"`
	League             string  `gorm:"size:100" json:"league"`
	Match              string  `gorm:"size:100" json:"match"`
	Status             string  `gorm:"size:20" json:"status"`
	WinLostDate        WinTime `json:"winlostDate"`
	LiveScore          string  `gorm:"size:20" json:"liveScore"`
	HtScore            string  `gorm:"size:20" json:"htScore"`
	FtScore            string  `gorm:"size:20" json:"ftScore"`
	CustomeizedBetType string  `gorm:"size:50" json:"customeizedBetType"`
	KickOffTime        WinTime `json:"kickOffTime"`
	IsHalfWonLose      bool    `json:"isHalfWonLose"`
}
