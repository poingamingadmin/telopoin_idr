package fastspin

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const BalanceRatio = 1000.0

// ===== DTOs =====
type TransferRequest struct {
	TransferID   string  `json:"transferId"`
	MerchantCode string  `json:"merchantCode"`
	MerchantTxID string  `json:"merchantTxId,omitempty"`
	AcctID       string  `json:"acctId"`
	Currency     string  `json:"currency"`
	Amount       float64 `json:"amount"`
	Type         int     `json:"type"`
	TicketID     string  `json:"ticketId,omitempty"`
	Channel      string  `json:"channel"`
	GameCode     string  `json:"gameCode"`
	ReferenceID  string  `json:"referenceId,omitempty"`
	PlayerIP     string  `json:"playerIp,omitempty"`
	GameFeature  string  `json:"gameFeature,omitempty"`
	TransferTime string  `json:"transferTime,omitempty"`
	SerialNo     string  `json:"serialNo"`
	SpecialGame  *struct {
		Type     string `json:"type,omitempty"`
		Count    int    `json:"count,omitempty"`
		Sequence int    `json:"sequence,omitempty"`
	} `json:"specialGame,omitempty"`
	RefTicketIds []string `json:"refTicketIds,omitempty"`
}

type TransferResponse struct {
	TransferID   string  `json:"transferId"`
	MerchantTxID string  `json:"merchantTxId,omitempty"`
	AcctID       string  `json:"acctId"`
	Balance      float64 `json:"balance"`
	Code         int     `json:"code"`
	Msg          string  `json:"msg"`
	SerialNo     string  `json:"serialNo"`
}

type GetBalanceRequest struct {
	SerialNo     string  `json:"serialNo"`
	MerchantCode string  `json:"merchantCode"`
	AcctId       string  `json:"acctId"`
	GameCode     *string `json:"gameCode,omitempty"`
}

type AcctInfo struct {
	UserName string  `json:"userName,omitempty"`
	Currency string  `json:"currency"`
	AcctId   string  `json:"acctId"`
	Balance  float64 `json:"balance"`
	SiteId   string  `json:"siteId,omitempty"`
}

type GetBalanceResponse struct {
	AcctInfo     AcctInfo `json:"acctInfo"`
	MerchantCode string   `json:"merchantCode"`
	Msg          string   `json:"msg"`
	Code         int      `json:"code"`
	SerialNo     string   `json:"serialNo"`
}

// ===== Dispatcher =====
func GatewayHandler(c *fiber.Ctx) error {
	headers := c.GetReqHeaders()
	headersJson, _ := json.Marshal(headers)
	log.Printf("[DEBUG] Gateway headers: %s\n", string(headersJson))

	api := strings.ToLower(c.Get("Api"))
	log.Printf("[DEBUG] Routing API=%s\n", api)

	switch api {
	case "getbalance":
		return GetBalanceHandler(c)
	case "transfer":
		return TransferHandler(c)
	default:
		log.Printf("[WARN] Unknown API header: %s\n", api)
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"code": -99,
			"msg":  "Unknown API",
		})
	}
}

// ===== Handlers =====
func GetBalanceHandler(c *fiber.Ctx) error {
	var req GetBalanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"code": -1, "msg": "Invalid request format"})
	}

	req.AcctId = strings.ToLower(strings.TrimSpace(req.AcctId))
	if req.AcctId == "" || req.MerchantCode == "" || req.SerialNo == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"code": -2, "msg": "Missing required fields"})
	}

	var user models.User
	if err := database.DB.Where("user_code = ?", req.AcctId).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(fiber.Map{"code": 1001, "msg": "User not found", "serialNo": req.SerialNo})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"code": 500, "msg": "DB error", "error": err.Error()})
	}

	displayBalance := user.Balance / BalanceRatio

	resp := GetBalanceResponse{
		AcctInfo: AcctInfo{
			UserName: user.UserCode,
			Currency: user.Currency,
			AcctId:   user.UserCode,
			Balance:  displayBalance,
		},
		MerchantCode: req.MerchantCode,
		Msg:          "success",
		Code:         0,
		SerialNo:     req.SerialNo,
	}
	return c.JSON(resp)
}

