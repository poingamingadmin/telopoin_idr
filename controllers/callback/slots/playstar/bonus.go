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

	"telo/database" // pastikan ada package ini yg expose var DB *gorm.DB
	"telo/models"
)

type BonusResponse struct {
	StatusCode int    `json:"status_code"`
	Balance    uint64 `json:"balance,omitempty"`
}

func BonusAwardHandler(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// === Params ===
	accessToken := c.Query("access_token")
	if strings.TrimSpace(accessToken) == "" {
		return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 1})
	}

	bonusIDStr := c.Query("bonus_id")
	bonusID, err := strconv.ParseUint(bonusIDStr, 10, 64)
	if err != nil {
		return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 2})
	}

	bonusRewardStr := c.Query("bonus_reward")
	bonusReward, err := strconv.ParseUint(bonusRewardStr, 10, 64)
	if err != nil || bonusReward == 0 {
		return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 5})
	}

	bonusType := c.Query("bonus_type")
	if len(bonusType) > 12 {
		return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 5})
	}

	gameID := c.Query("game_id")
	subgameID64, _ := strconv.ParseUint(c.Query("subgame_id"), 10, 16)
	subgameID := uint16(subgameID64)
	txnIDStr := c.Query("txn_id")
	txnID, _ := strconv.ParseUint(txnIDStr, 10, 64)

	memberID := c.Query("member_id")
	if memberID == "" {
		return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 5})
	}

	// === Cari user & lock row ===
	var user models.User
	if err := database.DB.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", memberID).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 1})
		}
		return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 5})
	}

	// === Update saldo ===
	balanceBefore := user.Balance
	balanceAfter := balanceBefore + float64(bonusReward)

	user.Balance = balanceAfter
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 5})
	}

	// === Update UserGameTransaction (tambah Bonus) ===
	var gameTrx models.UserGameTransaction
	if err := database.DB.Where("provider = ? AND provider_tx = ?", "Playstar", fmt.Sprintf("%d", txnID)).
		First(&gameTrx).Error; err == nil {
		gameTrx.BonusAmount += int64(bonusReward)
		gameTrx.BalanceBefore = balanceBefore
		gameTrx.BalanceAfter = balanceAfter
		gameTrx.Status = "BONUS"
		gameTrx.Note = fmt.Sprintf("Bonus awarded type=%s id=%d", bonusType, bonusID)
		if err := database.DB.Save(&gameTrx).Error; err != nil {
			return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 5})
		}
	} else {
		// fallback: create baru kalau belum ada transaksi game sebelumnya
		gameTrx = models.UserGameTransaction{
			UserID:        user.ID,
			UserCode:      user.UserCode,
			AgentCode:     user.AgentCode,
			Provider:      "Playstar",
			GameID:        gameID,
			SubGameID:     subgameID,
			ProviderTx:    fmt.Sprintf("%d", txnID),
			BetAmount:     0,
			WinAmount:     0,
			BonusAmount:   int64(bonusReward),
			Currency:      user.Currency,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
			Status:        "BONUS",
			Note:          fmt.Sprintf("Bonus awarded type=%s id=%d", bonusType, bonusID),
			RefID:         fmt.Sprintf("PSBONUS-%d", bonusID),
		}
		if err := database.DB.Create(&gameTrx).Error; err != nil {
			return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 5})
		}
	}

	fmt.Printf("[Playstar][Bonus] %s User=%s Bonus=%d NewBalance=%.2f\n",
		now.Format("2006-01-02 15:04:05"), user.UserCode, bonusReward, balanceAfter)

	return c.Status(http.StatusOK).JSON(BonusResponse{StatusCode: 0, Balance: uint64(balanceAfter)})
}
