package services

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"airboard/config"
	"airboard/models"

	"gorm.io/gorm"
)

// xoauth2Auth implements XOAUTH2 SASL authentication for OAuth 2.0 SMTP
type xoauth2Auth struct {
	username string
	token    string
}

// Start begins XOAUTH2 authentication
func (a *xoauth2Auth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// XOAUTH2 auth string format: "user={username}\x01auth=Bearer {token}\x01\x01"
	authString := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", a.username, a.token)
	log.Printf("[Email OAuth] XOAUTH2 auth for user: %s (token length: %d chars)", a.username, len(a.token))
	return "XOAUTH2", []byte(authString), nil
}

// Next handles server challenges (XOAUTH2 typically doesn't have challenges)
func (a *xoauth2Auth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		// XOAUTH2 should not have challenges, but if it does, return empty
		return []byte(""), nil
	}
	return nil, nil
}

// XOAUTH2 creates an smtp.Auth that uses XOAUTH2 authentication
func XOAUTH2(username, token string) smtp.Auth {
	return &xoauth2Auth{username, token}
}

// EmailService gère l'envoi des emails
type EmailService struct {
	db     *gorm.DB
	config *config.Config
}

// NewEmailService crée une nouvelle instance du service email
func NewEmailService(db *gorm.DB, cfg *config.Config) *EmailService {
	return &EmailService{
		db:     db,
		config: cfg,
	}
}

// NewsEmailData contient les données pour le template news
type NewsEmailData struct {
	Title       string
	Summary     string
	Author      string
	Link        string
	AppName     string
	PublishedAt string
}

// ApplicationEmailData contient les données pour le template application
type ApplicationEmailData struct {
	Name        string
	Description string
	URL         string
	AppGroup    string
	AppName     string
}

// EventEmailData contient les données pour le template event
type EventEmailData struct {
	Title       string
	Description string
	StartDate   string
	EndDate     string
	Location    string
	Link        string
	AppName     string
}

// AnnouncementEmailData contient les données pour le template announcement
type AnnouncementEmailData struct {
	Title   string
	Content string
	Type    string
	AppName string
}

