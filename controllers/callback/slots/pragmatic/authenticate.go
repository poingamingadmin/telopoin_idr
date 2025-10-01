package pragmatic

import (
	"log"
	"strings"
	"time"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

type AuthenticateRequest struct {
	ProviderID string `form:"providerId"`
	Token      string `form:"token"`
	Hash       string `form:"hash"`
	GameID     string `form:"gameId,omitempty"`
	IpAddress  string `form:"ipAddress,omitempty"`
}

func AuthenticateHandler(c *fiber.Ctx) error {
	start := time.Now()

	var req AuthenticateRequest

	log.Printf("📥 [PragmaticAuth] Raw Body: %s", string(c.Body()))

	if err := c.BodyParser(&req); err != nil {
		log.Printf("[PRAGMATIC] ❌ Failed to parse request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":       1000,
			"description": "INVALID PARAMETER",
		})
	}

	// 🔍 Validasi required fields
	if req.ProviderID == "" || req.Token == "" || req.Hash == "" {
		log.Printf("[PRAGMATIC] ❌ Missing required fields")
		return c.JSON(fiber.Map{
			"error":       1001,
			"description": "Missing required parameters",
		})
	}

	// 🔍 Validasi providerId
	if req.ProviderID != "pragmaticplay" {
		log.Printf("[PRAGMATIC] ❌ Invalid providerId: %s", req.ProviderID)
		return c.JSON(fiber.Map{
			"error":       1002,
			"description": "Invalid providerId",
		})
	}

	// TODO: validasi hash (sama kayak BalanceHandler, bisa pakai verifyHashPragmatic)

	// 🔍 Cari user dari token
	var user models.User
	if err := database.DB.Where("user_code = ?", req.Token).First(&user).Error; err != nil {
		log.Printf("[PRAGMATIC] ❌ User not found: %s", req.Token)
		return c.JSON(fiber.Map{
			"error":       2001,
			"description": "User not found",
		})
	}

	if !user.IsActive {
		log.Printf("[PRAGMATIC] ❌ User inactive: %s", req.Token)
		return c.JSON(fiber.Map{
			"error":       2002,
			"description": "User inactive",
		})
	}

	log.Printf("[PRAGMATIC] ✅ Auth Success | user=%s | balance=%.2f | duration=%v",
		user.UserCode, user.Balance, time.Since(start))

	// 🔹 Response sukses
	return c.JSON(fiber.Map{
		"userId":      user.UserCode,
		"currency":    strings.ToUpper(user.Currency),
		"cash":        user.Balance,
		"bonus":       0.0,
		"country":     user.Country,
		"token":       req.Token,
		"error":       0,
		"description": "Success",
	})
}
