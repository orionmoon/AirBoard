package handlers

import (
	"fmt"
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

type EventsHandler struct {
	db *gorm.DB
}

func NewEventsHandler(db *gorm.DB) *EventsHandler {
	return &EventsHandler{db: db}
}

// GetEvents - Liste des événements (accessible à tous les utilisateurs connectés)
func (h *EventsHandler) GetEvents(c *gin.Context) {
	var events []models.Event

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	offset := (page - 1) * pageSize

	// Construction de la requête
	query := h.db.Model(&models.Event{}).
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("TargetGroups")

	// Explicit soft delete filter (complex WHERE clauses may bypass GORM's automatic filter)
	query = query.Where("deleted_at IS NULL")

	// Filtres
	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	if priority := c.Query("priority"); priority != "" {
		query = query.Where("priority = ?", priority)
	}

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// Filtre par tags (supporte plusieurs tags séparés par des virgules)
	if tags := c.Query("tags"); tags != "" {
		tagIDs := strings.Split(tags, ",")
		if len(tagIDs) > 0 {
			query = query.Joins("JOIN event_tags ON event_tags.event_id = events.id").
				Where("event_tags.tag_id IN ?", tagIDs).
				Group("events.id").
				Having("COUNT(DISTINCT event_tags.tag_id) = ?", len(tagIDs))
		}
	}

	// Filtre par plage de dates
	if startDate := c.Query("start_date"); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("start_date >= ?", parsed)
		}
	}

	if endDate := c.Query("end_date"); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			// Inclure les événements qui commencent avant end_date
			query = query.Where("start_date <= ?", parsed)
		}
	}

	// Filtre événements à venir / passés
	if upcoming := c.Query("upcoming"); upcoming == "true" {
		query = query.Where("(end_date IS NULL AND start_date >= ?) OR (end_date IS NOT NULL AND end_date >= ?)",
			time.Now(), time.Now())
	} else if past := c.Query("past"); past == "true" {
		query = query.Where("(end_date IS NOT NULL AND end_date < ?) OR (end_date IS NULL AND start_date < ?)",
			time.Now(), time.Now())
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

		query = query.Where("title ILIKE ? OR description ILIKE ? OR location ILIKE ?",
			"%"+sanitizedSearch+"%", "%"+sanitizedSearch+"%", "%"+sanitizedSearch+"%")
	}

	// Filtre published only et visibilité par groupes selon le rôle
	userRole := c.GetString("role")
	userID := c.GetUint("user_id")

	if userRole == "admin" {
		// Admin voit tout (publié + brouillons)
	} else if userRole == "group_admin" {
		managedGroupIDs := middleware.GetManagedGroupIDs(c)
		isAdminInterface := strings.HasPrefix(c.Request.URL.Path, "/api/v1/group-admin/events")

		if isAdminInterface {
			// Interface d'administration : événements qu'il peut gérer
			if len(managedGroupIDs) > 0 {
				query = query.Where(`
					(author_id = ?) OR
					EXISTS (
						SELECT 1 FROM event_target_groups
						WHERE event_target_groups.event_id = events.id
						AND event_target_groups.group_id IN (?)
					)
				`, userID, managedGroupIDs)
			} else {
				query = query.Where("author_id = ?", userID)
			}
		} else {
			// Interface publique : événements publiés publics + ceux de ses groupes
			query = query.Where(`
				(is_published = ? AND (published_at IS NULL OR published_at <= ?) AND (
					(SELECT COUNT(*) FROM event_target_groups WHERE event_target_groups.event_id = events.id) = 0 OR
					EXISTS (
						SELECT 1 FROM event_target_groups
						WHERE event_target_groups.event_id = events.id
						AND event_target_groups.group_id IN (?)
					)
				))
			`, true, time.Now(), managedGroupIDs)
		}
	} else if userRole == "editor" {
		// Editor voit : événements publiques + ses propres brouillons
		query = query.Where("(is_published = ? AND (published_at IS NULL OR published_at <= ?)) OR author_id = ?",
			true, time.Now(), userID)
	} else {
		// User régulier voit : événements publiques + événements ciblant ses groupes
		query = query.Where("is_published = ?", true).
			Where("(published_at IS NULL OR published_at <= ?)", time.Now())

		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		if len(userGroupIDs) > 0 {
			query = query.Where(`
				(SELECT COUNT(*) FROM event_target_groups WHERE event_target_groups.event_id = events.id) = 0
				OR EXISTS (
					SELECT 1 FROM event_target_groups
					WHERE event_target_groups.event_id = events.id
					AND event_target_groups.group_id IN (?)
				)
			`, userGroupIDs)
		} else {
			// Si pas de groupes, voir seulement les événements globaux
			query = query.Where("(SELECT COUNT(*) FROM event_target_groups WHERE event_target_groups.event_id = events.id) = 0")
		}
	}

	// Tri
	sortBy := c.DefaultQuery("sort", "start_date")
	sortOrder := c.DefaultQuery("order", "asc")
	if sortBy == "start_date" || sortBy == "created_at" || sortBy == "priority" {
		query = query.Order(sortBy + " " + sortOrder)
	} else {
		query = query.Order("start_date asc")
	}

	// Compter le total
	var total int64
	query.Count(&total)

	// Récupérer les événements avec pagination
	query.Offset(offset).Limit(pageSize).Find(&events)

	// Calculer le nombre total de pages
	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.EventListResponse{
		Events:     events,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// GetCalendarView - Vue calendrier avec événements expandus (CRITIQUE)
func (h *EventsHandler) GetCalendarView(c *gin.Context) {
	// Paramètres de plage de dates (requis)
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date et end_date sont requis (format YYYY-MM-DD)"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format start_date invalide (YYYY-MM-DD requis)"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format end_date invalide (YYYY-MM-DD requis)"})
		return
	}

	// Récupérer tous les événements dans la plage (+ filtres de visibilité)
	var allEvents []models.Event

	query := h.db.Model(&models.Event{}).
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("TargetGroups").
		Where("is_published = ?", true).
		Where("published_at IS NULL OR published_at <= ?", time.Now())

	// Filtrer par plage de dates
	// Inclure les événements qui ont une partie dans la période demandée
	query = query.Where(
		"(start_date <= ? AND (end_date IS NULL OR end_date >= ?)) OR (end_date IS NOT NULL AND end_date >= ? AND start_date <= ?)",
		endDate, startDate, endDate, startDate)

	// Appliquer filtres de visibilité selon rôle
	userRole := c.GetString("role")
	userID := c.GetUint("user_id")

	if userRole == "admin" {
		// Admin voit tout
	} else if userRole == "group_admin" {
		managedGroupIDs := middleware.GetManagedGroupIDs(c)
		if len(managedGroupIDs) > 0 {
			query = query.Where(`
				(SELECT COUNT(*) FROM event_target_groups WHERE event_target_groups.event_id = events.id) = 0
				OR EXISTS (
					SELECT 1 FROM event_target_groups
					WHERE event_target_groups.event_id = events.id
					AND event_target_groups.group_id IN (?)
				)
			`, managedGroupIDs)
		} else {
			query = query.Where("(SELECT COUNT(*) FROM event_target_groups WHERE event_target_groups.event_id = events.id) = 0")
		}
	} else {
		// User régulier : événements publics + ses groupes
		var userGroupIDs []uint
		h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)

		if len(userGroupIDs) > 0 {
			query = query.Where(`
				(SELECT COUNT(*) FROM event_target_groups WHERE event_target_groups.event_id = events.id) = 0
				OR EXISTS (
					SELECT 1 FROM event_target_groups
					WHERE event_target_groups.event_id = events.id
					AND event_target_groups.group_id IN (?)
				)
			`, userGroupIDs)
		} else {
			query = query.Where("(SELECT COUNT(*) FROM event_target_groups WHERE event_target_groups.event_id = events.id) = 0")
		}
	}

	// Filtres optionnels
	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	query.Find(&allEvents)

	// Séparer événements récurrents et non-récurrents
	var normalEvents []models.Event
	var recurringEvents []models.Event

	for _, event := range allEvents {
		if event.IsRecurring {
			recurringEvents = append(recurringEvents, event)
		} else {
			normalEvents = append(normalEvents, event)
		}
	}

	// Expander les événements récurrents
	recurringInstances := services.ExpandRecurringEvents(recurringEvents, startDate, endDate)

	// Calculer le résumé
	publicCount := 0
	privateCount := 0

	for _, event := range allEvents {
		var targetGroupCount int64
		h.db.Model(&event).Association("TargetGroups").Count()
		if targetGroupCount == 0 {
			publicCount++
		} else {
			privateCount++
		}
	}

	summary := models.EventCalendarSummary{
		TotalEvents:    len(allEvents),
		PublicEvents:   publicCount,
		PrivateEvents:  privateCount,
		RecurringCount: len(recurringEvents),
	}

	c.JSON(http.StatusOK, models.EventCalendarResponse{
		Events:             normalEvents,
		RecurringInstances: recurringInstances,
		Summary:            summary,
	})
}

