package services

import (
	"fmt"
	"math"
	"time"

	"airboard/models"

	"gorm.io/gorm"
)

type GamificationService struct {
	db *gorm.DB
}

func NewGamificationService(db *gorm.DB) *GamificationService {
	return &GamificationService{db: db}
}

// AwardXP accorde des points à un utilisateur et vérifie le passage de niveau
func (s *GamificationService) AwardXP(userID uint, amount int64, reason string, metadata string) error {
	if amount <= 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Récupérer ou créer le profil
		var profile models.GamificationProfile
		if err := tx.Where("user_id = ?", userID).FirstOrCreate(&profile, models.GamificationProfile{UserID: userID}).Error; err != nil {
			return err
		}

		// 2. Enregistrer la transaction
		transaction := models.XPTransaction{
			UserID:   userID,
			Amount:   amount,
			Reason:   reason,
			Metadata: metadata,
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		// 3. Mettre à jour l'XP et vérifier le niveau
		oldLevel := profile.Level
		profile.XP += amount

		// Formula: Level = floor(sqrt(XP / 100)) + 1
		newLevel := int(math.Floor(math.Sqrt(float64(profile.XP)/100))) + 1
		if newLevel < 1 {
			newLevel = 1
		}

		profile.Level = newLevel
		if err := tx.Save(&profile).Error; err != nil {
			return err
		}

		// 4. Si passage de niveau, on peut déclencher une notification (TODO)
		if newLevel > oldLevel {
			fmt.Printf("User %d leveled up to %d!\n", userID, newLevel)
		}

		// 5. Vérifier les achievements liés à cette action
		return s.CheckAchievements(tx, userID, reason)
	})
}

// CheckAchievements vérifie si l'utilisateur a débloqué de nouveaux badges
func (s *GamificationService) CheckAchievements(tx *gorm.DB, userID uint, triggerReason string) error {
	// Cette fonction sera étendue pour vérifier des conditions complexes
	// Pour l'instant, implémentons quelques succès simples

	switch triggerReason {
	case "app_click":
		return s.checkExplorerAchievement(tx, userID)
	case "news_read":
		return s.checkInformedAchievement(tx, userID)
	case "daily_login":
		return s.checkEarlyBirdAchievement(tx, userID)
	case "news_publish":
		return s.checkFirstNewsAchievement(tx, userID)
	case "event_publish":
		return s.checkEventMasterAchievement(tx, userID)
	case "poll_vote":
		return s.checkCitizenAchievement(tx, userID)
	case "poll_create":
		return s.checkPollsterAchievement(tx, userID)
	case "comment_create":
		return s.checkCommentatorAchievement(tx, userID)
	}

	return nil
}

func (s *GamificationService) checkExplorerAchievement(tx *gorm.DB, userID uint) error {
	var count int64
	tx.Model(&models.ApplicationClick{}).Where("user_id = ?", userID).Distinct("application_id").Count(&count)

	if count >= 10 {
		return s.UnlockAchievement(tx, userID, "explorer")
	}
	return nil
}

func (s *GamificationService) checkInformedAchievement(tx *gorm.DB, userID uint) error {
	var count int64
	tx.Model(&models.NewsRead{}).Where("user_id = ?", userID).Count(&count)

	if count >= 20 {
		return s.UnlockAchievement(tx, userID, "informed")
	}
	return nil
}

func (s *GamificationService) checkEarlyBirdAchievement(tx *gorm.DB, userID uint) error {
	now := time.Now()
	// Check if current time is before 8:30 AM
	if now.Hour() < 8 || (now.Hour() == 8 && now.Minute() < 30) {
		return s.UnlockAchievement(tx, userID, "early_bird")
	}
	return nil
}

func (s *GamificationService) checkFirstNewsAchievement(tx *gorm.DB, userID uint) error {
	var count int64
	tx.Model(&models.News{}).Where("author_id = ?", userID).Count(&count)

	if count >= 1 {
		return s.UnlockAchievement(tx, userID, "first_news")
	}
	return nil
}

func (s *GamificationService) checkEventMasterAchievement(tx *gorm.DB, userID uint) error {
	var count int64
	tx.Model(&models.Event{}).Where("author_id = ?", userID).Count(&count)

	if count >= 5 {
		return s.UnlockAchievement(tx, userID, "event_master")
	}
	return nil
}

func (s *GamificationService) checkCitizenAchievement(tx *gorm.DB, userID uint) error {
	var count int64
	tx.Model(&models.PollVote{}).Where("user_id = ?", userID).Count(&count)

	if count >= 5 {
		return s.UnlockAchievement(tx, userID, "citizen")
	}
	return nil
}

func (s *GamificationService) checkPollsterAchievement(tx *gorm.DB, userID uint) error {
	var count int64
	tx.Model(&models.Poll{}).Where("author_id = ?", userID).Count(&count)

	if count >= 3 {
		return s.UnlockAchievement(tx, userID, "pollster")
	}
	return nil
}

