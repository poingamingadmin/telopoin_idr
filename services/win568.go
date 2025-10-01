package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"telo/database"
	"telo/models"
	"time"

	"gorm.io/gorm"
)

func FetchWin568BetListDaily(portfolio, companyKey, serverID string) error {
	now := time.Now().UTC()

	// Awal hari (00:00:00 UTC)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := start.Add(24 * time.Hour)

	for start.Before(endOfDay) {
		end := start.Add(30 * time.Minute)
		if end.After(endOfDay) {
			end = endOfDay
		}

		if err := fetchWin568Range(portfolio, companyKey, serverID, start, end); err != nil {
			log.Printf("❌ Error fetching range %s → %s: %v", start, end, err)
		}

		start = end
	}

	return nil
}

func fetchWin568Range(portfolio, companyKey, serverID string, startDate, endDate time.Time) error {
	payload := map[string]any{
		"portfolio":     portfolio,
		"startDate":     startDate.Format(time.RFC3339),
		"endDate":       endDate.Format(time.RFC3339),
		"companyKey":    companyKey,
		"isGetDownline": true,
		"language":      "en",
		"serverId":      serverID,
	}

	body, _ := json.MarshalIndent(payload, "", "  ")
	url := os.Getenv("WIN568_API_URL") + "/web-root/restricted/report/v2/get-bet-list-by-modify-date.aspx"

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
		Result   []models.Win568Bet `json:"result"`
		ServerId string             `json:"serverId"`
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

	newCount := 0
	for _, bet := range result.Result {

		if err := SaveWin568Bet(&bet); err != nil {
			log.Printf("❌ Failed save bet %s: %v", bet.RefNo, err)
		} else {
			newCount++
		}
	}

	return nil
}

func SaveWin568Bet(bet *models.Win568Bet) error {
	var existing models.Win568Bet
	err := database.DB.Where("ref_no = ?", bet.RefNo).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return database.DB.Create(bet).Error
		}
		return err
	}
	return nil
}
