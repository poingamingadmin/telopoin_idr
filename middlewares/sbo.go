package middlewares

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

func SboAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			CompanyKey string `json:"CompanyKey"`
		}

		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"AccountName":  "",
				"Balance":      0,
				"ErrorCode":    422,
				"ErrorMessage": "INVALID_JSON",
			})
		}

		if body.CompanyKey != os.Getenv("WIN568_COMPANY_KEY") {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"ErrorCode":    4,
				"ErrorMessage": "CompanyKey Error",
				"Balance":      0,
			})
		}

		return c.Next()
	}
}
