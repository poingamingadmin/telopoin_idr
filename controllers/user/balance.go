package user

import (
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

type CheckBalanceRequest struct {
	UserCode string `json:"user_code"`
}

func CheckUserBalance(c *fiber.Ctx) error {
	var req CheckBalanceRequest
	if err := c.BodyParser(&req); err != nil {
		return helpers.JSONError(c, "INVALID_JSON")
	}

	if req.UserCode == "" {
		return helpers.JSONError(c, "USER_CODE_REQUIRED")
	}

	agent, ok := c.Locals("agent").(models.Agent)
	if !ok {
		return helpers.JSONError(c, "INVALID_AGENT_SESSION")
	}

	var user models.User
	if err := database.DB.Where("user_code = ? AND agent_code = ? AND is_active = true", req.UserCode, agent.AgentCode).First(&user).Error; err != nil {
		return helpers.JSONError(c, "USER_NOT_FOUND_OR_UNAUTHORIZED")
	}

	return helpers.JSONSuccess(c, "Balance retrieved successfully", fiber.Map{
		"user_code": user.UserCode,
		"balance":   user.Balance,
		"currency":  user.Currency,
	})
}
