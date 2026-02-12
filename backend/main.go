package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"airboard/config"
	"airboard/handlers"
	"airboard/middleware"
	"airboard/models"
	"airboard/services"
	"airboard/services/chat" // Import chat service

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Charger la configuration
	cfg := config.LoadConfig()

	// Configuration Gin
	gin.SetMode(cfg.Server.Mode)

	// Connexion à la base de données
	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), &gorm.Config{})
	if err != nil {
		log.Fatal("Erreur de connexion à la base de données:", err)
	}

	// Migrations
	if err := db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.AppGroup{},
		&models.Application{},
		&models.AppSettings{},
		&models.OAuthProvider{},
		&models.ApplicationClick{},
		&models.Announcement{},
		&models.News{},
		&models.NewsCategory{},
		&models.Tag{},
		&models.NewsReaction{},
		&models.NewsRead{},
		&models.Event{},
		&models.EventCategory{},
		&models.SMTPConfig{},
		&models.EmailOAuthConfig{},
		&models.EmailTemplate{},
		&models.EmailNotificationLog{},
		&models.Media{},
		&models.Comment{},
		&models.Feedback{},
		&models.CommentSettings{},
		&models.Notification{},
		&models.Poll{},
		&models.PollOption{},
		&models.PollVote{},
		&models.ChatMessage{},         // Chat
		&models.GamificationProfile{}, // Gamification
		&models.Achievement{},
		&models.UserAchievement{},
		&models.XPTransaction{},
		&models.HeroMessage{}, // Dynamic Hero Messages
	); err != nil {
		log.Fatal("Erreur lors des migrations:", err)
	}

	// Créer les index uniques pour éviter les doublons
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_feedback_user_entity ON feedbacks(user_id, entity_type, entity_id)").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique pour feedbacks: %v", err)
	}

	// Index unique pour éviter qu'un utilisateur vote plusieurs fois sur la même option
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_poll_vote_user_option ON poll_votes(poll_id, user_id, poll_option_id)").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique pour poll_votes: %v", err)
	}

	// Fix: Corriger les contraintes d'unicité sur les slugs pour permettre la réutilisation après soft delete
	// News slug
	db.Exec("DROP INDEX IF EXISTS idx_news_slug")
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_news_slug ON news(slug) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique partiel pour news.slug: %v", err)
	} else {
		log.Println("✓ Index unique partiel créé/vérifié pour news.slug")
	}

	// NewsCategory slug
	db.Exec("DROP INDEX IF EXISTS idx_category_slug")
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_category_slug ON news_categories(slug) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique partiel pour news_categories.slug: %v", err)
	} else {
		log.Println("✓ Index unique partiel créé/vérifié pour news_categories.slug")
	}

	// Tag slug
	db.Exec("DROP INDEX IF EXISTS idx_tag_slug")
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_slug ON tags(slug) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique partiel pour tags.slug: %v", err)
	} else {
		log.Println("✓ Index unique partiel créé/vérifié pour tags.slug")
	}

	// Tag name
	db.Exec("DROP INDEX IF EXISTS idx_tag_name")
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_name ON tags(name) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique partiel pour tags.name: %v", err)
	} else {
		log.Println("✓ Index unique partiel créé/vérifié pour tags.name")
	}

	// Event slug
	db.Exec("DROP INDEX IF EXISTS idx_event_slug")
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_event_slug ON events(slug) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique partiel pour events.slug: %v", err)
	} else {
		log.Println("✓ Index unique partiel créé/vérifié pour events.slug")
	}

	// EventCategory name
	db.Exec("DROP INDEX IF EXISTS idx_event_category_name")
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_event_category_name ON event_categories(name) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique partiel pour event_categories.name: %v", err)
	} else {
		log.Println("✓ Index unique partiel créé/vérifié pour event_categories.name")
	}

	// EventCategory slug
	db.Exec("DROP INDEX IF EXISTS idx_event_category_slug")
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_event_category_slug ON event_categories(slug) WHERE deleted_at IS NULL").Error; err != nil {
		log.Printf("Avertissement: Impossible de créer l'index unique partiel pour event_categories.slug: %v", err)
	} else {
		log.Println("✓ Index unique partiel créé/vérifié pour event_categories.slug")
	}

	// Créer les données initiales
	if err := createInitialData(db, cfg); err != nil {
		log.Fatalf("Erreur lors de la création des données initiales: %v", err)
	}

	// Initialiser le service email global
	InitEmailService(db, cfg)

	// Initialiser le service de stockage
	storageService, err := services.NewLocalStorage(cfg.Storage.UploadDir, cfg.Storage.BaseURL)
	if err != nil {
		log.Fatal("Erreur d'initialisation du service de stockage:", err)
	}

	// Initialisation des middlewares
	authMiddleware := middleware.NewAuthMiddleware(cfg, db)
	ssoMiddleware := middleware.NewSSOMiddleware(db, cfg)
	csrfManager := middleware.NewCSRFManager()

	mediaHandler := handlers.NewMediaHandler(db, storageService)

	// Gamification
	gamificationService := services.NewGamificationService(db)

	// Initialisation des handlers
	authHandler := handlers.NewAuthHandler(db, authMiddleware, cfg.Server.SignupEnabled, cfg, gamificationService)
	dashboardHandler := handlers.NewDashboardHandler(db)
	adminHandler := handlers.NewAdminHandler(db, cfg, gamificationService)
	groupAdminHandler := handlers.NewGroupAdminHandler(db)
	settingsHandler := handlers.NewSettingsHandler(db)
	oauthHandler := handlers.NewOAuthHandler(db, authMiddleware)
	favoritesHandler := handlers.NewFavoritesHandler(db)
	analyticsHandler := handlers.NewAnalyticsHandler(db, gamificationService)
	announcementHandler := handlers.NewAnnouncementHandler(db)
	newsHandler := handlers.NewNewsHandler(db, cfg, gamificationService)
	eventsHandler := handlers.NewEventsHandler(db, gamificationService)
	homeHandler := handlers.NewHomeHandler(db)
	versionHandler := handlers.NewVersionHandler()
	emailHandler := handlers.NewEmailHandler(db, cfg)
	commentHandler := handlers.NewCommentHandler(db, gamificationService)
	feedbackHandler := handlers.NewFeedbackHandler(db)
	notificationHandler := handlers.NewNotificationHandler(db)
	pollsHandler := handlers.NewPollsHandler(db, gamificationService)
	gamificationHandler := handlers.NewGamificationHandler(db, gamificationService)
	searchHandler := handlers.NewSearchHandler(db)

	// Seeding gamification
	if err := gamificationService.SeedAchievements(); err != nil {
		log.Printf("Erreur lors du seeding des achievements: %v", err)
	}

	// Initialisation du Chat
	chatHub := chat.NewHub()
	go chatHub.Run()
	chatHandler := handlers.NewChatHandler(db, chatHub)

	// Configuration du routeur sécurisée
	gin.SetMode(cfg.Server.Mode)

	// Créer un routeur personnalisé avec configuration sécurisée
	router := gin.New()

	// Configuration des proxies de confiance (sécurisée)
	trustedProxies := []string{
		"127.0.0.1", // Localhost
		"::1",       // IPv6 localhost
	}

	// En production, ajouter les réseaux privés si nécessaire
	if cfg.Server.Mode == "release" {
		trustedProxies = append(trustedProxies,
			"172.16.0.0/12",  // Docker networks
			"192.168.0.0/16", // Private networks
			"10.0.0.0/8",     // Private networks
		)
	}

	// Définir les proxies de confiance pour éviter l'IP spoofing
	router.SetTrustedProxies(trustedProxies)

	// Middleware de logging sécurisé avec vraie IP
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] \"%s %s %s\" %d %v %s %s %s\n",
			param.TimeStamp.Format("2006/01/02 15:04:05"),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.ClientIP, // Utilise l'IP réelle après SetTrustedProxies
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	// Middleware de récupération d'erreurs
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Printf("Panic occurred: %s", err)
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
	}))

	// Middleware CORS
	router.Use(middleware.SetupCORS(cfg))

	// Middleware SSO (détection des headers Authentik)
	router.Use(ssoMiddleware.DetectSSO())

	// Serve uploaded files statically
	router.Static("/uploads", cfg.Storage.UploadDir)

	// Routes publiques
	api := router.Group("/api/v1")
	{
		// Gamification
		gamification := api.Group("/gamification")
		gamification.Use(authMiddleware.RequireAuth())
		{
			gamification.GET("/profile", gamificationHandler.GetMyProfile)
			gamification.GET("/achievements", gamificationHandler.GetMyAchievements)
			gamification.GET("/achievements/all", gamificationHandler.GetAllAchievements)
			gamification.GET("/leaderboard", gamificationHandler.GetLeaderboard)
			gamification.GET("/transactions", gamificationHandler.GetMyTransactions)
		}

		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/register", authHandler.Register)
			auth.POST("/refresh", authHandler.RefreshToken)

			// Route pour vérifier si l'inscription est activée
			signup := auth.Group("/signup")
			{
				signup.GET("/status", authHandler.GetSignupStatus)
			}

			// Route SSO auto-login (accessible publiquement mais nécessite headers Authentik)
			sso := auth.Group("/sso")
			{
				sso.GET("/auto-login", authHandler.SSOAutoLogin)
			}

			// Routes OAuth publiques
			oauth := auth.Group("/oauth")
			{
				oauth.GET("/providers", oauthHandler.GetEnabledProviders)
				oauth.GET("/:provider/initiate", oauthHandler.InitiateOAuth)
				// Accepter GET et POST pour le callback (Microsoft redirige en GET, frontend peut POST)
				oauth.GET("/:provider/callback", oauthHandler.OAuthCallback)
				oauth.POST("/:provider/callback", oauthHandler.OAuthCallback)
			}
		}

		// Routes version (publiques)
		version := api.Group("/version")
		{
			version.GET("", versionHandler.GetVersion)
			version.GET("/check-updates", versionHandler.CheckForUpdates)
		}
	}

	// Routes protégées - Ordre correct: Auth d'abord, puis CSRF
	protected := api.Group("/")
	protected.Use(authMiddleware.RequireAuth())
	protected.Use(middleware.OptionalCSRFProtection(csrfManager))
	{
		// Route pour générer un token CSRF
		protected.POST("/auth/csrf-token", middleware.CSRFTokenHandler(csrfManager))

		// Profil utilisateur
		protected.GET("/auth/profile", authHandler.GetProfile)
		protected.PUT("/auth/profile", authHandler.UpdateProfile)
		protected.POST("/auth/change-password", authHandler.ChangePassword)
		protected.POST("/auth/avatar", authHandler.UploadAvatar)
		protected.DELETE("/auth/avatar", authHandler.DeleteAvatar)

		// Dashboard
		protected.GET("/dashboard", dashboardHandler.GetDashboard)

		// Home page
		protected.GET("/home", homeHandler.GetHomeData)

		// Routes favorites
		user := protected.Group("/user")
		{
			user.GET("/favorites", favoritesHandler.GetUserFavorites)
			user.POST("/favorites", favoritesHandler.AddFavorite)
			user.DELETE("/favorites/:id", favoritesHandler.RemoveFavorite)
			user.GET("/favorites/:id/check", favoritesHandler.IsFavorite)
		}

		// Routes analytics (tracking accessible à tous les utilisateurs connectés)
		analytics := protected.Group("/analytics")
		{
			analytics.POST("/track", analyticsHandler.TrackClick)
		}

		// Recherche globale
		protected.GET("/search", searchHandler.GlobalSearch)

		// Routes announcements (accessible à tous les utilisateurs connectés)
		protected.GET("/announcements", announcementHandler.GetActiveAnnouncements)

		// Routes News Hub (accessible à tous les utilisateurs connectés)
		news := protected.Group("/news")
		{
			news.GET("", newsHandler.GetNews) // Liste des news avec filtres

			// Routes spécifiques d'abord (avant les routes avec paramètres)
			news.GET("/unread/count", newsHandler.GetUnreadCount) // Nombre de news non lues
			news.GET("/categories", newsHandler.GetCategories)    // Catégories (lecture seule)
			news.GET("/tags", newsHandler.GetTags)                // Tags (lecture seule)

			// Routes avec ID numérique
			news.POST("/:id/view", newsHandler.IncrementView)     // Incrémenter les vues
			news.GET("/:id/reactions", newsHandler.GetReactions)  // Récupérer les réactions
			news.POST("/:id/react", newsHandler.AddReaction)      // Ajouter une réaction
			news.DELETE("/:id/react", newsHandler.RemoveReaction) // Retirer une réaction

			// Route slug en dernier (greedy wildcard)
			news.GET("/article/:slug", newsHandler.GetNewsBySlug) // Récupérer une news par slug
		}

		// Routes Media (accessible à tous les utilisateurs connectés - editors et admins peuvent uploader)
		media := protected.Group("/media")
		{
			media.GET("", mediaHandler.GetMediaList)       // Liste des médias avec pagination et filtres
			media.GET("/:id", mediaHandler.GetMedia)       // Récupérer un média par ID
			media.DELETE("/:id", mediaHandler.DeleteMedia) // Supprimer un média (uploader ou admin)
		}

		// Routes Events (accessible à tous les utilisateurs connectés)
		events := protected.Group("/events")
		{
			events.GET("", eventsHandler.GetEvents)                // Liste des événements avec filtres
			events.GET("/calendar", eventsHandler.GetCalendarView) // Vue calendrier (expand récurrences)
			events.GET("/categories", eventsHandler.GetCategories) // Catégories (lecture seule)
			events.GET("/:slug", eventsHandler.GetEventBySlug)     // Récupérer un événement par slug
		}

		// Routes Commentaires (accessible à tous les utilisateurs connectés)
		comments := protected.Group("/comments")
		{
			comments.GET("", commentHandler.GetComments)                 // Récupérer les commentaires d'une entité
			comments.POST("", commentHandler.CreateComment)              // Créer un commentaire
			comments.PUT("/:id", commentHandler.UpdateComment)           // Modifier un commentaire
			comments.DELETE("/:id", commentHandler.DeleteComment)        // Supprimer un commentaire
			comments.GET("/settings", commentHandler.GetCommentSettings) // Récupérer les paramètres
		}

		// Routes Feedback (accessible à tous les utilisateurs connectés)
		feedback := protected.Group("/feedback")
		{
			feedback.GET("/stats", feedbackHandler.GetFeedbackStats) // Statistiques de feedback
			feedback.POST("", feedbackHandler.AddFeedback)           // Ajouter/modifier un feedback
			feedback.DELETE("", feedbackHandler.RemoveFeedback)      // Supprimer un feedback
		}

		// Routes Notifications (accessible à tous les utilisateurs connectés)
		notifications := protected.Group("/notifications")
		{
			notifications.GET("", notificationHandler.GetNotifications)            // Récupérer les notifications
			notifications.GET("/unread/count", notificationHandler.GetUnreadCount) // Nombre de notifications non lues
			notifications.GET("/stats", notificationHandler.GetNotificationStats)  // Statistiques
			notifications.PUT("/:id/read", notificationHandler.MarkAsRead)         // Marquer comme lue
			notifications.PUT("/read-all", notificationHandler.MarkAllAsRead)      // Tout marquer comme lu
			notifications.DELETE("/:id", notificationHandler.DeleteNotification)   // Supprimer une notification
			notifications.DELETE("/read/all", notificationHandler.DeleteAllRead)   // Supprimer toutes les notifications lues
		}

		// Routes Polls (accessible à tous les utilisateurs connectés)
		polls := protected.Group("/polls")
		{
			polls.GET("", pollsHandler.GetPolls)                   // Liste des sondages avec filtres
			polls.GET("/:id", pollsHandler.GetPollByID)            // Récupérer un sondage par ID
			polls.POST("/:id/vote", pollsHandler.Vote)             // Voter pour un sondage
			polls.GET("/:id/results", pollsHandler.GetPollResults) // Récupérer les résultats d'un sondage
		}

		// Routes Chat (accessible à tous les utilisateurs connectés)
		chatGroup := protected.Group("/chat")
		{
			chatGroup.GET("/contacts", chatHandler.GetContacts)
			chatGroup.GET("/history", chatHandler.GetHistory)
			chatGroup.DELETE("/messages/:id", chatHandler.DeleteMessage)
			chatGroup.DELETE("/history", chatHandler.ClearConversation)
		}

		// Route WebSocket (publique mais sécurisée par token en query param si nécessaire, ou gérée par middleware si header supporté)
		// Note : protected.Group use authMiddleware, which mostly looks for Authorization Header.
		// Native WebSockets in browsers don't support custom headers easily.
		// We might need a separate route group or accept query param auth in middleware.
		// For MVP, if AuthMiddleware supports checking Cookie or Query Token, it works.
		// Let's assume AuthMiddleware checks header. We might need to make WS explicit.
		// IMPORTANT: For now, we put it under protected, implying the client must find a way to pass auth (e.g. Protocol header or if we change middleware).
		// EASIER: Make it public but do manual check as we implemented in ServeWS.
		// But ServeWS relies on context UserID set by Middleware.
		// -> We will stick to protected and assume client sends token (e.g. via library that supports it or cookie).
		// If using browser native WebSocket, we usually use logic in ServeWS to parse query param "?token=..." if header missing.
		// Let's explicitly allow /ws to be outside main protected group if needed, but for now we try inside.
		// actually, our ServeWS implementation checks context "user_id". So it NEEDS the middleware.
		// We can tell middleware to look at query param "d_token" or "token".
		// For this implementation, let's keep it here.
		protected.GET("/ws", chatHandler.ServeWS)

		// Routes admin
		admin := protected.Group("/admin")
		admin.Use(authMiddleware.RequireAdmin())
		{
			// Gestion des groupes d'applications
			admin.GET("/app-groups", adminHandler.GetAppGroups)
			admin.POST("/app-groups", adminHandler.CreateAppGroup)
			admin.PUT("/app-groups/:id", adminHandler.UpdateAppGroup)
			admin.DELETE("/app-groups/:id", adminHandler.DeleteAppGroup)

			// Gestion des applications
			admin.GET("/applications", adminHandler.GetApplications)
			admin.POST("/applications", adminHandler.CreateApplication)
			admin.PUT("/applications/:id", adminHandler.UpdateApplication)
			admin.DELETE("/applications/:id", adminHandler.DeleteApplication)

			// Gestion des utilisateurs
			admin.GET("/users", adminHandler.GetUsers)
			admin.POST("/users", adminHandler.CreateUser)
			admin.PUT("/users/:id", adminHandler.UpdateUser)
			admin.DELETE("/users/:id", adminHandler.DeleteUser)
			admin.GET("/users/deleted", adminHandler.GetDeletedUsers)
			admin.POST("/users/:id/restore", adminHandler.RestoreUser)
			admin.DELETE("/users/:id/permanent", adminHandler.PermanentlyDeleteUser)

			// Gestion des groupes d'utilisateurs
			admin.GET("/groups", adminHandler.GetGroups)
			admin.POST("/groups", adminHandler.CreateGroup)
			admin.PUT("/groups/:id", adminHandler.UpdateGroup)
			admin.DELETE("/groups/:id", adminHandler.DeleteGroup)

			// Gestion des group admins (admin uniquement)
			admin.GET("/groups/:id/admins", adminHandler.GetGroupAdmins)
			admin.PUT("/groups/:id/admins", adminHandler.AssignGroupAdmins)

			// Gestion des paramètres de l'application
			admin.GET("/settings", settingsHandler.GetAppSettings)
			admin.PUT("/settings", settingsHandler.UpdateAppSettings)
			admin.POST("/settings/reset", settingsHandler.ResetAppSettings)

			// Gestion des messages Hero
			admin.GET("/settings/hero-messages", settingsHandler.GetHeroMessages)
			admin.POST("/settings/hero-messages", settingsHandler.CreateHeroMessage)
			admin.PUT("/settings/hero-messages/:id", settingsHandler.UpdateHeroMessage)
			admin.DELETE("/settings/hero-messages/:id", settingsHandler.DeleteHeroMessage)

			// Gestion des fournisseurs OAuth
			admin.GET("/oauth/providers", oauthHandler.GetAllProviders)
			admin.PUT("/oauth/providers/:id", oauthHandler.UpdateProvider)

			// Analytics (réservé aux admins)
			admin.GET("/analytics/dashboard", analyticsHandler.GetDashboard)
			admin.GET("/analytics/applications/:id", analyticsHandler.GetApplicationStats)
			admin.GET("/analytics/users/:id", analyticsHandler.GetUserStats)

			// Gestion des annonces (réservé aux admins)
			admin.GET("/announcements", announcementHandler.GetAllAnnouncements)
			admin.GET("/announcements/:id", announcementHandler.GetAnnouncement)
			admin.POST("/announcements", announcementHandler.CreateAnnouncement)
			admin.PUT("/announcements/:id", announcementHandler.UpdateAnnouncement)
			admin.DELETE("/announcements/:id", announcementHandler.DeleteAnnouncement)

			// Gestion de la base de données
			admin.POST("/database/reset", adminHandler.ResetDatabase)

			// Gestion des catégories de news (admin uniquement)
			admin.POST("/news/categories", newsHandler.CreateCategory)
			admin.PUT("/news/categories/:id", newsHandler.UpdateCategory)
			admin.DELETE("/news/categories/:id", newsHandler.DeleteCategory)

			// Épingler des news (admin uniquement)
			admin.POST("/news/:id/pin", newsHandler.TogglePin)

			// Analytics News (admin uniquement)
			admin.GET("/news/analytics", newsHandler.GetAnalytics)

			// Gestion des événements (admin uniquement)
			admin.GET("/events", eventsHandler.ListEvents)
			admin.POST("/events", eventsHandler.CreateEvent)
			admin.PUT("/events/:id", eventsHandler.UpdateEvent)
			admin.DELETE("/events/:id", eventsHandler.DeleteEvent)

			// Gestion des catégories d'événements (admin uniquement)
			admin.POST("/events/categories", eventsHandler.CreateCategory)
			admin.PUT("/events/categories/:id", eventsHandler.UpdateCategory)
			admin.DELETE("/events/categories/:id", eventsHandler.DeleteCategory)

			// Analytics Events (admin uniquement)
			admin.GET("/events/analytics", eventsHandler.GetAnalytics)

			// Gestion des jours fériés (admin uniquement)
			admin.GET("/events/holidays/countries", eventsHandler.GetAvailableCountries)
			admin.GET("/events/holidays/preview", eventsHandler.PreviewHolidays)
			admin.POST("/events/holidays/import", eventsHandler.ImportHolidays)
			admin.DELETE("/events/holidays", eventsHandler.DeleteHolidays)

			// Gestion des emails et notifications
			admin.GET("/email/smtp", emailHandler.GetSMTPConfig)
			admin.PUT("/email/smtp", emailHandler.UpdateSMTPConfig)
			admin.POST("/email/smtp/test", emailHandler.TestSMTPConfig)
			admin.GET("/email/templates", emailHandler.GetEmailTemplates)
			admin.GET("/email/templates/variables", emailHandler.GetTemplateVariables)
			admin.GET("/email/templates/:type", emailHandler.GetEmailTemplate)
			admin.PUT("/email/templates/:type", emailHandler.UpdateEmailTemplate)
			admin.POST("/email/templates/:type/reset", emailHandler.ResetEmailTemplate)
			admin.GET("/email/templates/:type/preview", emailHandler.PreviewTemplate)
			admin.GET("/email/logs", emailHandler.GetEmailLogs)

			// OAuth 2.0 configuration for email (admin only)
			admin.GET("/email/oauth", emailHandler.GetOAuthConfig)
			admin.PUT("/email/oauth", emailHandler.UpdateOAuthConfig)
			admin.POST("/email/oauth/test", emailHandler.TestOAuthConnection)
			admin.POST("/email/oauth/refresh", emailHandler.RefreshOAuthToken)
			admin.GET("/email/health", emailHandler.GetEmailHealthStatus)

			// Gestion des commentaires (modération - admin uniquement)
			admin.GET("/comments/pending", commentHandler.GetPendingComments)     // Commentaires en attente
			admin.POST("/comments/moderate", commentHandler.ModerateComment)      // Modérer un commentaire
			admin.PUT("/comments/settings", commentHandler.UpdateCommentSettings) // Mettre à jour les paramètres

			// Gestion des feedbacks (admin uniquement)
			admin.GET("/feedback/all", feedbackHandler.GetAllFeedback) // Tous les feedbacks d'une entité

			// Gestion des sondages (admin uniquement)
			admin.POST("/polls", pollsHandler.CreatePoll)
			admin.PUT("/polls/:id", pollsHandler.UpdatePoll)
			admin.DELETE("/polls/:id", pollsHandler.DeletePoll)
			admin.POST("/polls/:id/close", pollsHandler.ClosePoll)
			admin.GET("/polls/analytics", pollsHandler.GetAnalytics)

			// Gestion des médias (admin uniquement)
			admin.GET("/media", mediaHandler.GetMediaList)        // Liste des médias avec pagination et filtres
			admin.GET("/media/:id", mediaHandler.GetMedia)        // Récupérer un média par ID
			admin.POST("/media/upload", mediaHandler.UploadMedia) // Uploader un média
			admin.PUT("/media/:id", mediaHandler.UpdateMedia)     // Mettre à jour les métadonnées d'un média
			admin.DELETE("/media/:id", mediaHandler.DeleteMedia)  // Supprimer un média
		}

		// Routes editor (admin et editor peuvent créer/modifier des news et événements)
		editor := protected.Group("/editor")
		editor.Use(authMiddleware.RequireEditor())
		{
			// Gestion des news
			editor.POST("/news", newsHandler.CreateNews)
			editor.PUT("/news/:id", newsHandler.UpdateNews)
			editor.DELETE("/news/:id", newsHandler.DeleteNews)

			// Gestion des tags (editors peuvent créer des tags)
			editor.POST("/news/tags", newsHandler.CreateTag)
			editor.PUT("/news/tags/:id", newsHandler.UpdateTag)
			editor.DELETE("/news/tags/:id", newsHandler.DeleteTag)

			// Upload de médias (editors, group_admins et admins peuvent uploader)
			editor.POST("/media/upload", mediaHandler.UploadMedia)

			// Gestion des événements
			editor.POST("/events", eventsHandler.CreateEvent)
			editor.PUT("/events/:id", eventsHandler.UpdateEvent)
			editor.DELETE("/events/:id", eventsHandler.DeleteEvent)

			// Modération des commentaires (editors peuvent aussi modérer)
			editor.GET("/comments/pending", commentHandler.GetPendingComments)
			editor.POST("/comments/moderate", commentHandler.ModerateComment)

			// Gestion des sondages (editors peuvent créer/modifier/supprimer des sondages)
			editor.POST("/polls", pollsHandler.CreatePoll)
			editor.PUT("/polls/:id", pollsHandler.UpdatePoll)
			editor.DELETE("/polls/:id", pollsHandler.DeletePoll)
		}

		// Routes group-admin (gestion limitée au périmètre)
		groupAdmin := protected.Group("/group-admin")
		groupAdmin.Use(authMiddleware.RequireGroupAdmin())
		{
			// AppGroups (scoped)
			groupAdmin.GET("/app-groups", groupAdminHandler.GetAppGroups)
			groupAdmin.POST("/app-groups", adminHandler.CreateAppGroup)
			groupAdmin.PUT("/app-groups/:id", adminHandler.UpdateAppGroup)
			groupAdmin.DELETE("/app-groups/:id", adminHandler.DeleteAppGroup)

			// Applications (scoped)
			groupAdmin.GET("/applications", groupAdminHandler.GetApplications)
			groupAdmin.POST("/applications", groupAdminHandler.CreateApplication)
			groupAdmin.PUT("/applications/:id", groupAdminHandler.UpdateApplication)
			groupAdmin.DELETE("/applications/:id", groupAdminHandler.DeleteApplication)

			// News (scoped)
			groupAdmin.GET("/news", newsHandler.GetNews) // Liste des news avec filtrage automatique par rôle
			groupAdmin.POST("/news", newsHandler.CreateNews)
			groupAdmin.PUT("/news/:id", newsHandler.UpdateNews)
			groupAdmin.DELETE("/news/:id", newsHandler.DeleteNews)

			// Upload de médias
			groupAdmin.POST("/media/upload", mediaHandler.UploadMedia)

			// Tags (group admin peut créer/modifier des tags)
			groupAdmin.POST("/news/tags", newsHandler.CreateTag)
			groupAdmin.PUT("/news/tags/:id", newsHandler.UpdateTag)
			groupAdmin.DELETE("/news/tags/:id", newsHandler.DeleteTag)

			// Categories (group admin peut créer/modifier des catégories)
			groupAdmin.POST("/news/categories", newsHandler.CreateCategory)
			groupAdmin.PUT("/news/categories/:id", newsHandler.UpdateCategory)
			groupAdmin.DELETE("/news/categories/:id", newsHandler.DeleteCategory)

			// Events (scoped)
			groupAdmin.GET("/events", eventsHandler.GetEvents) // Liste des événements avec filtrage automatique par rôle
			groupAdmin.POST("/events", eventsHandler.CreateEventGroupAdmin)
			groupAdmin.PUT("/events/:id", eventsHandler.UpdateEventGroupAdmin)
			groupAdmin.DELETE("/events/:id", eventsHandler.DeleteEventGroupAdmin)

			// Polls (scoped - group admin peut gérer les sondages de ses groupes)
			groupAdmin.GET("/polls", pollsHandler.GetPolls) // Liste des sondages avec filtrage automatique par rôle
			groupAdmin.POST("/polls", pollsHandler.CreatePoll)
			groupAdmin.PUT("/polls/:id", pollsHandler.UpdatePoll)
			groupAdmin.DELETE("/polls/:id", pollsHandler.DeletePoll)
			groupAdmin.POST("/polls/:id/close", pollsHandler.ClosePoll)

			// Info sur les groupes administrés
			groupAdmin.GET("/managed-groups", groupAdminHandler.GetManagedGroups)
		}
	}

	// Route de santé
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Airboard API is running",
		})
	})

	// Documentation Swagger (optionnel)
	// router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Printf("🚀 Serveur Airboard démarré sur le port %s", cfg.Server.Port)
	log.Printf("📊 Dashboard: http://localhost:%s/health", cfg.Server.Port)
	log.Printf("📚 Mode: %s", cfg.Server.Mode)

	// Démarrer le serveur
	router.Run(":" + cfg.Server.Port)
}

