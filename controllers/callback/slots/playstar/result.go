package playstar

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"telo/database"
	"telo/models"
)

type ResultResponse struct {
	StatusCode int    `json:"status_code"`
	Balance    uint64 `json:"balance,omitempty"`
}

func ResultHandler(c *fiber.Ctx) error {
	accessToken := c.Query("access_token")
	if strings.TrimSpace(accessToken) == "" {
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 1})
	}

	txnIDStr := c.Query("txn_id")
	txnID, err := strconv.ParseUint(txnIDStr, 10, 64)
	if err != nil {
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 2})
	}

	totalWin, _ := strconv.ParseUint(c.Query("total_win"), 10, 64)
	bonusWin, _ := strconv.ParseUint(c.Query("bonus_win"), 10, 64)
	gameID := c.Query("game_id")
	subgameID64, _ := strconv.ParseUint(c.Query("subgame_id"), 10, 16)
	subgameID := uint16(subgameID64)
	ts, _ := strconv.ParseUint(c.Query("ts"), 10, 64)
	jpContrib, _ := strconv.ParseFloat(c.Query("jp_contrib"), 64)
	jpContrib = math.Round(jpContrib*100) / 100
	winAmt, _ := strconv.ParseUint(c.Query("winamt"), 10, 64)
	memberID := c.Query("member_id")

	// --- cari PlaystarTransaction ---
	var betTxn models.PlaystarTransaction
	if err := database.DB.Where("txn_id = ?", txnID).First(&betTxn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 2})
		}
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 5})
	}

	// --- lock user ---
	var user models.User
	if err := database.DB.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", memberID).
		First(&user).Error; err != nil {
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 1})
	}

	// --- update balance ---
	balanceBefore := user.Balance
	balanceAfter := balanceBefore + float64(totalWin)
	user.Balance = balanceAfter
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 5})
	}

	// --- update PlaystarTransaction ---
	betTxn.TotalWin = totalWin
	betTxn.BonusWin = bonusWin
	betTxn.GameID = gameID
	betTxn.SubGameID = subgameID
	betTxn.TS = ts
	betTxn.JPContrib = jpContrib
	betTxn.WinAmt = winAmt
	betTxn.MemberID = memberID
	if err := database.DB.Save(&betTxn).Error; err != nil {
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 5})
	}

	// --- update UserGameTransaction (dari BET -> RESULT) ---
	var gameTrx models.UserGameTransaction
	if err := database.DB.Where("provider = ? AND provider_tx = ?", "Playstar", fmt.Sprintf("%d", txnID)).
		First(&gameTrx).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// kalau belum ada BET → error system
			return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 5})
		}
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 5})
	}

	gameTrx.WinAmount = int64(totalWin)
	gameTrx.BonusAmount = int64(bonusWin)
	gameTrx.JPContrib = jpContrib
	gameTrx.GameID = gameID
	gameTrx.SubGameID = subgameID
	gameTrx.BalanceBefore = balanceBefore
	gameTrx.BalanceAfter = balanceAfter
	gameTrx.Status = "RESULT"
	gameTrx.Note = "Result credited"
	if err := database.DB.Save(&gameTrx).Error; err != nil {
		return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 5})
	}

	return c.Status(http.StatusOK).JSON(ResultResponse{StatusCode: 0, Balance: uint64(balanceAfter)})
}
