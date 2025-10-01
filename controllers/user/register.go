package user

import (
	"strings"
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

type RegisterUserRequest struct {
	UserCode string `json:"user_code"`
	Country  string `json:"country"`
	Currency string `json:"currency"`
}

// Mapping country -> daftar currency yang diperbolehkan
var allowedCountryCurrencies = map[string][]string{
	"ID": {"IDR", "USD"},
	"MY": {"MYR", "USD"},
	"TH": {"THB", "USD"},
	"VN": {"VND", "USD"},
	"KH": {"KHR", "USD"},
	"US": {"USD"},
}

func RegisterUser(c *fiber.Ctx) error {
	var req RegisterUserRequest

	if err := c.BodyParser(&req); err != nil {
		return helpers.JSONError(c, "INVALID_JSON")
	}

	agent, ok := c.Locals("agent").(models.Agent)
	if !ok {
		return helpers.JSONError(c, "INVALID_AGENT_SESSION")
	}

	countryKey := strings.ToUpper(strings.TrimSpace(req.Country))
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))

	// cek country valid atau tidak
	allowedCurrencies, ok := allowedCountryCurrencies[countryKey]
	if !ok {
		return helpers.JSONError(c, "UNSUPPORTED_COUNTRY")
	}

	// cek currency valid atau tidak
	validCurrency := false
	for _, ccy := range allowedCurrencies {
		if ccy == currency {
			validCurrency = true
			break
		}
	}
	if !validCurrency {
		return helpers.JSONError(c, "INVALID_CURRENCY_FOR_COUNTRY")
	}

	finalUserCode := strings.ToLower(agent.AgentCode) + "_" + strings.ToLower(req.UserCode)

	var existing models.User
	if err := database.DB.Where("user_code = ?", finalUserCode).First(&existing).Error; err == nil {
		return helpers.JSONError(c, "USER_ALREADY_EXISTS")
	}

	user := models.User{
		UserCode:  finalUserCode,
		AgentCode: agent.AgentCode,
		Country:   countryKey,
		Currency:  currency,
		Balance:   0,
		IsActive:  true,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return helpers.JSONError(c, "FAILED_TO_REGISTER_USER")
	}

	resp := fiber.Map{
		"user_code":  user.UserCode,
		"agent_code": user.AgentCode,
		"country":    user.Country,
		"currency":   user.Currency,
	}

	return helpers.JSONSuccess(c, "User registered successfully", resp)
}
