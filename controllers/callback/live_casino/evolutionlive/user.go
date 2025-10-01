package evolutionlive

import (
	"errors"
	"log"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type Channel struct {
	Type string `json:"type"`
}

type CheckUserRequest struct {
	UserID  string  `json:"userId"`
	SID     string  `json:"sid"`
	Channel Channel `json:"channel"`
	UUID    string  `json:"uuid"`
}

func UserHandler(c *fiber.Ctx) error {
	db := c.Locals("db").(*gorm.DB)

	log.Println("[EVOLUTIONLIVE] 📥 Incoming request to UserHandler (Check User)")

	// === 1. Log body mentah yang diterima ===
	rawBody := c.Body()
	log.Printf("[EVOLUTIONLIVE] 📩 Raw Request Body: %s", string(rawBody))

	// === 2. Parse request JSON ===
	var req CheckUserRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("[EVOLUTIONLIVE] ❌ Failed to parse request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "INVALID_PARAMETER",
			"uuid":   req.UUID,
		})
	}

	// === 3. Log hasil parsing ===
	log.Printf("[EVOLUTIONLIVE] 🧪 Parsed Request => UserID=%s | SID=%s | ChannelType=%s | UUID=%s",
		req.UserID, req.SID, req.Channel.Type, req.UUID)

	// === 4. Cek apakah user ada ===
	var user models.User
	if err := db.Where("user_code = ?", req.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[EVOLUTIONLIVE] ❌ User not found: %s", req.UserID)
		} else {
			log.Printf("[EVOLUTIONLIVE] ❌ Database error while fetching user: %v", err)
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status": "INVALID_TOKEN_ID",
			"uuid":   req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] ✅ User found: ID=%d | Code=%s | Balance=%.2f",
		user.ID, user.UserCode, user.Balance)

	// === 5. Cek apakah session valid ===
	var session models.Session
	if err := db.Where("s_id = ?", req.SID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[EVOLUTIONLIVE] ❌ Session not found for SID=%s | User=%s", req.SID, req.UserID)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status": "INVALID_SID",
				"uuid":   req.UUID,
			})
		}

		log.Printf("[EVOLUTIONLIVE] ❌ Database error while fetching session: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "INTERNAL_ERROR",
			"uuid":   req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] ✅ Session check passed: SID=%s | UserID=%d | CreatedAt=%v",
		session.SID, session.UserID, session.CreatedAt)

	// === 6. Kirim response akhir ===
	resp := fiber.Map{
		"status": "OK",
		"sid":    session.SID,
		"uuid":   req.UUID,
	}

	log.Printf("[EVOLUTIONLIVE] 📤 Response for UserID=%s: %+v", req.UserID, resp)
	return c.JSON(resp)
}
