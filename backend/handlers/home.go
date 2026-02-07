package handlers

import (
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"airboard/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HomeHandler struct {
	db *gorm.DB
}

func NewHomeHandler(db *gorm.DB) *HomeHandler {
	return &HomeHandler{db: db}
}

// Response structures
type HomeStats struct {
	// Admin stats
	TotalUsers     int64 `json:"total_users,omitempty"`
	TotalGroups    int64 `json:"total_groups,omitempty"`
	TotalAppGroups int64 `json:"total_app_groups,omitempty"`
	TotalApps      int64 `json:"total_apps,omitempty"`
	TotalNews      int64 `json:"total_news,omitempty"`
	TotalEvents    int64 `json:"total_events,omitempty"`
	TotalPolls     int64 `json:"total_polls,omitempty"`

	// Group admin stats
	ManagedGroupsCount    int64 `json:"managed_groups_count,omitempty"`
	TotalMembersCount     int64 `json:"total_members_count,omitempty"`
	ManagedAppGroupsCount int64 `json:"managed_app_groups_count,omitempty"`
	ManagedAppsCount      int64 `json:"managed_apps_count,omitempty"`
	ManagedNewsCount      int64 `json:"managed_news_count,omitempty"`
	ManagedPollsCount     int64 `json:"managed_polls_count,omitempty"`
}

type GamificationSummary struct {
	Level           int                  `json:"level"`
	XP              int64                `json:"xp"`
	NextLevelXP     int64                `json:"next_level_xp"`
	ProgressPercent int                  `json:"progress_percent"`
	RecentBadges    []models.Achievement `json:"recent_badges"`
}

type HomeResponse struct {
	FavoriteApps    []models.Application  `json:"favorite_apps"`
	NewApps         []models.Application  `json:"new_apps"`
	TodayEvents     []models.Event        `json:"today_events"`
	UpcomingEvents  []models.Event        `json:"upcoming_events"`
	RecentNews      []models.News         `json:"recent_news"`
	Polls           []models.Poll         `json:"polls"`
	Announcements   []models.Announcement `json:"announcements"`
	Stats           *HomeStats            `json:"stats,omitempty"`
	Gamification    *GamificationSummary  `json:"gamification,omitempty"`
	UserRole        string                `json:"user_role"`
	ManagedGroupIDs []uint                `json:"managed_group_ids,omitempty"`
	AppSettings     *models.AppSettings   `json:"app_settings,omitempty"`
	HeroMessages    []models.HeroMessage  `json:"hero_messages,omitempty"`
}

