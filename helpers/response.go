package helpers

import (
	"math"

	"github.com/gofiber/fiber/v2"
)

func JSONSuccess(c *fiber.Ctx, message string, data any) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": message,
		"data":    data,
	})
}

func JSONError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"success": false,
		"message": message,
		"data":    nil,
	})
}

func FormatFloat(num float64, precision int) float64 {
	pow := math.Pow(10, float64(precision))
	return math.Round(num*pow) / pow
}
