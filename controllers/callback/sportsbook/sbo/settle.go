// file: sportsbook/sbo/settle.go
package sbo

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettleRequest struct {
	CompanyKey      string         `json:"CompanyKey"`
	Username        string         `json:"Username"`
	TransferCode    string         `json:"TransferCode"`
	TransactionId   string         `json:"TransactionId"` // 🔑 khusus ProductType=9 (WM)
	WinLoss         float64        `json:"WinLoss"`
	ResultType      int            `json:"ResultType"` // 0: Win, 1: Lose, 2: Draw/Refund
	ResultTime      string         `json:"ResultTime"`
	ProductType     int            `json:"ProductType"`
	GameType        int            `json:"GameType"`
	GameResult      *string        `json:"GameResult"`
	CommissionStake float64        `json:"CommissionStake"`
	Gpid            int            `json:"Gpid"`
	IsCashOut       bool           `json:"IsCashOut"`
	ExtraInfo       map[string]any `json:"ExtraInfo"`
}

func normalizeRFC3339(s string) string {
	if s == "" {
		return ""
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", time.RubyDate}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return s
}

func SettleHandler(c *fiber.Ctx) error {
	var req SettleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    1,
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
				resp = fiber.Map{"ErrorCode": 2, "ErrorMessage": "User not found"}
				return nil
			}
			return err
		}

		if err := normalizeAndPersist(tx, &user); err != nil {
			return err
		}

		if req.ProductType == 9 {
			if len(req.Username) == 0 || len(req.TransferCode) == 0 {
				resp = fiber.Map{"ErrorCode": 3, "ErrorMessage": "Invalid request format"}
				return nil
			}

			var user models.User
			if err := tx.Where("user_code = ?", req.Username).First(&user).Error; err != nil {
				resp = fiber.Map{"ErrorCode": 1, "ErrorMessage": "User not found", "Balance": 0}
				return nil
			}

			// Load all sub-bets for this transfer and user, lock rows for concurrency safety
			var bets []models.WmSubBet
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).
				Find(&bets).Error; err != nil {
				return err
			}
			if len(bets) == 0 {
				resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Bet Not Found",
					"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}

			// Enforce order-level settle idempotency: if any already Settled, reject with 2001
			for i := range bets {
				if bets[i].Status == "Settled" {
					resp = fiber.Map{"ErrorCode": 2001, "ErrorMessage": "Bet Already Settled",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				}
			}

			// If specific TransactionId provided, target that sub-bet
			var candidate *models.WmSubBet
			if strings.TrimSpace(req.TransactionId) != "" {
				for i := range bets {
					b := &bets[i]
					if b.TransactionId == req.TransactionId {
						switch b.Status {
						case "Running":
							candidate = b
						case "Void":
							resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled",
								"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
							return nil
						default:
							resp = fiber.Map{"ErrorCode": 2001, "ErrorMessage": "Bet Already Settled",
								"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
							return nil
						}
						break
					}
				}
				if candidate == nil {
					resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Bet Not Found",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				}
			} else {
				// No TransactionId => select any Running sub-bet
				for i := range bets {
					if bets[i].Status == "Running" {
						candidate = &bets[i]
						break
					}
				}
				if candidate == nil {
					// If all voids
					allVoid := true
					for i := range bets {
						if bets[i].Status != "Void" {
							allVoid = false
							break
						}
					}
					if allVoid {
						resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled",
							"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
						return nil
					}
					resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Bet Not Found",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				}
			}

			// Settle the candidate and credit user with WinLoss (in internal units)
			newWinLoss := convertToInternalValueWithCurrency(user.Currency, req.WinLoss)
			// Race-safe status update: only settle if still Running
			res := tx.Model(candidate).Where("id = ? AND status = ?", candidate.ID, "Running").
				Updates(map[string]any{"status": "Settled", "win_loss": newWinLoss})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				resp = fiber.Map{"ErrorCode": 2001, "ErrorMessage": "Bet Already Settled",
					"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}

			user.Balance = roundInternalBalance(user.Currency, user.Balance+newWinLoss)
			if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
				return err
			}

			resp = fiber.Map{
				"ErrorCode":    0,
				"AccountName":  req.Username,
				"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
				"TransferCode": req.TransferCode,
				"WinLoss":      req.WinLoss,
				"ResultType":   req.ResultType,
			}
			return nil
		}

		var trx models.X568WinTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).First(&trx).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Bet not found", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}
			return err
		}

		switch trx.Status {
		case "Settled":
			resp = fiber.Map{"ErrorCode": 2001, "ErrorMessage": "Bet Already Settled", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		case "Void":
			resp = fiber.Map{"ErrorCode": 2002, "ErrorMessage": "Bet Already Canceled", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		case "Running":
		default:
			resp = fiber.Map{"ErrorCode": 8, "ErrorMessage": "Bet in an un-settleable state", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		}

		var creditAmount float64
		if req.IsCashOut {
			creditAmount = req.WinLoss
		} else {
			switch req.ResultType {
			case 0: // Win: Credit the total payout (WinLoss)
				creditAmount = req.WinLoss
			case 1: // Lose: No credit is given (stake is lost).
				creditAmount = 0
			case 2: // Draw/Refund: Credit the original stake back to the player.
				creditAmount = trx.Amount
			default:
				// Handle other potential result types if they exist.
				creditAmount = 0
			}
		}

		if creditAmount > 0 {
			rate := getRate(user.Currency)
			inc := roundInternalBalance(user.Currency, creditAmount*rate)
			user.Balance = roundInternalBalance(user.Currency, user.Balance+inc)
			if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
				return err
			}
		}

		// Merge ExtraInfo (Preserve original bet info, add settlement details)
		var oldInfo map[string]any
		_ = json.Unmarshal(trx.ExtraInfo, &oldInfo)
		if oldInfo == nil {
			oldInfo = make(map[string]any)
		}
		for k, v := range req.ExtraInfo {
			oldInfo[k] = v // Merge new info from request
		}
		oldInfo["settledAt"] = time.Now().Format(time.RFC3339)
		oldInfo["resultType"] = req.ResultType
		oldInfo["resultTime"] = normalizeRFC3339(req.ResultTime)
		oldInfo["gameResult"] = req.GameResult
		newExtra, _ := json.Marshal(oldInfo)

		res := tx.Model(&trx).
			Where("id = ? AND status = ?", trx.ID, "Running").
			Updates(map[string]any{
				"status":       "Settled",
				"win_loss":     req.WinLoss,
				"is_cash_out":  req.IsCashOut,
				"product_type": req.ProductType,
				"game_type":    req.GameType,
				"gpid":         req.Gpid,
				"rollback":     false,
				"extra_info":   newExtra,
			})

		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			resp = fiber.Map{"ErrorCode": 2001, "ErrorMessage": "Bet Already Settled", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
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
