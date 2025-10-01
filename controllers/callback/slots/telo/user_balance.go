package telo

import (
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

type UserBalanceRequest struct {
	UserCode string `json:"user_code"`
}

func CheckUserBalance(c *fiber.Ctx) error {
	var req UserBalanceRequest
	if err := c.BodyParser(&req); err != nil {
		return helpers.TeloError(c, "INVALID_JSON")
	}

	var user models.User
	err := database.DB.Where("user_code = ? AND is_active = true", req.UserCode).First(&user).Error
	if err != nil {
		return helpers.TeloError(c, "INVALID_USER")
	}

	return helpers.TeloSuccess(c, int64(user.Balance))
}
