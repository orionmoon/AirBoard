package handlers

import (
	"log"
	"net/http"
	"strconv"

	"airboard/config"
	"airboard/middleware"
	"airboard/models"
	"airboard/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db                  *gorm.DB
	bcryptCost          int
	gamificationService *services.GamificationService
}

func NewAdminHandler(db *gorm.DB, cfg *config.Config, gs *services.GamificationService) *AdminHandler {
	return &AdminHandler{
		db:                  db,
		bcryptCost:          cfg.Security.BcryptCost,
		gamificationService: gs,
	}
}

// ============ GESTION DES GROUPES D'APPLICATIONS ============

// @Summary Lister les groupes d'applications
// @Description Récupère tous les groupes d'applications (admin uniquement)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Numéro de page" default(1)
// @Param limit query int false "Nombre d'éléments par page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /admin/app-groups [get]
func (h *AdminHandler) GetAppGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var appGroups []models.AppGroup
	var total int64

	// Filtrage selon le rôle
	query := h.db.Model(&models.AppGroup{})

	// Vérifier si l'utilisateur est admin d'au moins un groupe
	managedGroupIDs := middleware.GetManagedGroupIDs(c)
	if len(managedGroupIDs) > 0 {
		// Group admin voit les AppGroups des groupes qu'il administre
		// (même logique que le dashboard pour la cohérence)
		// Récupérer tous les AppGroups accessibles aux groupes administrés
		query = query.Where(`
			(
				-- AppGroups publics
				(is_private = false)
			)
			OR
			(
				-- AppGroups privés appartant aux groupes administrés
				is_private = true AND owner_group_id IN ?
			)
		`, managedGroupIDs)
	}
	// Admin voit tout (pas de filtre)

	query.Count(&total)
	if err := query.Preload("Applications").
		Preload("OwnerGroup").
		Order("\"order\" ASC, name ASC").
		Limit(limit).Offset(offset).
		Find(&appGroups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la récupération des groupes d'applications",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	totalPages := int(total)/limit + 1
	if int(total)%limit == 0 && total > 0 {
		totalPages = int(total) / limit
	}

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       appGroups,
		Total:      total,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	})
}

// @Summary Créer un groupe d'applications
// @Description Crée un nouveau groupe d'applications (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param appGroup body models.AppGroup true "Données du groupe d'applications"
// @Success 201 {object} models.AppGroup
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /admin/app-groups [post]
func (h *AdminHandler) CreateAppGroup(c *gin.Context) {
	var appGroup models.AppGroup
	if err := c.ShouldBindJSON(&appGroup); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Pour les admins de groupe (utilisateurs qui administrent au moins un groupe), l'AppGroup est automatiquement privé et appartient au premier groupe administré
	managedGroupIDs := middleware.GetManagedGroupIDs(c)
	isGroupAdmin := len(managedGroupIDs) > 0

	if isGroupAdmin {
		if len(managedGroupIDs) == 0 {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "Forbidden",
				Message: "Vous n'administrez aucun groupe",
				Code:    http.StatusForbidden,
			})
			return
		}

		// Forcer IsPrivate = true et définir OwnerGroupID pour les admins de groupe
		appGroup.IsPrivate = true
		ownerID := managedGroupIDs[0]
		appGroup.OwnerGroupID = &ownerID
	}
	// Admin global peut créer des AppGroups publics ou privés selon les données fournies

	// Créer l'AppGroup
	if err := h.db.Create(&appGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la création du groupe d'applications",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Pour les admins de groupe, lier automatiquement l'AppGroup à leurs groupes administrés via group_app_groups
	if isGroupAdmin {
		for _, groupID := range managedGroupIDs {
			h.db.Exec("INSERT INTO group_app_groups (group_id, app_group_id) VALUES (?, ?)", groupID, appGroup.ID)
		}
	}

	c.JSON(http.StatusCreated, appGroup)
}