// Main handler
func (h *HomeHandler) GetHomeData(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "Unauthorized",
			Message: "User not authenticated",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	role, _ := c.Get("role")
	managedGroupIDs, _ := c.Get("managed_group_ids")

	response := HomeResponse{
		UserRole: role.(string),
	}

	if managedGroupIDs != nil {
		response.ManagedGroupIDs = managedGroupIDs.([]uint)
	}

	// Use WaitGroup for parallel queries
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect response writes

	// 1. Load Favorite Apps (user-specific, no cache)
	wg.Add(1)
	go func() {
		defer wg.Done()
		apps, err := h.getFavoriteApps(userID.(uint))
		if err != nil {
			log.Printf("[HOME] Failed to load favorites: %v", err)
			apps = []models.Application{}
		}
		mu.Lock()
		response.FavoriteApps = apps
		mu.Unlock()
	}()

	// 2. Load New Apps (filtered by user groups)
	wg.Add(1)
	go func() {
		defer wg.Done()
		apps, err := h.getNewApps(userID.(uint), role.(string))
		if err != nil {
			log.Printf("[HOME] Failed to load new apps: %v", err)
			apps = []models.Application{}
		}
		mu.Lock()
		response.NewApps = apps
		mu.Unlock()
	}()

	// 3. Load Today's Events
	wg.Add(1)
	go func() {
		defer wg.Done()
		events, err := h.getTodayEvents(userID.(uint), role.(string))
		if err != nil {
			log.Printf("[HOME] Failed to load today's events: %v", err)
			events = []models.Event{}
		}
		mu.Lock()
		response.TodayEvents = events
		mu.Unlock()
	}()

	// 4. Load Upcoming Events
	wg.Add(1)
	go func() {
		defer wg.Done()
		events, err := h.getUpcomingEvents(userID.(uint), role.(string))
		if err != nil {
			log.Printf("[HOME] Failed to load upcoming events: %v", err)
			events = []models.Event{}
		}
		mu.Lock()
		response.UpcomingEvents = events
		mu.Unlock()
	}()

	// 5. Load Recent News
	wg.Add(1)
	go func() {
		defer wg.Done()
		news, err := h.getRecentNews(userID.(uint), role.(string))
		if err != nil {
			log.Printf("[HOME] Failed to load recent news: %v", err)
			news = []models.News{}
		}
		mu.Lock()
		response.RecentNews = news
		mu.Unlock()
	}()

	// 6. Load Recent Polls
	wg.Add(1)
	go func() {
		defer wg.Done()
		polls, err := h.getRecentPolls(userID.(uint), role.(string))
		if err != nil {
			log.Printf("[HOME] Failed to load recent polls: %v", err)
			polls = []models.Poll{}
		}
		mu.Lock()
		response.Polls = polls
		mu.Unlock()
	}()

	// 7. Load Announcements (cached)
	wg.Add(1)
	go func() {
		defer wg.Done()
		announcements, err := h.getAnnouncementsWithCache()
		if err != nil {
			log.Printf("[HOME] Failed to load announcements: %v", err)
			announcements = []models.Announcement{}
		}
		mu.Lock()
		response.Announcements = announcements
		mu.Unlock()
	}()

	// 8. Load Stats (role-specific)
	if role == "admin" || len(response.ManagedGroupIDs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats := h.getStats(userID.(uint), role.(string), response.ManagedGroupIDs)
			mu.Lock()
			response.Stats = stats
			mu.Unlock()
		}()
	}

	// 9. Load App Settings (cached)
	wg.Add(1)
	go func() {
		defer wg.Done()
		settings, err := h.getAppSettingsWithCache()
		if err != nil {
			log.Printf("[HOME] Failed to load app settings: %v", err)
			settings = nil
		}
		mu.Lock()
		response.AppSettings = settings
		mu.Unlock()
	}()

	// 10. Load Hero Messages (cached)
	wg.Add(1)
	go func() {
		defer wg.Done()
		messages, err := h.getHeroMessagesWithCache()
		if err != nil {
			log.Printf("[HOME] Failed to load hero messages: %v", err)
			messages = []models.HeroMessage{}
		}
		mu.Lock()
		response.HeroMessages = messages
		mu.Unlock()
	}()

	// 11. Load Gamification Summary
	wg.Add(1)
	go func() {
		defer wg.Done()
		summary := h.getGamificationSummary(userID.(uint))
		mu.Lock()
		response.Gamification = summary
		mu.Unlock()
	}()

	wg.Wait()

	// Set cache headers
	c.Header("Cache-Control", "private, max-age=60")
	c.Header("Vary", "Authorization")

	c.JSON(http.StatusOK, response)
}

// Helper: Get favorite apps
func (h *HomeHandler) getFavoriteApps(userID uint) ([]models.Application, error) {
	var apps []models.Application
	err := h.db.Joins("JOIN user_favorites ON applications.id = user_favorites.application_id").
		Preload("AppGroup").
		Where("user_favorites.user_id = ? AND applications.is_active = ?", userID, true).
		Order("applications.name ASC").
		Find(&apps).Error
	return apps, err
}