// SendNotification envoie des notifications email aux groupes cibles
func (s *EmailService) SendNotification(templateType string, contentID uint, targetGroupIDs []uint) error {
	// Récupérer la config SMTP avec la config OAuth si disponible
	var smtpConfig models.SMTPConfig
	if err := s.db.Preload("EmailOAuthConfig").First(&smtpConfig).Error; err != nil {
		log.Printf("[Email] SMTP non configuré: %v", err)
		return fmt.Errorf("SMTP non configuré: %w", err)
	}
	if !smtpConfig.IsEnabled {
		log.Println("[Email] SMTP désactivé, notification ignorée")
		return nil
	}

	// Log du mode d'authentification utilisé
	if smtpConfig.UseOAuth && smtpConfig.EmailOAuthConfig != nil && smtpConfig.EmailOAuthConfig.IsEnabled {
		log.Printf("[Email] Mode OAuth 2.0 activé (provider: %s, grant: %s)",
			smtpConfig.EmailOAuthConfig.Provider, smtpConfig.EmailOAuthConfig.GrantType)
	} else {
		log.Printf("[Email] Mode SMTP classique (password)")
	}

	// Récupérer le template
	var emailTemplate models.EmailTemplate
	if err := s.db.Where("type = ? AND is_enabled = ?", templateType, true).First(&emailTemplate).Error; err != nil {
		log.Printf("[Email] Template '%s' non trouvé ou désactivé: %v", templateType, err)
		return fmt.Errorf("template non trouvé ou désactivé: %w", err)
	}

	// Récupérer les destinataires selon les groupes cibles
	var recipients []string
	if len(targetGroupIDs) == 0 {
		// Notification globale - envoyer à tous les utilisateurs actifs
		s.db.Model(&models.User{}).Where("is_active = ? AND email != '' AND email IS NOT NULL", true).Pluck("email", &recipients)
	} else {
		// Notification ciblée par groupe
		s.db.Table("users").
			Select("DISTINCT users.email").
			Joins("JOIN user_groups ON user_groups.user_id = users.id").
			Where("user_groups.group_id IN ? AND users.is_active = ? AND users.email != '' AND users.email IS NOT NULL", targetGroupIDs, true).
			Pluck("email", &recipients)
	}

	if len(recipients) == 0 {
		log.Println("[Email] Aucun destinataire trouvé, notification ignorée")
		return nil
	}

	log.Printf("[Email] Envoi de %d notifications de type '%s'", len(recipients), templateType)

	// Créer l'entrée de log
	now := time.Now()
	notifLog := models.EmailNotificationLog{
		TemplateType:   templateType,
		ContentID:      contentID,
		RecipientCount: len(recipients),
		Status:         "sending",
		SentAt:         &now,
	}
	s.db.Create(&notifLog)

	// Préparer les données du template selon le type
	emailData, contentTitle, err := s.prepareEmailData(templateType, contentID)
	if err != nil {
		notifLog.Status = "failed"
		notifLog.ErrorMessage = err.Error()
		s.db.Save(&notifLog)
		return err
	}
	notifLog.ContentTitle = contentTitle

	// Parser et exécuter les templates
	subject, err := s.ExecuteTemplate(emailTemplate.Subject, emailData)
	if err != nil {
		notifLog.Status = "failed"
		notifLog.ErrorMessage = "Erreur template sujet: " + err.Error()
		s.db.Save(&notifLog)
		return err
	}

	htmlBody, err := s.ExecuteTemplate(emailTemplate.HTMLBody, emailData)
	if err != nil {
		notifLog.Status = "failed"
		notifLog.ErrorMessage = "Erreur template corps: " + err.Error()
		s.db.Save(&notifLog)
		return err
	}

	// Envoyer les emails
	successCount := 0
	failureCount := 0
	var lastError string

	for _, recipient := range recipients {
		if err := s.sendEmail(&smtpConfig, recipient, subject, htmlBody); err != nil {
			log.Printf("[Email] Échec envoi à %s: %v", recipient, err)
			failureCount++
			lastError = err.Error()
		} else {
			successCount++
		}
	}

	// Mettre à jour le log
	notifLog.SuccessCount = successCount
	notifLog.FailureCount = failureCount
	completedAt := time.Now()
	notifLog.CompletedAt = &completedAt

	if failureCount > 0 && successCount == 0 {
		notifLog.Status = "failed"
		notifLog.ErrorMessage = lastError
	} else if failureCount > 0 {
		notifLog.Status = "completed"
		notifLog.ErrorMessage = fmt.Sprintf("%d échecs sur %d", failureCount, len(recipients))
	} else {
		notifLog.Status = "completed"
	}

	s.db.Save(&notifLog)

	log.Printf("[Email] Notification terminée: %d succès, %d échecs", successCount, failureCount)
	return nil
}

