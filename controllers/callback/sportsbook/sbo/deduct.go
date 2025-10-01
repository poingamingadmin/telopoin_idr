// file: sportsbook/sbo/deduct.go
package sbo

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeductRequest struct {
	CompanyKey      string         `json:"CompanyKey"`
	Username        string         `json:"Username"`
	Amount          float64        `json:"Amount"`
	TransferCode    string         `json:"TransferCode"`
	TransactionId   string         `json:"TransactionId"`
	BetTime         string         `json:"BetTime"`
	ProductType     int            `json:"ProductType"`
	GameType        int            `json:"GameType"`
	GameRoundId     *string        `json:"GameRoundId"`
	GamePeriodId    *string        `json:"GamePeriodId"`
	OrderDetail     string         `json:"OrderDetail"`
	PlayerIp        *string        `json:"PlayerIp"`
	GameTypeName    *string        `json:"GameTypeName"`
	Gpid            int            `json:"Gpid"`
	GameId          int            `json:"GameId"`
	ExtraInfo       map[string]any `json:"ExtraInfo"`
	CommissionStake float64        `json:"CommissionStake"`
}

const eps = 1e-6

func isIncremental(pt int) bool {
	return pt == 3 || pt == 7 || pt == 9
}

func betAmountForResponse(req DeductRequest) float64 {
	if req.CommissionStake > 0 {
		return req.CommissionStake
	}
	return req.Amount
}

func parsedBetTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Now()
}

func attachCommissionStake(info map[string]any, stake float64) map[string]any {
	if stake <= 0 {
		return info
	}
	if info == nil {
		info = make(map[string]any)
	}
	info["commissionStake"] = stake
	return info
}

