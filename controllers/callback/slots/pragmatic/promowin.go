// handlers/pragmatic_promowin_controller.go
package pragmatic

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// POST /promoWin.html (x-www-form-urlencoded)
func PromoWin(c *fiber.Ctx) error {
	// Content-Type check
	ct := strings.ToLower(c.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      "USD",
			"cash":          0.0,
			"bonus":         0.0,
			"error":         1000,
			"description":   "Invalid content type",
		})
	}

	// Required params
	providerId := c.FormValue("providerId")
	userId := c.FormValue("userId")
	campaignId := c.FormValue("campaignId")
	campaignType := c.FormValue("campaignType")
	amountStr := c.FormValue("amount")
	currencyReq := c.FormValue("currency")
	reference := c.FormValue("reference")
	hash := c.FormValue("hash")
	timestamp := c.FormValue("timestamp")

	if providerId == "" || userId == "" || campaignId == "" || campaignType == "" ||
		amountStr == "" || currencyReq == "" || reference == "" || hash == "" || timestamp == "" {
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      "USD",
			"cash":          0.0,
			"bonus":         0.0,
			"error":         1001,
			"description":   "Missing required parameters",
		})
	}

	// TODO: validasi providerId & hash kalau perlu

	// Parse amount
	winAmt, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || winAmt < 0 {
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      "USD",
			"cash":          0.0,
			"bonus":         0.0,
			"error":         3002,
			"description":   "Invalid amount",
		})
	}
	winCents := int64(math.Round(winAmt * 100))

	// Idempotency check
	var existed models.UserGameTransaction
	if err := database.DB.Where("provider_tx = ? AND provider = ?", reference, "PRAGMATIC").
		First(&existed).Error; err == nil {
		var user models.User
		_ = database.DB.First(&user, existed.UserID).Error
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"transactionId": existed.ID,
			"currency":      user.Currency,
			"cash":          user.Balance,
			"bonus":         0.0,
			"error":         0,
			"description":   "Success",
		})
	}

	// Transaksikan & lock user
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", userId).First(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      "USD",
			"cash":          0.0,
			"bonus":         0.0,
			"error":         2001,
			"description":   "User not found",
		})
	}
	if !user.IsActive {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      user.Currency,
			"cash":          0.0,
			"bonus":         0.0,
			"error":         2002,
			"description":   "User inactive",
		})
	}

	// (opsional) validasi currencyReq vs user.Currency
	// if strings.ToUpper(currencyReq) != strings.ToUpper(user.Currency) {
	// 	tx.Rollback()
	// 	return c.JSON(fiber.Map{
	// 		"transactionId": "",
	// 		"currency":      user.Currency,
	// 		"cash":          user.Balance,
	// 		"bonus":         0.0,
	// 		"error":         3003,
	// 		"description":   "Currency mismatch",
	// 	})
	// }

	before := user.Balance
	if winAmt > 0 {
		user.Balance += winAmt
		if err := tx.Save(&user).Error; err != nil {
			tx.Rollback()
			return c.JSON(fiber.Map{
				"transactionId": "",
				"currency":      user.Currency,
				"cash":          before,
				"bonus":         0.0,
				"error":         5002,
				"description":   "Failed to update balance",
			})
		}
	}

	// Catat UserGameTransaction
	ugtx := models.UserGameTransaction{
		UserID:        user.ID,
		UserCode:      user.UserCode,
		AgentCode:     user.AgentCode,
		GameID:        "", // campaign-based, no game
		ProviderTx:    reference,
		Provider:      "PRAGMATIC",
		BonusAmount:   winCents,
		Currency:      user.Currency,
		BalanceBefore: before,
		BalanceAfter:  user.Balance,
		Status:        "Settled",
		Note:          "Pragmatic PromoWin " + campaignType + " " + campaignId,
		RefID:         reference,
	}
	if err := tx.Create(&ugtx).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      user.Currency,
			"cash":          before,
			"bonus":         0.0,
			"error":         5003,
			"description":   "Failed to create UserGameTransaction",
		})
	}

	// Catat PragmaticTransaction
	prTx := models.PragmaticTransaction{
		UserID:        strconv.FormatUint(uint64(user.ID), 10),
		Currency:      user.Currency,
		CampaignID:    &campaignId,
		CampaignType:  &campaignType,
		Cash:          decimal.NewFromFloat(user.Balance),
		Amount:        decimal.NewFromFloat(winAmt),
		TotalBalance:  decimal.NewFromFloat(user.Balance),
		Reference:     reference,
		TransactionID: reference,
		Token:         user.UserCode,
		ErrorCode:     intPtr(0),
		Description:   strPtr("Success"),
	}
	if err := tx.Create(&prTx).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      user.Currency,
			"cash":          before,
			"bonus":         0.0,
			"error":         5005,
			"description":   "Failed to create PragmaticTransaction",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.JSON(fiber.Map{
			"transactionId": "",
			"currency":      user.Currency,
			"cash":          before,
			"bonus":         0.0,
			"error":         5006,
			"description":   "Commit failed",
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"transactionId": ugtx.ID,
		"currency":      user.Currency,
		"cash":          user.Balance,
		"bonus":         0.0,
		"error":         0,
		"description":   "Success",
	})
}
