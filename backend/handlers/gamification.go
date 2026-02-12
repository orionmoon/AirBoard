package handlers

import (
	"net/http"

	"airboard/models"
	"airboard/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GamificationHandler struct {
	db                  *gorm.DB
	gamificationService *services.GamificationService
}

func NewGamificationHandler(db *gorm.DB, gs *services.GamificationService) *GamificationHandler {
	return &GamificationHandler{
		db:                  db,
		gamificationService: gs,
	}
}

// GetMyProfile récupère le profil de gamification de l'utilisateur connecté
func (h *GamificationHandler) GetMyProfile(c *gin.Context) {
	userID := c.GetUint("user_id")

	var profile models.GamificationProfile
	if err := h.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Créer un profil par défaut si inexistant
			profile = models.GamificationProfile{UserID: userID, Level: 1, XP: 0}
			h.db.Create(&profile)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la récupération du profil"})
			return
		}
	}

	c.JSON(http.StatusOK, profile)
}

// GetMyAchievements récupère les badges débloqués par l'utilisateur
func (h *GamificationHandler) GetMyAchievements(c *gin.Context) {
	userID := c.GetUint("user_id")

	var userAchievements []models.UserAchievement
	if err := h.db.Preload("Achievement").Where("user_id = ?", userID).Find(&userAchievements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la récupération des badges"})
		return
	}

	c.JSON(http.StatusOK, userAchievements)
}

// GetAllAchievements récupère tous les badges configurés
func (h *GamificationHandler) GetAllAchievements(c *gin.Context) {
	role := c.GetString("role")
	managedGroupIDs, _ := c.Get("managed_group_ids")

	// Un utilisateur est considéré comme "contributeur" s'il est admin, editor,
	// ou s'il administre au moins un groupe.
	isContributor := role == "admin" || role == "editor"
	if ids, ok := managedGroupIDs.([]uint); ok && len(ids) > 0 {
		isContributor = true
	}

	var achievements []models.Achievement
	query := h.db

	// Si l'utilisateur n'est pas un contributeur, on cache les badges de catégorie "contributor"
	if !isContributor {
		query = query.Where("category != ?", "contributor")
	}

	if err := query.Find(&achievements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la récupération des badges"})
		return
	}

	c.JSON(http.StatusOK, achievements)
}

// GetLeaderboard récupère le classement XP
func (h *GamificationHandler) GetLeaderboard(c *gin.Context) {
	var leaderboard []struct {
		UserID    uint   `json:"user_id"`
		Username  string `json:"username"`
		XP        int64  `json:"xp"`
		Level     int    `json:"level"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	h.db.Table("gamification_profiles").
		Select("gamification_profiles.user_id, users.username, users.first_name, users.last_name, gamification_profiles.xp, gamification_profiles.level").
		Joins("JOIN users ON users.id = gamification_profiles.user_id").
		Where("users.is_active = ? AND users.deleted_at IS NULL", true).
		Order("gamification_profiles.xp DESC").
		Limit(10).
		Scan(&leaderboard)

	c.JSON(http.StatusOK, leaderboard)
}

// GetMyTransactions récupère l'historique des gains de points de l'utilisateur
func (h *GamificationHandler) GetMyTransactions(c *gin.Context) {
	userID := c.GetUint("user_id")

	var transactions []models.XPTransaction
	if err := h.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(20).
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la récupération de l'historique"})
		return
	}

	c.JSON(http.StatusOK, transactions)
}