// Helper: Get new apps (last 5 apps user has access to)
func (h *HomeHandler) getNewApps(userID uint, role string) ([]models.Application, error) {
	var apps []models.Application

	if role == "admin" {
		// Admins see all apps
		err := h.db.Where("is_active = ?", true).
			Preload("AppGroup").
			Order("created_at DESC").
			Limit(5).
			Find(&apps).Error
		return apps, err
	}

	// Users/Group Admins see apps in their groups + managed groups + public apps
	err := h.db.Distinct("applications.*").
		Joins("JOIN app_groups ON applications.app_group_id = app_groups.id").
		Joins("LEFT JOIN group_app_groups ON app_groups.id = group_app_groups.app_group_id").
		Joins("LEFT JOIN user_groups ON group_app_groups.group_id = user_groups.group_id").
		Joins("LEFT JOIN group_admins ON group_app_groups.group_id = group_admins.group_id AND group_admins.user_id = ?", userID).
		Where("applications.is_active = ? AND (app_groups.is_private = ? OR user_groups.user_id = ? OR group_admins.user_id = ?)",
			true, false, userID, userID).
		Preload("AppGroup").
		Order("applications.created_at DESC").
		Limit(5).
		Find(&apps).Error
	return apps, err
}

// Helper: Get today's events (events starting today only)
func (h *HomeHandler) getTodayEvents(userID uint, role string) ([]models.Event, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var events []models.Event

	query := h.db.Where("is_published = ? AND ((start_date >= ? AND start_date < ?) OR (start_date < ? AND end_date >= ?))",
		true, startOfDay, endOfDay, startOfDay, startOfDay).
		Preload("Author").
		Preload("Category").
		Order("start_date ASC")

	err := query.Find(&events).Error
	return events, err
}

// Helper: Get upcoming events (next 5 events starting after today)
func (h *HomeHandler) getUpcomingEvents(userID uint, role string) ([]models.Event, error) {
	tomorrow := time.Now().Add(24 * time.Hour)
	startOfTomorrow := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())

	var events []models.Event

	query := h.db.Where("is_published = ? AND start_date > ?", true, startOfTomorrow).
		Preload("Author").
		Preload("Category").
		Order("start_date ASC").
		Limit(5)

	err := query.Find(&events).Error
	return events, err
}

// Helper: Get recent news (last 5 published articles filtered by user groups)
func (h *HomeHandler) getRecentNews(userID uint, role string) ([]models.News, error) {
	var news []models.News
	var err error

	// Les admins voient tous les articles publiés
	if role == "admin" {
		err = h.db.Where("is_published = ?", true).
			Preload("Author").
			Preload("Category").
			Preload("Tags").
			Preload("TargetGroups").
			Order("is_pinned DESC, published_at DESC").
			Limit(5).
			Find(&news).Error

		if err != nil {
			return news, err
		}
	} else {
		// Récupérer les groupes administrés ET les groupes d'appartenance
		var managedGroupIDs []uint
		h.db.Table("group_admins").Where("user_id = ?", userID).Pluck("group_id", &managedGroupIDs)

		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		// Combiner les deux listes (appartenance + administration)
		allGroupIDs := make(map[uint]bool)
		for _, id := range userGroupIDs {
			allGroupIDs[id] = true
		}
		for _, id := range managedGroupIDs {
			allGroupIDs[id] = true
		}

		var combinedGroupIDs []uint
		for id := range allGroupIDs {
			combinedGroupIDs = append(combinedGroupIDs, id)
		}

		if len(combinedGroupIDs) > 0 {
			// L'utilisateur appartient à des groupes ou en administre
			// Afficher les articles globaux (sans target_groups) OU les articles ciblant ses groupes
			err = h.db.Where("is_published = ?", true).
				Where(`
					(SELECT COUNT(*) FROM news_target_groups WHERE news_target_groups.news_id = news.id) = 0
					OR EXISTS (
						SELECT 1 FROM news_target_groups
						WHERE news_target_groups.news_id = news.id
						AND news_target_groups.group_id IN (?)
					)
				`, combinedGroupIDs).
				Preload("Author").
				Preload("Category").
				Preload("Tags").
				Preload("TargetGroups").
				Order("is_pinned DESC, published_at DESC").
				Limit(5).
				Find(&news).Error

			if err != nil {
				return news, err
			}
		} else {
			// L'utilisateur n'appartient à aucun groupe : seulement les articles globaux
			err = h.db.Where("is_published = ?", true).
				Where("(SELECT COUNT(*) FROM news_target_groups WHERE news_target_groups.news_id = news.id) = 0").
				Preload("Author").
				Preload("Category").
				Preload("Tags").
				Preload("TargetGroups").
				Order("is_pinned DESC, published_at DESC").
				Limit(5).
				Find(&news).Error

			if err != nil {
				return news, err
			}
		}
	}

	// Ajouter les compteurs de commentaires et réactions pour chaque article
	for i := range news {
		// Compter les commentaires
		var commentCount int64
		h.db.Model(&models.Comment{}).Where("entity_type = ? AND entity_id = ? AND status = ?", "news", news[i].ID, "approved").Count(&commentCount)
		news[i].CommentCount = int(commentCount)

		// Compter les réactions
		var reactionCount int64
		h.db.Model(&models.Feedback{}).Where("entity_type = ? AND entity_id = ?", "news", news[i].ID).Count(&reactionCount)
		news[i].ReactionCount = int(reactionCount)
	}

	return news, err
}