// @Summary Modifier un groupe d'applications
// @Description Modifie un groupe d'applications existant (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID du groupe d'applications"
// @Param appGroup body models.AppGroup true "Données modifiées"
// @Success 200 {object} models.AppGroup
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /admin/app-groups/{id} [put]
func (h *AdminHandler) UpdateAppGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var appGroup models.AppGroup
	if err := h.db.First(&appGroup, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Groupe d'applications non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Vérification des permissions pour les admins de groupe
	// Un admin de groupe peut modifier un AppGroup seulement si c'est un AppGroup privé appartenant à son groupe
	if !middleware.CanManageAppGroupWithDB(c, appGroup.ID, h.db) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Forbidden",
			Message: "Vous ne pouvez pas modifier ce groupe d'applications. Seuls les AppGroups privés appartenant à votre groupe peuvent être modifiés.",
			Code:    http.StatusForbidden,
		})
		return
	}

	var updateData models.AppGroup
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if err := h.db.Model(&appGroup).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la modification",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, appGroup)
}

// @Summary Supprimer un groupe d'applications
// @Description Supprime un groupe d'applications (admin uniquement)
// @Tags Admin
// @Security BearerAuth
// @Param id path int true "ID du groupe d'applications"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /admin/app-groups/{id} [delete]
func (h *AdminHandler) DeleteAppGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Vérification des permissions pour les admins de groupe
	// Un admin de groupe peut supprimer un AppGroup seulement si c'est un AppGroup privé appartenant à son groupe
	if !middleware.CanManageAppGroupWithDB(c, uint(id), h.db) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Forbidden",
			Message: "Vous ne pouvez pas supprimer ce groupe d'applications. Seuls les AppGroups privés appartenant à votre groupe peuvent être supprimés.",
			Code:    http.StatusForbidden,
		})
		return
	}

	// Vérifier si le groupe existe
	var appGroup models.AppGroup
	if err := h.db.First(&appGroup, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Groupe d'applications non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Supprimer d'abord les applications associées (hard delete)
	if err := h.db.Unscoped().Where("app_group_id = ?", id).Delete(&models.Application{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la suppression des applications",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Supprimer les associations many-to-many (group_app_groups)
	if err := h.db.Exec("DELETE FROM group_app_groups WHERE app_group_id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la suppression des associations",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Supprimer le groupe d'applications (hard delete pour permettre la recréation avec le même nom)
	if err := h.db.Unscoped().Delete(&models.AppGroup{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la suppression",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Groupe d'applications supprimé avec succès",
	})
}

// ============ GESTION DES APPLICATIONS ============

// @Summary Lister les applications
// @Description Récupère toutes les applications (admin uniquement)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Numéro de page" default(1)
// @Param limit query int false "Nombre d'éléments par page" default(10)
// @Param app_group_id query int false "Filtrer par groupe d'applications"
// @Success 200 {object} models.PaginatedResponse
// @Router /admin/applications [get]
func (h *AdminHandler) GetApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	appGroupID := c.Query("app_group_id")
	offset := (page - 1) * limit

	var applications []models.Application
	var total int64

	query := h.db.Model(&models.Application{})
	if appGroupID != "" {
		query = query.Where("app_group_id = ?", appGroupID)
	}

	// Filtrage selon le rôle pour les admins de groupe
	role := c.GetString("role")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)
	if role != "admin" && len(managedGroupIDs) > 0 {
		// Admin de groupe voit les applications des AppGroups des groupes qu'il administre
		// (même logique que le dashboard pour la cohérence)
		if len(managedGroupIDs) > 0 {
			// Récupérer toutes les applications dans les AppGroups accessibles aux groupes administrés
			query = query.Joins("JOIN app_groups ON applications.app_group_id = app_groups.id").
				Where(`
					(
						-- Applications dans des AppGroups publics
						(app_groups.is_private = false)
					)
					OR
					(
						-- Applications dans des AppGroups privés appartant aux groupes administrés
						app_groups.is_private = true AND app_groups.owner_group_id IN ?
					)
				`, managedGroupIDs)
		} else {
			// Aucun groupe administré = aucune application visible
			query = query.Where("1 = 0")
		}
	}

	query.Count(&total)
	if err := query.Preload("AppGroup").
		Order("\"order\" ASC, name ASC").
		Limit(limit).Offset(offset).
		Find(&applications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la récupération des applications",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	totalPages := int(total)/limit + 1
	if int(total)%limit == 0 && total > 0 {
		totalPages = int(total) / limit
	}

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       applications,
		Total:      total,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	})
}

