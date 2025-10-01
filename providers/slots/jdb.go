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
	"time"

	"telo/database"
	"telo/models"
	"telo/providers"

	"github.com/joho/godotenv"
)

type Win568JDB struct {
	ApiURL     string
	CompanyKey string
	ServerID   string
}

func (p *Win568JDB) StartGame(req providers.LaunchRequest) (string, error) {
	start := time.Now()

	var user models.User
	if err := database.DB.Where("user_code = ?", req.UserCode).First(&user).Error; err != nil {
		log.Printf("❌ [StartGame] User not found: %s", req.UserCode)
		return "", fmt.Errorf("user not found: %w", err)
	}

	username := req.UserCode
	if len(username) < 6 {
		username = fmt.Sprintf("%s_user", username)
	}

	// 🔹 Payload untuk New Login API
	payload := map[string]any{
		"CompanyKey": p.CompanyKey,
		"ServerId":   p.ServerID,
		"Username":   username,
		"Portfolio":  "SeamlessGame",
		"Lang":       req.Lang,
		"Device":     map[string]string{"mobile": "m", "desktop": "d"}[req.Platform],
		"GpId":       "1058",
		"GameId":     req.GameCode,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ [StartGame] Failed to marshal payload: %v", err)
		return "", err
	}

	log.Printf("📤 [StartGame] URL: %s", p.ApiURL)
	log.Printf("📤 [StartGame] Payload: %s", string(jsonBody))

	// 🔹 Kirim request
	resp, err := http.Post(p.ApiURL+"/web-root/restricted/player/v2/login.aspx", "application/json", bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to launch game, status: %s", resp.Status)
	}

	// 🔹 Parsing response
	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.URL == "" {
		return "", errors.New("no login URL returned")
	}

	log.Printf("✅ [StartGame] Success - UserCode: %s | LaunchURL: %s | Duration: %v",
		user.UserCode, result.URL, time.Since(start))

	return result.URL, nil
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

	providers.RegisterProvider("JDB", &Win568JDB{
		ApiURL:     apiURL,
		CompanyKey: companyKey,
		ServerID:   serverID,
	})
}
