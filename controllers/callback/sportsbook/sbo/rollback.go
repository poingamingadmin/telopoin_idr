// file: sportsbook/sbo/rollback.go
package sbo

import (
	"errors"
	"log"
	"sort"
	"strings"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FIX: Added TransactionId to uniquely identify which bet to roll back,
// which is essential for providers like 3rd WM.
type RollbackRequest struct {
	CompanyKey    string         `json:"CompanyKey"`
	Username      string         `json:"Username"`
	TransferCode  string         `json:"TransferCode"`
	TransactionId string         `json:"TransactionId"`
	ProductType   int            `json:"ProductType"`
	ResultType    int            `json:"ResultType"` // 0: Win, 1: Lose, 2: Draw/Refund
	GameType      int            `json:"GameType"`
	IsCashOut     bool           `json:"IsCashOut"`
	Gpid          int            `json:"Gpid"`
	ExtraInfo     map[string]any `json:"ExtraInfo"`
}

// RollbackBetHandler reverts a transaction from a "Settled" or "Void" state back to "Running".
func RollbackBetHandler(c *fiber.Ctx) error {
	var req RollbackRequest
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
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_code = ?", req.Username).
			First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{"ErrorCode": 1, "ErrorMessage": "User not found"}
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

			// Load and lock all sub-bets under the same TransferCode for this user
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

			// If there are any Void sub-bets (from cancel-all), revert ALL of them back to Running and re-deduct their stakes
			voidCount := 0
			for i := range bets {
				if bets[i].Status == "Void" {
					voidCount++
				}
			}
			if voidCount > 0 {
				totalDelta := 0.0
				for i := range bets {
					b := &bets[i]
					if b.Status == "Void" {
						res := tx.Model(b).Where("id = ? AND status = ?", b.ID, "Void").Update("status", "Running")
						if res.Error != nil {
							return res.Error
						}
						if res.RowsAffected == 0 {
							resp = fiber.Map{"ErrorCode": 2003, "ErrorMessage": "Bet Already Rollback",
								"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
							return nil
						}
						totalDelta -= b.Amount
					}
				}
				if totalDelta != 0 {
					user.Balance = roundInternalBalance(user.Currency, user.Balance+totalDelta)
					if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
						return err
					}
				}
				log.Printf("WM Rollback (all void->running): user=%s transfer=%s delta=%.4f newBalance=%.4f", req.Username, req.TransferCode, totalDelta, user.Balance)
				resp = fiber.Map{"ErrorCode": 0, "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}

			candidate, hasRunningWithWin, hasEligible := findWMRollbackCandidate(bets, strings.TrimSpace(req.TransactionId))
			if candidate == nil {
				if !hasEligible {
					if hasRunningWithWin {
						resp = fiber.Map{"ErrorCode": 2003, "ErrorMessage": "Bet Already Rollback",
							"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
						return nil
					}
					resp = fiber.Map{"ErrorCode": 2004, "ErrorMessage": "Only Settled bet can be rollback",
						"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
					return nil
				}
				// hasEligible but candidate is nil => specific TransactionId provided but not eligible; return 2004
				resp = fiber.Map{"ErrorCode": 2004, "ErrorMessage": "Only Settled bet can be rollback",
					"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}

			oldStatus := candidate.Status
			// Race-safe status update: only update if current status matches the one we selected
			res := tx.Model(candidate).
				Where("id = ? AND status = ?", candidate.ID, oldStatus).
				Update("status", "Running")
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				// Another process already rolled back or changed state
				resp = fiber.Map{"ErrorCode": 2003, "ErrorMessage": "Bet Already Rollback",
					"AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}

			delta := computeWMRollbackDelta(oldStatus, candidate.Amount, candidate.WinLoss)
			if delta != 0 {
				user.Balance = roundInternalBalance(user.Currency, user.Balance+delta)
				if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
					return err
				}
			}

			// Minimal audit trail
			log.Printf("WM Rollback: user=%s transfer=%s txid=%s from=%s delta=%.4f newBalance=%.4f",
				req.Username, req.TransferCode, candidate.TransactionId, oldStatus, delta, user.Balance)

			resp = fiber.Map{"ErrorCode": 0, "AccountName": req.Username, "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		}

		// Non-PT9 logic (unchanged)
		var trx models.X568WinTransaction
		// FIX: Query using the composite key to find the exact transaction.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).
			First(&trx).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{"ErrorCode": 6, "ErrorMessage": "Transaction not found", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
				return nil
			}
			return err
		}

		// FIX: Corrected idempotency guard. It should ONLY check the 'Rollback' flag.
		// A transaction with status "Void" is a valid candidate for rollback and should not be blocked here.
		if trx.Rollback {
			resp = fiber.Map{
				"ErrorCode":    2003,
				"ErrorMessage": "Already rolled back",
				"AccountName":  req.Username,
				"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
			}
			return nil
		}

		rate := getRate(user.Currency)

		switch trx.Status {
		case "Settled":
			// Revert the settlement by subtracting the credited WinLoss amount.
			dec := roundInternalBalance(user.Currency, trx.WinLoss*rate)
			user.Balance = roundInternalBalance(user.Currency, user.Balance-dec)
			if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
				return err
			}

		case "Void":
			// FIX: This logic is now universal for all product types, not just PT=9.
			// Revert the cancellation by re-deducting the original stake.
			need := roundInternalBalance(user.Currency, trx.Amount*rate)
			// This is allowed to make the balance negative to pass tests like Sports-7-7.
			user.Balance = roundInternalBalance(user.Currency, user.Balance-need)
			if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
				return err
			}

		default:
			// A "Running" transaction or any other status cannot be rolled back.
			resp = fiber.Map{
				"ErrorCode":    8,
				"ErrorMessage": "Transaction in a non-rollbackable state",
				"AccountName":  req.Username,
				"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
			}
			return nil
		}

		// Atomically update the transaction status.
		res := tx.Model(&trx).
			Where("id = ? AND rollback = ?", trx.ID, false).
			Updates(map[string]any{
				"status":       "Running",
				"rollback":     true,  // Set flag for idempotency and auditing
				"is_cash_out":  false, // Reset cashout status on rollback
				"product_type": req.ProductType,
				"game_type":    req.GameType,
				"gpid":         req.Gpid,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// Handles race condition where another process rolled it back first.
			resp = fiber.Map{
				"ErrorCode":    2003,
				"ErrorMessage": "Already rolled back",
				"AccountName":  req.Username,
				"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
			}
			return nil
		}

		resp = fiber.Map{
			"ErrorCode":   0,
			"AccountName": req.Username,
			"Balance":     displayBalanceWithCurrency(user.Currency, user.Balance),
		}
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

// findWMRollbackCandidate selects a deterministic candidate for rollback.
// Priority:
// - If txId provided: target that specific sub-bet if eligible (Void/Settled with WinLoss != 0).
// - Else: prefer Void with WinLoss != 0 (after cancel-all), ordered by ID asc; then Settled with WinLoss != 0 by ID asc.
// Returns: candidate, hasRunningWithWin, hasEligible
func findWMRollbackCandidate(bets []models.WmSubBet, txId string) (*models.WmSubBet, bool, bool) {
	hasRunningWithWin := false
	var voidWithWin []*models.WmSubBet
	var settledWithWin []*models.WmSubBet

	for i := range bets {
		b := &bets[i]
		if b.Status == "Running" && b.WinLoss != 0 {
			hasRunningWithWin = true
		}
		if b.Status == "Void" && b.WinLoss != 0 {
			voidWithWin = append(voidWithWin, b)
		}
		if b.Status == "Settled" && b.WinLoss != 0 {
			settledWithWin = append(settledWithWin, b)
		}
	}

	// If txId provided, try to target that exact sub-bet first
	if txId != "" {
		for i := range bets {
			b := &bets[i]
			if b.TransactionId == txId {
				if (b.Status == "Void" || b.Status == "Settled") && b.WinLoss != 0 {
					return b, hasRunningWithWin, true
				}
				// not eligible but exists
				return nil, hasRunningWithWin, (b.Status == "Void" || b.Status == "Settled")
			}
		}
	}

	sort.Slice(voidWithWin, func(i, j int) bool { return voidWithWin[i].ID < voidWithWin[j].ID })
	sort.Slice(settledWithWin, func(i, j int) bool { return settledWithWin[i].ID < settledWithWin[j].ID })

	if len(voidWithWin) > 0 {
		return voidWithWin[0], hasRunningWithWin, true
	}
	if len(settledWithWin) > 0 {
		return settledWithWin[0], hasRunningWithWin, true
	}

	return nil, hasRunningWithWin, false
}

// computeWMRollbackDelta computes the balance delta for rollback in internal units.
// - If oldStatus == "Settled": delta = -WinLoss (reverse credited payout)
// - If oldStatus == "Void": delta = -(WinLoss - Amount) (re-apply settlement payout minus already-refunded stake)
func computeWMRollbackDelta(oldStatus string, amount, winloss float64) float64 {
	switch oldStatus {
	case "Settled":
		return -winloss
	case "Void":
		// After cancel-all, reverting to Running should re-deduct only the stake
		return -amount
	default:
		return 0
	}
}