// @Summary Créer une application
// @Description Crée une nouvelle application (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param application body models.Application true "Données de l'application"
// @Success 201 {object} models.Application
// @Router /admin/applications [post]
func (h *AdminHandler) CreateApplication(c *gin.Context) {
	var application models.Application
	if err := c.ShouldBindJSON(&application); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Vérification des permissions pour les admins de groupe
	// Un admin de groupe peut créer une application seulement dans un AppGroup privé qu'il possède
	if !middleware.CanManageAppGroupWithDB(c, application.AppGroupID, h.db) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Forbidden",
			Message: "Vous ne pouvez pas créer d'application dans cet AppGroup. Seuls les AppGroups privés appartenant à votre groupe sont autorisés.",
			Code:    http.StatusForbidden,
		})
		return
	}

	if err := h.db.Create(&application).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la création de l'application",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Charger la relation AppGroup
	h.db.Preload("AppGroup").First(&application, application.ID)

	// Envoyer une notification email si l'application est active
	if application.IsActive {
		go func() {
			emailService := services.NewEmailService(h.db, config.LoadConfig())
			// Récupérer les groupes liés à l'AppGroup
			var targetGroupIDs []uint
			h.db.Table("group_app_groups").Where("app_group_id = ?", application.AppGroupID).Pluck("group_id", &targetGroupIDs)
			if err := emailService.SendNotification("application", application.ID, targetGroupIDs); err != nil {
				log.Printf("[Email] Échec de l'envoi de la notification application: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, application)
}

// @Summary Modifier une application
// @Description Modifie une application existante (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID de l'application"
// @Param application body models.Application true "Données modifiées"
// @Success 200 {object} models.Application
// @Router /admin/applications/{id} [put]
func (h *AdminHandler) UpdateApplication(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var application models.Application
	if err := h.db.Preload("AppGroup").First(&application, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Application non trouvée",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Vérification des permissions pour les admins de groupe
	// Un admin de groupe peut modifier une application seulement si elle est dans un AppGroup privé qu'il possède
	if application.AppGroup == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "AppGroup introuvable pour cette application",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	if !middleware.CanManageAppGroupWithDB(c, application.AppGroup.ID, h.db) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Forbidden",
			Message: "Vous ne pouvez pas modifier cette application. Seules les applications des AppGroups privés appartenant à votre groupe sont autorisées.",
			Code:    http.StatusForbidden,
		})
		return
	}

	var updateData models.Application
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if err := h.db.Model(&application).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la modification",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Charger la relation AppGroup
	h.db.Preload("AppGroup").First(&application, application.ID)

	c.JSON(http.StatusOK, application)
}

// @Summary Supprimer une application
// @Description Supprime une application (admin uniquement)
// @Tags Admin
// @Security BearerAuth
// @Param id path int true "ID de l'application"
// @Success 200 {object} models.SuccessResponse
// @Router /admin/applications/{id} [delete]
func (h *AdminHandler) DeleteApplication(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Vérification des permissions pour les admins de groupe
	role := c.GetString("role")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)
	if role != "admin" && len(managedGroupIDs) > 0 {
		var application models.Application
		if err := h.db.Preload("AppGroup").First(&application, id).Error; err != nil {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "Not Found",
				Message: "Application non trouvée",
				Code:    http.StatusNotFound,
			})
			return
		}

		// Vérifier que l'application appartient à un AppGroup privé que l'admin de groupe possède
		if application.AppGroup == nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Internal Server Error",
				Message: "AppGroup introuvable pour cette application",
				Code:    http.StatusInternalServerError,
			})
			return
		}

		if !middleware.CanManageAppGroupWithDB(c, application.AppGroup.ID, h.db) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "Forbidden",
				Message: "Vous ne pouvez pas supprimer cette application. Seules les applications des AppGroups privés appartenant à votre groupe sont autorisées.",
				Code:    http.StatusForbidden,
			})
			return
		}
	}

	if err := h.db.Delete(&models.Application{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la suppression",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Application supprimée avec succès",
	})
}

// ============ GESTION DES UTILISATEURS ============

// @Summary Lister les utilisateurs
// @Description Récupère tous les utilisateurs (admin uniquement)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.User
// @Router /admin/users [get]
func (h *AdminHandler) GetUsers(c *gin.Context) {
	var users []models.User
	if err := h.db.Preload("Groups").Preload("AdminOfGroups").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la récupération des utilisateurs",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Masquer les mots de passe
	for i := range users {
		users[i].Password = ""
	}

	c.JSON(http.StatusOK, users)
}

