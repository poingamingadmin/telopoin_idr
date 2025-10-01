package agent

import (
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type RegisterAgentRequest struct {
	Username string  `json:"username"`
	Currency string  `json:"currency"`
	GGR      float64 `json:"ggr"`
}

func RegisterAgent(c *fiber.Ctx) error {
	var req RegisterAgentRequest
	if err := c.BodyParser(&req); err != nil {
		return helpers.JSONError(c, "INVALID_JSON")
	}

	if req.GGR <= 0 {
		req.GGR = 15
	}

	agentCode := helpers.GenerateAgentCode()
	secretKey := uuid.New().String()

	var existing models.Agent
	if err := database.DB.Where("agent_code = ?", agentCode).First(&existing).Error; err == nil {
		return helpers.JSONError(c, "AGENT_CODE_ALREADY_EXISTS")
	}

	agent := models.Agent{
		Username:  req.Username,
		AgentCode: agentCode,
		SecretKey: secretKey,
		Currency:  req.Currency,
		GGR:       req.GGR,
		Balance:   0,
		IsActive:  true,
	}

	if err := database.DB.Create(&agent).Error; err != nil {
		return helpers.JSONError(c, "FAILED_TO_REGISTER_AGENT")
	}

	return helpers.JSONSuccess(c, "Agent registered successfully", fiber.Map{
		"username":   agent.Username,
		"agent_code": agent.AgentCode,
		"secret_key": agent.SecretKey,
		"currency":   agent.Currency,
		"ggr":        agent.GGR,
	})
}
