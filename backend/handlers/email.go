package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"airboard/config"
	"airboard/models"
	"airboard/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// EmailHandler gère les endpoints de configuration email
type EmailHandler struct {
	db           *gorm.DB
	emailService *services.EmailService
	config       *config.Config
}

// NewEmailHandler crée une nouvelle instance du handler email
func NewEmailHandler(db *gorm.DB, cfg *config.Config) *EmailHandler {
	return &EmailHandler{
		db:           db,
		emailService: services.NewEmailService(db, cfg),
		config:       cfg,
	}
}

// GetSMTPConfig retourne la configuration email actuelle (OAuth uniquement)
func (h *EmailHandler) GetSMTPConfig(c *gin.Context) {
	var config models.SMTPConfig

	result := h.db.Preload("EmailOAuthConfig").First(&config)
	if result.Error == gorm.ErrRecordNotFound {
		// Retourner une config vide
		c.JSON(http.StatusOK, models.SMTPConfig{
			UseOAuth: true,
			FromName: "Airboard",
		})
		return
	} else if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la récupération de la configuration email"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateSMTPConfig crée ou met à jour la configuration email (OAuth uniquement)
func (h *EmailHandler) UpdateSMTPConfig(c *gin.Context) {
	var req models.SMTPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var smtpConfig models.SMTPConfig
	h.db.First(&smtpConfig)

	smtpConfig.FromEmail = req.FromEmail
	smtpConfig.FromName = req.FromName
	smtpConfig.IsEnabled = req.IsEnabled
	smtpConfig.UseOAuth = true // Toujours OAuth maintenant

	if smtpConfig.ID == 0 {
		if err := h.db.Create(&smtpConfig).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la création de la configuration email"})
			return
		}
	} else {
		if err := h.db.Save(&smtpConfig).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la mise à jour de la configuration email"})
			return
		}
	}

	c.JSON(http.StatusOK, smtpConfig)
}

// TestSMTPConfig envoie un email de test via OAuth
func (h *EmailHandler) TestSMTPConfig(c *gin.Context) {
	var req models.TestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Email] Erreur binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Données invalides: " + err.Error()})
		return
	}

	log.Printf("[Email] Test email OAuth demandé pour: %s", req.ToEmail)

	var smtpConfig models.SMTPConfig
	if err := h.db.Preload("EmailOAuthConfig").First(&smtpConfig).Error; err != nil {
		log.Printf("[Email] Configuration email non trouvée: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Configuration email non trouvée. Veuillez d'abord configurer OAuth."})
		return
	}

	if err := h.emailService.TestConnection(&smtpConfig, req.ToEmail); err != nil {
		now := time.Now()
		smtpConfig.LastTestedAt = &now
		smtpConfig.LastTestSuccess = false
		h.db.Save(&smtpConfig)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Test échoué: " + err.Error()})
		return
	}

	now := time.Now()
	smtpConfig.LastTestedAt = &now
	smtpConfig.LastTestSuccess = true
	h.db.Save(&smtpConfig)

	c.JSON(http.StatusOK, gin.H{"message": "Email de test envoyé avec succès via OAuth"})
}

// GetEmailTemplates retourne tous les templates d'email
func (h *EmailHandler) GetEmailTemplates(c *gin.Context) {
	var templates []models.EmailTemplate

	if err := h.db.Order("type").Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la récupération des templates"})
		return
	}

	// Si aucun template, créer les templates par défaut
	if len(templates) == 0 {
		defaultTemplates := models.GetDefaultEmailTemplates()
		for _, t := range defaultTemplates {
			h.db.Create(&t)
		}
		h.db.Order("type").Find(&templates)
	}

	c.JSON(http.StatusOK, templates)
}