// @Summary Créer un utilisateur
// @Description Crée un nouvel utilisateur (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body models.RegisterRequest true "Données de l'utilisateur"
// @Success 201 {object} models.User
// @Router /admin/users [post]
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var createData struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
		IsActive  bool   `json:"is_active"`
		GroupIDs  []uint `json:"group_ids"`
	}

	if err := c.ShouldBindJSON(&createData); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Vérifier si l'utilisateur existe déjà
	var existingUser models.User
	if err := h.db.Where("username = ? OR email = ?", createData.Username, createData.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Conflict",
			Message: "Nom d'utilisateur ou email déjà utilisé",
			Code:    http.StatusConflict,
		})
		return
	}

	// Hasher le mot de passe avec coût sécurisé (12 minimum - OWASP 2025)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(createData.Password), h.bcryptCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors du hashage du mot de passe",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	user := models.User{
		Username:  createData.Username,
		Email:     createData.Email,
		Password:  string(hashedPassword),
		FirstName: createData.FirstName,
		LastName:  createData.LastName,
		Role:      createData.Role,
		IsActive:  createData.IsActive,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la création de l'utilisateur",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Associer les groupes si fournis
	if len(createData.GroupIDs) > 0 {
		var groups []models.Group
		if err := h.db.Where("id IN ?", createData.GroupIDs).Find(&groups).Error; err == nil {
			h.db.Model(&user).Association("Groups").Replace(groups)
		}
	}

	user.Password = ""
	h.db.Preload("Groups").First(&user, user.ID)

	c.JSON(http.StatusCreated, user)
}

