package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"airboard/config"
	"airboard/middleware"
	"airboard/models"
	"airboard/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NewsHandler struct {
	db     *gorm.DB
	config *config.Config
}

func NewNewsHandler(db *gorm.DB, cfg *config.Config) *NewsHandler {
	return &NewsHandler{db: db, config: cfg}
}

// GetNews - Liste des news (accessible à tous les utilisateurs connectés)
func (h *NewsHandler) GetNews(c *gin.Context) {
	var news []models.News

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	offset := (page - 1) * pageSize

	// Construction de la requête
	query := h.db.Model(&models.News{}).
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("Reactions").
		Preload("TargetGroups")

	log.Printf("[DEBUG GetNews] Starting query for role=%s, userID=%d, path=%s", c.GetString("role"), c.GetUint("user_id"), c.Request.URL.Path)

	// Explicit soft delete filter (complex WHERE clauses may bypass GORM's automatic filter)
	query = query.Where("deleted_at IS NULL")

	// Filtres
	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	if priority := c.Query("priority"); priority != "" {
		query = query.Where("priority = ?", priority)
	}

	if newsType := c.Query("type"); newsType != "" {
		query = query.Where("type = ?", newsType)
	}

	// Filtre par tags (supporte plusieurs tags séparés par des virgules)
	if tags := c.Query("tags"); tags != "" {
		tagIDs := strings.Split(tags, ",")
		if len(tagIDs) > 0 {
			query = query.Joins("JOIN news_tags ON news_tags.news_id = news.id").
				Where("news_tags.tag_id IN ?", tagIDs).
				Group("news.id").
				Having("COUNT(DISTINCT news_tags.tag_id) = ?", len(tagIDs))
		}
	} else if tagID := c.Query("tag_id"); tagID != "" {
		// Garde la compatibilité avec l'ancien paramètre tag_id
		query = query.Joins("JOIN news_tags ON news_tags.news_id = news.id").
			Where("news_tags.tag_id = ?", tagID)
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

		query = query.Where("title ILIKE ? OR summary ILIKE ?", "%"+sanitizedSearch+"%", "%"+sanitizedSearch+"%")
	}

	// Filtre published only et visibilité par groupes selon le rôle
	userRole := c.GetString("role")
	userID := c.GetUint("user_id")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)

	if userRole == "admin" {
		// Admin voit tout (publié + brouillons)
	} else if len(managedGroupIDs) > 0 {
		// Group admin (utilisateur qui administre au moins un groupe) voit UNIQUEMENT dans l'interface d'administration :
		// 1. Ses propres articles (publiés + brouillons)
		// 2. Les articles (publiés + brouillons) ciblant les groupes qu'il administre
		// Note : il ne voit PAS les articles publics globaux qu'il ne peut pas gérer

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

		// Si le group_admin accède via /group-admin/news, on filtre strictement
		// Sinon (lecture publique via /news), il voit les news publiques comme un user normal
		isAdminInterface := strings.HasPrefix(c.Request.URL.Path, "/api/v1/group-admin/news")

		if isAdminInterface {
			// Interface d'administration : seulement les news qu'il peut gérer
			if len(managedGroupIDs) > 0 {
				query = query.Where(`
					(author_id = ?) OR
					EXISTS (
						SELECT 1 FROM news_target_groups
						WHERE news_target_groups.news_id = news.id
						AND news_target_groups.group_id IN (?)
					)
				`, userID, managedGroupIDs)
			} else {
				// Pas de groupes gérés : uniquement ses propres articles
				query = query.Where("author_id = ?", userID)
			}
		} else {
			// Interface publique : voir ses brouillons + news publiées globales + celles de ses groupes (appartenance + administration)
			if len(combinedGroupIDs) > 0 {
				query = query.Where(`
					(author_id = ?) OR
					(is_published = ? AND (published_at IS NULL OR published_at <= ?) AND (
						(SELECT COUNT(*) FROM news_target_groups WHERE news_target_groups.news_id = news.id) = 0 OR
						EXISTS (
							SELECT 1 FROM news_target_groups
							WHERE news_target_groups.news_id = news.id
							AND news_target_groups.group_id IN (?)
						)
					))
				`, userID, true, time.Now(), combinedGroupIDs)
			} else {
				// Pas de groupes : ses brouillons + news publiques globales
				query = query.Where(`
					(author_id = ?) OR
					(is_published = ? AND (published_at IS NULL OR published_at <= ?) AND
						(SELECT COUNT(*) FROM news_target_groups WHERE news_target_groups.news_id = news.id) = 0
					)
				`, userID, true, time.Now())
			}
		}
	} else if userRole == "editor" {
		// Editor voit : news publiques + ses propres brouillons
		query = query.Where("(is_published = ? AND (published_at IS NULL OR published_at <= ?)) OR author_id = ?",
			true, time.Now(), userID)
	} else {
		// User régulier voit : news publiques + news ciblant ses groupes
		query = query.Where("is_published = ?", true).
			Where("(published_at IS NULL OR published_at <= ?)", time.Now())

		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		if len(userGroupIDs) > 0 {
			query = query.Where(`
				(SELECT COUNT(*) FROM news_target_groups WHERE news_target_groups.news_id = news.id) = 0
				OR EXISTS (
					SELECT 1 FROM news_target_groups
					WHERE news_target_groups.news_id = news.id
					AND news_target_groups.group_id IN (?)
				)
			`, userGroupIDs)
		} else {
			// Si pas de groupes, voir seulement les news globales
			query = query.Where("(SELECT COUNT(*) FROM news_target_groups WHERE news_target_groups.news_id = news.id) = 0")
		}
	}

	// Tri
	sortBy := c.DefaultQuery("sort", "published_at")
	sortOrder := c.DefaultQuery("order", "desc")
	if sortBy == "pinned" {
		query = query.Order("is_pinned DESC, published_at DESC")
	} else {
		query = query.Order(sortBy + " " + sortOrder)
	}

	// Compte total
	var total int64
	query.Count(&total)

	// Récupération avec pagination
	if err := query.Offset(offset).Limit(pageSize).Find(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		return
	}

	// Calcul du nombre de pages
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	log.Printf("[DEBUG GetNews] Returning %d news (total=%d, page=%d, totalPages=%d)", len(news), total, page, totalPages)
	for i, n := range news {
		log.Printf("[DEBUG GetNews] News[%d]: ID=%d, Title=%s, DeletedAt=%v", i, n.ID, n.Title, n.DeletedAt)
	}

	c.JSON(http.StatusOK, models.NewsListResponse{
		News:       news,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// GetNewsBySlug - Récupérer une news par son slug
func (h *NewsHandler) GetNewsBySlug(c *gin.Context) {
	slug := c.Param("slug")

	var news models.News
	if err := h.db.Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("Reactions").
		Preload("TargetGroups").
		Where("slug = ?", slug).
		First(&news).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "News not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		}
		return
	}

	// Vérifier les permissions selon le rôle
	userRole := c.GetString("role")
	userID := c.GetUint("user_id")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)

	if userRole == "admin" {
		// Admin voit tout
		c.JSON(http.StatusOK, news)
		return
	}

	// Vérifier si publié
	if !news.IsPublished {
		// Seul l'auteur ou un editor/admin de groupe peut voir un brouillon
		if (userRole == "editor" || len(managedGroupIDs) > 0) && news.AuthorID == userID {
			c.JSON(http.StatusOK, news)
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "News not published"})
		return
	}

	// Article publié : vérifier les groupes cibles
	// Si pas de groupes cibles, c'est une news globale (accessible à tous)
	if len(news.TargetGroups) == 0 {
		c.JSON(http.StatusOK, news)
		return
	}

	// Récupérer les groupes de l'utilisateur (membership)
	var userGroupIDs []uint
	h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

	// Vérifier l'accès : l'utilisateur doit être membre OU admin d'au moins un groupe cible
	hasAccess := false
	for _, targetGroup := range news.TargetGroups {
		// Vérifier si l'utilisateur est membre du groupe cible
		for _, userGroupID := range userGroupIDs {
			if targetGroup.ID == userGroupID {
				hasAccess = true
				break
			}
		}
		if hasAccess {
			break
		}

		// Vérifier si l'utilisateur administre le groupe cible (pour les group_admin)
		for _, managedID := range managedGroupIDs {
			if targetGroup.ID == managedID {
				hasAccess = true
				break
			}
		}
		if hasAccess {
			break
		}
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this news"})
		return
	}

	c.JSON(http.StatusOK, news)
}

