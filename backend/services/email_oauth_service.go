package services

import (
	"airboard/config"
	"airboard/models"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"gorm.io/gorm"
)

// EmailOAuthService gère l'authentification OAuth 2.0 pour l'envoi d'emails
type EmailOAuthService struct {
	db     *gorm.DB
	config *config.Config
}

// NewEmailOAuthService crée une nouvelle instance du service OAuth email
func NewEmailOAuthService(db *gorm.DB, cfg *config.Config) *EmailOAuthService {
	return &EmailOAuthService{
		db:     db,
		config: cfg,
	}
}

// GetValidAccessToken récupère un access token valide (auto-refresh si expiré)
func (s *EmailOAuthService) GetValidAccessToken(oauthConfig *models.EmailOAuthConfig) (string, error) {
	// Vérifier si le token existe et n'est pas expiré (buffer de 5 minutes)
	if oauthConfig.AccessToken != "" && oauthConfig.ExpiresAt != nil {
		bufferTime := time.Now().Add(5 * time.Minute)
		if oauthConfig.ExpiresAt.After(bufferTime) {
			// Token encore valide, le déchiffrer et retourner
			token, err := s.DecryptToken(oauthConfig.AccessToken)
			if err != nil {
				log.Printf("[Email OAuth] Erreur déchiffrement token: %v", err)
				return "", fmt.Errorf("failed to decrypt access token: %w", err)
			}
			log.Printf("[Email OAuth] Token valide jusqu'à %s", oauthConfig.ExpiresAt.Format(time.RFC3339))
			return token, nil
		}
	}

	// Token expiré ou manquant, rafraîchir
	log.Printf("[Email OAuth] Token expiré ou manquant, rafraîchissement nécessaire")
	if err := s.RefreshAccessToken(oauthConfig); err != nil {
		return "", fmt.Errorf("failed to refresh access token: %w", err)
	}

	// Récupérer le token fraîchement rafraîchi
	if err := s.db.First(oauthConfig, oauthConfig.ID).Error; err != nil {
		return "", fmt.Errorf("failed to reload OAuth config: %w", err)
	}

	token, err := s.DecryptToken(oauthConfig.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt refreshed token: %w", err)
	}

	return token, nil
}

// RefreshAccessToken rafraîchit l'access token selon le grant type
func (s *EmailOAuthService) RefreshAccessToken(oauthConfig *models.EmailOAuthConfig) error {
	log.Printf("[Email OAuth] RefreshAccessToken appelé - Grant type: %s, Provider: %s",
		oauthConfig.GrantType, oauthConfig.Provider)

	switch oauthConfig.GrantType {
	case "client_credentials":
		return s.AcquireClientCredentialsToken(oauthConfig)
	case "refresh_token":
		return s.RefreshWithRefreshToken(oauthConfig)
	default:
		return fmt.Errorf("grant type non supporté: %s", oauthConfig.GrantType)
	}
}

// AcquireClientCredentialsToken acquiert un token via client credentials flow
func (s *EmailOAuthService) AcquireClientCredentialsToken(oauthConfig *models.EmailOAuthConfig) error {
	log.Printf("[Email OAuth] AcquireClientCredentialsToken - Tenant: %s, Client: %s",
		oauthConfig.TenantID, oauthConfig.ClientID)

	// Déchiffrer le client secret
	clientSecret, err := s.DecryptToken(oauthConfig.ClientSecret)
	if err != nil {
		return fmt.Errorf("failed to decrypt client secret: %w", err)
	}

	// Remplacer {tenant} dans le token URL
	tokenURL := strings.ReplaceAll(oauthConfig.TokenURL, "{tenant}", oauthConfig.TenantID)
	log.Printf("[Email OAuth] Token URL: %s", tokenURL)
	log.Printf("[Email OAuth] Scopes: '%s'", oauthConfig.Scopes)
	log.Printf("[Email OAuth] Client ID: %s", oauthConfig.ClientID)

	// Configurer le client OAuth2 avec client credentials
	ccConfig := clientcredentials.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes:       strings.Fields(oauthConfig.Scopes), // Convertir string en []string
	}
	log.Printf("[Email OAuth] Scopes as array: %v", ccConfig.Scopes)

	// Acquérir le token
	ctx := context.Background()
	token, err := ccConfig.Token(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("Échec acquisition token: %v", err)
		log.Printf("[Email OAuth] %s", errMsg)
		oauthConfig.LastRefreshError = errMsg
		now := time.Now()
		oauthConfig.LastTokenRefresh = &now
		s.db.Save(oauthConfig)
		return fmt.Errorf("failed to acquire token: %w", err)
	}

	log.Printf("[Email OAuth] Token acquis avec succès - Type: %s, Expire: %s",
		token.TokenType, token.Expiry.Format(time.RFC3339))

	// Chiffrer et stocker le token
	encryptedToken, err := s.EncryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	// Mettre à jour la config OAuth avec le nouveau token
	oauthConfig.AccessToken = encryptedToken
	oauthConfig.TokenType = token.TokenType
	oauthConfig.ExpiresAt = &token.Expiry
	now := time.Now()
	oauthConfig.LastTokenRefresh = &now
	oauthConfig.LastRefreshError = "" // Effacer les erreurs précédentes

	if err := s.db.Save(oauthConfig).Error; err != nil {
		return fmt.Errorf("failed to save OAuth config: %w", err)
	}

	log.Printf("[Email OAuth] Token stocké avec succès - Expire à %s", token.Expiry.Format(time.RFC3339))
	return nil
}

