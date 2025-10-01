package user

import (
	"strings"
	"telo/helpers"
	"telo/providers"

	"github.com/gofiber/fiber/v2"
)

func LaunchGameHandler(c *fiber.Ctx) error {
	var req providers.LaunchRequest

	if err := c.BodyParser(&req); err != nil {
		return helpers.JSONError(c, "INVALID_JSON")
	}

	launcher := providers.GetProvider(req.ProviderCode)
	if launcher == nil {
		return helpers.JSONError(c, "UNSUPPORTED_PROVIDER")
	}

	launchURL, err := launcher.StartGame(req)
	if err != nil {
		return helpers.JSONError(c, "FAILED_TO_START_GAME: "+err.Error())
	}

	// 🔧 Normalisasi supaya selalu pakai https://
	if strings.HasPrefix(launchURL, "//") {
		launchURL = "https:" + launchURL
	}

	return helpers.JSONSuccess(c, "Game launched successfully", fiber.Map{
		"launch_url": launchURL,
	})
}