func TransferHandler(c *fiber.Ctx) error {
	var req TransferRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("[ERROR] Body parse failed: %v\n", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"code": -1, "msg": "Invalid request format"})
	}

	req.AcctID = strings.ToLower(strings.TrimSpace(req.AcctID))
	if req.TransferID == "" || req.AcctID == "" || req.Currency == "" || req.SerialNo == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"code": -2, "msg": "Missing required fields"})
	}

	// Check duplicate transferId
	var existing models.FastSpinTransaction
	if err := database.DB.Where("transfer_id = ?", req.TransferID).First(&existing).Error; err == nil {
		return c.JSON(TransferResponse{
			TransferID:   existing.TransferID,
			MerchantTxID: existing.MerchantTxID,
			AcctID:       existing.AcctID,
			Balance:      existing.BalanceAfter / BalanceRatio,
			Code:         existing.Code,
			Msg:          existing.Msg,
			SerialNo:     existing.SerialNo,
		})
	}

	// Fetch user
	var user models.User
	if err := database.DB.Where("user_code = ?", req.AcctID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(fiber.Map{"code": 1001, "msg": "User not found", "serialNo": req.SerialNo})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"code": 500, "msg": "DB error", "error": err.Error()})
	}

	balanceBefore := user.Balance
	balanceAfter := balanceBefore

	amountInternal := req.Amount * BalanceRatio

	switch req.Type {
	case 1: // place bet
		if user.Balance < amountInternal {
			return c.JSON(fiber.Map{"code": 1002, "msg": "Insufficient balance", "serialNo": req.SerialNo})
		}
		balanceAfter = user.Balance - amountInternal
	case 2: // cancel bet
		var refTx models.FastSpinTransaction
		if err := database.DB.Where("transfer_id = ? AND type = 1", req.ReferenceID).First(&refTx).Error; err != nil {
			return c.JSON(fiber.Map{"code": 109, "msg": "Reference bet not found", "serialNo": req.SerialNo})
		}
		balanceAfter = user.Balance + amountInternal
	case 4: // payout
		balanceAfter = user.Balance + amountInternal
	default:
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"code": -3, "msg": "Invalid transfer type"})
	}

	// Update user balance
	if err := database.DB.Model(&user).Update("balance", balanceAfter).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"code": 500, "msg": "DB update error", "error": err.Error()})
	}

	// Save transaction
	tx := models.FastSpinTransaction{
		TransferID:    req.TransferID,
		MerchantCode:  req.MerchantCode,
		MerchantTxID:  req.MerchantTxID,
		AcctID:        req.AcctID,
		Currency:      req.Currency,
		Amount:        req.Amount,
		Type:          req.Type,
		TicketID:      req.TicketID,
		Channel:       req.Channel,
		GameCode:      req.GameCode,
		ReferenceID:   req.ReferenceID,
		PlayerIP:      req.PlayerIP,
		GameFeature:   req.GameFeature,
		TransferTime:  req.TransferTime,
		SpecialType:   "",
		SpecialCount:  0,
		SpecialSeq:    0,
		RefTicketIds:  strings.Join(req.RefTicketIds, ","),
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Status:        "Success",
		Msg:           "success",
		Code:          0,
		SerialNo:      req.SerialNo,
	}
	if req.SpecialGame != nil {
		tx.SpecialType = req.SpecialGame.Type
		tx.SpecialCount = req.SpecialGame.Count
		tx.SpecialSeq = req.SpecialGame.Sequence
	}
	tx.CreatedAt = time.Now()

	if err := database.DB.Create(&tx).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"code": 500, "msg": "DB insert error", "error": err.Error()})
	}

	resp := TransferResponse{
		TransferID:   tx.TransferID,
		MerchantTxID: tx.MerchantTxID,
		AcctID:       tx.AcctID,
		Balance:      tx.BalanceAfter / BalanceRatio,
		Code:         0,
		Msg:          "success",
		SerialNo:     tx.SerialNo,
	}
	return c.JSON(resp)
}
