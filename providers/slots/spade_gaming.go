package slots

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"telo/database"
	"telo/models"
	"telo/providers"
)

type SpadeGamingLauncher struct {
	ApiURL string
}

func (p *SpadeGamingLauncher) StartGame(req providers.LaunchRequest) (string, error) {
	var user models.User
	if err := database.DB.Where("user_code = ?", req.UserCode).First(&user).Error; err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	merchantCode := os.Getenv("SPADE_GAMING_MERCHANT_CODE")
	secretKey := os.Getenv("SPADE_GAMING_SECRET_KEY")
	serialNo := strconv.FormatInt(time.Now().UnixNano(), 10)

	acctInfo := map[string]any{
		"acctId":   user.UserCode,
		"userName": user.UserCode,
		"currency": user.Currency,
		"balance":  formatBalance(user.Balance),
		"siteId":   os.Getenv("SPADE_GAMING_SITE_ID"),
	}

	tokenRaw := merchantCode + secretKey + serialNo
	hash := md5.Sum([]byte(tokenRaw))
	token := hex.EncodeToString(hash[:])

	payload := map[string]any{
		"merchantCode": merchantCode,
		"acctInfo":     acctInfo,
		"token":        token,
		"acctIp":       req.IP,
		"game":         req.GameCode,
		"language":     getLanguageCode(req.Lang),
		"serialNo":     serialNo,
		"mobile":       isMobilePlatform(req.Platform),
		"fun":          false,
		"menuMode":     true,
		"fullScreen":   true,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload failed: %w", err)
	}

	digestRaw := string(jsonBody) + secretKey
	digestHash := md5.Sum([]byte(digestRaw))
	digest := hex.EncodeToString(digestHash[:])

	httpReq, err := http.NewRequest("POST", p.ApiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("API", "getAuthorize")
	httpReq.Header.Set("DataType", "JSON")
	httpReq.Header.Set("Digest", digest)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("launch game failed, status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Token    string `json:"token"`
		SerialNo string `json:"serialNo"`
		GameUrl  string `json:"gameUrl"`
		Msg      string `json:"msg"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	if result.Code != 0 {
		errorMsg := result.Msg
		if errorMsg == "" {
			errorMsg = result.Message
		}
		return "", fmt.Errorf("launch failed (code %d): %s", result.Code, errorMsg)
	}

	if result.GameUrl == "" {
		return "", fmt.Errorf("empty game URL received")
	}

	return result.GameUrl, nil
}

func init() {
	providers.RegisterProvider("SPADEGAMING", &SpadeGamingLauncher{
		ApiURL: os.Getenv("SPADE_GAMING_API_URL") + "/",
	})
}