func createInitialData(db *gorm.DB, cfg *config.Config) (err error) {
	// Commencer une transaction pour garantir la cohérence des données
	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = fmt.Errorf("panic during data initialization: %v", r)
		}
	}()

	// Créer ou réinitialiser un utilisateur admin par défaut
	var adminUser models.User
	// Utiliser coût bcrypt sécurisé (12 minimum - OWASP 2025)
	hashedAdminPassword, bcryptErr := bcrypt.GenerateFromPassword([]byte("admin123"), cfg.Security.BcryptCost)
	if bcryptErr != nil {
		return fmt.Errorf("failed to hash admin password: %w", bcryptErr)
	}

	if err = tx.Unscoped().Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Créer l'utilisateur admin
			adminUser = models.User{
				Username:  "admin",
				Email:     "admin@airboard.com",
				Password:  string(hashedAdminPassword),
				FirstName: "Admin",
				LastName:  "Airboard",
				Role:      "admin",
				IsActive:  true,
			}
			if err = tx.Create(&adminUser).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create admin user: %w", err)
			}
			log.Println("✅ Utilisateur admin créé: admin@airboard.com / admin123")
		} else {
			tx.Rollback()
			return fmt.Errorf("failed to check for existing admin user: %w", err)
		}
	} else {
		// Réinitialiser le mot de passe si l'utilisateur existe déjà
		adminUser.Password = string(hashedAdminPassword)
		adminUser.IsActive = true
		adminUser.Role = "admin"
		if err = tx.Save(&adminUser).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update admin user: %w", err)
		}
		log.Println("🔄 Mot de passe admin réinitialisé: admin@airboard.com / admin123")
	}

	// Créer un utilisateur normal par défaut
	var normalUser models.User
	var userExists bool
	if err = tx.Unscoped().Where("username = ?", "user").First(&normalUser).Error; err == nil {
		userExists = true
	} else if err != gorm.ErrRecordNotFound {
		tx.Rollback()
		return fmt.Errorf("failed to check for existing user: %w", err)
	}

	if !userExists {
		// Utiliser coût bcrypt sécurisé (12 minimum - OWASP 2025)
		hashedUserPassword, bcryptErr := bcrypt.GenerateFromPassword([]byte("user123"), cfg.Security.BcryptCost)
		if bcryptErr != nil {
			tx.Rollback()
			return fmt.Errorf("failed to hash user password: %w", bcryptErr)
		}

		user := models.User{
			Username:  "user",
			Email:     "user@airboard.com",
			Password:  string(hashedUserPassword),
			FirstName: "User",
			LastName:  "Demo",
			Role:      "user",
			IsActive:  true,
		}
		if err = tx.Create(&user).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create default user: %w", err)
		}
		log.Println("✅ Utilisateur demo créé: user@airboard.com / user123")
	} else {
		// Récupérer l'utilisateur existant (y compris les soft-deleted)
		if err = tx.Unscoped().Where("username = ?", "user").First(&normalUser).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to retrieve existing user: %w", err)
		}
	}

	// Créer des groupes d'applications de démonstration
	var devGroup models.AppGroup
	if err = tx.Unscoped().Where("name = ?", "Développement").First(&devGroup).Error; err == gorm.ErrRecordNotFound {
		devGroup = models.AppGroup{
			Name:        "Développement",
			Description: "Applications de développement",
			Color:       "#3B82F6",
			Icon:        "mdi:code-tags",
			Order:       1,
			IsActive:    true,
		}
		if err = tx.Create(&devGroup).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create development app group: %w", err)
		}

		// Applications de développement
		apps := []models.Application{
			{
				Name:         "GitLab",
				Description:  "Gestion de code source",
				URL:          "https://gitlab.com",
				Icon:         "mdi:gitlab",
				Color:        "#FC6D26",
				Order:        1,
				IsActive:     true,
				OpenInNewTab: true,
				AppGroupID:   devGroup.ID,
			},
			{
				Name:         "Jenkins",
				Description:  "Intégration continue",
				URL:          "https://jenkins.io",
				Icon:         "mdi:robot-industrial",
				Color:        "#D33833",
				Order:        2,
				IsActive:     true,
				OpenInNewTab: true,
				AppGroupID:   devGroup.ID,
			},
		}
		for _, app := range apps {
			if err = tx.Create(&app).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create application %s: %w", app.Name, err)
			}
		}
		log.Println("✅ Groupe Développement créé avec applications de demo")
	} else if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to check for development app group: %w", err)
	}

	// Créer un groupe Production
	var prodGroup models.AppGroup
	if err = tx.Unscoped().Where("name = ?", "Production").First(&prodGroup).Error; err == gorm.ErrRecordNotFound {
		prodGroup = models.AppGroup{
			Name:        "Production",
			Description: "Applications de production",
			Color:       "#10B981",
			Icon:        "mdi:server",
			Order:       2,
			IsActive:    true,
		}
		if err = tx.Create(&prodGroup).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create production app group: %w", err)
		}

		// Applications de production
		apps := []models.Application{
			{
				Name:         "Grafana",
				Description:  "Monitoring et métriques",
				URL:          "https://grafana.com",
				Icon:         "mdi:chart-line",
				Color:        "#F46800",
				Order:        1,
				IsActive:     true,
				OpenInNewTab: true,
				AppGroupID:   prodGroup.ID,
			},
			{
				Name:         "Prometheus",
				Description:  "Collecte de métriques",
				URL:          "https://prometheus.io",
				Icon:         "mdi:database-search",
				Color:        "#E6522C",
				Order:        2,
				IsActive:     true,
				OpenInNewTab: true,
				AppGroupID:   prodGroup.ID,
			},
		}
		for _, app := range apps {
			if err = tx.Create(&app).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create application %s: %w", app.Name, err)
			}
		}
		log.Println("✅ Groupe Production créé avec applications de demo")
	} else if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to check for production app group: %w", err)
	}

	// Créer un groupe d'utilisateurs de démonstration
	var demoGroup models.Group
	if err = tx.Unscoped().Where("name = ?", "Développeurs").First(&demoGroup).Error; err == gorm.ErrRecordNotFound {
		demoGroup = models.Group{
			Name:        "Développeurs",
			Description: "Équipe de développement",
			Color:       "#8B5CF6",
			IsActive:    true,
		}
		if err = tx.Create(&demoGroup).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create developers group: %w", err)
		}

		// Associer l'utilisateur normal au groupe
		if err = tx.Model(&demoGroup).Association("Users").Append(&normalUser); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to associate user with developers group: %w", err)
		}

		// Associer le groupe aux groupes d'applications
		if err = tx.Model(&demoGroup).Association("AppGroups").Append(&devGroup); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to associate dev group with developers group: %w", err)
		}

		if err = tx.Model(&demoGroup).Association("AppGroups").Append(&prodGroup); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to associate prod group with developers group: %w", err)
		}

		log.Println("✅ Groupe d'utilisateurs Développeurs créé avec permissions")
	} else if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to check for developers group: %w", err)
	}

	// Créer un groupe "Common" par défaut pour tous les nouveaux utilisateurs
	var commonGroup models.Group
	if err = tx.Unscoped().Where("LOWER(name) = ?", "common").First(&commonGroup).Error; err == gorm.ErrRecordNotFound {
		commonGroup = models.Group{
			Name:        "Common",
			Description: "Groupe par défaut pour tous les utilisateurs",
			Color:       "#6B7280",
			IsActive:    true,
		}
		if err = tx.Create(&commonGroup).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create common group: %w", err)
		}
		log.Println("✅ Groupe d'utilisateurs Common créé")
	} else if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		return fmt.Errorf("failed to check for common group: %w", err)
	}

	// Créer les fournisseurs OAuth par défaut
	if err = createDefaultOAuthProviders(tx, cfg, adminUser); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create OAuth providers: %w", err)
	}

	// Créer les templates email par défaut
	if err = createDefaultEmailTemplates(tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create email templates: %w", err)
	}

	// Valider la transaction
	if err = tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func createDefaultOAuthProviders(db *gorm.DB, cfg *config.Config, adminUser models.User) error {
	// Construire les redirect URIs basées sur PUBLIC_URL
	publicURL := cfg.Server.PublicURL

	// Google OAuth
	var googleProvider models.OAuthProvider
	err := db.Where("provider_name = ?", "google").First(&googleProvider).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check for existing Google OAuth provider: %w", err)
	}
	if err == gorm.ErrRecordNotFound {
		// Créer si n'existe pas
		googleProvider = models.OAuthProvider{
			ProviderName: "google",
			DisplayName:  "Google",
			Icon:         "mdi:google",
			IsEnabled:    false,
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
			Scopes:       "openid email profile",
			RedirectURI:  publicURL + "/auth/oauth/google/callback",
		}
		if err = db.Create(&googleProvider).Error; err != nil {
			return fmt.Errorf("failed to create Google OAuth provider: %w", err)
		}
		log.Printf("✅ Google OAuth provider créé (désactivé par défaut) - Redirect: %s", googleProvider.RedirectURI)
	} else {
		// Mettre à jour le redirect URI si différent
		newRedirectURI := publicURL + "/auth/oauth/google/callback"
		if googleProvider.RedirectURI != newRedirectURI {
			googleProvider.RedirectURI = newRedirectURI
			if err = db.Save(&googleProvider).Error; err != nil {
				return fmt.Errorf("failed to update Google OAuth redirect URI: %w", err)
			}
			log.Printf("🔄 Google OAuth redirect URI mis à jour: %s", googleProvider.RedirectURI)
		}
	}

	// Microsoft OAuth
	var microsoftProvider models.OAuthProvider
	err = db.Where("provider_name = ?", "microsoft").First(&microsoftProvider).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check for existing Microsoft OAuth provider: %w", err)
	}
	if err == gorm.ErrRecordNotFound {
		// Créer si n'existe pas
		microsoftProvider = models.OAuthProvider{
			ProviderName: "microsoft",
			DisplayName:  "Microsoft",
			Icon:         "mdi:microsoft",
			IsEnabled:    false,
			AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			UserInfoURL:  "https://graph.microsoft.com/v1.0/me",
			Scopes:       "openid email profile User.Read",
			RedirectURI:  publicURL + "/auth/oauth/microsoft/callback",
		}
		if err = db.Create(&microsoftProvider).Error; err != nil {
			return fmt.Errorf("failed to create Microsoft OAuth provider: %w", err)
		}
		log.Printf("✅ Microsoft OAuth provider créé (désactivé par défaut) - Redirect: %s", microsoftProvider.RedirectURI)
	} else {
		// Mettre à jour le redirect URI si différent
		newRedirectURI := publicURL + "/auth/oauth/microsoft/callback"
		if microsoftProvider.RedirectURI != newRedirectURI {
			microsoftProvider.RedirectURI = newRedirectURI
			if err = db.Save(&microsoftProvider).Error; err != nil {
				return fmt.Errorf("failed to update Microsoft OAuth redirect URI: %w", err)
			}
			log.Printf("🔄 Microsoft OAuth redirect URI mis à jour: %s", microsoftProvider.RedirectURI)
		}
	}

	// Créer des catégories d'événements de démonstration
	var meetingsCategory models.EventCategory
	if err = db.Unscoped().Where("slug = ?", "reunions").First(&meetingsCategory).Error; err == gorm.ErrRecordNotFound {
		meetingsCategory = models.EventCategory{
			Name:        "Réunions",
			Slug:        "reunions",
			Description: "Réunions et assemblées",
			Icon:        "mdi:account-group",
			Color:       "#3B82F6", // Bleu
			Order:       1,
			IsActive:    true,
		}
		if err = db.Create(&meetingsCategory).Error; err != nil {
			return fmt.Errorf("failed to create meetings event category: %w", err)
		}
		log.Println("✅ Catégorie d'événements 'Réunions' créée")
	} else if err != nil {
		return fmt.Errorf("failed to check for meetings event category: %w", err)
	}

	var trainingsCategory models.EventCategory
	if err = db.Unscoped().Where("slug = ?", "formations").First(&trainingsCategory).Error; err == gorm.ErrRecordNotFound {
		trainingsCategory = models.EventCategory{
			Name:        "Formations",
			Slug:        "formations",
			Description: "Formations et ateliers",
			Icon:        "mdi:school",
			Color:       "#F59E0B", // Orange
			Order:       2,
			IsActive:    true,
		}
		if err = db.Create(&trainingsCategory).Error; err != nil {
			return fmt.Errorf("failed to create trainings event category: %w", err)
		}
		log.Println("✅ Catégorie d'événements 'Formations' créée")
	} else if err != nil {
		return fmt.Errorf("failed to check for trainings event category: %w", err)
	}

	var socialCategory models.EventCategory
	if err = db.Unscoped().Where("slug = ?", "evenements-sociaux").First(&socialCategory).Error; err == gorm.ErrRecordNotFound {
		socialCategory = models.EventCategory{
			Name:        "Événements Sociaux",
			Slug:        "evenements-sociaux",
			Description: "Événements sociaux et célébrations",
			Icon:        "mdi:party-popper",
			Color:       "#10B981", // Vert
			Order:       3,
			IsActive:    true,
		}
		if err = db.Create(&socialCategory).Error; err != nil {
			return fmt.Errorf("failed to create social event category: %w", err)
		}
		log.Println("✅ Catégorie d'événements 'Événements Sociaux' créée")
	} else if err != nil {
		return fmt.Errorf("failed to check for social event category: %w", err)
	}

	var holidaysCategory models.EventCategory
	if err = db.Unscoped().Where("slug = ?", "jours-feries").First(&holidaysCategory).Error; err == gorm.ErrRecordNotFound {
		holidaysCategory = models.EventCategory{
			Name:        "Jours Fériés",
			Slug:        "jours-feries",
			Description: "Jours fériés et congés",
			Icon:        "mdi:palm-tree",
			Color:       "#EF4444", // Rouge
			Order:       4,
			IsActive:    true,
		}
		if err = db.Create(&holidaysCategory).Error; err != nil {
			return fmt.Errorf("failed to create holidays event category: %w", err)
		}
		log.Println("✅ Catégorie d'événements 'Jours Fériés' créée")
	} else if err != nil {
		return fmt.Errorf("failed to check for holidays event category: %w", err)
	}

	// Créer des événements de démonstration
	var eventCount int64
	if err = db.Model(&models.Event{}).Unscoped().Count(&eventCount).Error; err != nil {
		return fmt.Errorf("failed to count existing events: %w", err)
	}
	if eventCount == 0 {
		now := time.Now()
		nextWeek := now.AddDate(0, 0, 7)
		nextMonth := now.AddDate(0, 1, 0)
		lastMonth := now.AddDate(0, -1, 0)

		// 1. Événement one-time public (semaine prochaine)
		publishedAt := now
		event1 := models.Event{
			Title:       "Assemblée Générale",
			Slug:        "assemblee-generale",
			Description: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Assemblée générale annuelle de l'entreprise. Tous les employés sont invités à participer."}]}]}`,
			StartDate:   nextWeek,
			EndDate:     &nextWeek,
			IsAllDay:    false,
			Timezone:    "UTC",
			Location:    "Salle de conférence A",
			Color:       meetingsCategory.Color,
			Priority:    "important",
			Status:      "confirmed",
			IsPublished: true,
			PublishedAt: &publishedAt,
			AuthorID:    adminUser.ID,
			CategoryID:  &meetingsCategory.ID,
		}
		if err = db.Create(&event1).Error; err != nil {
			return fmt.Errorf("failed to create general assembly event: %w", err)
		}

		// 2. Événement récurrent hebdomadaire (Daily Standup)
		recurrenceRule := `{"type":"weekly","interval":1,"days_of_week":[1,3,5],"end_type":"never"}`
		event2 := models.Event{
			Title:          "Daily Standup",
			Slug:           "daily-standup",
			Description:    `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Réunion quotidienne de synchronisation d'équipe (15 min)."}]}]}`,
			StartDate:      time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.UTC),
			IsAllDay:       false,
			Timezone:       "UTC",
			IsRecurring:    true,
			RecurrenceRule: recurrenceRule,
			Location:       "Salle de réunion B",
			Color:          meetingsCategory.Color,
			Priority:       "normal",
			Status:         "confirmed",
			IsPublished:    true,
			PublishedAt:    &publishedAt,
			AuthorID:       adminUser.ID,
			CategoryID:     &meetingsCategory.ID,
		}
		if err = db.Create(&event2).Error; err != nil {
			return fmt.Errorf("failed to create daily standup event: %w", err)
		}

		// 3. All-day holiday (Jour de l'An 2026)
		newYear := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		event3 := models.Event{
			Title:       "Jour de l'An",
			Slug:        "jour-de-l-an-2026",
			Description: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Jour férié - Bonne année !"}]}]}`,
			StartDate:   newYear,
			IsAllDay:    true,
			Timezone:    "UTC",
			Color:       holidaysCategory.Color,
			Priority:    "normal",
			Status:      "confirmed",
			IsPublished: true,
			PublishedAt: &publishedAt,
			AuthorID:    adminUser.ID,
			CategoryID:  &holidaysCategory.ID,
		}
		if err = db.Create(&event3).Error; err != nil {
			return fmt.Errorf("failed to create new year event: %w", err)
		}

		// 4. Group-scoped event (IT group only)
		// Récupérer le groupe IT s'il existe
		var itGroup models.Group
		if err = db.Where("name = ?", "IT").First(&itGroup).Error; err == nil {
			event4 := models.Event{
				Title:       "Formation Sécurité",
				Slug:        "formation-securite",
				Description: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Formation obligatoire sur les bonnes pratiques de sécurité informatique."}]}]}`,
				StartDate:   nextMonth,
				EndDate:     &nextMonth,
				IsAllDay:    false,
				Timezone:    "UTC",
				Location:    "Salle de formation",
				Color:       trainingsCategory.Color,
				Priority:    "high",
				Status:      "confirmed",
				IsPublished: true,
				PublishedAt: &publishedAt,
				AuthorID:    adminUser.ID,
				CategoryID:  &trainingsCategory.ID,
			}
			if err = db.Create(&event4).Error; err != nil {
				return fmt.Errorf("failed to create security training event: %w", err)
			}
			if err := db.Model(&event4).Association("TargetGroups").Append(&itGroup); err != nil {
				return fmt.Errorf("failed to associate security training with IT group: %v", err)
			}
		}

		// 5. Récurrent mensuel (premier lundi du mois)
		recurrenceMonthly := `{"type":"monthly","interval":1,"day_of_month":1,"end_type":"never"}`
		firstMonday := time.Date(now.Year(), now.Month(), 1, 10, 0, 0, 0, time.UTC)
		for firstMonday.Weekday() != time.Monday {
			firstMonday = firstMonday.AddDate(0, 0, 1)
		}
		event5 := models.Event{
			Title:          "Séminaire Mensuel",
			Slug:           "seminaire-mensuel",
			Description:    `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Séminaire mensuel de présentation des nouveautés et partage de connaissances."}]}]}`,
			StartDate:      firstMonday,
			IsAllDay:       false,
			Timezone:       "UTC",
			IsRecurring:    true,
			RecurrenceRule: recurrenceMonthly,
			Location:       "Auditorium",
			Color:          meetingsCategory.Color,
			Priority:       "normal",
			Status:         "confirmed",
			IsPublished:    true,
			PublishedAt:    &publishedAt,
			AuthorID:       adminUser.ID,
			CategoryID:     &meetingsCategory.ID,
		}
		if err = db.Create(&event5).Error; err != nil {
			return fmt.Errorf("failed to create monthly seminar event: %w", err)
		}

		// 6. Événement passé (pour tester l'indicateur "passé")
		event6 := models.Event{
			Title:       "Revue Trimestrielle Q4 2024",
			Slug:        "revue-trimestrielle-q4-2024",
			Description: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Revue des résultats du quatrième trimestre 2024."}]}]}`,
			StartDate:   lastMonth,
			EndDate:     &lastMonth,
			IsAllDay:    false,
			Timezone:    "UTC",
			Location:    "Salle de conférence A",
			Color:       meetingsCategory.Color,
			Priority:    "normal",
			Status:      "confirmed",
			IsPublished: true,
			PublishedAt: &lastMonth,
			AuthorID:    adminUser.ID,
			CategoryID:  &meetingsCategory.ID,
		}
		if err = db.Create(&event6).Error; err != nil {
			return fmt.Errorf("failed to create quarterly review event: %w", err)
		}

		log.Println("✅ Événements de démonstration créés (6 événements)")
	}

	// Créer les paramètres de commentaires par défaut
	var commentSettings models.CommentSettings
	if err = db.First(&commentSettings).Error; err == gorm.ErrRecordNotFound {
		commentSettings = models.CommentSettings{
			CommentsEnabled:      true,
			NewsCommentsEnabled:  true,
			AppCommentsEnabled:   false, // Désactivé par défaut pour les applications
			EventCommentsEnabled: true,
			RequireModeration:    false, // Auto-approuvé par défaut
			AllowAnonymous:       false,
			MaxCommentLength:     1000,
		}
		if err = db.Create(&commentSettings).Error; err != nil {
			return fmt.Errorf("failed to create comment settings: %w", err)
		}
		log.Println("✅ Paramètres de commentaires créés (activés par défaut)")
	} else if err != nil {
		return fmt.Errorf("failed to check for comment settings: %w", err)
	}

	return nil
}

func createDefaultEmailTemplates(db *gorm.DB) error {
	// Vérifier si des templates existent déjà
	var count int64
	if err := db.Model(&models.EmailTemplate{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count email templates: %w", err)
	}

	if count > 0 {
		return nil // Templates déjà créés
	}

	// Créer les templates par défaut
	templates := models.GetDefaultEmailTemplates()
	for _, t := range templates {
		if err := db.Create(&t).Error; err != nil {
			return fmt.Errorf("failed to create email template %s: %w", t.Type, err)
		}
	}

	log.Println("✅ Templates d'email par défaut créés (4 templates)")
	return nil
}

// Variable globale pour le service email (utilisée par les handlers)
var emailService *services.EmailService

// InitEmailService initialise le service email global
func InitEmailService(db *gorm.DB, cfg *config.Config) {
	emailService = services.NewEmailService(db, cfg)
}

// GetEmailService retourne le service email global
func GetEmailService() *services.EmailService {
	return emailService
}
