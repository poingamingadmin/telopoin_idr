package casino

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"telo/database"
	"telo/models"
	"telo/providers"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type EvolutionLive struct {
	ApiURL string
}

func (p *EvolutionLive) StartGame(req providers.LaunchRequest) (string, error) {
	start := time.Now()
	log.Println("🚀 [StartGame] ===== Evolution StartGame Handler Triggered =====")

	// === 1. Log request awal ===
	log.Printf("[StartGame] 📩 LaunchRequest received: UserCode=%s | Lang=%s | IP=%s | Platform=%s",
		req.UserCode, req.Lang, req.IP, req.Platform)

	// === 2. Ambil user dari database ===
	var user models.User
	if err := database.DB.Where("user_code = ?", req.UserCode).First(&user).Error; err != nil {
		log.Printf("❌ [StartGame] User not found in DB: %s | Error: %v", req.UserCode, err)
		return "", fmt.Errorf("user not found: %w", err)
	}
	log.Printf("✅ [StartGame] User found: ID=%d | Code=%s | Balance=%.2f | Country=%s | Currency=%s",
		user.ID, user.UserCode, user.Balance, user.Country, user.Currency)

	// === 3. Buat atau perbarui session ===
	var session models.Session
	if err := database.DB.Where("user_id = ?", user.ID).First(&session).Error; err == nil {
		log.Printf("[StartGame] 🔁 Existing session found: SID=%s | ExpiredAt=%v", session.SID, session.ExpiresAt)
		session.ExpiresAt = time.Now().Add(24 * time.Hour)
		if err := database.DB.Save(&session).Error; err != nil {
			log.Printf("⚠️ [StartGame] Failed to update session expiry: %v", err)
		} else {
			log.Printf("✅ [StartGame] Session expiry updated: %v", session.ExpiresAt)
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("[StartGame] 🆕 No session found for user %s, creating a new one...", user.UserCode)
		session = models.Session{
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := database.DB.Create(&session).Error; err != nil {
			log.Printf("❌ [StartGame] Failed to create new session: %v", err)
			return "", err
		}
		log.Printf("✅ [StartGame] New session created: SID=%s | ExpiresAt=%v", session.SID, session.ExpiresAt)
	} else {
		log.Printf("❌ [StartGame] Unexpected DB error during session lookup: %v", err)
		return "", err
	}

	// === 4. Generate UUID ===
	uuid := fmt.Sprintf("req-%s", req.UserCode)
	log.Printf("[StartGame] 🆔 Generated UUID: %s", uuid)

	// === 5. Pisahkan firstName & lastName dari user_code ===
	parts := strings.Split(user.UserCode, "_")
	firstName, lastName := "", ""
	if len(parts) == 2 {
		firstName = strings.TrimLeft(parts[0], "0123456789")[:2]
		lastName = parts[1]
		if len(lastName) > 10 {
			lastName = lastName[:10]
		}
	}
	log.Printf("[StartGame] 👤 Parsed Player Name => firstName=%s | lastName=%s", firstName, lastName)

	// === 6. Siapkan payload sesuai dokumentasi Evolution ===
	payload := map[string]any{
		"uuid": uuid,
		"player": map[string]any{
			"id":        req.UserCode,
			"update":    true,
			"firstName": firstName,
			"lastName":  lastName,
			"country":   user.Country,
			"language":  req.Lang,
			"currency":  user.Currency,
			"nickname":  user.UserCode,
			"session": map[string]any{
				"id": session.SID,
				"ip": req.IP,
			},
		},
		"config": map[string]any{
			"game": map[string]any{
				"category":  "blackjack",
				"interface": "view1",
				"table": map[string]any{
					"id": "k4r2ejwx4eqqb6tv",
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
	log.Printf("📤 [StartGame] Payload JSON:\n%s", string(jsonBody))

	// === 7. Kirim request ke Evolution ===
	resp, err := http.Post(p.ApiURL, "application/json", bytes.NewBuffer(jsonBody))
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
	log.Printf("📥 [StartGame] Response Body:\n%s", string(bodyBytes))

	// === 8. Validasi status HTTP ===
	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ [StartGame] Non-200 response from Evolution: %s", resp.Status)
		return "", fmt.Errorf("failed to launch game, status: %s", resp.Status)
	}

	// === 9. Decode respons ===
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
		log.Printf("❌ [StartGame] No 'entry' or 'entryEmbedded' found in response")
		return "", errors.New("launch URL not found in response")
	}

	// === 10. Sukses ===
	log.Printf("✅ [StartGame] SUCCESS - UserCode=%s | SID=%s | LaunchURL=%s | Duration=%v",
		user.UserCode, session.SID, launchURL, time.Since(start))

	log.Println("🏁 [StartGame] ===== Evolution StartGame Completed =====")

	return launchURL, nil
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	apiURL := os.Getenv("EVOLUTION_API_URL_LIVE")
	if apiURL == "" {
		panic("❌ ENV EVOLUTION_API_URL not set")
	}

	providers.RegisterProvider("EVOLUTIONLIVE", &EvolutionLive{
		ApiURL: apiURL,
	})
}
