package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"airboard/middleware"
	"airboard/models"
	"airboard/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PollsHandler struct {
	db *gorm.DB
}

func NewPollsHandler(db *gorm.DB) *PollsHandler {
	return &PollsHandler{db: db}
}

// GetPolls - Liste des sondages (accessible à tous les utilisateurs connectés)
func (h *PollsHandler) GetPolls(c *gin.Context) {
	var polls []models.Poll

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	offset := (page - 1) * pageSize

	// Construction de la requête
	query := h.db.Model(&models.Poll{}).
		Preload("Author").
		Preload("TargetGroups").
		Preload("Options").
		Preload("News").
		Preload("Announcement")

	log.Printf("[DEBUG GetPolls] Starting query for role=%s, userID=%d, path=%s", c.GetString("role"), c.GetUint("user_id"), c.Request.URL.Path)

	// Explicit soft delete filter (complex WHERE clauses may bypass GORM's automatic filter)
	query = query.Where("deleted_at IS NULL")

	// Filtres
	if status := c.Query("status"); status != "" {
		if status == "active" {
			query = query.Where("is_active = ?", true)
		} else if status == "closed" {
			query = query.Where("is_active = ?", false)
		}
	}

	if pollType := c.Query("type"); pollType != "" {
		query = query.Where("poll_type = ?", pollType)
	}

	// Filtre par news
	if newsID := c.Query("news_id"); newsID != "" {
		query = query.Where("news_id = ?", newsID)
	}

	// Filtre par annonce
	if announcementID := c.Query("announcement_id"); announcementID != "" {
		query = query.Where("announcement_id = ?", announcementID)
	}

	// Recherche sécurisée avec validation et assainissement
	if search := c.Query("search"); search != "" {
		// Valider l'input de recherche
		if len(search) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La recherche doit contenir au moins 2 caractères"})
			return
		}
		if len(search) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La recherche ne peut pas dépasser 100 caractères"})
			return
		}

		// Vérifier les caractères interdits pour prévenir SQL injection et XSS
		forbiddenChars := []string{"<", ">", "&", "\"", "'", "(", ")", "=", "+", ";", "--", "/*", "*/", "union", "select", "insert", "update", "delete", "drop", "create"}
		lowerSearch := strings.ToLower(search)
		for _, char := range forbiddenChars {
			if strings.Contains(lowerSearch, char) {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Caractère ou terme interdit dans la recherche: %s", char)})
				return
			}
		}

		// Assainir l'input pour les requêtes LIKE
		sanitizedSearch := strings.ReplaceAll(search, "%", "\\%")
		sanitizedSearch = strings.ReplaceAll(sanitizedSearch, "_", "\\_")

		query = query.Where("title ILIKE ? OR description ILIKE ?", "%"+sanitizedSearch+"%", "%"+sanitizedSearch+"%")
	}

	// Filtre de visibilité par groupes selon le rôle
	userRole := c.GetString("role")
	userID := c.GetUint("user_id")

	if userRole == "admin" {
		// Admin voit tout
	} else if userRole == "group_admin" {
		// Group admin : distinguer interface admin vs interface publique
		managedGroupIDs := middleware.GetManagedGroupIDs(c)

		// Récupérer aussi les groupes d'appartenance pour la lecture publique
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

		// Si le group_admin accède via /group-admin/polls, on filtre strictement
		// Sinon (lecture publique via /polls), il voit les sondages comme un user normal
		isAdminInterface := strings.HasPrefix(c.Request.URL.Path, "/api/v1/group-admin/polls")

		if isAdminInterface {
			// Interface d'administration : seulement les sondages qu'il peut gérer
			if len(managedGroupIDs) > 0 {
				query = query.Where(`
					(author_id = ?) OR
					EXISTS (
						SELECT 1 FROM poll_target_groups
						WHERE poll_target_groups.poll_id = polls.id
						AND poll_target_groups.group_id IN (?)
					)
				`, userID, managedGroupIDs)
			} else {
				// Pas de groupes gérés : uniquement ses propres sondages
				query = query.Where("author_id = ?", userID)
			}
		} else {
			// Interface publique : sondages globaux + ceux de ses groupes (appartenance + administration)
			if len(combinedGroupIDs) > 0 {
				query = query.Where(`
					(SELECT COUNT(*) FROM poll_target_groups WHERE poll_target_groups.poll_id = polls.id) = 0
					OR EXISTS (
						SELECT 1 FROM poll_target_groups
						WHERE poll_target_groups.poll_id = polls.id
						AND poll_target_groups.group_id IN (?)
					)
				`, combinedGroupIDs)
			} else {
				// Si pas de groupes, voir seulement les sondages globaux
				query = query.Where("(SELECT COUNT(*) FROM poll_target_groups WHERE poll_target_groups.poll_id = polls.id) = 0")
			}
		}
	} else {
		// User régulier ou editor : sondages globaux + ceux de leurs groupes
		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		if len(userGroupIDs) > 0 {
			query = query.Where(`
				(SELECT COUNT(*) FROM poll_target_groups WHERE poll_target_groups.poll_id = polls.id) = 0
				OR EXISTS (
					SELECT 1 FROM poll_target_groups
					WHERE poll_target_groups.poll_id = polls.id
					AND poll_target_groups.group_id IN (?)
				)
			`, userGroupIDs)
		} else {
			// Si pas de groupes, voir seulement les sondages globaux
			query = query.Where("(SELECT COUNT(*) FROM poll_target_groups WHERE poll_target_groups.poll_id = polls.id) = 0")
		}
	}

	// Tri
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")
	query = query.Order(sortBy + " " + sortOrder)

	// Compte total
	var total int64
	query.Count(&total)

	// Récupération avec pagination
	if err := query.Offset(offset).Limit(pageSize).Find(&polls).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch polls"})
		return
	}

	// Calcul du nombre de pages
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.PollListResponse{
		Polls:      polls,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// GetPollByID - Récupérer un sondage par son ID
func (h *PollsHandler) GetPollByID(c *gin.Context) {
	id := c.Param("id")

	var poll models.Poll
	if err := h.db.Preload("Author").
		Preload("TargetGroups").
		Preload("Options").
		Preload("News").
		Preload("Announcement").
		First(&poll, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Poll not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch poll"})
		}
		return
	}

	// Vérifier les permissions selon le rôle
	userRole := c.GetString("role")
	userID := c.GetUint("user_id")

	if userRole == "admin" {
		// Admin voit tout
		c.JSON(http.StatusOK, poll)
		return
	}

	// Vérifier l'accès pour group_admin et user
	hasAccess := false

	// Récupérer les groupes administrés ET les groupes d'appartenance
	managedGroupIDs := middleware.GetManagedGroupIDs(c)
	var userGroupIDs []uint
	h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

	// Combiner les deux listes
	allGroupIDs := make(map[uint]bool)
	for _, id := range userGroupIDs {
		allGroupIDs[id] = true
	}
	for _, id := range managedGroupIDs {
		allGroupIDs[id] = true
	}

	// Si pas de groupes cibles, c'est un sondage global
	if len(poll.TargetGroups) == 0 {
		hasAccess = true
	} else {
		// Vérifier si l'utilisateur appartient ou administre au moins un groupe cible
		for _, targetGroup := range poll.TargetGroups {
			if allGroupIDs[targetGroup.ID] {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this poll"})
		return
	}

	c.JSON(http.StatusOK, poll)
}

// CreatePoll - Créer un sondage (admin/group_admin/editor)
func (h *PollsHandler) CreatePoll(c *gin.Context) {
	var req models.PollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Récupérer l'ID de l'utilisateur connecté
	userID := c.GetUint("user_id")
	userRole := c.GetString("role")

	// Validation : au moins 2 options, max 10
	if len(req.Options) < 2 || len(req.Options) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Poll must have between 2 and 10 options"})
		return
	}

	// Vérifier les permissions pour les groupes cibles (group_admin)
	if userRole == "group_admin" && len(req.TargetGroupIDs) > 0 {
		managedGroupIDs := middleware.GetManagedGroupIDs(c)
		for _, targetGroupID := range req.TargetGroupIDs {
			canManage := false
			for _, managedID := range managedGroupIDs {
				if targetGroupID == managedID {
					canManage = true
					break
				}
			}
			if !canManage {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Vous ne pouvez cibler que les groupes que vous administrez",
				})
				return
			}
		}
	}

	poll := models.Poll{
		Title:          req.Title,
		Description:    req.Description,
		PollType:       req.PollType,
		IsAnonymous:    req.IsAnonymous,
		IsActive:       req.IsActive,
		ShowResults:    req.ShowResults,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		NewsID:         req.NewsID,
		AnnouncementID: req.AnnouncementID,
		AuthorID:       userID,
	}

	// Créer le sondage
	if err := h.db.Create(&poll).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create poll"})
		return
	}

	// Créer les options
	for i, optReq := range req.Options {
		option := models.PollOption{
			PollID: poll.ID,
			Text:   optReq.Text,
			Order:  i,
		}
		if err := h.db.Create(&option).Error; err != nil {
			// Rollback : supprimer le sondage créé
			h.db.Delete(&poll)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create poll options"})
			return
		}
	}

	// Associer les groupes cibles
	if len(req.TargetGroupIDs) > 0 {
		var groups []models.Group
		h.db.Where("id IN ?", req.TargetGroupIDs).Find(&groups)
		h.db.Model(&poll).Association("TargetGroups").Replace(groups)
	}

	// Recharger avec les relations
	h.db.Preload("Author").
		Preload("TargetGroups").
		Preload("Options").
		Preload("News").
		Preload("Announcement").
		First(&poll, poll.ID)

	// Envoyer des notifications
	if poll.IsActive {
		go func() {
			notifService := services.NewNotificationService(h.db)

			// Récupérer les utilisateurs à notifier
			var userIDs []uint
			if len(poll.TargetGroups) > 0 {
				// Notifier les membres des groupes cibles
				var groupIDs []uint
				for _, g := range poll.TargetGroups {
					groupIDs = append(groupIDs, g.ID)
				}
				h.db.Table("user_groups").
					Where("group_id IN ?", groupIDs).
					Distinct("user_id").
					Pluck("user_id", &userIDs)
			} else {
				// Sondage global : notifier tous les utilisateurs actifs
				h.db.Model(&models.User{}).
					Where("is_active = ?", true).
					Where("id != ?", userID). // Ne pas notifier l'auteur
					Pluck("id", &userIDs)
			}

			// Envoyer la notification
			if len(userIDs) > 0 {
				if err := notifService.NotifyNewPoll(poll.Title, poll.ID, userIDs); err != nil {
					log.Printf("[Notification] Échec de l'envoi de la notification de sondage: %v", err)
				}
			}
		}()
	}

	c.JSON(http.StatusCreated, poll)
}

