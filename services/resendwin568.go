package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"telo/database"
	"telo/models"
)

func ResendWin568Orders(portfolio, companyKey, serverID string) error {
	var bets []models.Win568Bet

	if err := database.DB.
		Where("is_resend = ? AND status IN ?", false,
			[]string{"won", "lose", "draw", "bonus", "GameProviderPromotion", "void", "running"}).
		Find(&bets).Error; err != nil {
		return err
	}

	if len(bets) == 0 {
		return nil
	}

	// kumpulin txnId (pakai RefNo)
	var txnIds []string
	for _, bet := range bets {
		txnIds = append(txnIds, bet.RefNo)
	}

	payload := map[string]any{
		"txnId":      strings.Join(txnIds, ","),
		"portfolio":  portfolio,
		"companyKey": companyKey,
		"serverId":   serverID,
	}

	body, _ := json.MarshalIndent(payload, "", "  ")
	url := os.Getenv("WIN568_API_URL") + "/web-root/restricted/seamless-wallet/resend-order"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)

	var result struct {
		ServerId string `json:"serverId"`
		Error    struct {
			ID  int    `json:"id"`
			Msg string `json:"msg"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rawResp, &result); err != nil {
		return fmt.Errorf("decode error: %v", err)
	}

	if result.Error.ID != 0 {
		return fmt.Errorf("API error: %s", result.Error.Msg)
	}

	for _, bet := range bets {
		database.DB.Model(&bet).
			Updates(map[string]any{
				"is_resend":    true,
				"resend_count": bet.ResendCount + 1,
			})
	}
	return nil
}
