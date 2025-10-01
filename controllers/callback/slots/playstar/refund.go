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

	"telo/database" // pastikan ada package database yg expose var DB *gorm.DB
	"telo/models"
)

type RefundResponse struct {
	StatusCode int    `json:"status_code"`
	Balance    uint64 `json:"balance,omitempty"`
}

func RefundHandler(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// === Params ===
	accessToken := c.Query("access_token")
	if strings.TrimSpace(accessToken) == "" {
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 1})
	}

	txnIDStr := c.Query("txn_id")
	txnID, err := strconv.ParseUint(txnIDStr, 10, 64)
	if err != nil {
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 2})
	}

	memberID := c.Query("member_id")
	if memberID == "" {
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
	}

	gameID := c.Query("game_id")

	// === Cari transaksi BET (PlaystarTransaction) ===
	var betTxn models.PlaystarTransaction
	if err := database.DB.Where("txn_id = ?", txnID).First(&betTxn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 2})
		}
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
	}

	if betTxn.BetAmt == 0 {
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
	}

	// === Lock user ===
	var user models.User
	if err := database.DB.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", memberID).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 1})
		}
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
	}

	// === Update saldo ===
	balanceBefore := user.Balance
	betAmt := float64(betTxn.BetAmt)
	balanceAfter := balanceBefore + betAmt

	user.Balance = balanceAfter
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
	}

	// === Update UserGameTransaction (BET -> REFUND) ===
	var gameTrx models.UserGameTransaction
	if err := database.DB.Where("provider = ? AND provider_tx = ?", "Playstar", fmt.Sprintf("%d", txnID)).
		First(&gameTrx).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// tidak ada log BET sebelumnya → system error
			return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
		}
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
	}

	gameTrx.BalanceBefore = balanceBefore
	gameTrx.BalanceAfter = balanceAfter
	gameTrx.Status = "REFUND"
	gameTrx.Note = fmt.Sprintf("Refunded bet for game %s", gameID)

	if err := database.DB.Save(&gameTrx).Error; err != nil {
		return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 5})
	}

	fmt.Printf("[Playstar][Refund] %s User=%s Refund=%d NewBalance=%.2f\n",
		now.Format("2006-01-02 15:04:05"), user.UserCode, betTxn.BetAmt, balanceAfter)

	return c.Status(http.StatusOK).JSON(RefundResponse{StatusCode: 0, Balance: uint64(balanceAfter)})
}
