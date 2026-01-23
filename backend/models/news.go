package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

// News représente un article du News Hub
type News struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Slug        string     `json:"slug" gorm:"not null;uniqueIndex:idx_news_slug,where:deleted_at IS NULL"`
	Title       string     `json:"title" gorm:"not null"`
	Summary     string     `json:"summary" gorm:"type:varchar(300)"` // Résumé court (max 300 chars)
	Content     string     `json:"content" gorm:"type:text"`         // Contenu riche (JSON Tiptap)
	CoverImage  string     `json:"cover_image"`                      // URL de l'image de couverture (pour plus tard)
	Type        string     `json:"type" gorm:"default:'article'"`    // article, tutorial, announcement, faq
	Priority    string     `json:"priority" gorm:"default:'normal'"` // urgent, important, normal
	IsPinned    bool       `json:"is_pinned" gorm:"default:false"`
	IsPublished bool       `json:"is_published" gorm:"default:false"`
	PublishedAt *time.Time `json:"published_at"`
	ExpiresAt   *time.Time `json:"expires_at"` // Auto-archivage après cette date
	ViewCount   int        `json:"view_count" gorm:"default:0"`
	ReadingTime int        `json:"reading_time"` // Temps de lecture estimé (minutes)

	// Relations
	AuthorID   uint          `json:"author_id"`
	Author     User          `json:"author" gorm:"foreignKey:AuthorID"`
	CategoryID *uint         `json:"category_id"`
	Category   *NewsCategory `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
	Tags       []Tag         `json:"tags" gorm:"many2many:news_tags;"`

	// Groupes cibles (visibilité)
	TargetGroups []Group `json:"target_groups" gorm:"many2many:news_target_groups;"`
	// Si vide, visible par tous

	// Reactions
	Reactions []NewsReaction `json:"reactions,omitempty" gorm:"foreignKey:NewsID"`

	// Compteurs calculés (non persistés)
	CommentCount  int `json:"comment_count" gorm:"-"`
	ReactionCount int `json:"reaction_count" gorm:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// NewsCategory représente une catégorie de news
type NewsCategory struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"not null;uniqueIndex"`
	Slug        string         `json:"slug" gorm:"not null;uniqueIndex:idx_category_slug,where:deleted_at IS NULL"`
	Description string         `json:"description"`
	Icon        string         `json:"icon" gorm:"default:'mdi:newspaper'"` // Icône Iconify
	Color       string         `json:"color" gorm:"default:'#3B82F6'"`      // Couleur hex
	Order       int            `json:"order" gorm:"default:0"`              // Ordre d'affichage
	IsActive    bool           `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// Tag représente un tag/étiquette
type Tag struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"not null;uniqueIndex:idx_tag_name,where:deleted_at IS NULL"`
	Slug      string         `json:"slug" gorm:"not null;uniqueIndex:idx_tag_slug,where:deleted_at IS NULL"`
	Color     string         `json:"color" gorm:"default:'#6B7280'"` // Couleur hex
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// NewsReaction représente une réaction emoji sur une news
type NewsReaction struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	NewsID       uint      `json:"news_id"`
	UserID       uint      `json:"user_id"`
	ReactionType string    `json:"reaction_type"` // thumbs_up, heart, celebrate
	CreatedAt    time.Time `json:"created_at"`

	// Relations
	News News `json:"-" gorm:"foreignKey:NewsID"`
	User User `json:"-" gorm:"foreignKey:UserID"`

	// Index composite pour éviter les doublons (un user = une réaction par news)
	// sera géré en contrainte unique
}

// NewsRead permet de tracker qui a lu quoi (pour plus tard)
type NewsRead struct {
	ID     uint      `json:"id" gorm:"primaryKey"`
	NewsID uint      `json:"news_id"`
	UserID uint      `json:"user_id"`
	ReadAt time.Time `json:"read_at"`

	// Relations
	News News `json:"-" gorm:"foreignKey:NewsID"`
	User User `json:"-" gorm:"foreignKey:UserID"`
}

// NewsRequest pour la création/modification de news
type NewsRequest struct {
	Title          string     `json:"title" binding:"required"`
	Summary        string     `json:"summary"`
	Content        string     `json:"content"`
	CoverImage     string     `json:"cover_image"`
	Type           string     `json:"type"`
	Priority       string     `json:"priority"`
	IsPinned       bool       `json:"is_pinned"`
	IsPublished    bool       `json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CategoryID     *uint      `json:"category_id"`
	TagIDs         []uint     `json:"tag_ids"`          // IDs des tags
	TargetGroupIDs []uint     `json:"target_group_ids"` // IDs des groupes cibles
}

