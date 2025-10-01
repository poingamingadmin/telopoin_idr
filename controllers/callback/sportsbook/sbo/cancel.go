// file: sportsbook/sbo/cancel.go
package sbo

import (
	"encoding/json"
	"errors"
	"strings"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CancelRequest struct {
	CompanyKey    string `json:"CompanyKey"`
	Username      string `json:"Username"`
	TransferCode  string `json:"TransferCode"`
	ProductType   int    `json:"ProductType"`
	GameType      int    `json:"GameType"`
	TransactionId string `json:"TransactionId"`
	IsCancelAll   bool   `json:"IsCancelAll"`
}

func CancelBetHandler(c *fiber.Ctx) error {
	var req CancelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    3,
			"ErrorMessage": "Invalid request format",
		})
	}

	req.Username = strings.TrimSpace(req.Username)
	req.TransferCode = strings.TrimSpace(req.TransferCode)
	if req.Username == "" || req.TransferCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    422,
			"ErrorMessage": "Username and TransferCode are required",
		})
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

		if err := normalizeAndPersist(tx, &user); err != nil {
			return err
		}

		// Handle WM (ProductType 9) cancel for sub-bets
		if req.ProductType == 9 {
			// --- Validate input ---
			if len(req.Username) == 0 || len(req.TransferCode) == 0 {
				resp = fiber.Map{
					"ErrorCode":    3,
					"ErrorMessage": "Invalid request format",
				}
				return nil
			}

			var bets []models.WmSubBet
			q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username)
			if !req.IsCancelAll {
				if strings.TrimSpace(req.TransactionId) == "" {
					resp = fiber.Map{"ErrorCode": 3, "ErrorMessage": "Invalid request format"}
					return nil
				}
				q = q.Where("transaction_id = ?", req.TransactionId)
			}
			q = q.Order("id ASC")
			if err := q.Find(&bets).Error; err != nil {
				return err
			}

			if len(bets) == 0 {
				// Fallback: handle bonus transactions stored in X568WinTransaction
				var btrx models.X568WinTransaction
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).
					First(&btrx).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Bet Not Found",
							"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
						return nil
					}
					return err
				}

				// Check if it's a bonus-like transaction
				var meta map[string]any
				_ = json.Unmarshal(btrx.ExtraInfo, &meta)
				isBonus := (btrx.Amount == 0 && btrx.WinLoss > 0) || (meta != nil && meta["bonus"] == true)
				if !isBonus {
					resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Bet Not Found",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				}

				switch btrx.Status {
				case "Void":
					resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				case "Settled":
					// Reverse credited bonus
					rate := getRate(user.Currency)
					debit := roundInternalBalance(user.Currency, btrx.WinLoss*rate)
					user.Balance = roundInternalBalance(user.Currency, user.Balance-debit)
					if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
						return err
					}
					res := tx.Model(&btrx).Where("id = ? AND status = ?", btrx.ID, "Settled").Update("status", "Void")
					if res.Error != nil {
						return res.Error
					}
					if res.RowsAffected == 0 {
						resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled",
							"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
						return nil
					}
					resp = fiber.Map{"ErrorCode": 0, "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				default:
					resp = fiber.Map{"ErrorCode": 8, "ErrorMessage": "Bonus in a non-cancellable state",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				}
			}

			// Compute changes without mutating state
			totalChange := 0.0
			for i := range bets {
				b := &bets[i]
				switch b.Status {
				case "Running":
					totalChange += b.Amount
				case "Settled":
					if req.IsCancelAll {
						totalChange += (b.Amount - b.WinLoss)
					} else {
						resp = fiber.Map{"ErrorCode": 2001, "ErrorMessage": "Bet Already Settled",
							"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
						return nil
					}
				case "Void":
					resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				case "Rollback":
					resp = fiber.Map{"ErrorCode": 2003, "ErrorMessage": "Bet Already Rollback",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				}
			}

			// Second pass: update statuses with status guard and update member balance
			for i := range bets {
				b := &bets[i]
				if b.Status == "Running" || b.Status == "Settled" {
					res := tx.Model(b).Where("id = ? AND status = ?", b.ID, b.Status).Update("status", "Void")
					if res.Error != nil {
						return res.Error
					}
					if res.RowsAffected == 0 {
						resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled",
							"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
						return nil
					}
				}
			}

			if totalChange != 0 {
				user.Balance = roundInternalBalance(user.Currency, user.Balance+totalChange)
				if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
					return err
				}
			}

			resp = fiber.Map{
				"ErrorCode":   0,
				"AccountName": req.Username,
				"Balance":     displayBalanceWithCurrency(user.Currency, user.Balance),
			}
			return nil
		}

		var trx models.X568WinTransaction

		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username)

		if !req.IsCancelAll && req.TransactionId != "" {
			query = query.Where("transaction_id = ?", req.TransactionId)
		}

		if err := query.First(&trx).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Transaction not found", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}
			return err
		}

		var balanceUpdated bool
		var balanceChange float64
		rate := getRate(user.Currency)

		switch trx.Status {
		case "Void":
			resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled", "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil

		case "Running":
			balanceChange = trx.Amount
			balanceUpdated = true

		case "Settled":
			balanceChange = trx.Amount - trx.WinLoss
			balanceUpdated = true

		default:
			resp = fiber.Map{"ErrorCode": 8, "ErrorMessage": "Bet in a non-cancellable state", "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		}

		if balanceUpdated {
			refund := roundInternalBalance(user.Currency, balanceChange*rate)
			user.Balance = roundInternalBalance(user.Currency, user.Balance+refund)
			if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
				return err
			}
		}

		res := tx.Model(&trx).
			Where("id = ? AND status IN (?)", trx.ID, []string{"Running", "Settled"}).
			Updates(map[string]any{
				"status": "Void",
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled", "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		}

		resp = fiber.Map{"ErrorCode": 0, "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"ErrorCode":    7,
			"ErrorMessage": "Transaction failed",
		})
	}
	return c.JSON(resp)
}
