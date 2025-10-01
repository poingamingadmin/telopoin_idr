package slots

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"telo/database"
	"telo/models"
	"telo/providers"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type EvolutionSlot struct {
	ApiURL string
}

func (p *EvolutionSlot) StartGame(req providers.LaunchRequest) (string, error) {
	start := time.Now()

	var user models.User
	if err := database.DB.Where("user_code = ?", req.UserCode).First(&user).Error; err != nil {
		log.Printf("❌ [StartGame] User not found: %s", req.UserCode)
		return "", fmt.Errorf("user not found: %w", err)
	}

	uuid := fmt.Sprintf("req-%s", req.UserCode)

	var session models.Session
	if err := database.DB.Where("user_id = ?", user.ID).First(&session).Error; err == nil {
		session.ExpiresAt = time.Now().Add(24 * time.Hour)
		database.DB.Save(&session)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		session = models.Session{
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		database.DB.Create(&session)
	}

	parts := strings.Split(user.UserCode, "_")
	firstName := "Player"
	lastName := user.UserCode

	// fungsi bantu untuk hapus semua angka
	removeDigits := func(s string) string {
		re := regexp.MustCompile(`[0-9]`)
		return re.ReplaceAllString(s, "")
	}

	if len(parts) == 2 {
		fn := removeDigits(parts[0])
		ln := removeDigits(parts[1])

		if len(fn) >= 2 {
			firstName = fn[:2]
		} else if len(fn) > 0 {
			firstName = fn
		}

		if len(ln) > 10 {
			lastName = ln[:10]
		} else if len(ln) > 0 {
			lastName = ln
		}
	}

	payload := map[string]any{
		"uuid": uuid,
		"player": map[string]any{
			"id":        req.UserCode,
			"update":    true,
			"firstName": firstName,
			"lastName":  lastName,
			"country":   user.Country,
			"nickname":  user.UserCode,
			"language":  req.Lang,
			"currency":  user.Currency,
			"session": map[string]any{
				"id": session.SID,
				"ip": req.IP,
			},
		},
		"config": map[string]any{
			"game": map[string]any{
				"category":  req.GameType,
				"interface": "view1",
				"table": map[string]any{
					"id": req.GameCode,
				},
			},
			"channel": map[string]any{
				"wrapped": false,
				"mobile":  req.Platform == "mobile",
			},
		},
	}

	jsonBody, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Printf("❌ [StartGame] Failed to marshal payload: %v", err)
		return "", err
	}

	log.Printf("📤 [StartGame] URL: %s", p.ApiURL)
	log.Printf("📤 [StartGame] Payload:\n%s", string(jsonBody))

	// ✅ Explicit Content-Type header
	httpReq, err := http.NewRequest("POST", p.ApiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("❌ [StartGame] HTTP request failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ [StartGame] Failed to read response body: %v", err)
		return "", err
	}

	log.Printf("📥 [StartGame] HTTP Status: %s", resp.Status)
	log.Printf("📥 [StartGame] Response Body: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ [StartGame] Non-200 status: %s", resp.Status)
		return "", fmt.Errorf("failed to launch game, status: %s", resp.Status)
	}

	var result struct {
		Entry         string `json:"entry"`
		EntryEmbedded string `json:"entryEmbedded"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		log.Printf("❌ [StartGame] Failed to decode JSON response: %v", err)
		return "", err
	}

	launchURL := result.Entry
	if launchURL == "" {
		launchURL = result.EntryEmbedded
	}
	if launchURL == "" {
		log.Printf("❌ [StartGame] No valid launch URL found in response")
		return "", errors.New("launch URL not found in response")
	}

	log.Printf("✅ [StartGame] Success - UserCode: %s | SID: %s | LaunchURL: %s | Duration: %v",
		user.UserCode, session.SID, launchURL, time.Since(start))

	return launchURL, nil
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	apiURL := os.Getenv("EVOLUTION_API_URL_SLOT")

	if apiURL == "" {
		panic("❌ ENV EVOLUTION_API_URL not set")
	}

	providers.RegisterProvider("EVOLUTIONSLOT", &EvolutionSlot{
		ApiURL: apiURL,
	})
}
