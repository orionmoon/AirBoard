package models

import (
	"strconv"
	"time"

	"gorm.io/gorm"
)

// Event représente un événement dans le calendrier
type Event struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Slug        string `json:"slug" gorm:"size:255;not null;uniqueIndex:idx_event_slug,where:deleted_at IS NULL"`
	Title       string `json:"title" gorm:"not null;size:255"`
	Description string `json:"description" gorm:"type:text"` // Contenu riche (JSON Tiptap)

	// Dates & Times
	StartDate time.Time  `json:"start_date" gorm:"not null;index:idx_events_date_range"`
	EndDate   *time.Time `json:"end_date" gorm:"index:idx_events_date_range"`
	IsAllDay  bool       `json:"is_all_day" gorm:"default:false"`
	Timezone  string     `json:"timezone" gorm:"size:50;default:'UTC'"` // Format IANA (ex: "Europe/Paris")

	// Recurrence
	IsRecurring          bool       `json:"is_recurring" gorm:"default:false;index"`
	RecurrenceRule       string     `json:"recurrence_rule" gorm:"type:text"` // JSON RecurrencePattern
	RecurrenceEnd        *time.Time `json:"recurrence_end"`
	RecurrenceExceptions string     `json:"recurrence_exceptions" gorm:"type:text"` // JSON array de dates annulées

	// Metadata
	Location      string `json:"location" gorm:"size:500"`
	ExternalLinks string `json:"external_links" gorm:"type:text"`           // JSON [{title, url, icon}]
	Color         string `json:"color" gorm:"size:20;default:'#3B82F6'"`    // Couleur hex pour affichage calendrier
	Priority      string `json:"priority" gorm:"size:20;default:'normal'"`  // urgent, high, normal, low
	Status        string `json:"status" gorm:"size:20;default:'confirmed'"` // confirmed, tentative, cancelled
	CoverImage    string `json:"cover_image" gorm:"size:500"`

	// Publication
	IsPublished bool       `json:"is_published" gorm:"default:false;index"`
	PublishedAt *time.Time `json:"published_at"`

	// Jour férié
	IsHoliday   bool   `json:"is_holiday" gorm:"default:false;index"`
	CountryCode string `json:"country_code" gorm:"size:5"` // Code pays ISO (ex: FR, US, MA)

	// Relations
	AuthorID   uint           `json:"author_id" gorm:"not null;index"`
	Author     User           `json:"author" gorm:"constraint:OnDelete:CASCADE;foreignKey:AuthorID"`
	CategoryID *uint          `json:"category_id"`
	Category   *EventCategory `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
	Tags       []Tag          `json:"tags" gorm:"many2many:event_tags;"`

	// Groupes cibles (visibilité)
	TargetGroups []Group `json:"target_groups" gorm:"many2many:event_target_groups;"`
	// Si vide, visible par tous (événement public)

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// EventCategory représente une catégorie d'événements
type EventCategory struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"size:100;not null;uniqueIndex:idx_event_category_name,where:deleted_at IS NULL"`
	Slug        string         `json:"slug" gorm:"size:100;not null;uniqueIndex:idx_event_category_slug,where:deleted_at IS NULL"`
	Description string         `json:"description" gorm:"size:500"`
	Icon        string         `json:"icon" gorm:"size:100;default:'mdi:calendar'"` // Icône Iconify
	Color       string         `json:"color" gorm:"size:20;default:'#3B82F6'"`      // Couleur hex
	Order       int            `json:"order" gorm:"default:0"`                      // Ordre d'affichage
	IsActive    bool           `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// RecurrencePattern définit la structure JSON pour les règles de récurrence
type RecurrencePattern struct {
	Type            string  `json:"type"`                       // daily, weekly, monthly, yearly
	Interval        int     `json:"interval"`                   // Répéter tous les N jours/semaines/mois/années
	DaysOfWeek      []int   `json:"days_of_week,omitempty"`     // Pour hebdomadaire: 0=Dim, 1=Lun...6=Sam
	DayOfMonth      int     `json:"day_of_month,omitempty"`     // Pour mensuel: 1-31
	EndType         string  `json:"end_type"`                   // never, on_date, after_count
	EndDate         *string `json:"end_date,omitempty"`         // Format YYYY-MM-DD
	OccurrenceCount int     `json:"occurrence_count,omitempty"` // Nombre d'occurrences
}

// EventRequest pour la création/modification d'événements
type EventRequest struct {
	Title                string     `json:"title" binding:"required"`
	Description          string     `json:"description"`
	StartDate            time.Time  `json:"start_date" binding:"required"`
	EndDate              *time.Time `json:"end_date"`
	IsAllDay             bool       `json:"is_all_day"`
	Timezone             string     `json:"timezone"`
	IsRecurring          bool       `json:"is_recurring"`
	RecurrenceRule       string     `json:"recurrence_rule"` // JSON string
	RecurrenceEnd        *time.Time `json:"recurrence_end"`
	RecurrenceExceptions string     `json:"recurrence_exceptions"` // JSON string
	Location             string     `json:"location"`
	ExternalLinks        string     `json:"external_links"` // JSON string
	Color                string     `json:"color"`
	Priority             string     `json:"priority"`
	Status               string     `json:"status"`
	CoverImage           string     `json:"cover_image"`
	IsPublished          bool       `json:"is_published"`
	PublishedAt          *time.Time `json:"published_at"`
	CategoryID           *uint      `json:"category_id"`
	TagIDs               []uint     `json:"tag_ids"`          // IDs des tags
	TargetGroupIDs       []uint     `json:"target_group_ids"` // IDs des groupes cibles
}

// EventCategoryRequest pour la création/modification de catégories d'événements
type EventCategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Order       int    `json:"order"`
	IsActive    bool   `json:"is_active"`
}

// EventListResponse pour les listes avec pagination
type EventListResponse struct {
	Events     []Event `json:"events"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}

