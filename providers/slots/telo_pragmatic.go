package slots

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"telo/database"
	"telo/models"
	"telo/providers"
)

type TeloLauncherPP struct {
	ApiURL string
}

func (p *TeloLauncherPP) StartGame(req providers.LaunchRequest) (string, error) {
	var user models.User
	if err := database.DB.Where("user_code = ?", req.UserCode).First(&user).Error; err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	payload := map[string]any{
		"agent_code":    os.Getenv("TELO_AGENT_CODE"),
		"agent_token":   os.Getenv("TELO_AGENT_TOKEN"),
		"user_code":     req.UserCode,
		"game_type":     req.GameType,
		"provider_code": "PRAGMATIC",
		"game_code":     req.GameCode,
		"lang":          req.Lang,
		"user_balance":  user.Balance,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("❌ [StartGame] Failed to marshal payload:", err)
		return "", err
	}

	resp, err := http.Post(p.ApiURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Println("❌ [StartGame] HTTP request failed:", err)
		return "", err
	}
	defer resp.Body.Close()
	for key, value := range resp.Header {
		fmt.Printf("    %s: %v\n", key, value)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("❌ [StartGame] Failed to read response body:", err)
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to launch game, status: %s", resp.Status)
	}

	var result struct {
		LaunchURL string `json:"launch_url"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		fmt.Println("❌ [StartGame] Failed to decode JSON response:", err)
		return "", err
	}

	return result.LaunchURL, nil
}

func init() {
	providers.RegisterProvider("TPRAGMATIC", &TeloLauncherPP{
		ApiURL: "https://api.telo.is/api/v2/game_launch",
	})
}