// UpdatePoll - Modifier un sondage
func (h *PollsHandler) UpdatePoll(c *gin.Context) {
	id := c.Param("id")

	var poll models.Poll
	if err := h.db.Preload("Options").Preload("TargetGroups").First(&poll, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Poll not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch poll"})
		}
		return
	}

	// Vérifier les permissions
	userID := c.GetUint("user_id")
	userRole := c.GetString("role")

	canEdit := false
	if userRole == "admin" {
		canEdit = true
	} else if poll.AuthorID == userID {
		// L'auteur peut modifier son propre sondage
		canEdit = true
	} else if userRole == "group_admin" {
		// Group admin peut modifier les sondages ciblant ses groupes administrés
		managedGroupIDs := middleware.GetManagedGroupIDs(c)

		if len(poll.TargetGroups) > 0 {
			for _, targetGroup := range poll.TargetGroups {
				for _, managedID := range managedGroupIDs {
					if targetGroup.ID == managedID {
						canEdit = true
						break
					}
				}
				if canEdit {
					break
				}
			}
		}
	}

	if !canEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to edit this poll"})
		return
	}

	var req models.PollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vérifier les permissions pour les nouveaux groupes cibles (group_admin)
	if userRole == "group_admin" && len(req.TargetGroupIDs) > 0 {
		managedGroupIDs := middleware.GetManagedGroupIDs(c)
		for _, targetGroupID := range req.TargetGroupIDs {
			canManage := false
			for _, managedID := range managedGroupIDs {
				if targetGroupID == managedID {
					canManage = true
					break
				}
			}
			if !canManage {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Vous ne pouvez cibler que les groupes que vous administrez",
				})
				return
			}
		}
	}

	// Mise à jour des champs du sondage
	poll.Title = req.Title
	poll.Description = req.Description
	poll.PollType = req.PollType
	poll.IsAnonymous = req.IsAnonymous
	poll.ShowResults = req.ShowResults
	poll.StartDate = req.StartDate
	poll.EndDate = req.EndDate
	poll.NewsID = req.NewsID
	poll.AnnouncementID = req.AnnouncementID

	// Seul admin peut activer/désactiver
	if userRole == "admin" || poll.AuthorID == userID {
		poll.IsActive = req.IsActive
	}

	// Sauvegarder
	if err := h.db.Save(&poll).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update poll"})
		return
	}

	// Mettre à jour les options (supprimer les anciennes, créer les nouvelles)
	// Note : cette approche simple supprime et recrée les options
	// Pour une approche plus sophistiquée, on comparerait les IDs
	h.db.Where("poll_id = ?", poll.ID).Delete(&models.PollOption{})
	for i, optReq := range req.Options {
		option := models.PollOption{
			PollID: poll.ID,
			Text:   optReq.Text,
			Order:  i,
		}
		h.db.Create(&option)
	}

	// Mettre à jour les groupes cibles
	if req.TargetGroupIDs != nil {
		var groups []models.Group
		h.db.Where("id IN ?", req.TargetGroupIDs).Find(&groups)
		h.db.Model(&poll).Association("TargetGroups").Replace(groups)
	}

	// Recharger avec les relations
	h.db.Preload("Author").
		Preload("TargetGroups").
		Preload("Options").
		Preload("News").
		Preload("Announcement").
		First(&poll, poll.ID)

	c.JSON(http.StatusOK, poll)
}

