package handlers

import (
	"airboard/middleware"
	"airboard/models"
	"airboard/services"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CommentHandler struct {
	DB           *gorm.DB
	Gamification *services.GamificationService
}

func NewCommentHandler(db *gorm.DB, gamification *services.GamificationService) *CommentHandler {
	return &CommentHandler{
		DB:           db,
		Gamification: gamification,
	}
}

// GetComments récupère tous les commentaires pour une entité (news, app, event)
func (h *CommentHandler) GetComments(c *gin.Context) {
	entityType := c.Query("entity_type") // news, application, event
	entityIDStr := c.Query("entity_id")

	if entityType == "" || entityIDStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "MISSING_PARAMETERS",
			Message: "entity_type et entity_id sont requis",
			Code:    http.StatusBadRequest,
		})
		return
	}

	entityID, err := strconv.ParseUint(entityIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_ENTITY_ID",
			Message: "entity_id doit être un nombre valide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Vérifier les paramètres de commentaires
	var settings models.CommentSettings
	if err := h.DB.First(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération des paramètres",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Vérifier si les commentaires sont activés pour ce type d'entité
	if !settings.CommentsEnabled {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "COMMENTS_DISABLED",
			Message: "Les commentaires sont désactivés",
			Code:    http.StatusForbidden,
		})
		return
	}

	switch entityType {
	case "news":
		if !settings.NewsCommentsEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "NEWS_COMMENTS_DISABLED",
				Message: "Les commentaires sur les actualités sont désactivés",
				Code:    http.StatusForbidden,
			})
			return
		}
	case "application":
		if !settings.AppCommentsEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "APP_COMMENTS_DISABLED",
				Message: "Les commentaires sur les applications sont désactivés",
				Code:    http.StatusForbidden,
			})
			return
		}
	case "event":
		if !settings.EventCommentsEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "EVENT_COMMENTS_DISABLED",
				Message: "Les commentaires sur les événements sont désactivés",
				Code:    http.StatusForbidden,
			})
			return
		}
	}

	// Récupérer les commentaires
	var comments []models.Comment
	query := h.DB.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Preload("User").
		Preload("Moderator").
		Order("created_at DESC")

	// Si modération requise, ne montrer que les commentaires approuvés (sauf pour admin/editor/admin de groupe)
	userRole, exists := c.Get("role")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)
	if settings.RequireModeration && exists {
		role := userRole.(string)
		// Admin, editor, et utilisateurs qui administrent au moins un groupe peuvent voir tous les commentaires
		if role != "admin" && role != "editor" && len(managedGroupIDs) == 0 {
			query = query.Where("is_approved = ?", true)
		}
	} else if settings.RequireModeration {
		query = query.Where("is_approved = ?", true)
	}

	if err := query.Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération des commentaires",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comments": comments,
		"total":    len(comments),
	})
}

// CreateComment crée un nouveau commentaire
func (h *CommentHandler) CreateComment(c *gin.Context) {
	var req models.CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Récupérer l'utilisateur connecté
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Utilisateur non authentifié",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	// Vérifier les paramètres de commentaires
	var settings models.CommentSettings
	if err := h.DB.First(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération des paramètres",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	if !settings.CommentsEnabled {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "COMMENTS_DISABLED",
			Message: "Les commentaires sont désactivés",
			Code:    http.StatusForbidden,
		})
		return
	}

	// Vérifier la limite de longueur
	if len(req.Content) > settings.MaxCommentLength {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "COMMENT_TOO_LONG",
			Message: "Le commentaire est trop long",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Créer le commentaire
	comment := models.Comment{
		Content:    req.Content,
		UserID:     userID.(uint),
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		IsApproved: !settings.RequireModeration, // Auto-approuvé si pas de modération
		IsFlagged:  false,
		ParentID:   req.ParentID,
	}

	if err := h.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la création du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Recharger avec les relations
	if err := h.DB.Preload("User").First(&comment, comment.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Accorder des points XP pour la création d'un commentaire
	if h.Gamification != nil {
		// 10 XP pour un commentaire
		go func() {
			err := h.Gamification.AwardXP(userID.(uint), 10, "comment_create", "{}")
			if err != nil {
				// Log error but don't fail request
				println("Error awarding XP for comment:", err.Error())
			}
		}()
	}

	c.JSON(http.StatusCreated, models.SuccessResponse{
		Message: "Commentaire créé avec succès",
		Data:    comment,
	})
}

// UpdateComment met à jour un commentaire (uniquement par l'auteur)
func (h *CommentHandler) UpdateComment(c *gin.Context) {
	commentID := c.Param("id")

	var req models.CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Utilisateur non authentifié",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	userRole, _ := c.Get("role")

	// Récupérer le commentaire
	var comment models.Comment
	if err := h.DB.First(&comment, commentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "COMMENT_NOT_FOUND",
				Message: "Commentaire non trouvé",
				Code:    http.StatusNotFound,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Vérifier que l'utilisateur est l'auteur ou admin
	if comment.UserID != userID.(uint) && userRole != "admin" && userRole != "editor" {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "FORBIDDEN",
			Message: "Vous n'avez pas la permission de modifier ce commentaire",
			Code:    http.StatusForbidden,
		})
		return
	}

	// Mettre à jour le contenu
	comment.Content = req.Content

	if err := h.DB.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la mise à jour du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Recharger avec les relations
	if err := h.DB.Preload("User").First(&comment, comment.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Commentaire mis à jour avec succès",
		Data:    comment,
	})
}