// RecurringEventInstance représente une instance d'un événement récurrent
type RecurringEventInstance struct {
	Event        Event     `json:"event"`
	InstanceDate time.Time `json:"instance_date"` // Date de cette instance spécifique
	OriginalDate time.Time `json:"original_date"` // Date de l'événement maître
	IsCancelled  bool      `json:"is_cancelled"`  // True si dans RecurrenceExceptions
}

// EventCalendarResponse pour la vue calendrier avec événements expandus
type EventCalendarResponse struct {
	Events             []Event                  `json:"events"`              // Événements non-récurrents
	RecurringInstances []RecurringEventInstance `json:"recurring_instances"` // Instances expandues
	Summary            EventCalendarSummary     `json:"summary"`
}

// EventCalendarSummary résumé pour la vue calendrier
type EventCalendarSummary struct {
	TotalEvents    int `json:"total_events"`
	PublicEvents   int `json:"public_events"`
	PrivateEvents  int `json:"private_events"`
	RecurringCount int `json:"recurring_count"`
}

// EventStatsResponse pour les statistiques
type EventStatsResponse struct {
	TotalEvents      int64            `json:"total_events"`
	PublishedEvents  int64            `json:"published_events"`
	DraftEvents      int64            `json:"draft_events"`
	UpcomingEvents   int64            `json:"upcoming_events"`
	PastEvents       int64            `json:"past_events"`
	RecurringEvents  int64            `json:"recurring_events"`
	EventsByCategory map[string]int64 `json:"events_by_category"`
	EventsByPriority map[string]int64 `json:"events_by_priority"`
}

// BeforeSave hook pour générer le slug automatiquement
func (e *Event) BeforeSave(tx *gorm.DB) error {
	if e.Slug == "" {
		baseSlug := generateSlug(e.Title)
		e.Slug = e.generateUniqueSlug(tx, baseSlug)
	}
	return nil
}

// generateUniqueSlug génère un slug unique en ajoutant un suffixe si nécessaire
func (e *Event) generateUniqueSlug(tx *gorm.DB, baseSlug string) string {
	var count int64
	slug := baseSlug

	// Vérifier si le slug existe déjà
	for {
		// Compter les événements avec ce slug
		tx.Model(&Event{}).Where("slug = ?", slug).Count(&count)

		// Si aucun événement avec ce slug n'existe, on l'utilise
		if count == 0 {
			return slug
		}

		// Sinon, essayer avec un suffixe numérique
		count++
		slug = baseSlug + "-" + strconv.FormatInt(count, 10)

		// Limiter la longueur du slug (max 100 caractères)
		if len(slug) > 100 {
			// Si le slug avec suffixe dépasse la limite, tronquer le baseSlug
			maxBaseLength := 100 - len("-"+strconv.FormatInt(count, 10)) - 1
			if maxBaseLength > 0 {
				slug = baseSlug[:maxBaseLength] + "-" + strconv.FormatInt(count, 10)
			} else {
				// Cas extrême: si même le suffixe ne tient pas
				slug = "event-" + strconv.FormatInt(count, 10)
			}
		}

		// Éviter une boucle infinie (limite arbitraire)
		if count > 9999 {
			return baseSlug + "-" + strconv.FormatInt(time.Now().Unix(), 10)
		}
	}
}

// BeforeSave hook pour générer le slug automatiquement
func (c *EventCategory) BeforeSave(tx *gorm.DB) error {
	if c.Slug == "" {
		baseSlug := generateSlug(c.Name)
		c.Slug = c.generateUniqueCategorySlug(tx, baseSlug)
	}
	return nil
}

// generateUniqueCategorySlug génère un slug unique pour les catégories
func (c *EventCategory) generateUniqueCategorySlug(tx *gorm.DB, baseSlug string) string {
	var count int64
	slug := baseSlug

	// Vérifier si le slug existe déjà
	for {
		// Compter les catégories avec ce slug
		tx.Model(&EventCategory{}).Where("slug = ?", slug).Count(&count)

		// Si aucune catégorie avec ce slug n'existe, on l'utilise
		if count == 0 {
			return slug
		}

		// Sinon, essayer avec un suffixe numérique
		count++
		slug = baseSlug + "-" + strconv.FormatInt(count, 10)

		// Limiter la longueur du slug (max 100 caractères)
		if len(slug) > 100 {
			// Si le slug avec suffixe dépasse la limite, tronquer le baseSlug
			maxBaseLength := 100 - len("-"+strconv.FormatInt(count, 10)) - 1
			if maxBaseLength > 0 {
				slug = baseSlug[:maxBaseLength] + "-" + strconv.FormatInt(count, 10)
			} else {
				// Cas extrême: si même le suffixe ne tient pas
				slug = "category-" + strconv.FormatInt(count, 10)
			}
		}

		// Éviter une boucle infinie (limite arbitraire)
		if count > 9999 {
			return baseSlug + "-" + strconv.FormatInt(time.Now().Unix(), 10)
		}
	}
}

// TableName spécifie le nom de la table pour Event
func (Event) TableName() string {
	return "events"
}

// TableName spécifie le nom de la table pour EventCategory
func (EventCategory) TableName() string {
	return "event_categories"
}