// @Summary Modifier un utilisateur
// @Description Modifie un utilisateur existant (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID de l'utilisateur"
// @Param user body models.User true "Données modifiées"
// @Success 200 {object} models.User
// @Router /admin/users/{id} [put]
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Utilisateur non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	var updateData struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
		IsActive  *bool  `json:"is_active"`
		Password  string `json:"password,omitempty"`
		GroupIDs  []uint `json:"group_ids"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Mise à jour des champs
	if updateData.Username != "" {
		user.Username = updateData.Username
	}
	if updateData.Email != "" {
		user.Email = updateData.Email
	}
	if updateData.FirstName != "" {
		user.FirstName = updateData.FirstName
	}
	if updateData.LastName != "" {
		user.LastName = updateData.LastName
	}
	if updateData.Role != "" {
		user.Role = updateData.Role
	}
	if updateData.IsActive != nil {
		user.IsActive = *updateData.IsActive
	}

	// Hash du nouveau mot de passe si fourni avec coût sécurisé (12 minimum - OWASP 2025)
	if updateData.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(updateData.Password), h.bcryptCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Internal Server Error",
				Message: "Erreur lors du hashage du mot de passe",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		user.Password = string(hashedPassword)
	}

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la modification",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Mise à jour des groupes si fournis
	if updateData.GroupIDs != nil {
		var groups []models.Group
		if err := h.db.Where("id IN ?", updateData.GroupIDs).Find(&groups).Error; err == nil {
			h.db.Model(&user).Association("Groups").Replace(groups)
		}
	}

	// Masquer le mot de passe
	user.Password = ""

	// Charger les relations
	h.db.Preload("Groups").First(&user, user.ID)

	c.JSON(http.StatusOK, user)
}

// @Summary Supprimer un utilisateur
// @Description Supprime un utilisateur (admin uniquement)
// @Tags Admin
// @Security BearerAuth
// @Param id path int true "ID de l'utilisateur"
// @Success 200 {object} models.SuccessResponse
// @Router /admin/users/{id} [delete]
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Vérifier que l'utilisateur existe
	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Utilisateur non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Supprimer les associations avec les groupes
	h.db.Model(&user).Association("Groups").Clear()

	// Supprimer l'utilisateur
	if err := h.db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la suppression",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Utilisateur supprimé avec succès",
	})
}

// GetDeletedUsers récupère tous les utilisateurs supprimés (soft deleted)
func (h *AdminHandler) GetDeletedUsers(c *gin.Context) {
	var users []models.User

	// Utiliser Unscoped() pour voir les enregistrements soft deleted
	if err := h.db.Unscoped().Where("deleted_at IS NOT NULL").Preload("Groups").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la récupération des utilisateurs supprimés",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Masquer les mots de passe
	for i := range users {
		users[i].Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
	})
}

// RestoreUser restaure un utilisateur supprimé
func (h *AdminHandler) RestoreUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Trouver l'utilisateur supprimé
	var user models.User
	if err := h.db.Unscoped().First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Utilisateur non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Vérifier qu'il est bien supprimé
	if user.DeletedAt.Time.IsZero() {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Cet utilisateur n'est pas supprimé",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Restaurer l'utilisateur
	user.DeletedAt = gorm.DeletedAt{}
	if err := h.db.Unscoped().Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la restauration",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Utilisateur restauré avec succès",
		Data:    user,
	})
}

// PermanentlyDeleteUser supprime définitivement un utilisateur
func (h *AdminHandler) PermanentlyDeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Trouver l'utilisateur supprimé
	var user models.User
	if err := h.db.Unscoped().First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Utilisateur non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Vérifier qu'il est bien supprimé
	if user.DeletedAt.Time.IsZero() {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Cet utilisateur doit d'abord être supprimé",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Supprimer définitivement dans une transaction
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		// 1. Supprimer les associations many-to-many
		txUnscoped := tx.Unscoped()
		if err := txUnscoped.Model(&user).Association("Groups").Clear(); err != nil {
			return err
		}
		if err := txUnscoped.Model(&user).Association("Favorites").Clear(); err != nil {
			return err
		}
		if err := txUnscoped.Model(&user).Association("AdminOfGroups").Clear(); err != nil {
			return err
		}

		// 2. Supprimer les enregistrements liés à l'utilisateur
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.NewsReaction{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.NewsRead{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.ApplicationClick{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("sender_id = ? OR recipient_id = ?", user.ID, user.ID).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.Comment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.Feedback{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.PollVote{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.GamificationProfile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.UserAchievement{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.XPTransaction{}).Error; err != nil {
			return err
		}

		// 3. Nullifier les références d'auteur sur le contenu (préserver les articles/sondages)
		if err := tx.Model(&models.News{}).Where("author_id = ?", user.ID).Update("author_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Poll{}).Where("author_id = ?", user.ID).Update("author_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("author_id = ?", user.ID).Delete(&models.Event{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Media{}).Where("uploaded_by = ?", user.ID).Update("uploaded_by", nil).Error; err != nil {
			return err
		}
		// Nullifier moderated_by dans les commentaires
		if err := tx.Model(&models.Comment{}).Where("moderated_by = ?", user.ID).Update("moderated_by", nil).Error; err != nil {
			return err
		}

		// 4. Supprimer définitivement l'utilisateur
		if err := txUnscoped.Delete(&user).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la suppression définitive",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Utilisateur supprimé définitivement",
	})
}

// @Summary Assigner un utilisateur à des groupes
// @Description Assigne un utilisateur à des groupes d'utilisateurs (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID de l'utilisateur"
// @Param groups body []int true "IDs des groupes"
// @Success 200 {object} models.User
// @Router /admin/users/{id}/groups [put]
func (h *AdminHandler) AssignUserToGroups(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Utilisateur non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	var groupIDs []uint
	if err := c.ShouldBindJSON(&groupIDs); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "IDs de groupes invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Récupérer les groupes
	var groups []models.Group
	if err := h.db.Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la récupération des groupes",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Remplacer les associations
	if err := h.db.Model(&user).Association("Groups").Replace(groups); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de l'assignation",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Recharger avec les relations
	h.db.Preload("Groups").First(&user, user.ID)
	user.Password = ""

	c.JSON(http.StatusOK, user)
}

// @Summary Lister les groupes
// @Description Récupère tous les groupes d'utilisateurs (admin uniquement)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Group
// @Router /admin/groups [get]
func (h *AdminHandler) GetGroups(c *gin.Context) {
	var groups []models.Group
	if err := h.db.Preload("Users").Preload("AppGroups").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la récupération des groupes",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, groups)
}

// @Summary Créer un groupe
// @Description Crée un nouveau groupe d'utilisateurs (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param group body models.Group true "Données du groupe"
// @Success 201 {object} models.Group
// @Router /admin/groups [post]
func (h *AdminHandler) CreateGroup(c *gin.Context) {
	var createData struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
		IsActive    bool   `json:"is_active"`
		AppGroupIDs []uint `json:"app_group_ids"`
	}

	if err := c.ShouldBindJSON(&createData); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	group := models.Group{
		Name:        createData.Name,
		Description: createData.Description,
		Color:       createData.Color,
		IsActive:    createData.IsActive,
	}

	if err := h.db.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la création du groupe",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Associer les groupes d'applications si fournis
	if len(createData.AppGroupIDs) > 0 {
		var appGroups []models.AppGroup
		if err := h.db.Where("id IN ?", createData.AppGroupIDs).Find(&appGroups).Error; err == nil {
			h.db.Model(&group).Association("AppGroups").Replace(appGroups)
		}
	}

	// Charger les relations
	h.db.Preload("Users").Preload("AppGroups").First(&group, group.ID)

	c.JSON(http.StatusCreated, group)
}

// @Summary Modifier un groupe
// @Description Modifie un groupe d'utilisateurs existant (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID du groupe"
// @Param group body models.Group true "Données modifiées"
// @Success 200 {object} models.Group
// @Router /admin/groups/{id} [put]
func (h *AdminHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var group models.Group
	if err := h.db.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Groupe non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	var updateData struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
		IsActive    *bool  `json:"is_active"`
		AppGroupIDs []uint `json:"app_group_ids"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Données invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Mise à jour des champs
	if updateData.Name != "" {
		group.Name = updateData.Name
	}
	if updateData.Description != "" {
		group.Description = updateData.Description
	}
	if updateData.Color != "" {
		group.Color = updateData.Color
	}
	if updateData.IsActive != nil {
		group.IsActive = *updateData.IsActive
	}

	if err := h.db.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la modification",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Mise à jour des groupes d'applications si fournis
	if updateData.AppGroupIDs != nil {
		var appGroups []models.AppGroup
		// Si le tableau n'est pas vide, chercher les groupes
		if len(updateData.AppGroupIDs) > 0 {
			if err := h.db.Where("id IN ?", updateData.AppGroupIDs).Find(&appGroups).Error; err == nil {
				h.db.Model(&group).Association("AppGroups").Replace(appGroups)
			}
		} else {
			// Tableau vide signifie supprimer toutes les associations
			h.db.Model(&group).Association("AppGroups").Clear()
		}
	}

	// Charger les relations
	h.db.Preload("Users").Preload("AppGroups").First(&group, group.ID)

	c.JSON(http.StatusOK, group)
}

