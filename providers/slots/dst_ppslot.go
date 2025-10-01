package slots

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"telo/database"
	"telo/models"
	"telo/providers"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type DstPragmatic struct {
	ApiURL      string
	CompanyCode string
	ApiKey      string
}

func (p *DstPragmatic) StartGame(req providers.LaunchRequest) (string, error) {
	start := time.Now()

	// Cari user di DB
	var user models.User
	if err := database.DB.Where("user_code = ?", req.UserCode).First(&user).Error; err != nil {
		log.Printf("❌ [Exchange.StartGame] User not found: %s", req.UserCode)
		return "", fmt.Errorf("user not found: %w", err)
	}

	// Build payload sesuai spesifikasi
	payload := map[string]any{
		"companyCode":     os.Getenv("DST_COMPANY_CODE"),
		"apiKey":          os.Getenv("DST_API_KEY"),
		"loginId":         user.UserCode,
		"verificationKey": uuid.NewString(),
		"providerCode":    req.ProviderCode,
		"gameCode":        req.GameCode,
		"gameType":        req.GameType,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ [Exchange.StartGame] Failed to marshal payload: %v", err)
		return "", err
	}

	log.Printf("📤 [Exchange.StartGame] URL: %s", p.ApiURL)
	log.Printf("📤 [Exchange.StartGame] Payload: %s", string(jsonBody))

	// Kirim request
	resp, err := http.Post(p.ApiURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("❌ [Exchange.StartGame] HTTP request failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ [Exchange.StartGame] Failed to read response body: %v", err)
		return "", err
	}

	log.Printf("📥 [Exchange.StartGame] HTTP Status: %s", resp.Status)
	log.Printf("📥 [Exchange.StartGame] Response Body: %s", string(bodyBytes))

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to launch game, status: %s", resp.Status)
	}

	// Decode response (asumsikan ada field launchURL)
	var result struct {
		LaunchURL string `json:"launchUrl"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		log.Printf("❌ [Exchange.StartGame] Failed to decode response: %v", err)
		return "", err
	}

	if result.LaunchURL == "" {
		return "", fmt.Errorf("no launch URL found in response")
	}

	log.Printf("✅ [Exchange.StartGame] Success - UserCode: %s | LaunchURL: %s | Duration: %v",
		user.UserCode, result.LaunchURL, time.Since(start))

	return result.LaunchURL, nil
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("❌ Error loading .env file")
	}

	apiURL := os.Getenv("DST_API_URL")
	companyCode := os.Getenv("DST_COMPANY_CODE")
	apiKey := os.Getenv("DST_API_KEY")

	if apiURL == "" || companyCode == "" || apiKey == "" {
		panic("❌ ENV DstPragmatic_API_URL, DstPragmatic_COMPANY_CODE, or DstPragmatic_API_KEY not set")
	}

	providers.RegisterProvider("PPLCO", &DstPragmatic{
		ApiURL:      apiURL,
		CompanyCode: companyCode,
		ApiKey:      apiKey,
	})
}