// DeletePoll - Supprimer un sondage (soft delete)
func (h *PollsHandler) DeletePoll(c *gin.Context) {
	id := c.Param("id")

	var poll models.Poll
	if err := h.db.Preload("TargetGroups").First(&poll, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Poll not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch poll"})
		}
		return
	}

	// Vérifier les permissions
	userID := c.GetUint("user_id")
	userRole := c.GetString("role")

	canDelete := false
	if userRole == "admin" {
		canDelete = true
	} else if poll.AuthorID == userID {
		canDelete = true
	} else if userRole == "group_admin" {
		managedGroupIDs := middleware.GetManagedGroupIDs(c)

		if len(poll.TargetGroups) > 0 {
			for _, targetGroup := range poll.TargetGroups {
				for _, managedID := range managedGroupIDs {
					if targetGroup.ID == managedID {
						canDelete = true
						break
					}
				}
				if canDelete {
					break
				}
			}
		}
	}

	if !canDelete {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to delete this poll"})
		return
	}

	if err := h.db.Delete(&poll).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete poll"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Poll deleted successfully"})
}

// ClosePoll - Fermer un sondage (désactiver)
func (h *PollsHandler) ClosePoll(c *gin.Context) {
	id := c.Param("id")

	var poll models.Poll
	if err := h.db.First(&poll, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Poll not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch poll"})
		}
		return
	}

	// Vérifier les permissions (admin ou auteur)
	userID := c.GetUint("user_id")
	userRole := c.GetString("role")

	if userRole != "admin" && poll.AuthorID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to close this poll"})
		return
	}

	poll.IsActive = false
	if err := h.db.Save(&poll).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close poll"})
		return
	}

	c.JSON(http.StatusOK, poll)
}