// DeleteComment supprime un commentaire (par l'auteur ou admin)
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	commentID := c.Param("id")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Utilisateur non authentifié",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	userRole, _ := c.Get("role")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)

	// Récupérer le commentaire
	var comment models.Comment
	if err := h.DB.First(&comment, commentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "COMMENT_NOT_FOUND",
				Message: "Commentaire non trouvé",
				Code:    http.StatusNotFound,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Vérifier que l'utilisateur est l'auteur ou admin/editor/admin de groupe
	if comment.UserID != userID.(uint) && userRole != "admin" && userRole != "editor" && len(managedGroupIDs) == 0 {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "FORBIDDEN",
			Message: "Vous n'avez pas la permission de supprimer ce commentaire",
			Code:    http.StatusForbidden,
		})
		return
	}

	// Supprimer le commentaire
	if err := h.DB.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la suppression du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Commentaire supprimé avec succès",
	})
}

// ModerateComment modère un commentaire (admin/editor seulement)
func (h *CommentHandler) ModerateComment(c *gin.Context) {
	var req models.ModerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Utilisateur non authentifié",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	// Récupérer le commentaire
	var comment models.Comment
	if err := h.DB.First(&comment, req.CommentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "COMMENT_NOT_FOUND",
				Message: "Commentaire non trouvé",
				Code:    http.StatusNotFound,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Mettre à jour le statut de modération
	comment.IsApproved = req.IsApproved
	comment.IsFlagged = req.IsFlagged
	moderatorID := userID.(uint)
	comment.ModeratedBy = &moderatorID
	now := time.Now()
	comment.ModeratedAt = &now

	if err := h.DB.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la modération du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Recharger avec les relations
	if err := h.DB.Preload("User").Preload("Moderator").First(&comment, comment.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération du commentaire",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Commentaire modéré avec succès",
		Data:    comment,
	})
}

// GetPendingComments récupère tous les commentaires en attente de modération (admin/editor)
func (h *CommentHandler) GetPendingComments(c *gin.Context) {
	var comments []models.Comment
	if err := h.DB.Where("is_approved = ?", false).
		Preload("User").
		Order("created_at DESC").
		Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération des commentaires",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comments": comments,
		"total":    len(comments),
	})
}

// GetCommentSettings récupère les paramètres de commentaires
func (h *CommentHandler) GetCommentSettings(c *gin.Context) {
	var settings models.CommentSettings
	if err := h.DB.First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Créer les paramètres par défaut
			settings = models.CommentSettings{
				CommentsEnabled:      true,
				NewsCommentsEnabled:  true,
				AppCommentsEnabled:   false,
				EventCommentsEnabled: true,
				RequireModeration:    false,
				AllowAnonymous:       false,
				MaxCommentLength:     1000,
			}
			if err := h.DB.Create(&settings).Error; err != nil {
				c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Error:   "DATABASE_ERROR",
					Message: "Erreur lors de la création des paramètres",
					Code:    http.StatusInternalServerError,
				})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "DATABASE_ERROR",
				Message: "Erreur lors de la récupération des paramètres",
				Code:    http.StatusInternalServerError,
			})
			return
		}
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateCommentSettings met à jour les paramètres de commentaires (admin seulement)
func (h *CommentHandler) UpdateCommentSettings(c *gin.Context) {
	var req models.CommentSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}

	var settings models.CommentSettings
	if err := h.DB.First(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la récupération des paramètres",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Mettre à jour uniquement les champs fournis
	if req.CommentsEnabled != nil {
		settings.CommentsEnabled = *req.CommentsEnabled
	}
	if req.NewsCommentsEnabled != nil {
		settings.NewsCommentsEnabled = *req.NewsCommentsEnabled
	}
	if req.AppCommentsEnabled != nil {
		settings.AppCommentsEnabled = *req.AppCommentsEnabled
	}
	if req.EventCommentsEnabled != nil {
		settings.EventCommentsEnabled = *req.EventCommentsEnabled
	}
	if req.RequireModeration != nil {
		settings.RequireModeration = *req.RequireModeration
	}
	if req.AllowAnonymous != nil {
		settings.AllowAnonymous = *req.AllowAnonymous
	}
	if req.MaxCommentLength != nil {
		settings.MaxCommentLength = *req.MaxCommentLength
	}

	if err := h.DB.Save(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Erreur lors de la mise à jour des paramètres",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Paramètres mis à jour avec succès",
		Data:    settings,
	})
}
