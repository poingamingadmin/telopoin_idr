package evolutionlive

import (
	"log"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BalanceRequest struct {
	SID      string       `json:"sid"`
	UserID   string       `json:"userId"`
	Currency string       `json:"currency"`
	Game     *GamePayload `json:"game"`
	UUID     string       `json:"uuid"`
}

type GamePayload struct {
	Type    string       `json:"type"`
	Details *GameDetails `json:"details"`
}

type GameDetails struct {
	Table *GameTable `json:"table"`
}

type GameTable struct {
	ID  string `json:"id"`
	VID string `json:"vid"`
}

func BalanceHandler(c *fiber.Ctx) error {
	db := c.Locals("db").(*gorm.DB)

	var req BalanceRequest

	// === 1. Terima dan log payload awal ===
	body := c.Body()
	log.Printf("[EVOLUTIONLIVE] 📩 Incoming Balance Request: %s", string(body))

	if err := c.BodyParser(&req); err != nil {
		log.Printf("[EVOLUTIONLIVE] ❌ Failed to parse balance request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "INVALID_PARAMETER",
			"message": "INVALID PARAMETER",
			"uuid":    req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] 🧪 Parsed Request => SID=%s | UserID=%s | Currency=%s | UUID=%s",
		req.SID, req.UserID, req.Currency, req.UUID)

	if req.Game != nil && req.Game.Details != nil && req.Game.Details.Table != nil {
		log.Printf("[EVOLUTIONLIVE] 🎮 Game Info => Type=%s | TableID=%s | VID=%s",
			req.Game.Type, req.Game.Details.Table.ID, req.Game.Details.Table.VID)
	} else {
		log.Printf("[EVOLUTIONLIVE] ⚠️ No game details provided in request.")
	}

	// === 2. Cek apakah user ada ===
	var user models.User
	if err := db.Where("user_code = ?", req.UserID).First(&user).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] ❌ User not found: %s", req.UserID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "INVALID_TOKEN_ID",
			"message": "INVALID TOKEN ID",
			"uuid":    req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] ✅ User found: ID=%d | Code=%s | Balance=%.2f",
		user.ID, user.UserCode, user.Balance)

	// === 3. Cek validitas session ===
	var session models.Session
	if err := db.Where("s_id = ? AND user_id = ?", req.SID, user.ID).First(&session).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] ❌ Invalid session: SID=%s for user %s", req.SID, req.UserID)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "INVALID_SID",
			"message": "INVALID SID",
			"uuid":    req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] ✅ Session valid: SID=%s | UserID=%d | CreatedAt=%v",
		session.SID, session.UserID, session.CreatedAt)

	// === 4. Kirim response sukses ===
	resp := fiber.Map{
		"status":  "OK",
		"balance": helpers.FormatFloat(user.Balance, 2),
		"uuid":    req.UUID,
	}
	log.Printf("[EVOLUTIONLIVE] 📤 Balance response for user %s: %+v", req.UserID, resp)

	return c.JSON(resp)
}
