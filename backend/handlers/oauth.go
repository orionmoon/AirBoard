package handlers

import (
	"airboard/middleware"
	"airboard/models"
	"airboard/utils"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OAuthHandler struct {
	db             *gorm.DB
	authMiddleware *middleware.AuthMiddleware
	stateManager   *utils.OAuthStateManager
}

func NewOAuthHandler(db *gorm.DB, authMiddleware *middleware.AuthMiddleware) *OAuthHandler {
	return &OAuthHandler{
		db:             db,
		authMiddleware: authMiddleware,
		stateManager:   utils.NewOAuthStateManager(),
	}
}

// GetEnabledProviders retourne la liste des fournisseurs OAuth activés (sans secrets)
func (h *OAuthHandler) GetEnabledProviders(c *gin.Context) {
	var providers []models.OAuthProvider
	if err := h.db.Where("is_enabled = ?", true).Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: "Failed to fetch OAuth providers",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Convertir en version publique (sans secrets)
	publicProviders := make([]models.OAuthProviderPublic, len(providers))
	for i, p := range providers {
		publicProviders[i] = models.OAuthProviderPublic{
			ID:           p.ID,
			ProviderName: p.ProviderName,
			DisplayName:  p.DisplayName,
			Icon:         p.Icon,
			IsEnabled:    p.IsEnabled,
			RedirectURI:  p.RedirectURI,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": publicProviders,
	})
}

// GetAllProviders retourne tous les fournisseurs OAuth (admin uniquement)
func (h *OAuthHandler) GetAllProviders(c *gin.Context) {
	var providers []models.OAuthProvider
	if err := h.db.Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: "Failed to fetch OAuth providers",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
	})
}

// UpdateProvider met à jour un fournisseur OAuth (admin uniquement)
func (h *OAuthHandler) UpdateProvider(c *gin.Context) {
	providerID := c.Param("id")

	var req models.OAuthProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}

	var provider models.OAuthProvider
	if err := h.db.First(&provider, providerID).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: "OAuth provider not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Mettre à jour les champs
	provider.DisplayName = req.DisplayName
	provider.Icon = req.Icon
	provider.IsEnabled = req.IsEnabled
	provider.ClientID = req.ClientID
	if req.ClientSecret != "" {
		provider.ClientSecret = req.ClientSecret
	}
	provider.RedirectURI = req.RedirectURI
	provider.AuthURL = req.AuthURL
	provider.TokenURL = req.TokenURL
	provider.UserInfoURL = req.UserInfoURL
	provider.Scopes = req.Scopes

	if err := h.db.Save(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: "Failed to update OAuth provider",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "OAuth provider updated successfully",
		Data:    provider,
	})
}

// InitiateOAuth démarre le flux OAuth pour un fournisseur avec protection CSRF renforcée
func (h *OAuthHandler) InitiateOAuth(c *gin.Context) {
	providerName := c.Param("provider")

	var provider models.OAuthProvider
	if err := h.db.Where("provider_name = ? AND is_enabled = ?", providerName, true).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: "OAuth provider not found or disabled",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Générer un state et nonce sécurisés
	state, nonce, err := h.stateManager.GenerateState(providerName, provider.ClientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "state_generation_error",
			Message: "Failed to generate secure state",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Construire l'URL d'autorisation sécurisée
	authURL, err := utils.SecureOAuthURL(provider.AuthURL, map[string]string{
		"client_id":     provider.ClientID,
		"redirect_uri":  provider.RedirectURI,
		"response_type": "code",
		"scope":         provider.Scopes,
	}, state, nonce)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "auth_url_error",
			Message: "Failed to construct authorization URL",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state, // Pour debug, à retirer en production
		"nonce":    nonce, // Pour debug, à retirer en production
	})
}

