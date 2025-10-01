package sbo

import (
	"errors"
	"math"
	"strings"
	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GetBalanceRequest struct {
	CompanyKey string `json:"CompanyKey"`
	Username   string `json:"Username"`
}

func GetMemberBalanceHandler(c *fiber.Ctx) error {
	var req GetBalanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    422,
			"ErrorMessage": "Invalid request format",
		})
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    422,
			"ErrorMessage": "Username is required",
		})
	}

	var resp fiber.Map

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_code = ?", req.Username).
			First(&user).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				resp = fiber.Map{
					"ErrorCode":    1,
					"ErrorMessage": "User not found",
					"Balance":      0,
				}
				return nil
			}
			return err
		}

		if err := normalizeAndPersist(tx, &user); err != nil {
			return err
		}

		resp = fiber.Map{
			"ErrorCode":   0,
			"AccountName": req.Username,
			"Currency":    user.Currency,
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

func normalizeCurrency(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// Jumlah decimal tampilan berdasarkan mata uang
func decimalsForCurrency(currency string) int {
	switch normalizeCurrency(currency) {
	case "IDR", "VND":
		return 0
	default:
		return 2
	}
}

func getRate(currency string) float64 {
	switch normalizeCurrency(currency) {
	case "IDR", "VND":
		return 1000
	default:
		return 1
	}
}

func roundTo(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

func roundInternalBalance(currency string, v float64) float64 {
	return roundTo(v, decimalsForCurrency(currency))
}

func normalizeAndPersist(tx *gorm.DB, u *models.User) error {
	before := u.Balance
	u.Balance = roundInternalBalance(u.Currency, u.Balance)
	if u.Balance > -1e-9 && u.Balance < 0 {
		u.Balance = 0
	}
	if math.Abs(u.Balance-before) < 1e-9 {
		return nil
	}
	return tx.Model(u).Where("id = ?", u.ID).Update("balance", u.Balance).Error
}

func displayBalanceWithCurrency(currency string, internalBalance float64) float64 {
	return internalBalance / getRate(currency)
}

func convertToInternalValueWithCurrency(currency string, displayValue float64) float64 {
	return displayValue * getRate(currency)
}