// CreateNews - Créer une news (admin/editor uniquement)
func (h *NewsHandler) CreateNews(c *gin.Context) {
	var req models.NewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Récupérer l'ID de l'utilisateur connecté
	userID := c.GetUint("user_id")

	news := models.News{
		Title:       req.Title,
		Summary:     req.Summary,
		Content:     req.Content,
		CoverImage:  req.CoverImage,
		Type:        req.Type,
		Priority:    req.Priority,
		IsPinned:    req.IsPinned,
		IsPublished: req.IsPublished,
		PublishedAt: req.PublishedAt,
		ExpiresAt:   req.ExpiresAt,
		CategoryID:  req.CategoryID,
		AuthorID:    userID,
	}

	// Si publié sans date, mettre la date actuelle
	if news.IsPublished && news.PublishedAt == nil {
		now := time.Now()
		news.PublishedAt = &now
	}

	// Créer la news
	if err := h.db.Create(&news).Error; err != nil {
		log.Printf("[ERROR CreateNews] Failed to create news: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create news", "details": err.Error()})
		return
	}

	// Associer les tags
	if len(req.TagIDs) > 0 {
		var tags []models.Tag
		h.db.Where("id IN ?", req.TagIDs).Find(&tags)
		h.db.Model(&news).Association("Tags").Replace(tags)
	}

	// Associer les groupes cibles avec vérification pour group_admin
	if len(req.TargetGroupIDs) > 0 {
		userRole := c.GetString("role")
		managedGroupIDs := middleware.GetManagedGroupIDs(c)

		// Si admin de groupe (non-admin global), vérifier qu'il ne cible que ses groupes
		if userRole != "admin" && len(managedGroupIDs) > 0 {
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
					// Supprimer la news créée
					h.db.Delete(&news)
					return
				}
			}
		}

		var groups []models.Group
		h.db.Where("id IN ?", req.TargetGroupIDs).Find(&groups)
		h.db.Model(&news).Association("TargetGroups").Replace(groups)
	}

	// Recharger avec les relations
	h.db.Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("TargetGroups").
		First(&news, news.ID)

	// Envoyer une notification email si l'article est publié
	if news.IsPublished {
		go func() {
			log.Printf("[Email] Tentative d'envoi de notification pour news ID=%d, titre='%s'", news.ID, news.Title)
			// Utiliser la configuration stockée pour éviter de régénérer le secret JWT
			emailService := services.NewEmailService(h.db, h.config)
			var targetGroupIDs []uint
			for _, g := range news.TargetGroups {
				targetGroupIDs = append(targetGroupIDs, g.ID)
			}
			log.Printf("[Email] Groupes cibles pour news %d: %v", news.ID, targetGroupIDs)
			if err := emailService.SendNotification("news", news.ID, targetGroupIDs); err != nil {
				log.Printf("[Email] ❌ ÉCHEC notification news ID=%d: %v", news.ID, err)
			} else {
				log.Printf("[Email] ✅ Notification envoyée avec succès pour news ID=%d", news.ID)
			}
		}()

		// Créer des notifications pour les utilisateurs
		go func() {
			notifService := services.NewNotificationService(h.db)

			// Récupérer les utilisateurs à notifier
			var userIDs []uint
			if len(news.TargetGroups) > 0 {
				// Notifier les membres des groupes cibles
				var groupIDs []uint
				for _, g := range news.TargetGroups {
					groupIDs = append(groupIDs, g.ID)
				}
				h.db.Table("user_groups").
					Where("group_id IN ?", groupIDs).
					Distinct("user_id").
					Pluck("user_id", &userIDs)
			} else {
				// Article global : notifier tous les utilisateurs actifs
				h.db.Model(&models.User{}).
					Where("is_active = ?", true).
					Where("id != ?", userID). // Ne pas notifier l'auteur
					Pluck("id", &userIDs)
			}

			// Envoyer la notification
			if len(userIDs) > 0 {
				authorName := news.Author.FirstName + " " + news.Author.LastName
				if err := notifService.NotifyNewArticle(news.Title, news.Slug, authorName, userIDs); err != nil {
					log.Printf("[Notification] Échec de l'envoi de la notification: %v", err)
				}
			}
		}()
	}

	c.JSON(http.StatusCreated, news)
}