// GetEventBySlug - Détail d'un événement par slug ou ID
func (h *EventsHandler) GetEventBySlug(c *gin.Context) {
	identifier := c.Param("slug")

	var event models.Event
	query := h.db.Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("TargetGroups")

	// Essayer de traiter l'identifiant comme un ID numérique d'abord
	if eventID, err := strconv.Atoi(identifier); err == nil {
		// C'est un ID numérique valide
		query = query.Where("id = ?", eventID)
	} else {
		// Sinon, c'est un slug
		query = query.Where("slug = ?", identifier)
	}

	if err := query.First(&event).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Événement non trouvé"})
		return
	}

	// Vérifier la visibilité selon le rôle
	userRole := c.GetString("role")
	userID := c.GetUint("user_id")

	if userRole != "admin" {
		// Vérifier si l'événement est publié
		if !event.IsPublished {
			// Seul l'auteur peut voir un brouillon
			if event.AuthorID != userID {
				c.JSON(http.StatusForbidden, gin.H{"error": "Accès refusé"})
				return
			}
		}

		// Vérifier la visibilité par groupe
		var targetGroupCount int64
		h.db.Model(&event).Association("TargetGroups").Count()

		if targetGroupCount > 0 {
			// Événement privé : vérifier si l'utilisateur est dans un des groupes cibles
			var userGroupIDs []uint
			if userRole == "group_admin" {
				userGroupIDs = middleware.GetManagedGroupIDs(c)
			} else {
				h.db.Table("user_groups").Where("user_id = ?", userID).Pluck("group_id", &userGroupIDs)
			}

			// Vérifier l'intersection
			hasAccess := false
			for _, targetGroup := range event.TargetGroups {
				for _, userGroupID := range userGroupIDs {
					if targetGroup.ID == userGroupID {
						hasAccess = true
						break
					}
				}
				if hasAccess {
					break
				}
			}

			if !hasAccess && event.AuthorID != userID {
				c.JSON(http.StatusForbidden, gin.H{"error": "Accès refusé"})
				return
			}
		}
	}

	c.JSON(http.StatusOK, event)
}

// GetCategories - Liste des catégories d'événements
func (h *EventsHandler) GetCategories(c *gin.Context) {
	var categories []models.EventCategory

	h.db.Where("is_active = ?", true).Order("\"order\" asc").Find(&categories)

	c.JSON(http.StatusOK, categories)
}
