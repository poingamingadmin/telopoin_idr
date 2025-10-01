// file: sportsbook/sbo/bonus.go
package sbo

import (
	"encoding/json"
	"errors"
	"time"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FIX: Expanded request to include all fields from the CSV for complete data capture.
type BonusCreditRequest struct {
	CompanyKey              string         `json:"CompanyKey"`
	Username                string         `json:"Username"`
	TransferCode            string         `json:"TransferCode"`
	TransactionId           string         `json:"TransactionId"` // Capture this for consistency
	Amount                  float64        `json:"Amount"`
	BonusTime               string         `json:"BonusTime"`
	ProductType             int            `json:"ProductType"`
	GameType                int            `json:"GameType"`
	Gpid                    int            `json:"Gpid"`
	GameId                  int            `json:"GameId"`
	IsGameProviderPromotion bool           `json:"IsGameProviderPromotion"`
	BonusProvider           string         `json:"BonusProvider"`
	ExtraInfo               map[string]any `json:"ExtraInfo"`
}

// FIX: Added a dedicated request struct for canceling bonuses.
type CancelBonusRequest struct {
	CompanyKey    string `json:"CompanyKey"`
	Username      string `json:"Username"`
	TransferCode  string `json:"TransferCode"`
	TransactionId string `json:"TransactionId"`
	ProductType   int    `json:"ProductType"`
	GameType      int    `json:"GameType"`
	Gpid          int    `json:"Gpid"`
}

// BonusCreditHandler gives a user a bonus credit.
// It creates a 'Settled' transaction where WinLoss holds the bonus amount.
func BonusCreditHandler(c *fiber.Ctx) error {
	var req BonusCreditRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"ErrorCode": 422, "ErrorMessage": "Invalid request format"})
	}
	if req.Username == "" || req.TransferCode == "" || req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"ErrorCode": 422, "ErrorMessage": "Username, TransferCode, and positive Amount are required"})
	}
	// Default ProductType for bonus if not provided
	if req.ProductType == 0 {
		req.ProductType = 9
	}

	var resp fiber.Map
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_code = ?", req.Username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{"ErrorCode": 1, "ErrorMessage": "User not found"}
				return nil
			}
			return err
		}

		// Idempotency Check: Fail if a transaction with this TransferCode already exists.
		var existingCount int64
		tx.Model(&models.X568WinTransaction{}).Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).Count(&existingCount)
		if existingCount > 0 {
			resp = fiber.Map{"ErrorCode": 5003, "ErrorMessage": "Duplicate TransferCode", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		}

		// Apply bonus credit to the user's balance
		rate := getRate(user.Currency)
		credit := roundInternalBalance(user.Currency, req.Amount*rate)
		user.Balance = roundInternalBalance(user.Currency, user.Balance+credit)
		if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
			return err
		}

		// FIX: Store all relevant request details in ExtraInfo for auditing and consistency.
		meta := map[string]any{
			"bonus":                   true,
			"creditedAt":              time.Now().Format(time.RFC3339),
			"isGameProviderPromotion": req.IsGameProviderPromotion,
			"bonusProvider":           req.BonusProvider,
		}
		for k, v := range req.ExtraInfo {
			meta[k] = v
		}
		extraJSON, _ := json.Marshal(meta)

		bonusTime, _ := time.Parse(time.RFC3339, req.BonusTime)

		trx := models.X568WinTransaction{
			CompanyKey:    req.CompanyKey,
			Username:      req.Username,
			Amount:        0, // A bonus is not a stake, so Amount is 0.
			TransferCode:  req.TransferCode,
			TransactionId: req.TransactionId,
			ProductType:   req.ProductType,
			GameType:      req.GameType,
			Gpid:          req.Gpid,
			GameId:        req.GameId,
			BetTime:       bonusTime,
			Status:        "Settled",  // Bonuses are immediately settled.
			WinLoss:       req.Amount, // The credited amount is stored as WinLoss for easy reversal.
			ExtraInfo:     extraJSON,
		}
		if err := tx.Create(&trx).Error; err != nil {
			return err
		}

		resp = fiber.Map{"ErrorCode": 0, "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"ErrorCode": 7, "ErrorMessage": "Transaction failed"})
	}
	return c.JSON(resp)
}

func CancelBonusHandler(c *fiber.Ctx) error {
	var req CancelBonusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"ErrorCode": 422, "ErrorMessage": "Invalid request format"})
	}
	if req.Username == "" || req.TransferCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"ErrorCode": 422, "ErrorMessage": "Username and TransferCode are required"})
	}

	var resp fiber.Map
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_code = ?", req.Username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{"ErrorCode": 1, "ErrorMessage": "User not found"}
				return nil
			}
			return err
		}

		var trx models.X568WinTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).First(&trx).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Transaction not found", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}
			return err
		}

		// --- State Machine Logic for Cancellation ---
		switch trx.Status {
		case "Void":
			// Idempotency: Already canceled, return the correct error code.
			resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled", "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		case "Settled":
			// This is the main path: reverse the bonus.
			rate := getRate(user.Currency)
			// The original bonus was stored in WinLoss, so we subtract it.
			debit := roundInternalBalance(user.Currency, trx.WinLoss*rate)
			user.Balance = roundInternalBalance(user.Currency, user.Balance-debit)
			if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
				return err
			}

			// Atomically update the transaction status to "Void".
			res := tx.Model(&trx).Where("id = ? AND status = ?", trx.ID, "Settled").Update("status", "Void")
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				// Race condition: Another process just canceled it.
				resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled", "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}
		default:
			// Any other status is invalid for cancellation.
			resp = fiber.Map{"ErrorCode": 8, "ErrorMessage": "Bonus in a non-cancellable state", "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		}

		resp = fiber.Map{"ErrorCode": 0, "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"ErrorCode": 7, "ErrorMessage": "Transaction failed"})
	}
	return c.JSON(resp)
}
