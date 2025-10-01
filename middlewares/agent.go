package middlewares

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/gofiber/fiber/v2"
)

func AgentAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Signature string `json:"signature"`
		}

		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": 0,
				"msg":    "INVALID_JSON",
			})
		}

		masterCode := os.Getenv("MASTER_AGENT_CODE")
		masterSecret := os.Getenv("MASTER_AGENT_SECRET")

		data := masterCode + masterSecret

		h := hmac.New(sha256.New, []byte(masterSecret))
		h.Write([]byte(data))
		expectedSignature := hex.EncodeToString(h.Sum(nil))

		if body.Signature != expectedSignature {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status": 0,
				"msg":    "INVALID_SIGNATURE",
			})
		}

		return c.Next()
	}
}