// @Summary Supprimer un groupe
// @Description Supprime un groupe d'utilisateurs (admin uniquement)
// @Tags Admin
// @Security BearerAuth
// @Param id path int true "ID du groupe"
// @Success 200 {object} models.SuccessResponse
// @Router /admin/groups/{id} [delete]
func (h *AdminHandler) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var group models.Group
	if err := h.db.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Groupe non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Supprimer les associations
	h.db.Model(&group).Association("Users").Clear()
	h.db.Model(&group).Association("AppGroups").Clear()

	// Supprimer le groupe
	if err := h.db.Delete(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la suppression",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Groupe supprimé avec succès",
	})
}

// @Summary Assigner des groupes d'applications à un groupe d'utilisateurs
// @Description Définit les permissions d'un groupe d'utilisateurs (admin uniquement)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID du groupe d'utilisateurs"
// @Param app_groups body []int true "IDs des groupes d'applications"
// @Success 200 {object} models.Group
// @Router /admin/groups/{id}/app-groups [put]
func (h *AdminHandler) AssignGroupToAppGroups(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "ID invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var group models.Group
	if err := h.db.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Groupe non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	var appGroupIDs []uint
	if err := c.ShouldBindJSON(&appGroupIDs); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "IDs de groupes d'applications invalides",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Récupérer les groupes d'applications
	var appGroups []models.AppGroup
	if err := h.db.Where("id IN ?", appGroupIDs).Find(&appGroups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de la récupération des groupes d'applications",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Remplacer les associations
	if err := h.db.Model(&group).Association("AppGroups").Replace(appGroups); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal Server Error",
			Message: "Erreur lors de l'assignation",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Recharger avec les relations
	h.db.Preload("Users").Preload("AppGroups").First(&group, group.ID)

	c.JSON(http.StatusOK, group)
}

// ============ GESTION DE LA BASE DE DONNÉES ============

// @Summary Réinitialiser la base de données
// @Description Supprime toutes les données et recrée les données initiales (admin uniquement - ATTENTION: opération destructive)
// @Tags Admin
// @Security BearerAuth
// @Success 200 {object} models.SuccessResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /admin/database/reset [post]
func (h *AdminHandler) ResetDatabase(c *gin.Context) {
	// Liste de toutes les tables à réinitialiser (dans l'ordre de dépendances)
	tables := []string{
		// Tables de jointure many-to-many
		"user_favorites",
		"user_groups",
		"group_app_groups",
		"group_admins",
		"news_tags",

		// Tables dépendantes (analytics, reactions, etc.)
		"application_clicks",
		"news_reactions",
		"news_reads",
		"poll_votes",
		"comments",
		"feedbacks",
		"notifications",
		"email_notification_logs",

		// Tables avec relations
		"poll_options",
		"polls",
		"events",
		"event_categories",
		"news",
		"news_categories",
		"tags",
		"announcements",
		"applications",
		"app_groups",
		"media",
		"comment_settings",
		"email_templates",
		"smtp_configs",
		"email_oauth_configs",
		"oauth_providers",

		// Tables principales
		"users",
		"groups",
		"app_settings",
	}

	// Utiliser TRUNCATE CASCADE pour supprimer toutes les données et gérer automatiquement les dépendances
	// TRUNCATE est plus rapide que DELETE et réinitialise automatiquement les séquences
	for _, table := range tables {
		query := "TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE"
		if err := h.db.Exec(query).Error; err != nil {
			// Ignorer les erreurs si la table n'existe pas
			// (peut arriver si certaines fonctionnalités ne sont pas encore migrées)
			continue
		}
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Base de données réinitialisée avec succès. Veuillez redémarrer le serveur pour recréer les données initiales.",
		Data: map[string]interface{}{
			"note": "Les données de démonstration seront recréées au prochain démarrage du serveur",
		},
	})
}

