package middleware

import (
	"net/http"

	"airboard/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequireGroupAdmin vérifie que l'utilisateur est admin d'au moins un groupe
func (am *AuthMiddleware) RequireGroupAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Unauthorized",
				Message: "Authentication requise",
				Code:    http.StatusUnauthorized,
			})
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "Forbidden",
				Message: "Rôle invalide",
				Code:    http.StatusForbidden,
			})
			c.Abort()
			return
		}

		// Vérifier si l'utilisateur est admin d'au moins un groupe
		managedGroupIDsInterface, exists := c.Get("managed_group_ids")
		var managedGroupIDs []uint
		if exists {
			if mgids, ok := managedGroupIDsInterface.([]uint); ok {
				managedGroupIDs = mgids
			}
		}

		if len(managedGroupIDs) == 0 {
			// Si l'utilisateur n'est admin d'aucun groupe, il faut qu'il soit admin global
			if roleStr != "admin" {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Error:   "Forbidden",
					Message: "Rôle Admin ou Admin de groupe requis",
					Code:    http.StatusForbidden,
				})
				c.Abort()
				return
			}
		} else {
			// L'utilisateur est admin d'au moins un groupe, donc il peut accéder
			// Vérifier que ce n'est pas un utilisateur inactif ou non autorisé
			if roleStr == "" || roleStr == "banned" {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Error:   "Forbidden",
					Message: "Rôle Admin ou Admin de groupe requis",
					Code:    http.StatusForbidden,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// CanManageGroup vérifie si l'utilisateur peut gérer un groupe spécifique
func CanManageGroup(c *gin.Context, groupID uint) bool {
	role := c.GetString("role")
	if role == "admin" {
		return true // Admin global peut tout gérer
	}

	managedGroupIDsInterface, exists := c.Get("managed_group_ids")
	if !exists {
		return false
	}

	managedGroupIDs, ok := managedGroupIDsInterface.([]uint)
	if !ok {
		return false
	}

	for _, id := range managedGroupIDs {
		if id == groupID {
			return true
		}
	}

	return false
}

// GetManagedGroupIDs retourne les IDs des groupes gérés (helper)
func GetManagedGroupIDs(c *gin.Context) []uint {
	managedGroupIDsInterface, exists := c.Get("managed_group_ids")
	if !exists {
		return []uint{}
	}

	managedGroupIDs, ok := managedGroupIDsInterface.([]uint)
	if !ok {
		return []uint{}
	}

	return managedGroupIDs
}

// CanManageAppGroup vérifie si un utilisateur peut gérer un AppGroup spécifique
// Un utilisateur peut administrer un AppGroup uniquement si :
// - L'AppGroup est privé (IsPrivate = true)
// - ET l'AppGroup est lié à l'un des groupes qu'il administre (via group_app_groups)
// Les AppGroups publics (IsPrivate = false) ne peuvent être administrés que par l'admin global
func CanManageAppGroupWithDB(c *gin.Context, appGroupID uint, db interface{}) bool {
	role := c.GetString("role")
	if role == "admin" {
		return true // Admin global peut tout gérer
	}

	managedGroupIDs := GetManagedGroupIDs(c)
	if len(managedGroupIDs) == 0 {
		return false
	}

	// Cast vers *gorm.DB
	gormDB, ok := db.(*gorm.DB)
	if !ok {
		return false
	}

	// Vérifier si l'AppGroup est privé et lié à un des groupes administrés
	var count int64
	gormDB.Table("app_groups").
		Joins("JOIN group_app_groups ON group_app_groups.app_group_id = app_groups.id").
		Where("app_groups.id = ? AND app_groups.is_private = ? AND group_app_groups.group_id IN ? AND app_groups.deleted_at IS NULL",
			appGroupID, true, managedGroupIDs).
		Count(&count)

	return count > 0
}