// prepareEmailData prépare les données selon le type de contenu
func (s *EmailService) prepareEmailData(templateType string, contentID uint) (interface{}, string, error) {
	// Récupérer le nom de l'application
	var appSettings models.AppSettings
	s.db.First(&appSettings)
	appName := appSettings.AppName
	if appName == "" {
		appName = "Airboard"
	}

	switch templateType {
	case "news":
		var news models.News
		if err := s.db.Preload("Author").First(&news, contentID).Error; err != nil {
			return nil, "", fmt.Errorf("article non trouvé: %w", err)
		}
		authorName := strings.TrimSpace(news.Author.FirstName + " " + news.Author.LastName)
		if authorName == "" {
			authorName = news.Author.Username
		}
		publishedAt := ""
		if news.PublishedAt != nil {
			publishedAt = news.PublishedAt.Format("02/01/2006 à 15:04")
		} else {
			publishedAt = time.Now().Format("02/01/2006 à 15:04")
		}
		return NewsEmailData{
			Title:       news.Title,
			Summary:     news.Summary,
			Author:      authorName,
			Link:        fmt.Sprintf("%s/news/%s", s.config.Server.PublicURL, news.Slug),
			AppName:     appName,
			PublishedAt: publishedAt,
		}, news.Title, nil

	case "application":
		var app models.Application
		if err := s.db.Preload("AppGroup").First(&app, contentID).Error; err != nil {
			return nil, "", fmt.Errorf("application non trouvée: %w", err)
		}
		appGroupName := ""
		if app.AppGroup != nil {
			appGroupName = app.AppGroup.Name
		}
		return ApplicationEmailData{
			Name:        app.Name,
			Description: app.Description,
			URL:         app.URL,
			AppGroup:    appGroupName,
			AppName:     appName,
		}, app.Name, nil

	case "event":
		var event models.Event
		if err := s.db.First(&event, contentID).Error; err != nil {
			return nil, "", fmt.Errorf("événement non trouvé: %w", err)
		}
		endDate := ""
		if event.EndDate != nil {
			endDate = event.EndDate.Format("02/01/2006 à 15:04")
		}
		return EventEmailData{
			Title:       event.Title,
			Description: event.Description,
			StartDate:   event.StartDate.Format("02/01/2006 à 15:04"),
			EndDate:     endDate,
			Location:    event.Location,
			Link:        fmt.Sprintf("%s/events/%s", s.config.Server.PublicURL, event.Slug),
			AppName:     appName,
		}, event.Title, nil

	case "announcement":
		var announcement models.Announcement
		if err := s.db.First(&announcement, contentID).Error; err != nil {
			return nil, "", fmt.Errorf("annonce non trouvée: %w", err)
		}
		return AnnouncementEmailData{
			Title:   announcement.Title,
			Content: announcement.Content,
			Type:    announcement.Type,
			AppName: appName,
		}, announcement.Title, nil
	}

	return nil, "", fmt.Errorf("type de template inconnu: %s", templateType)
}

// ExecuteTemplate exécute un template Go avec les données fournies
func (s *EmailService) ExecuteTemplate(templateStr string, data interface{}) (string, error) {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("erreur parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("erreur exécution template: %w", err)
	}
	return buf.String(), nil
}

// sendEmail envoie un email via SMTP (routage OAuth ou Password)
func (s *EmailService) sendEmail(config *models.SMTPConfig, to, subject, htmlBody string) error {
	// Check if OAuth is enabled and configured
	if config.UseOAuth && config.EmailOAuthConfig != nil && config.EmailOAuthConfig.IsEnabled {
		log.Printf("[Email] Using OAuth 2.0 authentication for %s", to)
		return s.SendEmailWithOAuth(config, to, subject, htmlBody)
	}

	// Fall back to password-based authentication
	log.Printf("[Email] Using password authentication for %s", to)
	return s.sendEmailWithPassword(config, to, subject, htmlBody)
}

// sendEmailWithPassword envoie un email via authentification mot de passe (méthode classique)
func (s *EmailService) sendEmailWithPassword(config *models.SMTPConfig, to, subject, htmlBody string) error {
	// Déchiffrer le mot de passe
	password, err := s.DecryptPassword(config.Password)
	if err != nil {
		return fmt.Errorf("erreur déchiffrement mot de passe: %w", err)
	}

	// Construire le message
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	// Envoyer via SMTP
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	if config.UseTLS {
		return s.sendEmailTLS(addr, config.Username, password, config.FromEmail, to, msg.Bytes(), config.Host)
	} else if config.UseSTARTTLS {
		return s.sendEmailSTARTTLS(addr, config.Username, password, config.FromEmail, to, msg.Bytes(), config.Host)
	}

	// Envoi simple sans TLS
	var auth smtp.Auth
	if config.Username != "" && password != "" {
		auth = smtp.PlainAuth("", config.Username, password, config.Host)
	}
	return smtp.SendMail(addr, auth, config.FromEmail, []string{to}, msg.Bytes())
}