// CategoryRequest pour la création/modification de catégories
type CategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Order       int    `json:"order"`
	IsActive    bool   `json:"is_active"`
}

// TagRequest pour la création/modification de tags
type TagRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

// ReactionRequest pour ajouter une réaction
type ReactionRequest struct {
	ReactionType string `json:"reaction_type" binding:"required,oneof=thumbs_up heart celebrate"`
}

// NewsListResponse pour les listes avec pagination
type NewsListResponse struct {
	News       []News `json:"news"`
	Total      int64  `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
}

// NewsStatsResponse pour les statistiques
type NewsStatsResponse struct {
	TotalNews       int64            `json:"total_news"`
	PublishedNews   int64            `json:"published_news"`
	DraftNews       int64            `json:"draft_news"`
	TotalViews      int64            `json:"total_views"`
	TotalReactions  int64            `json:"total_reactions"`
	TopNews         []NewsWithStats  `json:"top_news"`
	ReactionsByType map[string]int64 `json:"reactions_by_type"`
}

// NewsWithStats pour les news avec stats supplémentaires
type NewsWithStats struct {
	News
	ReactionCount int64 `json:"reaction_count"`
}

// TableName spécifie le nom de la table pour NewsReaction
func (NewsReaction) TableName() string {
	return "news_reactions"
}

// TableName spécifie le nom de la table pour NewsRead
func (NewsRead) TableName() string {
	return "news_reads"
}

// BeforeSave hook pour générer le slug automatiquement
func (n *News) BeforeSave(tx *gorm.DB) error {
	if n.Slug == "" {
		baseSlug := generateSlug(n.Title)
		slug := baseSlug

		// Vérifier l'unicité du slug (seulement pour les enregistrements non supprimés)
		var count int64
		for i := 1; ; i++ {
			tx.Model(&News{}).Where("slug = ? AND deleted_at IS NULL AND id != ?", slug, n.ID).Count(&count)
			if count == 0 {
				break
			}
			// Si le slug existe déjà, ajouter un suffixe numérique
			slug = fmt.Sprintf("%s-%d", baseSlug, i)
		}

		n.Slug = slug
	}
	return nil
}

// BeforeSave hook pour générer le slug automatiquement
func (c *NewsCategory) BeforeSave(tx *gorm.DB) error {
	if c.Slug == "" {
		c.Slug = generateSlug(c.Name)
	}
	return nil
}

// BeforeSave hook pour générer le slug automatiquement
func (t *Tag) BeforeSave(tx *gorm.DB) error {
	if t.Slug == "" {
		t.Slug = generateSlug(t.Name)
	}
	return nil
}

// generateSlug génère un slug URL-friendly
func generateSlug(s string) string {
	// Convertir en minuscules
	slug := strings.ToLower(s)

	// Remplacer les espaces et caractères spéciaux par des tirets
	slug = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, slug)

	// Supprimer les tirets multiples
	re := regexp.MustCompile(`-+`)
	slug = re.ReplaceAllString(slug, "-")

	// Supprimer les tirets au début et à la fin
	slug = strings.Trim(slug, "-")

	// Limiter la longueur
	if len(slug) > 100 {
		slug = slug[:100]
	}

	return slug
}
