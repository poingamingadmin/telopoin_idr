package providers

import (
	"strings"
)

type LaunchRequest struct {
	UserCode     string `json:"user_code"`
	GameType     string `json:"game_type"`
	ProviderCode string `json:"provider_code"`
	GameCode     string `json:"game_code"`
	Lang         string `json:"lang"`
	Platform     string `json:"platform"`
	Currency     string `json:"currency"`
	IP           string `json:"ip"`
}

type GameProviderLauncher interface {
	StartGame(req LaunchRequest) (string, error)
}

var GameLaunchers = map[string]GameProviderLauncher{}

func RegisterProvider(name string, launcher GameProviderLauncher) {
	GameLaunchers[strings.ToLower(name)] = launcher
}

func GetProvider(name string) GameProviderLauncher {
	return GameLaunchers[strings.ToLower(name)]
}