// Helper: Get recent polls (last 10 polls - active and closed)
func (h *HomeHandler) getRecentPolls(userID uint, role string) ([]models.Poll, error) {
	var polls []models.Poll
	var err error

	// Les admins voient tous les sondages
	if role == "admin" {
		err = h.db.Preload("Author").
			Preload("Options").
			Order("created_at DESC").
			Limit(10).
			Find(&polls).Error

		if err != nil {
			return polls, err
		}
	} else {
		// Récupérer les groupes administrés ET les groupes d'appartenance
		var managedGroupIDs []uint
		h.db.Table("group_admins").Where("user_id = ?", userID).Pluck("group_id", &managedGroupIDs)

		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		// Combiner les deux listes (appartenance + administration)
		allGroupIDs := make(map[uint]bool)
		for _, id := range userGroupIDs {
			allGroupIDs[id] = true
		}
		for _, id := range managedGroupIDs {
			allGroupIDs[id] = true
		}

		var combinedGroupIDs []uint
		for id := range allGroupIDs {
			combinedGroupIDs = append(combinedGroupIDs, id)
		}

		if len(combinedGroupIDs) > 0 {
			// L'utilisateur appartient à des groupes ou en administre
			err = h.db.Preload("Author").
				Preload("Options").
				Where(`
					(SELECT COUNT(*) FROM poll_target_groups WHERE poll_target_groups.poll_id = polls.id) = 0
					OR EXISTS (
						SELECT 1 FROM poll_target_groups
						WHERE poll_target_groups.poll_id = polls.id
						AND poll_target_groups.group_id IN (?)
					)
				`, combinedGroupIDs).
				Order("created_at DESC").
				Limit(10).
				Find(&polls).Error

			if err != nil {
				return polls, err
			}
		} else {
			// L'utilisateur n'appartient à aucun groupe : seulement les sondages globaux
			err = h.db.Preload("Author").
				Preload("Options").
				Where("(SELECT COUNT(*) FROM poll_target_groups WHERE poll_target_groups.poll_id = polls.id) = 0").
				Order("created_at DESC").
				Limit(10).
				Find(&polls).Error

			if err != nil {
				return polls, err
			}
		}
	}

	// Calculer le nombre total de votes pour chaque sondage
	for i := range polls {
		var voteCount int64
		h.db.Model(&models.PollVote{}).Where("poll_id = ?", polls[i].ID).Count(&voteCount)
		polls[i].TotalVotes = int(voteCount)
	}

	return polls, err
}

