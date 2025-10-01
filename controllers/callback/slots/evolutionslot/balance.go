package evolutionslot

import (
	"errors"
	"log"
	"telo/helpers"
	"telo/models"
	"time"

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
	start := time.Now()
	db := c.Locals("db").(*gorm.DB)

	log.Println("🚀 [EVOLUTIONLIVE] ===== Incoming Balance Request =====")

	// === 1. Log raw body ===
	rawBody := string(c.Body())
	log.Printf("[EVOLUTIONLIVE] 📩 Raw Body: %s", rawBody)

	// === 2. Parse request ===
	var req BalanceRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("❌ [EVOLUTIONLIVE] Failed to parse balance request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "FAIL",
			"message": "INVALID PARAMETER",
			"uuid":    req.UUID,
		})
	}

	// === 3. Log hasil parsing ===
	log.Printf("[EVOLUTIONLIVE] 🧪 Parsed BalanceRequest => UserID=%s | SID=%s | Currency=%s | UUID=%s",
		req.UserID, req.SID, req.Currency, req.UUID)

	if req.Game != nil && req.Game.Details != nil && req.Game.Details.Table != nil {
		log.Printf("[EVOLUTIONLIVE] 🎮 Game Info => Type=%s | TableID=%s | VID=%s",
			req.Game.Type, req.Game.Details.Table.ID, req.Game.Details.Table.VID)
	} else {
		log.Println("[EVOLUTIONLIVE] ⚠️ No game details provided in request.")
	}

	// === 4. Cek user ===
	var user models.User
	if err := db.Where("user_code = ?", req.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("❌ [EVOLUTIONLIVE] User not found: %s", req.UserID)
		} else {
			log.Printf("❌ [EVOLUTIONLIVE] Database error while fetching user: %v", err)
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "FAIL",
			"message": "INVALID TOKEN ID",
			"uuid":    req.UUID,
		})
	}
	log.Printf("✅ [EVOLUTIONLIVE] User found: ID=%d | Code=%s | Balance=%.2f | Country=%s | Currency=%s",
		user.ID, user.UserCode, user.Balance, user.Country, user.Currency)

	// === 5. Cek session ===
	var session models.Session
	if err := db.Where("s_id = ? AND user_id = ?", req.SID, user.ID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("❌ [EVOLUTIONLIVE] Session not found: SID=%s | User=%s", req.SID, req.UserID)
		} else {
			log.Printf("❌ [EVOLUTIONLIVE] Database error while fetching session: %v", err)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "FAIL",
			"message": "INVALID SID",
			"uuid":    req.UUID,
		})
	}
	log.Printf("✅ [EVOLUTIONLIVE] Session valid: SID=%s | UserID=%d | CreatedAt=%v | ExpiresAt=%v",
		session.SID, session.UserID, session.CreatedAt, session.ExpiresAt)

	// === 6. Kirim response sukses ===
	resp := fiber.Map{
		"status":  "OK",
		"balance": helpers.FormatFloat(user.Balance, 2),
		"uuid":    req.UUID,
	}

	log.Printf("📤 [EVOLUTIONLIVE] Balance Response => %+v | Duration=%v",
		resp, time.Since(start))
	log.Println("🏁 [EVOLUTIONLIVE] ===== Balance Request Completed =====")

	return c.JSON(resp)
}
