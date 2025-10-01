package middlewares

import (
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

func UserAuthMiddleware(c *fiber.Ctx) error {
	agentCode := c.Get("X-Agent-Code")
	secretKey := c.Get("X-Secret-Key")

	if agentCode == "" || secretKey == "" {
		return helpers.JSONError(c, "AGENT_CODE_AND_SECRET_REQUIRED")
	}

	var agent models.Agent
	if err := database.DB.Where("agent_code = ? AND secret_key = ? AND is_active = true", agentCode, secretKey).First(&agent).Error; err != nil {
		return helpers.JSONError(c, "INVALID_AGENT_CREDENTIALS")
	}

	c.Locals("agent", agent)
	return c.Next()
}