// GetEmailTemplate retourne un template spécifique par type
func (h *EmailHandler) GetEmailTemplate(c *gin.Context) {
	templateType := c.Param("type")

	var template models.EmailTemplate
	if err := h.db.Where("type = ?", templateType).First(&template).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template non trouvé"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// UpdateEmailTemplate met à jour un template d'email
func (h *EmailHandler) UpdateEmailTemplate(c *gin.Context) {
	templateType := c.Param("type")

	var req models.EmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var template models.EmailTemplate
	if err := h.db.Where("type = ?", templateType).First(&template).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template non trouvé"})
		return
	}

	template.Subject = req.Subject
	template.HTMLBody = req.HTMLBody
	template.PlainTextBody = req.PlainTextBody
	template.IsEnabled = req.IsEnabled

	if err := h.db.Save(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la mise à jour du template"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// ResetEmailTemplate réinitialise un template à sa valeur par défaut
func (h *EmailHandler) ResetEmailTemplate(c *gin.Context) {
	templateType := c.Param("type")

	// Trouver le template par défaut
	defaultTemplates := models.GetDefaultEmailTemplates()
	var defaultTemplate *models.EmailTemplate
	for _, t := range defaultTemplates {
		if t.Type == templateType {
			defaultTemplate = &t
			break
		}
	}

	if defaultTemplate == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Type de template inconnu"})
		return
	}

	var template models.EmailTemplate
	if err := h.db.Where("type = ?", templateType).First(&template).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template non trouvé"})
		return
	}

	template.Subject = defaultTemplate.Subject
	template.HTMLBody = defaultTemplate.HTMLBody
	template.PlainTextBody = defaultTemplate.PlainTextBody
	template.Name = defaultTemplate.Name

	if err := h.db.Save(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la réinitialisation du template"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// PreviewTemplate retourne un aperçu du template avec des données exemple
func (h *EmailHandler) PreviewTemplate(c *gin.Context) {
	templateType := c.Param("type")

	var template models.EmailTemplate
	if err := h.db.Where("type = ?", templateType).First(&template).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template non trouvé"})
		return
	}

	// Récupérer les données exemple
	sampleData := h.emailService.GetSampleData(templateType)
	if sampleData == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Type de template inconnu"})
		return
	}

	// Exécuter les templates
	subject, err := h.emailService.ExecuteTemplate(template.Subject, sampleData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Erreur dans le sujet: " + err.Error()})
		return
	}

	body, err := h.emailService.ExecuteTemplate(template.HTMLBody, sampleData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Erreur dans le corps: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subject": subject,
		"body":    body,
	})
}

