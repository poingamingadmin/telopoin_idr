package helpers

import "github.com/gofiber/fiber/v2"

func TeloSuccess(c *fiber.Ctx, userBalance int64) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":       1,
		"user_balance": userBalance,
	})
}

func TeloError(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":       0,
		"user_balance": 0,
		"msg":          msg,
	})
}
