package handlers

import (
	"log"
	"net/http"
	"strconv"

	"airboard/models"
	"airboard/services/chat"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChatHandler struct {
	db  *gorm.DB
	hub *chat.Hub
}

func NewChatHandler(db *gorm.DB, hub *chat.Hub) *ChatHandler {
	return &ChatHandler{db: db, hub: hub}
}

// ServeWS handles WebSocket requests from the peer.
func (h *ChatHandler) ServeWS(c *gin.Context) {
	// Authentication is handled via Query Param ?token=... because WS doesn't support headers well in standard JS API
	// However, usually we can use cookies or validated headers before.
	// Here, we assume the middleware or logic handles extracting the user.
	// If standard Gin middleware is used, the token is in the header, but standard JS WebSocket object cannot set headers.
	// For simplicity in MVP, we might rely on the cookie if present, or query param.
	// Let's assume the user IS authenticated by the middleware (via query param extraction if needed).

	// Getting user from context (set by middleware)
	userID := c.GetUint("user_id")
	if userID == 0 {
		// Try to get from query param if middleware failed (e.g. strict header check)
		// This requires a custom middleware adaptation, but for now let's assume standard auth worked
		// or implementing a fallback if standard Middleware checks headers only.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	conn, err := chat.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &chat.Client{
		Hub:    h.hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		DB:     h.db,
		UserID: userID,
	}

	client.Hub.Register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.WritePump()
	go client.ReadPump()
}

// GetContacts returns list of users (DMs) and groups
func (h *ChatHandler) GetContacts(c *gin.Context) {
	userID := c.GetUint("user_id")

	var users []models.User
	var groups []models.Group

	// 1. Get all active users (excluding self)
	// Optimization: Only users in same groups or all users depending on privacy policy
	// For MVP: All active users
	h.db.Where("id != ? AND is_active = ?", userID, true).
		Select("id, username, first_name, last_name, avatar_url, role").
		Preload("Groups", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, name, color")
		}).
		Find(&users)

	// 2. Get user's groups
	h.db.Joins("JOIN user_groups on user_groups.group_id = groups.id").
		Where("user_groups.user_id = ?", userID).
		Find(&groups)

	c.JSON(http.StatusOK, gin.H{
		"users":  users,
		"groups": groups,
	})
}

// GetHistory returns chat history for a specific conversation
func (h *ChatHandler) GetHistory(c *gin.Context) {
	userID := c.GetUint("user_id")
	targetID, _ := strconv.Atoi(c.Query("target_id"))
	groupID, _ := strconv.Atoi(c.Query("group_id"))

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var messages []models.ChatMessage
	query := h.db.Model(&models.ChatMessage{}).
		Preload("Sender", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, username, first_name, last_name, avatar_url")
		}).
		Order("created_at desc").
		Limit(limit).
		Offset(offset)

	if groupID > 0 {
		// Group Chat
		query = query.Where("group_id = ?", groupID)
	} else if targetID > 0 {
		// Direct Message: (Sender = Me AND Recipient = Target) OR (Sender = Target AND Recipient = Me)
		log.Printf("[Chat] GetHistory DM: UserID=%d, TargetID=%d", userID, targetID)
		// Explicitly use string query to avoid ambiguous OR
		query = query.Where(
			"(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)",
			userID, targetID, targetID, userID,
		)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_id or group_id required"})
		return
	}

	query.Find(&messages)

	// Reverse order for frontend (oldest first) ? Or keep desc and frontend reverses it
	// keeping desc is better for pagination from most recent.

	c.JSON(http.StatusOK, messages)
}

// DeleteMessage deletes a single message (soft delete)
func (h *ChatHandler) DeleteMessage(c *gin.Context) {
	userID := c.GetUint("user_id")
	msgID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	var msg models.ChatMessage
	// Check content existence and ownership
	if err := h.db.First(&msg, msgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if msg.SenderID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own messages"})
		return
	}

	// Soft delete
	if err := h.db.Delete(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting message"})
		return
	}

	// Ideally execute a WS broadcast here to notify others removal, but for MVP local removal + refresh works
	// Or we can manually construct a "message_deleted" ws event.

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted"})
}

// ClearConversation deletes all messages in a conversation
func (h *ChatHandler) ClearConversation(c *gin.Context) {
	userID := c.GetUint("user_id")
	targetID, _ := strconv.Atoi(c.Query("target_id"))
	groupID, _ := strconv.Atoi(c.Query("group_id"))

	if groupID > 0 {
		// Usually only admins can clear a group chat? Or anyone?
		// For safety, let's say only group admins.
		// Check if user is admin of group
		var count int64
		h.db.Table("group_admins").Where("user_id = ? AND group_id = ?", userID, groupID).Count(&count)
		if count == 0 {
			// Also allow if user is global admin?
			// Let's keep it simple: Only group admin or system admin.
			// Currently strict: Group Admin.
			c.JSON(http.StatusForbidden, gin.H{"error": "Only group admins can clear group chat"})
			return
		}

		if err := h.db.Where("group_id = ?", groupID).Delete(&models.ChatMessage{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error clearing conversation"})
			return
		}
	} else if targetID > 0 {
		// DM: Delete all messages between these two users
		// NOTE: This usually deletes for BOTH.
		// A better approach for DM is "Hide History" (set a 'deleted_by_user_id' flag array),
		// but `Delete` removes it physically (soft delete) for everyone.
		// If user wants to delete THEIR copy, it's more complex.
		// Assuming user wants to "Delete All" messages (like "Clear Chat" in many apps).
		// We delete messages where (Sender=Me AND Recipient=Target) OR (Sender=Target AND Recipient=Me)

		// For simplicity/MVP: We execute soft delete on records.
		// WARNING: This deletes for the OTHER person too if we share the same record.
		// To do it properly like WhatsApp "Delete for me", we'd need a join table or array of "deleted_for".
		// But if the request is "supprimer tt les messages", often "Delete for everyone" is implied or just clearing "my view".

		// Let's implement "Delete for everyone" logic for now as it fits the single table model without migration.
		// Or strictly delete "my sent messages".

		// Let's delete ALL messages between them.
		h.db.Where(
			"(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)",
			userID, targetID, targetID, userID,
		).Delete(&models.ChatMessage{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_id or group_id required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conversation cleared"})
}
