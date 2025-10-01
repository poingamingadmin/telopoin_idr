// file: sportsbook/sbo/get_bet_status.go
package sbo

import (
	"errors"
	"strings"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type GetBetStatusRequest struct {
	CompanyKey    string `json:"CompanyKey"`
	Username      string `json:"Username"`
	TransferCode  string `json:"TransferCode"`
	TransactionId string `json:"TransactionId"` // ➕ WAJIB buat WM
	ProductType   int    `json:"ProductType"`   // ➕ buat deteksi WM
}

func GetBetStatusHandler(c *fiber.Ctx) error {
	var req GetBetStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"ErrorCode":    3,
			"ErrorMessage": "Invalid request format",
		})
	}

	req.Username = strings.TrimSpace(req.Username)
	req.TransferCode = strings.TrimSpace(req.TransferCode)
	if req.Username == "" || req.TransferCode == "" {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"ErrorCode":    3,
			"ErrorMessage": "Invalid request format",
		})
	}

	// ==== Khusus WM (ProductType 9) ====
	if req.ProductType == 9 {
		var wmBet models.WmSubBet
		if err := database.DB.
			Where("transfer_code = ? AND transaction_id = ? AND username = ?",
				req.TransferCode, req.TransactionId, req.Username).
			First(&wmBet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.JSON(fiber.Map{"ErrorCode": 6})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"ErrorCode":    7,
				"ErrorMessage": "Database error",
			})
		}

		return c.JSON(fiber.Map{
			"ErrorCode":     0,
			"TransferCode":  wmBet.TransferCode,
			"TransactionId": wmBet.TransactionId,
			"Status":        wmBet.Status,
		})
	}

	// ==== Default non-WM ====
	var trx models.X568WinTransaction
	if err := database.DB.
		Where("transfer_code = ? AND username = ?", req.TransferCode, req.Username).
		First(&trx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(fiber.Map{"ErrorCode": 6})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"ErrorCode":    7,
			"ErrorMessage": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"ErrorCode":     0,
		"TransferCode":  trx.TransferCode,
		"TransactionId": trx.TransactionId,
		"Status":        trx.Status,
	})
}
