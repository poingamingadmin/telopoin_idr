package agent

import (
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TopupAgentRequest struct {
	AgentCode string `json:"agent_code"`
	Amount    int64  `json:"amount"`
	Note      string `json:"note"`
}

func TopupAgentBalance(c *fiber.Ctx) error {
	var req TopupAgentRequest
	if err := c.BodyParser(&req); err != nil {
		return helpers.JSONError(c, "INVALID_JSON")
	}

	if req.AgentCode == "" || req.Amount <= 0 {
		return helpers.JSONError(c, "AGENT_CODE_AND_VALID_AMOUNT_REQUIRED")
	}

	var agent models.Agent
	if err := database.DB.Where("agent_code = ? AND is_active = true", req.AgentCode).First(&agent).Error; err != nil {
		return helpers.JSONError(c, "AGENT_NOT_FOUND")
	}

	before := agent.Balance
	amountTopup := float64(req.Amount)
	ggrRate := agent.GGR

	totalTopup := amountTopup / (ggrRate / 100.0)

	agent.Balance = int64(float64(agent.Balance) + totalTopup)

	if err := database.DB.Save(&agent).Error; err != nil {
		return helpers.JSONError(c, "FAILED_TO_UPDATE_BALANCE")
	}

	refID := uuid.New().String()

	note := req.Note
	if note == "" {
		note = "Top-up via API"
	}

	trx := models.AgentTransaction{
		AgentID:       agent.ID,
		AgentCode:     agent.AgentCode,
		TrxType:       "TOP_UP",
		Amount:        req.Amount,
		BalanceBefore: int64(before),
		BalanceAfter:  int64(agent.Balance),
		Currency:      agent.Currency,
		Note:          note,
		RefID:         refID,
	}

	_ = database.DB.Create(&trx)

	return helpers.JSONSuccess(c, "Agent top-up successful", fiber.Map{
		"agent_code":     agent.AgentCode,
		"balance":        agent.Balance,
		"currency":       agent.Currency,
		"ggr":            agent.GGR,
		"ref_id":         refID,
		"note":           note,
		"balance_before": before,
		"balance_after":  agent.Balance,
		"created_at":     trx.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}