// Vote - Voter pour un sondage
func (h *PollsHandler) Vote(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("user_id")

	var req models.PollVoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vérifier que le sondage existe et est actif
	var poll models.Poll
	if err := h.db.Preload("Options").Preload("TargetGroups").First(&poll, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Poll not found"})
		return
	}

	if !poll.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This poll is closed"})
		return
	}

	// Vérifier la période de vote
	now := time.Now()
	if poll.StartDate != nil && now.Before(*poll.StartDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This poll has not started yet"})
		return
	}
	if poll.EndDate != nil && now.After(*poll.EndDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This poll has ended"})
		return
	}

	// Vérifier que l'utilisateur a accès au sondage
	hasAccess := false
	if len(poll.TargetGroups) == 0 {
		hasAccess = true // Sondage global
	} else {
		// Récupérer les groupes administrés ET les groupes d'appartenance
		var managedGroupIDs []uint
		h.db.Table("group_admins").Where("user_id = ?", userID).Pluck("group_id", &managedGroupIDs)

		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		// Combiner les deux listes
		allGroupIDs := make(map[uint]bool)
		for _, id := range userGroupIDs {
			allGroupIDs[id] = true
		}
		for _, id := range managedGroupIDs {
			allGroupIDs[id] = true
		}

		for _, targetGroup := range poll.TargetGroups {
			if allGroupIDs[targetGroup.ID] {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this poll"})
		return
	}

	// Vérifier que les options existent et appartiennent au sondage
	var validOptions []models.PollOption
	if err := h.db.Where("poll_id = ? AND id IN ?", poll.ID, req.PollOptionIDs).Find(&validOptions).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid poll options"})
		return
	}

	if len(validOptions) != len(req.PollOptionIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Some poll options are invalid"})
		return
	}

	// Vérifier le type de sondage
	if poll.PollType == "single" && len(req.PollOptionIDs) > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This poll allows only one choice"})
		return
	}

	// Vérifier si l'utilisateur a déjà voté
	var existingVotes []models.PollVote
	h.db.Where("poll_id = ? AND user_id = ?", poll.ID, userID).Find(&existingVotes)

	if len(existingVotes) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already voted in this poll"})
		return
	}

	// Créer les nouveaux votes
	votesCreated := 0
	for _, optionID := range req.PollOptionIDs {
		vote := models.PollVote{
			PollID:       poll.ID,
			UserID:       userID,
			PollOptionID: optionID,
			VotedAt:      time.Now(),
		}
		if err := h.db.Create(&vote).Error; err != nil {
			log.Printf("[Vote] Erreur lors de la création du vote: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record vote"})
			return
		}
		votesCreated++
	}

	if votesCreated == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record vote"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Vote recorded successfully",
		"votes_count": votesCreated,
		"poll_id":     poll.ID,
		"option_ids":  req.PollOptionIDs,
	})
}