func (s *GamificationService) checkCommentatorAchievement(tx *gorm.DB, userID uint) error {
	var count int64
	tx.Model(&models.Comment{}).Where("user_id = ?", userID).Count(&count)

	if count >= 5 {
		return s.UnlockAchievement(tx, userID, "commentator")
	}
	return nil
}

// UnlockAchievement débloque un badge pour l'utilisateur
func (s *GamificationService) UnlockAchievement(tx *gorm.DB, userID uint, code string) error {
	var achievement models.Achievement
	if err := tx.Where("code = ?", code).First(&achievement).Error; err != nil {
		return nil // Achievement non configuré
	}

	// Vérifier si déjà débloqué
	var exists int64
	tx.Model(&models.UserAchievement{}).Where("user_id = ? AND achievement_id = ?", userID, achievement.ID).Count(&exists)
	if exists > 0 {
		return nil
	}

	// Restriction: Si c'est un badge de type "contributor", vérifier les droits de l'utilisateur
	if achievement.Category == "contributor" {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		// Vérifier si admin, editor ou s'il gère au moins un groupe
		isContributor := user.Role == "admin" || user.Role == "editor"
		if !isContributor {
			var groupAdminCount int64
			tx.Table("group_admins").Where("user_id = ?", userID).Count(&groupAdminCount)
			if groupAdminCount > 0 {
				isContributor = true
			}
		}

		if !isContributor {
			return nil // Ne pas débloquer pour un utilisateur simple
		}
	}

	// Débloquer
	ua := models.UserAchievement{
		UserID:        userID,
		AchievementID: achievement.ID,
		UnlockedAt:    time.Now(),
	}
	if err := tx.Create(&ua).Error; err != nil {
		return err
	}

	// Optionnel: Donner de l'XP bonus pour le badge
	if achievement.XPReward > 0 {
		// Attention: Appeler AwardXP récursivement ici pourrait créer une boucle infinie si mal géré.
		// On met à jour directement l'XP du profil dans cette transaction.
		var profile models.GamificationProfile
		tx.Where("user_id = ?", userID).First(&profile)
		profile.XP += achievement.XPReward
		tx.Save(&profile)

		// Enregistrer la transaction bonus
		tx.Create(&models.XPTransaction{
			UserID:   userID,
			Amount:   achievement.XPReward,
			Reason:   "achievement_unlock",
			Metadata: fmt.Sprintf("{\"code\": \"%s\"}", code),
		})
	}

	return nil
}

// SeedAchievements initialise les badges par défaut
func (s *GamificationService) SeedAchievements() error {
	achievements := []models.Achievement{
		{
			Code:        "early_bird",
			Name:        "Lève-tôt",
			Description: "Connectez-vous avant 8h30 du matin",
			Icon:        "mdi:weather-sunset-up",
			Color:       "#F59E0B",
			XPReward:    50,
			Category:    "user",
		},
		{
			Code:        "explorer",
			Name:        "Explorateur",
			Description: "Cliquez sur 10 applications différentes",
			Icon:        "mdi:compass-outline",
			Color:       "#3B82F6",
			XPReward:    100,
			Category:    "user",
		},
		{
			Code:        "informed",
			Name:        "Bien informé",
			Description: "Lisez 20 articles d'actualité",
			Icon:        "mdi:book-open-variant",
			Color:       "#10B981",
			XPReward:    150,
			Category:    "user",
		},
		{
			Code:        "first_news",
			Name:        "Premier scoop",
			Description: "Publiez votre premier article",
			Icon:        "mdi:newspaper-variant-outline",
			Color:       "#8B5CF6",
			XPReward:    200,
			Category:    "contributor",
		},
		{
			Code:        "event_master",
			Name:        "Maître des événements",
			Description: "Créez 5 événements",
			Icon:        "mdi:calendar-star",
			Color:       "#EF4444",
			XPReward:    300,
			Category:    "contributor",
		},
		{
			Code:        "citizen",
			Name:        "Citoyen modèle",
			Description: "Votez à 5 sondages",
			Icon:        "mdi:vote",
			Color:       "#10B981",
			XPReward:    100,
			Category:    "user",
		},
		{
			Code:        "pollster",
			Name:        "Sondeur",
			Description: "Créez 3 sondages",
			Icon:        "mdi:poll",
			Color:       "#F59E0B",
			XPReward:    150,
			Category:    "contributor",
		},
		{
			Code:        "commentator",
			Name:        "Commentateur",
			Description: "Publiez 5 commentaires",
			Icon:        "mdi:comment-text-multiple",
			Color:       "#6366F1",
			XPReward:    80,
			Category:    "user",
		},
	}

	for _, ach := range achievements {
		var existing models.Achievement
		if err := s.db.Where("code = ?", ach.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := s.db.Create(&ach).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			// Update description if changed
			if existing.Description != ach.Description {
				existing.Description = ach.Description
				s.db.Save(&existing)
			}
		}
	}

	return nil
}
