package playstar

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"telo/database" // pastikan package ini ada dan expose var DB *gorm.DB
	"telo/models"
)

type GetBalanceResponse struct {
	StatusCode int    `json:"status_code"`
	Balance    uint64 `json:"balance,omitempty"`
}

func GetBalanceHandler(c *fiber.Ctx) error {
	accessToken := c.Query("access_token")
	if strings.TrimSpace(accessToken) == "" {
		// invalid token dianggap sama dengan invalid member ID
		return c.Status(http.StatusOK).JSON(GetBalanceResponse{StatusCode: 1})
	}

	memberID := c.Query("member_id")
	if strings.TrimSpace(memberID) == "" {
		return c.Status(http.StatusOK).JSON(GetBalanceResponse{StatusCode: 1})
	}

	// cari user
	var user models.User
	if err := database.DB.Where("user_code = ?", memberID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(http.StatusOK).JSON(GetBalanceResponse{StatusCode: 1})
		}
		return c.Status(http.StatusOK).JSON(GetBalanceResponse{StatusCode: 5})
	}

	// return balance dalam cents
	return c.Status(http.StatusOK).JSON(GetBalanceResponse{
		StatusCode: 0,
		Balance:    uint64(user.Balance),
	})
}
