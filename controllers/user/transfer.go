package user

import (
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TransferRequest struct {
	UserCode string `json:"user_code"`
	Amount   int64  `json:"amount"`
	Note     string `json:"note"`
}

func TransferBalance(c *fiber.Ctx) error {
	var req TransferRequest
	if err := c.BodyParser(&req); err != nil {
		return helpers.JSONError(c, "INVALID_JSON")
	}

	if req.UserCode == "" || req.Amount == 0 {
		return helpers.JSONError(c, "USER_CODE_AND_AMOUNT_REQUIRED")
	}

	agent, ok := c.Locals("agent").(models.Agent)
	if !ok {
		return helpers.JSONError(c, "INVALID_AGENT_SESSION")
	}

	var user models.User
	if err := database.DB.
		Where("user_code = ? AND agent_code = ? AND is_active = true", req.UserCode, agent.AgentCode).
		First(&user).Error; err != nil {
		return helpers.JSONError(c, "USER_NOT_FOUND_OR_UNAUTHORIZED")
	}

	amountAbs := abs(req.Amount)

	if req.Amount < 0 && user.Balance < float64(amountAbs) {
		return helpers.JSONError(c, "INSUFFICIENT_USER_BALANCE")
	}

	if req.Amount > 0 && int64(agent.Balance) < amountAbs {
		return helpers.JSONError(c, "INSUFFICIENT_AGENT_BALANCE")
	}

	oldUserBalance := user.Balance
	oldAgentBalance := agent.Balance

	user.Balance += float64(req.Amount)
	agent.Balance -= req.Amount

	if err := database.DB.Save(&user).Error; err != nil {
		return helpers.JSONError(c, "FAILED_TO_UPDATE_USER_BALANCE")
	}

	if err := database.DB.Save(&agent).Error; err != nil {
		return helpers.JSONError(c, "FAILED_TO_UPDATE_AGENT_BALANCE")
	}

	trxType := "deposit"
	if req.Amount < 0 {
		trxType = "withdraw"
	}

	note := req.Note
	if note == "" {
		if trxType == "deposit" {
			note = "System deposit via API"
		} else {
			note = "System withdraw via API"
		}
	}

	refID := uuid.New().String()

	_ = database.DB.Create(&models.UserTransaction{
		UserID:        user.ID,
		AgentCode:     user.AgentCode,
		UserCode:      user.UserCode,
		TrxType:       trxType,
		Amount:        amountAbs,
		BalanceBefore: oldUserBalance,
		BalanceAfter:  user.Balance,
		Currency:      user.Currency,
		Note:          note,
		RefID:         refID,
	})

	_ = database.DB.Create(&models.AgentTransaction{
		AgentID:       agent.ID,
		AgentCode:     agent.AgentCode,
		TrxType:       trxType,
		Amount:        amountAbs,
		BalanceBefore: int64(oldAgentBalance),
		BalanceAfter:  int64(agent.Balance),
		Currency:      agent.Currency,
		Note:          note + " (user: " + user.UserCode + ")",
		RefID:         refID,
	})

	return helpers.JSONSuccess(c, "Balance updated successfully", fiber.Map{
		"user_code": user.UserCode,
		"balance":   user.Balance,
		"ref_id":    refID,
	})
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