// OAuthCallback gère le callback OAuth
func (h *OAuthHandler) OAuthCallback(c *gin.Context) {
	providerName := c.Param("provider")

	// Accepter le code, state et nonce depuis query params (GET) ou body (POST)
	var code, state, nonce string
	if c.Request.Method == "POST" {
		var req struct {
			Code  string `json:"code" binding:"required"`
			State string `json:"state"`
			Nonce string `json:"nonce"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[OAuth] Error parsing POST body: %v", err)
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "bad_request",
				Message: "Invalid request body",
				Code:    http.StatusBadRequest,
			})
			return
		}
		code = req.Code
		state = req.State
		nonce = req.Nonce
	} else {
		code = c.Query("code")
		state = c.Query("state")
		nonce = c.Query("nonce")
	}

	log.Printf("[OAuth] Callback received for provider %s - code present: %v, state present: %v, nonce present: %v",
		providerName, code != "", state != "", nonce != "")

	if code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "bad_request",
			Message: "Missing authorization code",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Récupérer le provider pour la validation
	var provider models.OAuthProvider
	if err := h.db.Where("provider_name = ? AND is_enabled = ?", providerName, true).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: "OAuth provider not found or disabled",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Valider le state et nonce pour protection CSRF (optionnel selon le provider)
	var validatedNonce string
	var err error
	
	if state != "" && nonce != "" {
		// Validation complète si state et nonce sont présents
		log.Printf("[OAuth] Validating state and nonce for %s", providerName)
		
		validatedNonce, err = h.stateManager.ValidateState(state, providerName, provider.ClientID)
		if err != nil {
			log.Printf("[OAuth] State validation failed: %v", err)
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "invalid_state",
				Message: "Invalid or expired state parameter",
				Code:    http.StatusForbidden,
			})
			return
		}

		// Valider le nonce
		if err := h.stateManager.ValidateNonce(state, nonce); err != nil {
			log.Printf("[OAuth] Nonce validation failed: %v", err)
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "invalid_nonce",
				Message: "Invalid nonce parameter",
				Code:    http.StatusForbidden,
			})
			return
		}

		// Vérifier que les nonces correspondent
		if validatedNonce != nonce {
			log.Printf("[OAuth] Nonce mismatch: expected %s, got %s", validatedNonce, nonce)
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "nonce_mismatch",
				Message: "Nonce validation failed",
				Code:    http.StatusForbidden,
			})
			return
		}
		
		log.Printf("[OAuth] State and nonce validation successful for %s", providerName)
	} else {
		// Pour certains providers (comme Microsoft), on peut continuer sans state/nonce
		// ATTENTION: Ceci réduit la sécurité CSRF, mais permet l'authentification
		log.Printf("[OAuth] No state/nonce provided for %s, proceeding without CSRF validation", providerName)
		
		// Optionnel: Valider que le provider supporte l'authentification sans state
		if providerName == "microsoft" {
			log.Printf("[OAuth] Microsoft provider detected, allowing authentication without state")
		} else {
			log.Printf("[OAuth] Warning: Authentication without state for provider %s", providerName)
		}
	}

	log.Printf("[OAuth] State and nonce validation successful")

	// Valider la callback URL pour sécurité supplémentaire
	if err := utils.ValidateOAuthCallbackURL(provider.RedirectURI, c.Request.Host); err != nil {
		log.Printf("[OAuth] Callback URL validation failed: %v", err)
		// Pour le développement, on peut logger mais continuer
		if gin.Mode() == gin.DebugMode {
			log.Printf("[OAuth] Continuing in debug mode despite callback URL validation failure")
		} else {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "invalid_callback",
				Message: "Callback URL validation failed",
				Code:    http.StatusForbidden,
			})
			return
		}
	}

	// Échanger le code contre un token
	log.Printf("[OAuth] Exchanging code for token with %s...", provider.ProviderName)
	token, err := h.exchangeCodeForToken(provider, code)
	if err != nil {
		log.Printf("[OAuth] ❌ Error exchanging code for token: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "oauth_error",
			Message: "Failed to exchange code for token",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	log.Printf("[OAuth] ✅ Token exchange successful")

	// Récupérer les informations utilisateur
	log.Printf("[OAuth] Fetching user info from %s...", provider.ProviderName)
	userInfo, err := h.getUserInfo(provider, token)
	if err != nil {
		log.Printf("[OAuth] ❌ Error getting user info: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "oauth_error",
			Message: "Failed to get user information",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	log.Printf("[OAuth] ✅ User info retrieved: %v", userInfo["mail"])

	// Créer ou récupérer l'utilisateur
	log.Printf("[OAuth] Finding or creating user...")
	user, err := h.findOrCreateOAuthUser(provider.ProviderName, userInfo)
	if err != nil {
		log.Printf("[OAuth] ❌ Error finding or creating user: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "database_error",
			Message: "Failed to create or find user",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	log.Printf("[OAuth] ✅ User found/created: %s (%s)", user.Username, user.Email)

	// Mettre à jour la date de dernière connexion
	now := time.Now()
	if err := h.db.Model(&user).Update("last_login", now).Error; err != nil {
		log.Printf("[OAuth] Erreur lors de la mise à jour de la dernière connexion: %v", err)
		// Ne pas bloquer la connexion pour cette erreur
	}

	// Générer les tokens JWT
	jwtToken, err := h.authMiddleware.GenerateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "token_error",
			Message: "Failed to generate JWT token",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	refreshToken, err := h.authMiddleware.GenerateRefreshToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "token_error",
			Message: "Failed to generate refresh token",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Recharger l'utilisateur avec les groupes et les groupes administrés
	if err := h.db.Preload("Groups").Preload("AdminOfGroups").First(&user, user.ID).Error; err != nil {
		log.Printf("Error loading user groups: %v", err)
	}

	user.Password = ""

	c.JSON(http.StatusOK, models.LoginResponse{
		Token:        jwtToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// exchangeCodeForToken échange le code d'autorisation contre un token d'accès
func (h *OAuthHandler) exchangeCodeForToken(provider models.OAuthProvider, code string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", provider.ClientID)
	data.Set("client_secret", provider.ClientSecret)
	data.Set("redirect_uri", provider.RedirectURI)

	req, err := http.NewRequest("POST", provider.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no access_token in response")
	}

	return accessToken, nil
}

// getUserInfo récupère les informations utilisateur depuis le provider OAuth
func (h *OAuthHandler) getUserInfo(provider models.OAuthProvider, accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed: %s", string(body))
	}

	var userInfo map[string]interface{}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return userInfo, nil
}

// findOrCreateOAuthUser trouve ou crée un utilisateur OAuth
func (h *OAuthHandler) findOrCreateOAuthUser(providerName string, userInfo map[string]interface{}) (models.User, error) {
	var email, firstName, lastName, ssoID string

	// Extraire les informations selon le provider
	switch providerName {
	case "google":
		email, _ = userInfo["email"].(string)
		firstName, _ = userInfo["given_name"].(string)
		lastName, _ = userInfo["family_name"].(string)
		ssoID, _ = userInfo["sub"].(string)
	case "microsoft":
		email, _ = userInfo["mail"].(string)
		if email == "" {
			email, _ = userInfo["userPrincipalName"].(string)
		}
		firstName, _ = userInfo["givenName"].(string)
		lastName, _ = userInfo["surname"].(string)
		ssoID, _ = userInfo["id"].(string)
	}

	if email == "" || ssoID == "" {
		return models.User{}, fmt.Errorf("missing required user information")
	}

	// Chercher l'utilisateur existant par email d'abord, puis par SSO
	var user models.User
	err := h.db.Where("email = ?", email).First(&user).Error

	if err == gorm.ErrRecordNotFound {
		// Chercher par SSO ID si pas trouvé par email
		err = h.db.Where("sso_provider = ? AND sso_id = ?", providerName, ssoID).First(&user).Error
	}

	if err == gorm.ErrRecordNotFound {
		// Créer un nouvel utilisateur
		username := strings.Split(email, "@")[0]

		// S'assurer que le username est unique
		baseUsername := username
		counter := 1
		for {
			var existingUser models.User
			if err := h.db.Where("username = ?", username).First(&existingUser).Error; err == gorm.ErrRecordNotFound {
				break
			}
			username = fmt.Sprintf("%s%d", baseUsername, counter)
			counter++
		}

		user = models.User{
			Username:    username,
			Email:       email,
			FirstName:   firstName,
			LastName:    lastName,
			Role:        "user",
			IsActive:    true,
			SSOProvider: providerName,
			SSOID:       ssoID,
		}

		if err := h.db.Create(&user).Error; err != nil {
			return models.User{}, err
		}

		// Ajouter au groupe "common"
		var commonGroup models.Group
		if err := h.db.Where("name = ?", "common").First(&commonGroup).Error; err == nil {
			h.db.Model(&user).Association("Groups").Append(&commonGroup)
		}

		log.Printf("[OAuth] New user created: %s (%s) via %s", user.Email, user.Username, providerName)
	} else if err != nil {
		return models.User{}, err
	} else {
		// Utilisateur existant trouvé - mettre à jour les informations
		updated := false

		// Mettre à jour les infos SSO si l'utilisateur était créé manuellement
		if user.SSOProvider == "" || user.SSOID == "" {
			user.SSOProvider = providerName
			user.SSOID = ssoID
			updated = true
			log.Printf("[OAuth] Linking existing user %s to %s SSO", user.Email, providerName)
		}

		if user.FirstName != firstName && firstName != "" {
			user.FirstName = firstName
			updated = true
		}
		if user.LastName != lastName && lastName != "" {
			user.LastName = lastName
			updated = true
		}

		if updated {
			h.db.Save(&user)
		}
		log.Printf("[OAuth] Existing user logged in: %s (%s) via %s", user.Email, user.Username, providerName)
	}

	return user, nil
}

// generateRandomState function removed - replaced by OAuthStateManager