// GetPollResults - Récupérer les résultats d'un sondage
func (h *PollsHandler) GetPollResults(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("user_id")
	userRole := c.GetString("role")

	var poll models.Poll
	if err := h.db.Preload("Author").
		Preload("TargetGroups").
		Preload("Options").
		First(&poll, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Poll not found"})
		return
	}

	// Vérifier que l'utilisateur a accès au sondage
	hasAccess := false
	if userRole == "admin" || poll.AuthorID == userID {
		hasAccess = true
	} else if len(poll.TargetGroups) == 0 {
		hasAccess = true
	} else {
		// Récupérer les groupes administrés ET les groupes d'appartenance
		var managedGroupIDs []uint
		h.db.Table("group_admins").Where("user_id = ?", userID).Pluck("group_id", &managedGroupIDs)

		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		// Combiner les deux listes
		allGroupIDs := make(map[uint]bool)
		for _, id := range userGroupIDs {
			allGroupIDs[id] = true
		}
		for _, id := range managedGroupIDs {
			allGroupIDs[id] = true
		}

		for _, targetGroup := range poll.TargetGroups {
			if allGroupIDs[targetGroup.ID] {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this poll"})
		return
	}

	// Vérifier si l'utilisateur peut voir les résultats selon ShowResults
	var userVotes []models.PollVote
	h.db.Where("poll_id = ? AND user_id = ?", poll.ID, userID).Find(&userVotes)
	userVoted := len(userVotes) > 0

	canSeeResults := false
	if poll.ShowResults == "always" {
		canSeeResults = true
	} else if poll.ShowResults == "after" && userVoted {
		canSeeResults = true
	} else if poll.ShowResults == "closed" && !poll.IsActive {
		canSeeResults = true
	}

	// Admin et auteur peuvent toujours voir les résultats
	if userRole == "admin" || poll.AuthorID == userID {
		canSeeResults = true
	}

	if !canSeeResults {
		c.JSON(http.StatusForbidden, gin.H{"error": "Results not available yet"})
		return
	}

	// Compter les votes totaux et les votants uniques
	var totalVotes int64
	var uniqueVoters int64
	h.db.Model(&models.PollVote{}).Where("poll_id = ?", poll.ID).Count(&totalVotes)
	h.db.Model(&models.PollVote{}).Where("poll_id = ?", poll.ID).Distinct("user_id").Count(&uniqueVoters)

	// Calculer les résultats par option
	var optionResults []models.PollOptionResult
	for _, option := range poll.Options {
		var voteCount int64
		h.db.Model(&models.PollVote{}).Where("poll_option_id = ?", option.ID).Count(&voteCount)

		percentage := 0.0
		if uniqueVoters > 0 {
			percentage = (float64(voteCount) / float64(uniqueVoters)) * 100
		}

		optionResults = append(optionResults, models.PollOptionResult{
			PollOption: option,
			VoteCount:  voteCount,
			Percentage: percentage,
		})
	}

	// Récupérer les votes de l'utilisateur
	var userVoteIDs []uint
	for _, vote := range userVotes {
		userVoteIDs = append(userVoteIDs, vote.PollOptionID)
	}

	// Détails des votants (seulement si non anonyme ET admin/auteur)
	var voterDetails []models.PollVoterDetail
	if !poll.IsAnonymous && (userRole == "admin" || poll.AuthorID == userID) {
		var votes []models.PollVote
		h.db.Preload("User").Where("poll_id = ?", poll.ID).Find(&votes)
		for _, vote := range votes {
			voterDetails = append(voterDetails, models.PollVoterDetail{
				User:         vote.User,
				PollOptionID: vote.PollOptionID,
				VotedAt:      vote.VotedAt,
			})
		}
	}

	response := models.PollResultsResponse{
		Poll:         poll,
		TotalVotes:   totalVotes,
		UniqueVoters: uniqueVoters,
		Options:      optionResults,
		UserVoted:    userVoted,
		UserVotes:    userVoteIDs,
		VoterDetails: voterDetails,
	}

	c.JSON(http.StatusOK, response)
}

// GetAnalytics - Statistiques des sondages (admin)
func (h *PollsHandler) GetAnalytics(c *gin.Context) {
	var stats models.PollStatsResponse

	// Total sondages
	h.db.Model(&models.Poll{}).Count(&stats.TotalPolls)

	// Sondages actifs
	h.db.Model(&models.Poll{}).Where("is_active = ?", true).Count(&stats.ActivePolls)

	// Sondages fermés
	h.db.Model(&models.Poll{}).Where("is_active = ?", false).Count(&stats.ClosedPolls)

	// Total votes
	h.db.Model(&models.PollVote{}).Count(&stats.TotalVotes)

	// Total votants uniques
	h.db.Model(&models.PollVote{}).Distinct("user_id").Count(&stats.TotalVoters)

	// Top 5 sondages (par nombre de votes)
	var topPolls []models.Poll
	h.db.Preload("Author").
		Order("created_at DESC").
		Limit(5).
		Find(&topPolls)

	stats.TopPolls = make([]models.PollWithStats, len(topPolls))
	for i, poll := range topPolls {
		var voteCount int64
		var voterCount int64
		h.db.Model(&models.PollVote{}).Where("poll_id = ?", poll.ID).Count(&voteCount)
		h.db.Model(&models.PollVote{}).Where("poll_id = ?", poll.ID).Distinct("user_id").Count(&voterCount)
		stats.TopPolls[i] = models.PollWithStats{
			Poll:       poll,
			VoteCount:  voteCount,
			VoterCount: voterCount,
		}
	}

	// Sondages récents
	h.db.Preload("Author").
		Order("created_at DESC").
		Limit(10).
		Find(&stats.RecentPolls)

	c.JSON(http.StatusOK, stats)
}