// SendEmailWithOAuth envoie un email via OAuth 2.0
// Pour Microsoft 365: utilise Microsoft Graph API (seule méthode supportée avec client credentials)
// Pour d'autres providers: utilise XOAUTH2 SMTP
func (s *EmailService) SendEmailWithOAuth(config *models.SMTPConfig, to, subject, htmlBody string) error {
	if config.EmailOAuthConfig == nil {
		return fmt.Errorf("OAuth config not found")
	}

	// Get valid access token (auto-refresh if needed)
	oauthService := NewEmailOAuthService(s.db, s.config)
	accessToken, err := oauthService.GetValidAccessToken(config.EmailOAuthConfig)
	if err != nil {
		return fmt.Errorf("failed to get OAuth token: %w", err)
	}

	log.Printf("[Email OAuth] Token acquis, envoi email à %s", to)

	// Pour Microsoft 365 avec client_credentials, utiliser Microsoft Graph API
	// SMTP AUTH avec OAuth ne fonctionne PAS avec client credentials flow
	if config.EmailOAuthConfig.Provider == "microsoft" && config.EmailOAuthConfig.GrantType == "client_credentials" {
		return s.sendEmailViaGraphAPI(config, accessToken, to, subject, htmlBody)
	}

	// Pour les autres cas (refresh_token flow ou autres providers), utiliser SMTP XOAUTH2
	return s.sendEmailViaSMTPOAuth(config, accessToken, to, subject, htmlBody)
}

// sendEmailViaGraphAPI envoie un email via Microsoft Graph API
// C'est la seule méthode supportée pour client credentials flow avec Microsoft 365
func (s *EmailService) sendEmailViaGraphAPI(config *models.SMTPConfig, accessToken, to, subject, htmlBody string) error {
	log.Printf("[Email OAuth] Envoi via Microsoft Graph API à %s", to)

	// Construire le payload JSON pour Graph API
	emailPayload := map[string]interface{}{
		"message": map[string]interface{}{
			"subject": subject,
			"body": map[string]interface{}{
				"contentType": "HTML",
				"content":     htmlBody,
			},
			"from": map[string]interface{}{
				"emailAddress": map[string]interface{}{
					"address": config.FromEmail,
					"name":    config.FromName,
				},
			},
			"toRecipients": []map[string]interface{}{
				{
					"emailAddress": map[string]interface{}{
						"address": to,
					},
				},
			},
		},
		"saveToSentItems": "false",
	}

	jsonPayload, err := json.Marshal(emailPayload)
	if err != nil {
		return fmt.Errorf("erreur sérialisation JSON: %w", err)
	}

	// Appeler l'API Microsoft Graph
	// URL: https://graph.microsoft.com/v1.0/users/{from_email}/sendMail
	graphURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/sendMail", config.FromEmail)

	req, err := http.NewRequest("POST", graphURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("erreur création requête: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erreur appel Graph API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[Email OAuth] Graph API erreur: %s", string(body))
		return fmt.Errorf("Graph API erreur (HTTP %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("[Email OAuth] Email envoyé via Graph API à %s", to)
	return nil
}

// sendEmailViaSMTPOAuth envoie un email via SMTP avec XOAUTH2
// Utilisé pour refresh_token flow ou providers non-Microsoft
func (s *EmailService) sendEmailViaSMTPOAuth(config *models.SMTPConfig, accessToken, to, subject, htmlBody string) error {
	// Construire le message
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	// Envoyer via SMTP avec XOAUTH2
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	if config.UseSTARTTLS || config.Port == 587 {
		return s.sendEmailOAuthSTARTTLS(addr, config.FromEmail, accessToken, to, msg.Bytes(), config.Host)
	} else if config.UseTLS || config.Port == 465 {
		return s.sendEmailOAuthTLS(addr, config.FromEmail, accessToken, to, msg.Bytes(), config.Host)
	}

	// Par défaut, utiliser STARTTLS (recommandé pour OAuth)
	return s.sendEmailOAuthSTARTTLS(addr, config.FromEmail, accessToken, to, msg.Bytes(), config.Host)
}

// sendEmailOAuthSTARTTLS envoie un email via STARTTLS avec XOAUTH2
func (s *EmailService) sendEmailOAuthSTARTTLS(addr, from, token, to string, msg []byte, host string) error {
	// Connexion initiale non chiffrée
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("erreur connexion SMTP: %w", err)
	}
	defer client.Close()

	// Vérifier les capacités du serveur
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("STARTTLS non supporté par le serveur")
	}

	// Upgrade vers TLS via STARTTLS
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("erreur STARTTLS: %w", err)
	}

	// Authentification XOAUTH2
	auth := XOAUTH2(from, token)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("erreur authentification XOAUTH2: %w", err)
	}

	log.Printf("[Email OAuth] Authentification XOAUTH2 réussie")

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("erreur MAIL FROM: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("erreur RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("erreur DATA: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("erreur écriture message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("erreur fermeture DATA: %w", err)
	}

	log.Printf("[Email OAuth] Email envoyé avec succès à %s", to)
	return client.Quit()
}