func createNewTransaction(tx *gorm.DB, user *models.User, req *DeductRequest, rate float64) (fiber.Map, error) {
	neededBalance := roundInternalBalance(user.Currency, req.Amount*rate)
	if neededBalance > user.Balance+eps {
		return fiber.Map{"ErrorCode": 5, "ErrorMessage": "Insufficient balance", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}, nil
	}

	user.Balance = roundInternalBalance(user.Currency, user.Balance-neededBalance)
	if err := tx.Model(user).Update("balance", user.Balance).Error; err != nil {
		return nil, err
	}

	betTime := parsedBetTime(req.BetTime)
	extraInfoJSON, _ := json.Marshal(attachCommissionStake(req.ExtraInfo, req.CommissionStake))

	newTx := models.X568WinTransaction{
		CompanyKey:    req.CompanyKey,
		Username:      req.Username,
		Amount:        req.Amount,
		TransferCode:  req.TransferCode,
		TransactionId: req.TransactionId,
		ProductType:   req.ProductType,
		GameType:      req.GameType,
		GameRoundId:   req.GameRoundId,
		GamePeriodId:  req.GamePeriodId,
		PlayerIp:      req.PlayerIp,
		GameTypeName:  req.GameTypeName,
		Gpid:          req.Gpid,
		GameId:        req.GameId,
		BetTime:       betTime,
		ExtraInfo:     extraInfoJSON,
		Status:        "Running",
		WinLoss:       0,
	}

	if err := tx.Create(&newTx).Error; err != nil {
		return nil, err
	}

	return fiber.Map{"ErrorCode": 0, "AccountName": req.Username, "BetAmount": betAmountForResponse(*req), "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}, nil
}

func DeductHandler(c *fiber.Ctx) error {
	var req DeductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    422,
			"ErrorMessage": "Invalid request format",
		})
	}

	req.Username = strings.TrimSpace(req.Username)
	req.TransferCode = strings.TrimSpace(req.TransferCode)
	if req.Username == "" || req.TransferCode == "" || req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    422,
			"ErrorMessage": "Username, TransferCode, and Amount are required",
		})
	}

	var user models.User
	var resp fiber.Map
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
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

		if req.ProductType == 9 {
			// --- Validate input ---
			if len(req.Username) == 0 || len(req.TransferCode) == 0 || len(req.TransactionId) == 0 || req.Amount <= 0 {
				resp = fiber.Map{
					"ErrorCode":    3,
					"ErrorMessage": "Invalid request format",
				}
				return nil
			}

			var user models.User
			if err := tx.Where("user_code = ?", req.Username).First(&user).Error; err != nil {
				resp = fiber.Map{
					"ErrorCode":    1,
					"ErrorMessage": "User not found",
					"Balance":      0,
				}
				return nil
			}

			// Check for duplicate transaction
			var existingBet models.WmSubBet
			err := tx.Where("transaction_id = ?", req.TransactionId).First(&existingBet).Error

			if err == nil {
				// Transaction ID already exists
				resp = fiber.Map{
					"ErrorCode":    5003,
					"ErrorMessage": "Duplicate Transaction",
					"AccountName":  req.Username,
					"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
				}
				return nil
			}

			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err // Database error
			}

			// Convert amount to internal value and check balance
			internalAmount := convertToInternalValueWithCurrency(user.Currency, req.Amount)

			if user.Balance < internalAmount {
				resp = fiber.Map{
					"ErrorCode":    5,
					"ErrorMessage": "Insufficient Balance",
					"AccountName":  req.Username,
					"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
				}
				return nil
			}

			// Create the bet record
			bet := models.WmSubBet{
				UserCode:      user.UserCode,
				Username:      req.Username,
				TransferCode:  req.TransferCode,
				TransactionId: req.TransactionId,
				GameType:      req.GameType,
				GameId:        req.GameId,
				Amount:        internalAmount, // Use converted internal value
				Status:        "Running",
				BetTime:       req.BetTime,
				OrderDetail:   req.OrderDetail,
			}

			if err := tx.Create(&bet).Error; err != nil {
				return err
			}

			// Update user balance
			user.Balance = roundInternalBalance(user.Currency, user.Balance-internalAmount)
			if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
				return err
			}

			resp = fiber.Map{
				"ErrorCode":   0,
				"AccountName": req.Username,
				"BetAmount":   req.Amount, // Return original amount, not internal value
				"Balance":     displayBalanceWithCurrency(user.Currency, user.Balance),
			}
			return nil
		}

		var trx models.X568WinTransaction
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).
			First(&trx).Error

		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			rate := getRate(user.Currency)
			newResp, err := createNewTransaction(tx, &user, &req, rate)
			if err != nil {
				return err
			}
			resp = newResp
			return nil
		}

		if findErr != nil {
			return findErr
		}

		if trx.Status != "Running" {
			resp = fiber.Map{"ErrorCode": 5003, "ErrorMessage": "Duplicate TransferCode", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
			return nil
		}

		if isIncremental(req.ProductType) {

			if isIncremental(req.ProductType) {
				if math.Abs(req.Amount-trx.Amount) <= eps {
					resp = fiber.Map{
						"ErrorCode":   0,
						"AccountName": req.Username,
						"BetAmount":   betAmountForResponse(req),
						"Balance":     displayBalanceWithCurrency(user.Currency, user.Balance),
					}
					return nil
				}

				if req.Amount < trx.Amount-eps {
					resp = fiber.Map{
						"ErrorCode":    7,
						"ErrorMessage": "Amount lower than previous for same TransferCode",
						"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
					}
					return nil
				}

				rate := getRate(user.Currency)
				diff := roundInternalBalance(user.Currency, (req.Amount-trx.Amount)*rate)

				if diff > user.Balance+eps {
					resp = fiber.Map{
						"ErrorCode":    5,
						"ErrorMessage": "Insufficient balance",
						"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
					}
					return nil
				}

				user.Balance = roundInternalBalance(user.Currency, user.Balance-diff)
				if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
					return err
				}
				if err := tx.Model(&trx).Update("amount", req.Amount).Error; err != nil {
					return err
				}

				resp = fiber.Map{
					"ErrorCode":   0,
					"AccountName": req.Username,
					"BetAmount":   betAmountForResponse(req),
					"Balance":     displayBalanceWithCurrency(user.Currency, user.Balance),
				}
				return nil
			}

			resp = fiber.Map{
				"ErrorCode":    5003,
				"ErrorMessage": "Duplicate Transaction",
				"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
			}
			return errors.New("duplicate transaction")
		}

		resp = fiber.Map{"ErrorCode": 5003, "ErrorMessage": "Duplicate TransferCode", "Balance": displayBalanceWithCurrency(user.Currency, user.Balance)}
		return nil
	})

	if txErr != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"ErrorCode":    5003,
			"ErrorMessage": "Duplicate Transaction",
			"Balance":      displayBalanceWithCurrency(user.Currency, user.Balance),
		})
	}
	return c.JSON(resp)
}