// Helper: Get announcements with cache
func (h *HomeHandler) getAnnouncementsWithCache() ([]models.Announcement, error) {
	homeCache.mu.RLock()
	cached := homeCache.announcements
	homeCache.mu.RUnlock()

	if cached.Data != nil && time.Now().Before(cached.ExpiresAt) {
		return cached.Data.([]models.Announcement), nil
	}

	// Cache miss - query database
	var announcements []models.Announcement
	now := time.Now()
	err := h.db.Where("is_active = ? AND (start_date IS NULL OR start_date <= ?) AND (end_date IS NULL OR end_date >= ?)",
		true, now, now).
		Order("priority DESC, created_at DESC").
		Find(&announcements).Error

	if err != nil {
		return nil, err
	}

	// Update cache
	homeCache.mu.Lock()
	homeCache.announcements = &CachedData{
		Data:      announcements,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	homeCache.mu.Unlock()

	return announcements, nil
}

// Helper: Get app settings with cache
func (h *HomeHandler) getAppSettingsWithCache() (*models.AppSettings, error) {
	homeCache.mu.RLock()
	cached := homeCache.appSettings
	homeCache.mu.RUnlock()

	if cached.Data != nil && time.Now().Before(cached.ExpiresAt) {
		settings := cached.Data.(models.AppSettings)
		return &settings, nil
	}

	// Cache miss - query database
	var settings models.AppSettings
	err := h.db.First(&settings).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default settings if none exist
			settings = models.AppSettings{
				AppName:         "Airboard",
				AppIcon:         "mdi:view-dashboard",
				DashboardTitle:  "Dashboard",
				WelcomeMessage:  "Welcome to your application portal",
				HomePageMessage: "Discover your personalized workspace",
			}
		} else {
			return nil, err
		}
	}

	// Update cache
	homeCache.mu.Lock()
	homeCache.appSettings = &CachedData{
		Data:      settings,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	homeCache.mu.Unlock()

	return &settings, nil
}

