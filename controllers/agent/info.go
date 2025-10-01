package agent

import (
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

func AgentInfo(c *fiber.Ctx) error {
	agentCode := c.Get("X-Agent-Code")
	secretKey := c.Get("X-Secret-Key")

	var agent models.Agent
	if err := database.DB.Where("agent_code = ? AND secret_key = ? AND is_active = true", agentCode, secretKey).
		First(&agent).Error; err != nil {
		return helpers.JSONError(c, "INVALID_AGENT_CREDENTIALS")
	}

	var totalUserBalance float64
	err := database.DB.Model(&models.User{}).
		Where("agent_code = ?", agent.AgentCode).
		Select("COALESCE(SUM(balance),0)").Scan(&totalUserBalance).Error

	if err != nil {
		return helpers.JSONError(c, "FAILED_TO_FETCH_USER_BALANCE")
	}

	return helpers.JSONSuccess(c, "Agent info retrieved successfully", fiber.Map{
		"username":           agent.Username,
		"agent_code":         agent.AgentCode,
		"agent_balance":      agent.Balance,
		"total_user_balance": totalUserBalance,
		"currency":           agent.Currency,
	})
}