// UpdateNews - Modifier une news
func (h *NewsHandler) UpdateNews(c *gin.Context) {
	slug := c.Param("id") // Le paramètre est en fait un slug

	var news models.News
	if err := h.db.Where("slug = ?", slug).First(&news).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "News not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		}
		return
	}

	// Vérifier les permissions
	// - admin peut tout modifier
	// - editor/group_admin peut modifier ses propres news
	// - admin de groupe peut modifier les news ciblant les groupes qu'il administre
	userID := c.GetUint("user_id")
	userRole := c.GetString("role")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)

	canEdit := false
	if userRole == "admin" {
		canEdit = true
	} else if news.AuthorID == userID {
		// L'auteur peut modifier sa propre news
		canEdit = true
	} else if len(managedGroupIDs) > 0 {
		// Admin de groupe peut modifier les news ciblant ses groupes administrés
		// Charger les groupes cibles de la news
		var newsWithGroups models.News
		h.db.Preload("TargetGroups").First(&newsWithGroups, news.ID)

		// Si la news n'a pas de groupes cibles, elle est publique → admin de groupe ne peut pas la modifier
		if len(newsWithGroups.TargetGroups) > 0 {
			// Vérifier si au moins un groupe cible est administré par l'admin de groupe
			for _, targetGroup := range newsWithGroups.TargetGroups {
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
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to edit this news"})
		return
	}

	var req models.NewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Mise à jour des champs
	news.Title = req.Title
	news.Summary = req.Summary
	news.Content = req.Content
	news.CoverImage = req.CoverImage
	news.Type = req.Type
	news.Priority = req.Priority
	news.CategoryID = req.CategoryID
	news.ExpiresAt = req.ExpiresAt

	// Seul admin peut épingler
	if userRole == "admin" {
		news.IsPinned = req.IsPinned
	}

	// Gestion de la publication
	wasPublished := news.IsPublished
	news.IsPublished = req.IsPublished
	if news.IsPublished && !wasPublished && news.PublishedAt == nil {
		now := time.Now()
		news.PublishedAt = &now
	}
	if req.PublishedAt != nil {
		news.PublishedAt = req.PublishedAt
	}

	// Sauvegarder
	if err := h.db.Save(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update news"})
		return
	}

	// Mettre à jour les tags
	if req.TagIDs != nil {
		var tags []models.Tag
		h.db.Where("id IN ?", req.TagIDs).Find(&tags)
		h.db.Model(&news).Association("Tags").Replace(tags)
	}

	// Mettre à jour les groupes cibles avec vérification pour les admins de groupe
	if req.TargetGroupIDs != nil {
		// Si admin de groupe (non-admin global), vérifier qu'il ne cible que ses groupes
		if userRole != "admin" && len(managedGroupIDs) > 0 && len(req.TargetGroupIDs) > 0 {
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

		var groups []models.Group
		h.db.Where("id IN ?", req.TargetGroupIDs).Find(&groups)
		h.db.Model(&news).Association("TargetGroups").Replace(groups)
	}

	// Recharger avec les relations
	h.db.Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("TargetGroups").
		First(&news, news.ID)

	c.JSON(http.StatusOK, news)
}

// DeleteNews - Supprimer une news (soft delete)
func (h *NewsHandler) DeleteNews(c *gin.Context) {
	id := c.Param("id")

	var news models.News
	if err := h.db.First(&news, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "News not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		}
		return
	}

	// Vérifier les permissions
	// - admin peut tout supprimer
	// - editor/group_admin peut supprimer ses propres news
	// - admin de groupe peut supprimer les news ciblant les groupes qu'il administre
	userID := c.GetUint("user_id")
	userRole := c.GetString("role")
	managedGroupIDs := middleware.GetManagedGroupIDs(c)

	canDelete := false
	if userRole == "admin" {
		canDelete = true
	} else if news.AuthorID == userID {
		// L'auteur peut supprimer sa propre news
		canDelete = true
	} else if len(managedGroupIDs) > 0 {
		// Admin de groupe peut supprimer les news ciblant ses groupes administrés
		// Charger les groupes cibles de la news
		var newsWithGroups models.News
		h.db.Preload("TargetGroups").First(&newsWithGroups, news.ID)

		// Si la news n'a pas de groupes cibles, elle est publique → admin de groupe ne peut pas la supprimer
		if len(newsWithGroups.TargetGroups) > 0 {
			// Vérifier si au moins un groupe cible est administré par l'admin de groupe
			for _, targetGroup := range newsWithGroups.TargetGroups {
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
		log.Printf("[DEBUG DeleteNews] Permission denied for user %d (role=%s) to delete news %d", userID, userRole, news.ID)
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to delete this news"})
		return
	}

	log.Printf("[DEBUG DeleteNews] Deleting news ID=%d by user %d (role=%s)", news.ID, userID, userRole)
	if err := h.db.Delete(&news).Error; err != nil {
		log.Printf("[ERROR DeleteNews] Failed to delete news %d: %v", news.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete news"})
		return
	}

	log.Printf("[DEBUG DeleteNews] Successfully deleted news ID=%d", news.ID)
	c.JSON(http.StatusOK, gin.H{"message": "News deleted successfully"})
}

// TogglePin - Épingler/désépingler une news (admin uniquement)
func (h *NewsHandler) TogglePin(c *gin.Context) {
	id := c.Param("id")

	var news models.News
	if err := h.db.Preload("TargetGroups").First(&news, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "News not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		}
		return
	}

	wasNotPinned := !news.IsPinned
	news.IsPinned = !news.IsPinned

	if err := h.db.Save(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update news"})
		return
	}

	// Si l'article est épinglé et publié, envoyer des notifications
	if news.IsPinned && wasNotPinned && news.IsPublished {
		go func() {
			notifService := services.NewNotificationService(h.db)

			// Récupérer les utilisateurs à notifier
			var userIDs []uint
			if len(news.TargetGroups) > 0 {
				// Notifier les membres des groupes cibles
				var groupIDs []uint
				for _, g := range news.TargetGroups {
					groupIDs = append(groupIDs, g.ID)
				}
				h.db.Table("user_groups").
					Where("group_id IN ?", groupIDs).
					Distinct("user_id").
					Pluck("user_id", &userIDs)
			} else {
				// Article global : notifier tous les utilisateurs actifs
				h.db.Model(&models.User{}).
					Where("is_active = ?", true).
					Pluck("id", &userIDs)
			}

			// Envoyer la notification
			if len(userIDs) > 0 {
				if err := notifService.NotifyPinnedArticle(news.Title, news.Slug, userIDs); err != nil {
					log.Printf("[Notification] Échec de l'envoi de la notification d'épinglage: %v", err)
				}
			}
		}()
	}

	c.JSON(http.StatusOK, news)
}

// IncrementView - Incrémenter le compteur de vues
func (h *NewsHandler) IncrementView(c *gin.Context) {
	id := c.Param("id")

	if err := h.db.Model(&models.News{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to increment view"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "View counted"})
}

// AddReaction - Ajouter une réaction à une news
func (h *NewsHandler) AddReaction(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("user_id")

	var req models.ReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vérifier si la news existe
	var news models.News
	if err := h.db.First(&news, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "News not found"})
		return
	}

	// Vérifier si l'utilisateur a déjà réagi
	var existingReaction models.NewsReaction
	err := h.db.Where("news_id = ? AND user_id = ?", id, userID).First(&existingReaction).Error

	if err == nil {
		// Déjà une réaction, la mettre à jour
		existingReaction.ReactionType = req.ReactionType
		if err := h.db.Save(&existingReaction).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reaction"})
			return
		}
		c.JSON(http.StatusOK, existingReaction)
		return
	}

	// Créer une nouvelle réaction
	newsID, _ := strconv.ParseUint(id, 10, 32)
	reaction := models.NewsReaction{
		NewsID:       uint(newsID),
		UserID:       userID,
		ReactionType: req.ReactionType,
	}

	if err := h.db.Create(&reaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reaction"})
		return
	}

	c.JSON(http.StatusCreated, reaction)
}

// RemoveReaction - Retirer une réaction
func (h *NewsHandler) RemoveReaction(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("user_id")

	result := h.db.Where("news_id = ? AND user_id = ?", id, userID).Delete(&models.NewsReaction{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove reaction"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reaction not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reaction removed"})
}

// GetReactions - Récupérer les réactions d'une news avec compteurs
func (h *NewsHandler) GetReactions(c *gin.Context) {
	id := c.Param("id")

	var reactions []models.NewsReaction
	if err := h.db.Where("news_id = ?", id).Find(&reactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reactions"})
		return
	}

	// Compter par type
	reactionCounts := make(map[string]int)
	for _, reaction := range reactions {
		reactionCounts[reaction.ReactionType]++
	}

	// Vérifier si l'utilisateur a réagi
	userID := c.GetUint("user_id")
	var userReaction *models.NewsReaction
	for _, reaction := range reactions {
		if reaction.UserID == userID {
			userReaction = &reaction
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"counts":        reactionCounts,
		"total":         len(reactions),
		"user_reaction": userReaction,
	})
}

// GetUnreadCount - Nombre de news non lues (pour le badge)
func (h *NewsHandler) GetUnreadCount(c *gin.Context) {
	// Compter les news publiées depuis la dernière visite
	// Pour MVP simple: toutes les news publiées dans les 30 derniers jours
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	var count int64
	h.db.Model(&models.News{}).
		Where("is_published = ?", true).
		Where("published_at >= ?", thirtyDaysAgo).
		Count(&count)

	// TODO: Implémenter le système de tracking de lecture (NewsRead)
	// Pour le moment, on retourne juste le compte des news récentes

	c.JSON(http.StatusOK, gin.H{
		"unread_count": count,
	})
}

// GetAnalytics - Statistiques du News Hub (admin/editor)
func (h *NewsHandler) GetAnalytics(c *gin.Context) {
	var stats models.NewsStatsResponse

	// Total news
	h.db.Model(&models.News{}).Count(&stats.TotalNews)

	// News publiées
	h.db.Model(&models.News{}).Where("is_published = ?", true).Count(&stats.PublishedNews)

	// News en brouillon
	h.db.Model(&models.News{}).Where("is_published = ?", false).Count(&stats.DraftNews)

	// Total vues
	h.db.Model(&models.News{}).Select("COALESCE(SUM(view_count), 0)").Scan(&stats.TotalViews)

	// Total réactions
	h.db.Model(&models.NewsReaction{}).Count(&stats.TotalReactions)

	// Top 5 news (par vues)
	var topNews []models.News
	h.db.Preload("Author").
		Preload("Category").
		Order("view_count DESC").
		Limit(5).
		Find(&topNews)

	// Convertir en NewsWithStats avec compteur de réactions
	stats.TopNews = make([]models.NewsWithStats, len(topNews))
	for i, news := range topNews {
		var reactionCount int64
		h.db.Model(&models.NewsReaction{}).Where("news_id = ?", news.ID).Count(&reactionCount)
		stats.TopNews[i] = models.NewsWithStats{
			News:          news,
			ReactionCount: reactionCount,
		}
	}

	// Réactions par type
	rows, err := h.db.Model(&models.NewsReaction{}).
		Select("reaction_type, COUNT(*) as count").
		Group("reaction_type").
		Rows()

	if err == nil {
		defer rows.Close()
		stats.ReactionsByType = make(map[string]int64)
		for rows.Next() {
			var reactionType string
			var count int64
			rows.Scan(&reactionType, &count)
			stats.ReactionsByType[reactionType] = count
		}
	}

	c.JSON(http.StatusOK, stats)
}