// RefreshWithRefreshToken rafraîchit le token avec le refresh token
func (s *EmailOAuthService) RefreshWithRefreshToken(oauthConfig *models.EmailOAuthConfig) error {
	log.Printf("[Email OAuth] RefreshWithRefreshToken - Provider: %s", oauthConfig.Provider)

	// Déchiffrer le refresh token
	refreshToken, err := s.DecryptToken(oauthConfig.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	// Déchiffrer le client secret
	clientSecret, err := s.DecryptToken(oauthConfig.ClientSecret)
	if err != nil {
		return fmt.Errorf("failed to decrypt client secret: %w", err)
	}

	// Remplacer {tenant} dans les URLs
	tokenURL := strings.ReplaceAll(oauthConfig.TokenURL, "{tenant}", oauthConfig.TenantID)

	// Configurer le client OAuth2
	oauth2Config := &oauth2.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL,
		},
		Scopes: strings.Fields(oauthConfig.Scopes),
	}

	// Créer un token source avec le refresh token
	tokenSource := oauth2Config.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})

	// Obtenir un nouveau token
	newToken, err := tokenSource.Token()
	if err != nil {
		errMsg := fmt.Sprintf("Échec refresh token: %v", err)
		log.Printf("[Email OAuth] %s", errMsg)
		oauthConfig.LastRefreshError = errMsg
		now := time.Now()
		oauthConfig.LastTokenRefresh = &now
		s.db.Save(oauthConfig)
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	log.Printf("[Email OAuth] Token rafraîchi avec succès - Expire: %s", newToken.Expiry.Format(time.RFC3339))

	// Chiffrer et stocker le nouveau token
	encryptedAccessToken, err := s.EncryptToken(newToken.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	// Mettre à jour la config OAuth
	oauthConfig.AccessToken = encryptedAccessToken
	oauthConfig.TokenType = newToken.TokenType
	oauthConfig.ExpiresAt = &newToken.Expiry
	now := time.Now()
	oauthConfig.LastTokenRefresh = &now
	oauthConfig.LastRefreshError = ""

	// Si un nouveau refresh token est fourni, le stocker aussi
	if newToken.RefreshToken != "" && newToken.RefreshToken != refreshToken {
		encryptedRefreshToken, err := s.EncryptToken(newToken.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt new refresh token: %w", err)
		}
		oauthConfig.RefreshToken = encryptedRefreshToken
		log.Printf("[Email OAuth] Nouveau refresh token stocké")
	}

	if err := s.db.Save(oauthConfig).Error; err != nil {
		return fmt.Errorf("failed to save OAuth config: %w", err)
	}

	return nil
}

// EncryptToken chiffre un token avec AES-256
func (s *EmailOAuthService) EncryptToken(token string) (string, error) {
	if token == "" {
		return "", nil
	}

	// Utiliser les 32 premiers octets du secret JWT comme clé (même méthode que les passwords)
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

	plaintext := []byte(token)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("erreur génération IV: %w", err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptToken déchiffre un token chiffré avec AES-256
func (s *EmailOAuthService) DecryptToken(encrypted string) (string, error) {
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

	// Nettoyer le token des caractères invalides pour les headers HTTP
	// (newlines, carriage returns, etc.)
	token := string(ciphertext)
	token = strings.TrimSpace(token)
	token = strings.ReplaceAll(token, "\n", "")
	token = strings.ReplaceAll(token, "\r", "")
	token = strings.ReplaceAll(token, "\t", "")

	return token, nil
}
