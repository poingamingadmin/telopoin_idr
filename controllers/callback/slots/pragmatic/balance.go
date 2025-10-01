package pragmatic

import (
	"log"
	"net/http"
	"strings"
	"time"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

func Balance(c *fiber.Ctx) error {
	start := time.Now()

	log.Printf("📥 [PragmaticBalance] Raw Body: %s", string(c.Body()))

	ct := strings.ToLower(c.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"currency":    "IDR",
			"cash":        0.0,
			"bonus":       0.0,
			"error":       1000,
			"description": "Invalid content type",
		})
	}

	providerId := c.FormValue("providerId")
	userId := c.FormValue("userId")
	hash := c.FormValue("hash")

	if providerId == "" || userId == "" || hash == "" {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"currency":    "IDR",
			"cash":        0.0,
			"bonus":       0.0,
			"error":       1001,
			"description": "Missing required parameters",
		})
	}

	// TODO: kalau mau, validasi providerId & hash di sini
	// ...

	var user models.User
	if err := database.DB.Where("user_code = ?", userId).First(&user).Error; err != nil {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"currency":    "IDR",
			"cash":        0.0,
			"bonus":       0.0,
			"error":       2001,
			"description": "User not found",
		})
	}

	if !user.IsActive {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"currency":    user.Currency,
			"cash":        0.0,
			"bonus":       0.0,
			"error":       2002,
			"description": "User inactive",
		})
	}

	log.Printf("[PRAGMATIC] ✅ Balance success | user=%s | balance=%.2f | duration=%v",
		user.UserCode, user.Balance, time.Since(start))

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"currency":    user.Currency,
		"cash":        user.Balance,
		"bonus":       0.0,
		"error":       0,
		"description": "Success",
	})
}

func intPtr(v int) *int32 {
	val := int32(v)
	return &val
}

func strPtr(s string) *string {
	return &s
}
