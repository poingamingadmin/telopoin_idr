package evolutionslot

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

	log.Println("🚀 [EVOLUTIONSLOT] ===== Incoming User Authentication Request =====")

	// === 1. Tampilkan raw body yang diterima ===
	rawBody := string(c.Body())
	log.Printf("[EVOLUTIONSLOT] 📩 Raw Request Body: %s", rawBody)

	// === 2. Parsing body JSON ke struct ===
	var req CheckUserRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("❌ [EVOLUTIONSLOT] Failed to parse JSON request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "FAIL",
			"uuid":   req.UUID,
		})
	}

	// === 3. Log hasil parsing ===
	log.Printf("[EVOLUTIONSLOT] 🧪 Parsed Request => UserID=%s | SID=%s | ChannelType=%s | UUID=%s",
		req.UserID, req.SID, req.Channel.Type, req.UUID)

	// === 4. Cek apakah User ada ===
	var user models.User
	if err := db.Where("user_code = ?", req.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("❌ [EVOLUTIONSLOT] User not found: %s", req.UserID)
		} else {
			log.Printf("❌ [EVOLUTIONSLOT] DB error while fetching user: %v", err)
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status": "FAIL",
			"uuid":   req.UUID,
		})
	}

	log.Printf("✅ [EVOLUTIONSLOT] User found: ID=%d | Code=%s | Balance=%.2f | Country=%s | Currency=%s",
		user.ID, user.UserCode, user.Balance, user.Country, user.Currency)

	// === 5. Cek apakah Session valid ===
	var session models.Session
	if err := db.Where("s_id = ?", req.SID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("❌ [EVOLUTIONSLOT] Session not found: SID=%s | User=%s", req.SID, req.UserID)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status": "FAIL",
				"uuid":   req.UUID,
			})
		}
		log.Printf("❌ [EVOLUTIONSLOT] DB error while checking session: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "FAIL",
			"uuid":   req.UUID,
		})
	}

	log.Printf("✅ [EVOLUTIONSLOT] Session valid: SID=%s | UserID=%d | CreatedAt=%v | ExpiresAt=%v",
		session.SID, session.UserID, session.CreatedAt, session.ExpiresAt)

	// === 6. Kirim respons akhir ===
	resp := fiber.Map{
		"status": "OK",
		"sid":    session.SID,
		"uuid":   req.UUID,
	}

	log.Printf("📤 [EVOLUTIONSLOT] Response: %+v", resp)
	log.Println("🏁 [EVOLUTIONSLOT] ===== User Authentication Completed =====")

	return c.JSON(resp)
}
