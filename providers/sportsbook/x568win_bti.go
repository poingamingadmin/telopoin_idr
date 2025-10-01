package sportsbook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"telo/database"
	"telo/models"
	"telo/providers"

	"github.com/joho/godotenv"
)

type Win568BTI struct {
	ApiURL     string
	CompanyKey string
	ServerID   string
}

func (p *Win568BTI) StartGame(req providers.LaunchRequest) (string, error) {
	start := time.Now()

	var user models.User
	if err := database.DB.Where("user_code = ?", req.UserCode).First(&user).Error; err != nil {
		log.Printf("❌ [StartGame] User not found: %s", req.UserCode)
		return "", fmt.Errorf("user not found: %w", err)
	}

	// ✅ Payload sesuai dokumentasi 3.2 Login
	payload := map[string]any{
		"CompanyKey": p.CompanyKey,
		"ServerId":   p.ServerID,
		"Username":   user.UserCode,
		"Portfolio":  "ThirdPartySportsBook", // bisa juga "SportsBook" tergantung tujuan
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Printf("📤 [StartGame] POST %s/web-root/restricted/player/login.aspx", p.ApiURL)
	log.Printf("📤 Payload: %s", string(jsonBody))

	// 🔥 Kirim request login
	resp, err := http.Post(p.ApiURL+"/web-root/restricted/player/login.aspx", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("📥 Status: %s", resp.Status)
	log.Printf("📥 Response: %s", string(bodyBytes))

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to login, status: %s", resp.Status)
	}

	// ✅ Parsing response
	var result struct {
		URL    string `json:"url"`
		Server string `json:"serverId"`
		Error  struct {
			ID  int    `json:"id"`
			Msg string `json:"msg"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error.ID != 0 {
		return "", fmt.Errorf("API error: %s", result.Error.Msg)
	}

	if result.URL == "" {
		return "", errors.New("no login URL returned")
	}

	// ✅ Susun URL final untuk redirect
	device := map[string]string{"mobile": "m", "desktop": "d"}[req.Platform]
	lang := req.Lang
	if lang == "" {
		lang = "en"
	}

	finalURL := fmt.Sprintf("https://%s&lang=%s&gpId=1022&device=%s",
		result.URL, lang, device)

	log.Printf("✅ [StartGame] Success - User: %s | URL: %s | Duration: %v",
		user.UserCode, finalURL, time.Since(start))

	return finalURL, nil
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	apiURL := os.Getenv("WIN568_API_URL")
	companyKey := os.Getenv("WIN568_COMPANY_KEY")
	serverID := os.Getenv("WIN568_SERVER_ID")

	if apiURL == "" || companyKey == "" || serverID == "" {
		panic("❌ ENV WIN568_API_URL / COMPANY_KEY / SERVER_ID not set")
	}

	providers.RegisterProvider("bti", &Win568BTI{
		ApiURL:     apiURL,
		CompanyKey: companyKey,
		ServerID:   serverID,
	})
}
