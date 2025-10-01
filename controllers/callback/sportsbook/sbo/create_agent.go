package sbo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Request struct (untuk parsing body dari client kita)
type CreateAgentRequest struct {
	CompanyKey       string `json:"CompanyKey"`
	ServerId         string `json:"ServerId"`
	Username         string `json:"Username"`
	Password         string `json:"Password"`
	Currency         string `json:"Currency"`
	Min              int    `json:"Min"`
	Max              int    `json:"Max"`
	MaxPerMatch      int    `json:"MaxPerMatch"`
	CasinoTableLimit int    `json:"CasinoTableLimit"`
	IsTwoFAEnabled   bool   `json:"IsTwoFAEnabled"`
}

func CreateAgent(c *fiber.Ctx) error {
	fmt.Println("=== CreateAgent handler dipanggil ===")
	var req CreateAgentRequest

	if err := c.BodyParser(&req); err != nil {
		fmt.Println("[CreateAgent] Invalid request payload")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"ErrorCode":    400,
			"ErrorMessage": "Invalid request payload",
		})
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	fmt.Printf("[CreateAgent] Request received at %s (Timezone: %s)\n",
		now.Format("2006-01-02 15:04:05"), loc)

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		fmt.Println("[CreateAgent] Failed to encode request")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"ErrorCode":    500,
			"ErrorMessage": "Failed to encode request",
		})
	}

	sboURL := "https://ex-api-yy2.ttbbyyllyy.com/web-root/restricted/agent/register-agent.aspx"
	resp, err := http.Post(sboURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Printf("[CreateAgent] ERROR at %s (%s): %v\n",
			now.Format("2006-01-02 15:04:05"), loc, err)
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{
			"ErrorCode":    502,
			"ErrorMessage": "Failed to reach SBO endpoint",
			"Detail":       err.Error(),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[CreateAgent] ERROR reading SBO response")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"ErrorCode":    500,
			"ErrorMessage": "Failed to read SBO response",
		})
	}

	fmt.Printf("[CreateAgent] Success response at %s (Timezone: %s), StatusCode: %d\n",
		now.Format("2006-01-02 15:04:05"), loc, resp.StatusCode)

	return c.Status(resp.StatusCode).Send(body)
}