// sendEmailOAuthTLS envoie un email via TLS direct avec XOAUTH2
func (s *EmailService) sendEmailOAuthTLS(addr, from, token, to string, msg []byte, host string) error {
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("erreur connexion TLS: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("erreur création client SMTP: %w", err)
	}
	defer client.Close()

	// Authentification XOAUTH2
	auth := XOAUTH2(from, token)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("erreur authentification XOAUTH2: %w", err)
	}

	log.Printf("[Email OAuth] Authentification XOAUTH2 réussie (TLS)")

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("erreur MAIL FROM: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("erreur RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("erreur DATA: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("erreur écriture message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("erreur fermeture DATA: %w", err)
	}

	log.Printf("[Email OAuth] Email envoyé avec succès à %s (TLS)", to)
	return client.Quit()
}

// sendEmailTLS envoie un email via TLS direct (port 465)
func (s *EmailService) sendEmailTLS(addr, username, password, from, to string, msg []byte, host string) error {
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("erreur connexion TLS: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("erreur création client SMTP: %w", err)
	}
	defer client.Close()

	// Authentification si credentials fournis
	if username != "" && password != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("erreur authentification: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("erreur MAIL FROM: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("erreur RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("erreur DATA: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("erreur écriture message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("erreur fermeture DATA: %w", err)
	}

	return client.Quit()
}

// sendEmailSTARTTLS envoie un email via STARTTLS (port 587)
func (s *EmailService) sendEmailSTARTTLS(addr, username, password, from, to string, msg []byte, host string) error {
	// Connexion initiale non chiffrée
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("erreur connexion SMTP: %w", err)
	}
	defer client.Close()

	// Vérifier les capacités du serveur
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("STARTTLS non supporté par le serveur")
	}

	// Upgrade vers TLS via STARTTLS avec configuration appropriée
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("erreur STARTTLS: %w", err)
	}

	// Authentification avec support Office365
	if username != "" && password != "" {
		// Office365 peut avoir des problèmes avec PlainAuth, essayer plusieurs méthodes

		// Méthode 1: Plain Auth standard avec le hostname complet
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err == nil {
			log.Printf("[Email] Authentification réussie avec PlainAuth")
		} else {
			// Méthode 2: Essayer avec juste le nom de domaine
			log.Printf("[Email] PlainAuth échoué: %v, tentative avec domaine", err)

			// Pour Office365, sometimes we need just the domain part
			hostWithoutPort := strings.Split(host, ":")[0]
			auth2 := smtp.PlainAuth("", username, password, hostWithoutPort)
			if err2 := client.Auth(auth2); err2 == nil {
				log.Printf("[Email] Authentification réussie avec domaine: %s", hostWithoutPort)
			} else {
				// Méthode 3: Essayer avec le domaine racine
				hostParts := strings.Split(hostWithoutPort, ".")
				if len(hostParts) >= 2 {
					domain := strings.Join(hostParts[len(hostParts)-2:], ".")
					auth3 := smtp.PlainAuth("", username, password, domain)
					if err3 := client.Auth(auth3); err3 == nil {
						log.Printf("[Email] Authentification réussie avec domaine racine: %s", domain)
					} else {
						// Toutes les méthodes ont échoué
						log.Printf("[Email] Toutes les méthodes d'authentification ont échoué")
						log.Printf("[Email] Erreurs: %v, %v, %v", err, err2, err3)

						// Fournir un message d'erreur détaillé avec instructions
						return fmt.Errorf("authentification Office365 échouée. Causes possibles: \n1) L'authentification de base est désactivée dans votre tenant Office365 \n2) Le compte nécessite l'authentification moderne (OAuth2) \n3) Les credentials sont incorrects \n\nSolution: Activez l'authentification de base dans Azure Portal > Azure Active Directory > Propriétés > Gérer l'accès conditional > Authentification de base")
					}
				} else {
					// Format hostname invalide
					return fmt.Errorf("format hostname invalide: %s", host)
				}
			}
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("erreur MAIL FROM: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("erreur RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("erreur DATA: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("erreur écriture message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("erreur fermeture DATA: %w", err)
	}

	return client.Quit()
}

// TestConnection teste la connexion SMTP avec un email de test
func (s *EmailService) TestConnection(smtpConfig *models.SMTPConfig, testEmail string) error {
	log.Printf("[Email] TestConnection appelé - Destinataire: %s, Host: %s, Port: %d, UseTLS: %v, UseSTARTTLS: %v",
		testEmail, smtpConfig.Host, smtpConfig.Port, smtpConfig.UseTLS, smtpConfig.UseSTARTTLS)

	password, err := s.DecryptPassword(smtpConfig.Password)
	if err != nil {
		log.Printf("[Email] Erreur déchiffrement mot de passe: %v", err)
		return fmt.Errorf("erreur déchiffrement mot de passe: %w", err)
	}

	// Récupérer le nom de l'application
	var appSettings models.AppSettings
	if err := s.db.First(&appSettings).Error; err != nil {
		log.Printf("[Email] Impossible de récupérer les paramètres: %v", err)
	}
	appName := appSettings.AppName
	if appName == "" {
		appName = "Airboard"
	}

	testSubject := fmt.Sprintf("%s - Email de test", appName)
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
<h2>Configuration SMTP réussie !</h2>
<p>Cet email confirme que votre configuration SMTP est correcte.</p>
<p>Les notifications email fonctionneront correctement.</p>
<p><strong>Serveur:</strong> %s<br>
<strong>Port:</strong> %d<br>
<strong>Sécurité:</strong> %s</p>
</div>
<div class="footer">
<p>Email de test envoyé le %s</p>
</div>
</div>
</body>
</html>`, appName, smtpConfig.Host, smtpConfig.Port,
		func() string {
			if smtpConfig.UseTLS {
				return "TLS (port 465)"
			} else if smtpConfig.UseSTARTTLS {
				return "STARTTLS (port 587)"
			}
			return "Aucune"
		}(), time.Now().Format("02/01/2006 à 15:04"))

	// Construire le message
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", smtpConfig.FromName, smtpConfig.FromEmail)
	headers["To"] = testEmail
	headers["Subject"] = testSubject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(testBody)

	// Envoyer via SMTP
	addr := fmt.Sprintf("%s:%d", smtpConfig.Host, smtpConfig.Port)
	log.Printf("[Email] Tentative d'envoi vers %s via %s", testEmail, addr)

	var sendErr error
	if smtpConfig.UseTLS {
		log.Printf("[Email] Utilisation de TLS direct (port 465)")
		sendErr = s.sendEmailTLS(addr, smtpConfig.Username, password, smtpConfig.FromEmail, testEmail, msg.Bytes(), smtpConfig.Host)
	} else if smtpConfig.UseSTARTTLS {
		log.Printf("[Email] Utilisation de STARTTLS (port 587)")
		sendErr = s.sendEmailSTARTTLS(addr, smtpConfig.Username, password, smtpConfig.FromEmail, testEmail, msg.Bytes(), smtpConfig.Host)
	} else {
		log.Printf("[Email] Utilisation de connexion non sécurisée")
		// Envoi simple sans TLS
		var auth smtp.Auth
		if smtpConfig.Username != "" && password != "" {
			auth = smtp.PlainAuth("", smtpConfig.Username, password, smtpConfig.Host)
		}
		sendErr = smtp.SendMail(addr, auth, smtpConfig.FromEmail, []string{testEmail}, msg.Bytes())
	}

	if sendErr != nil {
		log.Printf("[Email] Échec envoi email de test: %v", sendErr)
		return sendErr
	}

	log.Printf("[Email] Email de test envoyé avec succès à %s", testEmail)
	return nil
}

// EncryptPassword chiffre un mot de passe avec AES-256
func (s *EmailService) EncryptPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}

	// Utiliser les 32 premiers octets du secret JWT comme clé
	secret := s.config.JWT.Secret
	if len(secret) < 32 {
		// Padding si le secret est trop court
		secret = secret + strings.Repeat("0", 32-len(secret))
	}
	key := []byte(secret[:32])

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("erreur création cipher: %w", err)
	}

	plaintext := []byte(password)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("erreur génération IV: %w", err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPassword déchiffre un mot de passe chiffré avec AES-256
func (s *EmailService) DecryptPassword(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	// Utiliser les 32 premiers octets du secret JWT comme clé
	secret := s.config.JWT.Secret
	if len(secret) < 32 {
		// Padding si le secret est trop court
		secret = secret + strings.Repeat("0", 32-len(secret))
	}
	key := []byte(secret[:32])

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("erreur décodage base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("erreur création cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext trop court")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

// GetSampleData retourne des données exemple pour un type de template
func (s *EmailService) GetSampleData(templateType string) interface{} {
	// Récupérer le nom de l'application
	var appSettings models.AppSettings
	s.db.First(&appSettings)
	appName := appSettings.AppName
	if appName == "" {
		appName = "Airboard"
	}

	switch templateType {
	case "news":
		return NewsEmailData{
			Title:       "Exemple d'article",
			Summary:     "Ceci est un résumé exemple pour prévisualiser le template d'email.",
			Author:      "Jean Dupont",
			Link:        fmt.Sprintf("%s/news/exemple-article", s.config.Server.PublicURL),
			AppName:     appName,
			PublishedAt: time.Now().Format("02/01/2006 à 15:04"),
		}
	case "application":
		return ApplicationEmailData{
			Name:        "Application Exemple",
			Description: "Ceci est une description exemple pour prévisualiser le template.",
			URL:         "https://example.com/app",
			AppGroup:    "Développement",
			AppName:     appName,
		}
	case "event":
		return EventEmailData{
			Title:       "Événement Exemple",
			Description: "Ceci est une description exemple pour prévisualiser le template d'événement.",
			StartDate:   time.Now().Format("02/01/2006 à 15:04"),
			EndDate:     time.Now().Add(2 * time.Hour).Format("02/01/2006 à 15:04"),
			Location:    "Salle de conférence A",
			Link:        fmt.Sprintf("%s/events/exemple-evenement", s.config.Server.PublicURL),
			AppName:     appName,
		}
	case "announcement":
		return AnnouncementEmailData{
			Title:   "Annonce Exemple",
			Content: "Ceci est un contenu d'annonce exemple pour prévisualiser le template.",
			Type:    "info",
			AppName: appName,
		}
	}
	return nil
}
