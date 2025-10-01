package telo

import (
	"fmt"
	"telo/database"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
)

func ProcessSlotTransaction(c *fiber.Ctx) error {
	var txn models.TeloSlotTransaction
	if err := c.BodyParser(&txn); err != nil {
		return helpers.TeloError(c, "INVALID_JSON")
	}

	// Cek transaksi berdasarkan txn_id + txn_type
	var existingTxn models.TeloSlotTransaction
	db := database.DB.Where("txn_id = ? AND slot_txn_type = ?", txn.Slot.TxnID, txn.Slot.TxnType).First(&existingTxn)
	if db.Error == nil {
		// Jika sudah ada transaksi dengan kombinasi ini, abaikan, tapi kembalikan saldo user saat ini
		var user models.User
		if err := database.DB.Where("user_code = ?", txn.UserCode).First(&user).Error; err == nil {
			return helpers.TeloSuccess(c, int64(user.Balance))
		}
		return helpers.TeloError(c, "USER_NOT_FOUND")
	}

	// Cek apakah sudah ada transaksi dgn txn_id saja (berarti debit sudah diproses sebelumnya)
	var previousTxn models.TeloSlotTransaction
	if err := database.DB.Where("txn_id = ?", txn.Slot.TxnID).First(&previousTxn).Error; err == nil {
		// Update saja transaksi lama ini dengan data tambahan sesuai txn_type baru
		var user models.User
		if err := database.DB.Where("user_code = ? AND is_active = true", txn.UserCode).First(&user).Error; err != nil {
			return helpers.TeloError(c, "USER_NOT_FOUND")
		}

		bet, _ := txn.Slot.Bet.ToInt64()
		win, _ := txn.Slot.Win.ToInt64()

		switch txn.Slot.TxnType {
		case "credit":
			user.Balance += float64(win)
		case "debit_credit":
			user.Balance = user.Balance - float64(bet) + float64(win)
		}

		if err := database.DB.Save(&user).Error; err != nil {
			return helpers.TeloError(c, "FAILED_TO_UPDATE_USER")
		}

		// Update field balance & after balance
		previousTxn.UserBalance = models.FlexibleString(fmt.Sprintf("%f", user.Balance))
		previousTxn.Slot.UserAfterBalance = models.FlexibleString(fmt.Sprintf("%f", user.Balance))
		previousTxn.Slot.TxnType = txn.Slot.TxnType
		previousTxn.Slot.Win = txn.Slot.Win
		previousTxn.Slot.Bet = txn.Slot.Bet

		_ = database.DB.Save(&previousTxn)
		return helpers.TeloSuccess(c, int64(user.Balance))
	}

	// Transaksi baru (debit pertama kali)
	var user models.User
	if err := database.DB.Where("user_code = ? AND is_active = true", txn.UserCode).First(&user).Error; err != nil {
		return helpers.TeloError(c, "USER_NOT_FOUND")
	}

	bet, err := txn.Slot.Bet.ToInt64()
	if err != nil {
		return helpers.TeloError(c, "INVALID_BET_AMOUNT")
	}

	win, err := txn.Slot.Win.ToInt64()
	if err != nil {
		return helpers.TeloError(c, "INVALID_WIN_AMOUNT")
	}

	beforeBalance := user.Balance

	switch txn.Slot.TxnType {
	case "debit":
		if int64(user.Balance) < bet {
			return helpers.TeloError(c, "INSUFFICIENT_USER_FUNDS")
		}
		user.Balance -= float64(bet)
	case "credit":
		user.Balance += float64(win)
	case "debit_credit":
		if int64(user.Balance) < bet {
			return helpers.TeloError(c, "INSUFFICIENT_USER_FUNDS")
		}
		user.Balance = user.Balance - float64(bet) + float64(win)
	default:
		return helpers.TeloError(c, "INVALID_TXN_TYPE")
	}

	txn.UserBalance = models.FlexibleString(fmt.Sprintf("%f", user.Balance))
	txn.Slot.UserBeforeBalance = models.FlexibleString(fmt.Sprintf("%f", beforeBalance))
	txn.Slot.UserAfterBalance = models.FlexibleString(fmt.Sprintf("%f", user.Balance))

	if err := database.DB.Create(&txn).Error; err != nil {
		return helpers.TeloError(c, "FAILED_TO_SAVE_TRANSACTION")
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return helpers.TeloError(c, "FAILED_TO_UPDATE_USER")
	}

	return helpers.TeloSuccess(c, int64(user.Balance))
}