// GetEmailLogs retourne l'historique des notifications envoyées
func (h *EmailHandler) GetEmailLogs(c *gin.Context) {
	var logs []models.EmailNotificationLog

	if err := h.db.Order("created_at DESC").Limit(100).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la récupération des logs"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// GetTemplateVariables retourne les variables disponibles pour chaque type de template
func (h *EmailHandler) GetTemplateVariables(c *gin.Context) {
	variables := map[string][]map[string]string{
		"news": {
			{"name": "{{.Title}}", "description": "Titre de l'article"},
			{"name": "{{.Summary}}", "description": "Résumé de l'article"},
			{"name": "{{.Author}}", "description": "Nom de l'auteur"},
			{"name": "{{.Link}}", "description": "Lien vers l'article"},
			{"name": "{{.AppName}}", "description": "Nom de l'application"},
			{"name": "{{.PublishedAt}}", "description": "Date de publication"},
		},
		"application": {
			{"name": "{{.Name}}", "description": "Nom de l'application"},
			{"name": "{{.Description}}", "description": "Description de l'application"},
			{"name": "{{.URL}}", "description": "URL de l'application"},
			{"name": "{{.AppGroup}}", "description": "Catégorie de l'application"},
			{"name": "{{.AppName}}", "description": "Nom de l'application Airboard"},
		},
		"event": {
			{"name": "{{.Title}}", "description": "Titre de l'événement"},
			{"name": "{{.Description}}", "description": "Description de l'événement"},
			{"name": "{{.StartDate}}", "description": "Date de début"},
			{"name": "{{.EndDate}}", "description": "Date de fin"},
			{"name": "{{.Location}}", "description": "Lieu de l'événement"},
			{"name": "{{.Link}}", "description": "Lien vers l'événement"},
			{"name": "{{.AppName}}", "description": "Nom de l'application"},
		},
		"announcement": {
			{"name": "{{.Title}}", "description": "Titre de l'annonce"},
			{"name": "{{.Content}}", "description": "Contenu de l'annonce"},
			{"name": "{{.Type}}", "description": "Type d'annonce (info, warning, success, error)"},
			{"name": "{{.AppName}}", "description": "Nom de l'application"},
		},
	}

	c.JSON(http.StatusOK, variables)
}

// GetOAuthConfig retourne la configuration OAuth pour SMTP (sans secrets)
func (h *EmailHandler) GetOAuthConfig(c *gin.Context) {
	var smtpConfig models.SMTPConfig
	if err := h.db.Preload("EmailOAuthConfig").First(&smtpConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration SMTP non trouvée"})
		return
	}

	// Retourner OAuth config sans secrets
	if smtpConfig.EmailOAuthConfig != nil {
		// Masquer les secrets
		oauthConfig := *smtpConfig.EmailOAuthConfig
		oauthConfig.ClientSecret = ""
		oauthConfig.AccessToken = ""
		oauthConfig.RefreshToken = ""
		c.JSON(http.StatusOK, oauthConfig)
		return
	}

	c.JSON(http.StatusOK, nil)
}

// UpdateOAuthConfig crée ou met à jour la configuration OAuth
func (h *EmailHandler) UpdateOAuthConfig(c *gin.Context) {
	var req models.EmailOAuthConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Email OAuth] UpdateOAuthConfig - Provider: %s, Tenant: %s, Client: %s",
		req.Provider, req.TenantID, req.ClientID)

	// Get or create SMTP config
	var smtpConfig models.SMTPConfig
	if err := h.db.Preload("EmailOAuthConfig").First(&smtpConfig).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Veuillez d'abord configurer les paramètres SMTP"})
		return
	}

	oauthService := services.NewEmailOAuthService(h.db, h.config)

	// Create or update OAuth config
	var oauthConfig models.EmailOAuthConfig
	if smtpConfig.EmailOAuthConfig != nil {
		oauthConfig = *smtpConfig.EmailOAuthConfig
		log.Printf("[Email OAuth] Mise à jour config existante ID: %d", oauthConfig.ID)
	} else {
		oauthConfig.SMTPConfigID = smtpConfig.ID
		log.Printf("[Email OAuth] Création nouvelle config pour SMTP ID: %d", smtpConfig.ID)
	}

	// Update fields
	oauthConfig.Provider = req.Provider
	oauthConfig.TenantID = req.TenantID
	oauthConfig.ClientID = req.ClientID
	oauthConfig.GrantType = req.GrantType
	oauthConfig.IsEnabled = req.IsEnabled

	// Set scopes
	if req.Scopes != "" {
		oauthConfig.Scopes = req.Scopes
	} else {
		// Default scopes for Microsoft 365
		// For client_credentials flow, use Graph API scope (SMTP doesn't work with app-only auth)
		if req.GrantType == "client_credentials" {
			oauthConfig.Scopes = "https://graph.microsoft.com/.default"
		} else {
			// For delegated (refresh_token) flow, can use SMTP
			oauthConfig.Scopes = "https://outlook.office365.com/SMTP.Send"
		}
	}

	// Build token URLs with tenant ID
	if req.Provider == "microsoft" {
		// Set URLs with tenant placeholders if not already set
		if oauthConfig.AuthURL == "" || strings.Contains(oauthConfig.AuthURL, "{tenant}") {
			oauthConfig.AuthURL = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", req.TenantID)
		}
		if oauthConfig.TokenURL == "" || strings.Contains(oauthConfig.TokenURL, "{tenant}") {
			oauthConfig.TokenURL = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", req.TenantID)
		}
		log.Printf("[Email OAuth] Token URL configuré: %s", oauthConfig.TokenURL)
	}

	// Encrypt and store client secret if provided
	if req.ClientSecret != "" {
		encrypted, err := oauthService.EncryptToken(req.ClientSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors du chiffrement du secret client"})
			return
		}
		oauthConfig.ClientSecret = encrypted
		log.Printf("[Email OAuth] Client secret chiffré et stocké")
	}

	// For refresh_token flow, store the refresh token
	if req.GrantType == "refresh_token" && req.RefreshToken != "" {
		encrypted, err := oauthService.EncryptToken(req.RefreshToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors du chiffrement du refresh token"})
			return
		}
		oauthConfig.RefreshToken = encrypted
		log.Printf("[Email OAuth] Refresh token chiffré et stocké")
	}

	// Save OAuth config
	if oauthConfig.ID == 0 {
		if err := h.db.Create(&oauthConfig).Error; err != nil {
			log.Printf("[Email OAuth] Erreur création config: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la création de la configuration OAuth"})
			return
		}
		log.Printf("[Email OAuth] Config créée avec ID: %d", oauthConfig.ID)
	} else {
		if err := h.db.Save(&oauthConfig).Error; err != nil {
			log.Printf("[Email OAuth] Erreur sauvegarde config: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erreur lors de la mise à jour de la configuration OAuth"})
			return
		}
		log.Printf("[Email OAuth] Config mise à jour ID: %d", oauthConfig.ID)
	}

	// Try to acquire initial token if enabled
	if req.IsEnabled {
		log.Printf("[Email OAuth] Tentative acquisition token initial...")
		if err := oauthService.AcquireClientCredentialsToken(&oauthConfig); err != nil {
			log.Printf("[Email OAuth] Échec acquisition token initial: %v", err)
			c.JSON(http.StatusOK, gin.H{
				"message": "Configuration OAuth sauvegardée mais échec de l'acquisition du token",
				"error":   err.Error(),
			})
			return
		}
		log.Printf("[Email OAuth] Token initial acquis avec succès")
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration OAuth mise à jour avec succès"})
}