// Helper: Get hero messages with cache
func (h *HomeHandler) getHeroMessagesWithCache() ([]models.HeroMessage, error) {
	homeCache.mu.RLock()
	cached := homeCache.heroMessages
	homeCache.mu.RUnlock()

	if cached != nil && cached.Data != nil && time.Now().Before(cached.ExpiresAt) {
		return cached.Data.([]models.HeroMessage), nil
	}

	// Cache miss - query database
	var messages []models.HeroMessage
	err := h.db.Where("is_active = ?", true).Order("created_at DESC").Find(&messages).Error

	if err != nil {
		return nil, err
	}

	// Update cache
	homeCache.mu.Lock()
	homeCache.heroMessages = &CachedData{
		Data:      messages,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	homeCache.mu.Unlock()

	return messages, nil
}

// Helper: Get stats based on role
func (h *HomeHandler) getStats(userID uint, role string, managedGroupIDs []uint) *HomeStats {
	stats := &HomeStats{}

	if role == "admin" {
		// Count all active users
		h.db.Model(&models.User{}).Where("is_active = ?", true).Count(&stats.TotalUsers)

		// Count all active groups
		h.db.Model(&models.Group{}).Where("is_active = ?", true).Count(&stats.TotalGroups)

		// Count all app groups
		h.db.Model(&models.AppGroup{}).Count(&stats.TotalAppGroups)

		// Count all active applications
		h.db.Model(&models.Application{}).Where("is_active = ?", true).Count(&stats.TotalApps)

		// Count all published news
		h.db.Model(&models.News{}).Where("is_published = ?", true).Count(&stats.TotalNews)

		// Count all published events
		h.db.Model(&models.Event{}).Where("is_published = ?", true).Count(&stats.TotalEvents)

		// Count all polls
		h.db.Model(&models.Poll{}).Count(&stats.TotalPolls)
	} else if len(managedGroupIDs) > 0 {
		stats.ManagedGroupsCount = int64(len(managedGroupIDs))

		// Count total members in managed groups (distinct users)
		h.db.Model(&models.User{}).
			Joins("JOIN user_groups ON users.id = user_groups.user_id").
			Where("user_groups.group_id IN ? AND users.is_active = ?", managedGroupIDs, true).
			Distinct("users.id").
			Count(&stats.TotalMembersCount)

		// Count private app groups owned by managed groups
		h.db.Model(&models.AppGroup{}).
			Where("is_private = ? AND owner_group_id IN ?", true, managedGroupIDs).
			Count(&stats.ManagedAppGroupsCount)

		// Count apps in managed private app groups
		h.db.Model(&models.Application{}).
			Joins("JOIN app_groups ON applications.app_group_id = app_groups.id").
			Where("app_groups.owner_group_id IN ? AND applications.is_active = ?", managedGroupIDs, true).
			Count(&stats.ManagedAppsCount)

		// Count published news (group admins can manage all news)
		h.db.Model(&models.News{}).Where("is_published = ?", true).Count(&stats.ManagedNewsCount)

		// Count polls accessible to managed groups
		h.db.Model(&models.Poll{}).
			Joins("JOIN poll_target_groups ON polls.id = poll_target_groups.poll_id").
			Where("poll_target_groups.group_id IN ?", managedGroupIDs).
			Distinct("polls.id").
			Count(&stats.ManagedPollsCount)
	}

	return stats
}

// Helper: Get gamification summary for a user
func (h *HomeHandler) getGamificationSummary(userID uint) *GamificationSummary {
	var profile models.GamificationProfile
	if err := h.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			profile = models.GamificationProfile{UserID: userID, Level: 1, XP: 0}
			h.db.Create(&profile)
		} else {
			return nil
		}
	}

	// Calculate progress
	currentXP := profile.XP
	currentLevel := profile.Level
	xpForCurrentLevel := int64(math.Pow(float64(currentLevel-1), 2) * 100)
	xpForNextLevel := int64(math.Pow(float64(currentLevel), 2) * 100)

	rangeNeed := xpForNextLevel - xpForCurrentLevel
	xpInRange := currentXP - xpForCurrentLevel

	progressPercent := 0
	if rangeNeed > 0 {
		progressPercent = int((float64(xpInRange) / float64(rangeNeed)) * 100)
	}

	// Fetch recent badges
	var userAchievements []models.UserAchievement
	h.db.Preload("Achievement").
		Where("user_id = ?", userID).
		Order("unlocked_at DESC").
		Limit(3).
		Find(&userAchievements)

	recentBadges := make([]models.Achievement, 0)
	for _, ua := range userAchievements {
		recentBadges = append(recentBadges, ua.Achievement)
	}

	return &GamificationSummary{
		Level:           profile.Level,
		XP:              profile.XP,
		NextLevelXP:     xpForNextLevel,
		ProgressPercent: progressPercent,
		RecentBadges:    recentBadges,
	}
}

// Cache implementation
type CachedData struct {
	Data      interface{}
	ExpiresAt time.Time
}

type HomeCache struct {
	mu            sync.RWMutex
	announcements *CachedData
	appSettings   *CachedData
	heroMessages  *CachedData
}

var homeCache = &HomeCache{
	announcements: &CachedData{},
	appSettings:   &CachedData{},
	heroMessages:  &CachedData{},
}

// InvalidateAnnouncements can be called from admin handlers when announcements are modified
func (cache *HomeCache) InvalidateAnnouncements() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.announcements = &CachedData{}
}

// InvalidateAppSettings can be called from admin handlers when settings are modified
func (cache *HomeCache) InvalidateAppSettings() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.appSettings = &CachedData{}
}

// InvalidateHeroMessages can be called from admin handlers when hero messages are modified
func (cache *HomeCache) InvalidateHeroMessages() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.heroMessages = &CachedData{}
}
