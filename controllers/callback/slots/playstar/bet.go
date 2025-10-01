package playstar

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"telo/database" // pastikan ada package ini yg expose `var DB *gorm.DB`
	"telo/models"
)

type BetResponse struct {
	StatusCode int    `json:"status_code"`
	Balance    uint64 `json:"balance,omitempty"`
}

func BetHandler(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// --- ambil query params ---
	accessToken := c.Query("access_token")
	if strings.TrimSpace(accessToken) == "" {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 1})
	}

	txnIDStr := c.Query("txn_id")
	txnID, err := strconv.ParseUint(txnIDStr, 10, 64)
	if err != nil {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 2})
	}

	totalBetStr := c.Query("total_bet")
	totalBet, err := strconv.ParseUint(totalBetStr, 10, 64)
	if err != nil || totalBet == 0 {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 5})
	}

	gameID := c.Query("game_id")
	subgameIDStr := c.Query("subgame_id")
	subgameID64, _ := strconv.ParseUint(subgameIDStr, 10, 16)
	subgameID := uint16(subgameID64)

	tsStr := c.Query("ts")
	ts, _ := strconv.ParseUint(tsStr, 10, 64)

	memberID := c.Query("member_id")
	if memberID == "" {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 5})
	}

	// --- cari user & lock row ---
	var user models.User
	if err := database.DB.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", memberID).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 1})
		}
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 5})
	}

	// cek saldo cukup
	if user.Balance < float64(totalBet) {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 3})
	}

	// hitung saldo baru
	balanceBefore := user.Balance
	balanceAfter := balanceBefore - float64(totalBet)

	// update saldo user
	user.Balance = balanceAfter
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 5})
	}

	// simpan transaksi provider (opsional)
	playTxn := models.PlaystarTransaction{
		AccessToken: accessToken,
		TxnID:       txnID,
		GameID:      gameID,
		SubGameID:   subgameID,
		TS:          ts,
		BetAmt:      totalBet,
		MemberID:    memberID,
	}
	if err := database.DB.Create(&playTxn).Error; err != nil {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 5})
	}

	// catat transaksi umum (financial log)
	userTrx := models.UserTransaction{
		UserID:        user.ID,
		AgentCode:     user.AgentCode,
		UserCode:      user.UserCode,
		TrxType:       "BET",
		Amount:        int64(totalBet),
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Currency:      user.Currency,
		Note:          fmt.Sprintf("Playstar Bet %s", gameID),
		RefID:         fmt.Sprintf("PSBET-%d", txnID),
	}
	if err := database.DB.Create(&userTrx).Error; err != nil {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 5})
	}

	// catat transaksi game detail
	gameTrx := models.UserGameTransaction{
		UserID:        user.ID,
		UserCode:      user.UserCode,
		AgentCode:     user.AgentCode,
		Provider:      "Playstar",
		GameID:        gameID,
		SubGameID:     subgameID,
		ProviderTx:    fmt.Sprintf("%d", txnID),
		BetAmount:     int64(totalBet),
		WinAmount:     0,
		BonusAmount:   0,
		JPContrib:     0,
		Currency:      user.Currency,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Status:        "BET",
		Note:          "Bet request received",
		RefID:         fmt.Sprintf("PSBET-%d", txnID),
	}
	if err := database.DB.Create(&gameTrx).Error; err != nil {
		return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 5})
	}

	fmt.Printf("[Playstar][Bet] %s User=%s Bet=%d NewBalance=%.2f\n",
		now.Format("2006-01-02 15:04:05"), user.UserCode, totalBet, balanceAfter)

	return c.Status(http.StatusOK).JSON(BetResponse{StatusCode: 0, Balance: uint64(balanceAfter)})
}
