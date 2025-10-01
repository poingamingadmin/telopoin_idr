package middlewares

import (
	"os"
	"telo/database"

	"github.com/gofiber/fiber/v2"
)

func CheckEvolutionTokenLive() fiber.Handler {
	expected := os.Getenv("EVOLUTION_AUTH_TOKEN_LIVE")

	return func(c *fiber.Ctx) error {
		if c.Query("authToken") != expected {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  "INVALID_TOKEN_ID",
				"message": "Unauthorized: Invalid Evolution token",
			})
		}

		if database.DB == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "INTERNAL_ERROR",
				"message": "Database connection is not initialized",
			})
		}

		c.Locals("db", database.DB)

		return c.Next()
	}
}