// ============ GESTION DES GROUP ADMINS ============

// GetGroupAdmins récupère les admins d'un groupe
func (h *AdminHandler) GetGroupAdmins(c *gin.Context) {
	groupID := c.Param("id")

	// Charger le groupe avec ses admins
	var group models.Group
	if err := h.db.Preload("Users", "users.id IN (SELECT user_id FROM group_admins WHERE group_id = ?)", groupID).First(&group, groupID).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Groupe non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Récupérer les utilisateurs qui sont admins de ce groupe
	var adminUsers []models.User
	h.db.Table("users").
		Select("users.*").
		Joins("JOIN group_admins ON users.id = group_admins.user_id").
		Where("group_admins.group_id = ?", groupID).
		Find(&adminUsers)

	// Masquer les mots de passe
	for i := range adminUsers {
		adminUsers[i].Password = ""
	}

	c.JSON(http.StatusOK, adminUsers)
}

// AssignGroupAdmins assigne des utilisateurs comme admins d'un groupe
func (h *AdminHandler) AssignGroupAdmins(c *gin.Context) {
	groupID := c.Param("id")

	var request struct {
		UserIDs []uint `json:"user_ids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Bad Request",
			Message: "Format de requête invalide",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Charger le groupe
	var group models.Group
	if err := h.db.First(&group, groupID).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not Found",
			Message: "Groupe non trouvé",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Charger les utilisateurs à assigner
	var users []models.User
	if len(request.UserIDs) > 0 {
		if err := h.db.Where("id IN ?", request.UserIDs).Find(&users).Error; err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Bad Request",
				Message: "Certains utilisateurs n'ont pas été trouvés",
				Code:    http.StatusBadRequest,
			})
			return
		}
	}

	// Utiliser l'association GORM pour remplacer les admins
	// D'abord, supprimer toutes les associations existantes pour ce groupe
	h.db.Exec("DELETE FROM group_admins WHERE group_id = ?", groupID)

	// Puis ajouter les nouvelles associations
	if len(users) > 0 {
		for _, user := range users {
			// Utiliser Replace sur l'association AdminOfGroups de chaque utilisateur
			h.db.Model(&user).Association("AdminOfGroups").Append(&group)
		}
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Administrateurs de groupe assignés avec succès",
	})
}