// TestOAuthConnection teste la connexion OAuth SMTP
func (h *EmailHandler) TestOAuthConnection(c *gin.Context) {
	var req models.EmailOAuthTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Email OAuth] Test connexion OAuth demandé pour: %s", req.ToEmail)

	var smtpConfig models.SMTPConfig
	if err := h.db.Preload("EmailOAuthConfig").First(&smtpConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration SMTP non trouvée"})
		return
	}

	if smtpConfig.EmailOAuthConfig == nil || !smtpConfig.EmailOAuthConfig.IsEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth non configuré ou désactivé"})
		return
	}

	// Récupérer le nom de l'application
	var appSettings models.AppSettings
	h.db.First(&appSettings)
	appName := appSettings.AppName
	if appName == "" {
		appName = "Airboard"
	}

	// Send test email using OAuth
	testSubject := fmt.Sprintf("%s - Test OAuth SMTP", appName)
	testBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
.container { max-width: 600px; margin: 20px auto; background: white; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
.header { background: linear-gradient(135deg, #10B981 0%%, #059669 100%%); color: white; padding: 30px; text-align: center; }
.header h1 { margin: 0; font-size: 24px; }
.content { padding: 30px; text-align: center; }
.success-icon { font-size: 48px; margin-bottom: 20px; }
.badge { display: inline-block; background: #dcfce7; color: #166534; padding: 8px 16px; border-radius: 8px; font-weight: 600; margin: 10px 0; }
.footer { background: #f8fafc; padding: 20px; text-align: center; color: #6b7280; font-size: 12px; }
</style>
</head>
<body>
<div class="container">
<div class="header">
<h1>%s</h1>
</div>
<div class="content">
<div class="success-icon">✅</div>
<h2>OAuth 2.0 SMTP Test Réussi !</h2>
<p>Cet email confirme que votre configuration OAuth 2.0 est correcte.</p>
<div class="badge">🔐 Authentification moderne OAuth 2.0</div>
<p><strong>Provider:</strong> %s<br>
<strong>Serveur:</strong> %s<br>
<strong>Port:</strong> %d</p>
</div>
<div class="footer">
<p>Email de test envoyé le %s</p>
<p>Powered by OAuth 2.0 XOAUTH2 SASL</p>
</div>
</div>
</body>
</html>`, appName, smtpConfig.EmailOAuthConfig.Provider, smtpConfig.Host, smtpConfig.Port, time.Now().Format("02/01/2006 à 15:04"))

	// Temporairement activer OAuth pour le test
	originalUseOAuth := smtpConfig.UseOAuth
	smtpConfig.UseOAuth = true

	if err := h.emailService.SendEmailWithOAuth(&smtpConfig, req.ToEmail, testSubject, testBody); err != nil {
		log.Printf("[Email OAuth] Test échoué: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Test OAuth échoué: " + err.Error()})
		return
	}

	// Restaurer l'état original
	smtpConfig.UseOAuth = originalUseOAuth

	log.Printf("[Email OAuth] Test réussi pour %s", req.ToEmail)
	c.JSON(http.StatusOK, gin.H{"message": "Email de test OAuth envoyé avec succès"})
}

// RefreshOAuthToken rafraîchit manuellement le token OAuth
func (h *EmailHandler) RefreshOAuthToken(c *gin.Context) {
	log.Printf("[Email OAuth] Rafraîchissement manuel du token demandé")

	var smtpConfig models.SMTPConfig
	if err := h.db.Preload("EmailOAuthConfig").First(&smtpConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration SMTP non trouvée"})
		return
	}

	if smtpConfig.EmailOAuthConfig == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth non configuré"})
		return
	}

	oauthService := services.NewEmailOAuthService(h.db, h.config)
	if err := oauthService.RefreshAccessToken(smtpConfig.EmailOAuthConfig); err != nil {
		log.Printf("[Email OAuth] Échec rafraîchissement: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Échec du rafraîchissement du token: " + err.Error()})
		return
	}

	log.Printf("[Email OAuth] Token rafraîchi avec succès")
	c.JSON(http.StatusOK, gin.H{"message": "Token rafraîchi avec succès"})
}

// GetEmailHealthStatus retourne le statut de santé du système email
func (h *EmailHandler) GetEmailHealthStatus(c *gin.Context) {
	status := gin.H{
		"smtp_configured":     false,
		"smtp_enabled":        false,
		"oauth_configured":    false,
		"oauth_enabled":       false,
		"oauth_token_valid":   false,
		"oauth_token_expires": nil,
		"last_refresh_error":  "",
		"templates_enabled":   map[string]bool{},
		"recent_failures":     0,
		"recent_successes":    0,
	}

	// Vérifier la configuration SMTP
	var smtpConfig models.SMTPConfig
	if err := h.db.Preload("EmailOAuthConfig").First(&smtpConfig).Error; err == nil {
		status["smtp_configured"] = true
		status["smtp_enabled"] = smtpConfig.IsEnabled
		status["from_email"] = smtpConfig.FromEmail

		// Vérifier OAuth
		if smtpConfig.EmailOAuthConfig != nil {
			status["oauth_configured"] = true
			status["oauth_enabled"] = smtpConfig.EmailOAuthConfig.IsEnabled
			status["oauth_provider"] = smtpConfig.EmailOAuthConfig.Provider
			status["oauth_grant_type"] = smtpConfig.EmailOAuthConfig.GrantType

			// Vérifier validité du token
			if smtpConfig.EmailOAuthConfig.ExpiresAt != nil {
				status["oauth_token_expires"] = smtpConfig.EmailOAuthConfig.ExpiresAt
				if smtpConfig.EmailOAuthConfig.ExpiresAt.After(time.Now()) {
					status["oauth_token_valid"] = true
				}
			}

			// Dernière erreur de refresh
			if smtpConfig.EmailOAuthConfig.LastRefreshError != "" {
				status["last_refresh_error"] = smtpConfig.EmailOAuthConfig.LastRefreshError
			}

			// Dernière tentative de refresh
			if smtpConfig.EmailOAuthConfig.LastTokenRefresh != nil {
				status["last_token_refresh"] = smtpConfig.EmailOAuthConfig.LastTokenRefresh
			}
		}
	}

	// Vérifier les templates activés
	var templates []models.EmailTemplate
	if err := h.db.Find(&templates).Error; err == nil {
		templatesEnabled := make(map[string]bool)
		for _, t := range templates {
			templatesEnabled[t.Type] = t.IsEnabled
		}
		status["templates_enabled"] = templatesEnabled
	}

	// Compter les échecs/succès récents (dernières 24h)
	yesterday := time.Now().Add(-24 * time.Hour)
	var recentLogs []models.EmailNotificationLog
	if err := h.db.Where("created_at > ?", yesterday).Find(&recentLogs).Error; err == nil {
		failures := 0
		successes := 0
		for _, l := range recentLogs {
			if l.Status == "failed" {
				failures++
			} else if l.Status == "completed" {
				successes++
			}
		}
		status["recent_failures"] = failures
		status["recent_successes"] = successes
	}

	// Déterminer le statut global
	healthy := false
	if smtpConfig.IsEnabled && smtpConfig.EmailOAuthConfig != nil &&
		smtpConfig.EmailOAuthConfig.IsEnabled &&
		status["oauth_token_valid"].(bool) {
		healthy = true
	}
	status["healthy"] = healthy

	// Message de diagnostic
	if !status["smtp_configured"].(bool) {
		status["diagnostic"] = "Configuration SMTP non trouvée"
	} else if !status["smtp_enabled"].(bool) {
		status["diagnostic"] = "SMTP désactivé - les notifications email ne seront pas envoyées"
	} else if !status["oauth_configured"].(bool) {
		status["diagnostic"] = "OAuth non configuré"
	} else if !status["oauth_enabled"].(bool) {
		status["diagnostic"] = "OAuth désactivé"
	} else if !status["oauth_token_valid"].(bool) {
		status["diagnostic"] = "Token OAuth expiré ou invalide - tentez un rafraîchissement"
	} else if status["last_refresh_error"].(string) != "" {
		status["diagnostic"] = "Dernière erreur: " + status["last_refresh_error"].(string)
	} else {
		status["diagnostic"] = "Système email opérationnel"
	}

	c.JSON(http.StatusOK, status)
}
