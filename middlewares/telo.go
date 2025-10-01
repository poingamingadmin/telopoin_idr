package middlewares

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

func TeloAgentAuth() fiber.Handler {
	expectedCode := os.Getenv("TELO_AGENT_CODE")
	expectedSecret := os.Getenv("TELO_AGENT_SECRET")

	return func(c *fiber.Ctx) error {
		var body struct {
			AgentCode   string `json:"agent_code"`
			AgentSecret string `json:"agent_secret"`
		}

		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": 0,
				"msg":    "INVALID_JSON",
			})
		}

		if body.AgentCode != expectedCode || body.AgentSecret != expectedSecret {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status": 0,
				"msg":    "INVALID_AGENT_CREDENTIALS",
			})
		}

		return c.Next()
	}
}
