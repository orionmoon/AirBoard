import { defineStore } from 'pinia';
import api from '@/services/api';
import { useAuthStore } from './auth';

export const useChatStore = defineStore('chat', {
  state: () => ({
    socket: null,
    isConnected: false,
    reconnectAttempts: 0,
    
    // UI State
    isOpen: false,
    isMinimized: false,
    
    // Data
    contacts: {
      users: [],
      groups: []
    },
    activeConversation: null, // { type: 'user'|'group', id: 1, name: '...' }
    messages: {}, // Map: "user_1" -> [msgs], "group_2" -> [msgs]
    
    // Unread
    unreadCounts: {}, // "user_1" -> 5
  }),

  actions: {
    toggleChat() {
      this.isOpen = !this.isOpen;
      if (this.isOpen && !this.isConnected) {
        this.connect();
      }
      if (this.isOpen) {
        this.fetchContacts();
      }
    },

    openConversation(type, item) {
      this.activeConversation = {
        type, // 'user' or 'group'
        id: item.id,
        name: type === 'user' ? (item.first_name ? `${item.first_name} ${item.last_name}` : item.username) : item.name,
        avatar: item.avatar_url || item.icon,
        ...item
      };
      
      // Load history if needed
      this.loadHistory(type, item.id);
      
      // Reset unread
      const key = `${type}_${item.id}`;
      this.unreadCounts[key] = 0;
      
      this.isOpen = true;
    },
    
    closeConversation() {
      this.activeConversation = null;
    },

    async fetchContacts() {
      try {
        const response = await api.get('/chat/contacts');
        this.contacts = response.data; // { users: [], groups: [] }
      } catch (error) {
        console.error('Error fetching contacts:', error);
      }
    },

    async loadHistory(type, id) {
      const key = `${type}_${id}`;
      if (this.messages[key] && this.messages[key].length > 0) return; // Already loaded?
      
      try {
        const params = type === 'user' ? { target_id: id } : { group_id: id };
        const response = await api.get('/chat/history', { params });
        
        // Reverse to have oldest first (if backend returns newest first)
        // Backend returns created_at desc. We want asc for chat view implementation usually.
        this.messages[key] = response.data.reverse(); 
      } catch (error) {
        console.error('Error loading history:', error);
      }
    },

    connect() {
      if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
        return;
      }

      const authStore = useAuthStore();
      const token = authStore.token;
      
      if (!token) return;

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const host = window.location.host; // e.g. localhost:8080 or domain.com
      // Note: If dev server proxies /api, it might not proxy /ws. 
      // Assumption: backend is on same host/port or Nginx handles it.
      // If backend is on 8080 and frontend 3000, we need backend URL.
      // API Base URL usually handled in api.js.
      // Let's deduce from API_URL if possible or env.
      
      // Simple Hack for Dev: if on localhost:5173 or 3000 (Vite), assume backend 8080
      let wsUrl = `${protocol}//${host}/api/v1/ws?token=${token}`;
      
      // Check environment variable (VITE_API_URL from api.js)
      if (import.meta.env.VITE_API_URL) {
         try {
             // Handle if VITE_API_URL is absolute
             if (import.meta.env.VITE_API_URL.startsWith('http')) {
                 const url = new URL(import.meta.env.VITE_API_URL);
                 const wsProtocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
                 // If API_URL includes /api/v1, we should respect it or append /ws logic.
                 // Usually VITE_API_URL is base like http://localhost:8080/api/v1
                 // So we just replace protocol and append /ws.
                 // BUT if VITE_API_URL is just http://localhost:8080, we need /api/v1/ws.
                 // Let's assume VITE_API_URL points to /api/v1 usually.
                 // Safest is to use the full path constructed in api.js context logic.
                 
                 // However, let's look at api.js: baseURL is import.meta.env.VITE_API_URL || '/api/v1'
                 // Docs say default is /api/v1 relative.
                 
                 // If VITE_API_URL is fully qualified (e.g. http://api.app.com/api/v1), 
                 // then we replace protocol.
                 let path = url.pathname;
                 if (path.endsWith('/')) path = path.slice(0, -1);
                 
                 wsUrl = `${wsProtocol}//${url.host}${path}/ws?token=${token}`;
             }
         } catch (e) {
             console.error("Error parsing VITE_API_URL", e);
         }
      } else if (window.location.hostname === 'localhost' && (window.location.port === '5173' || window.location.port === '3000')) {
         // Force backend port 8080 for dev environment if no env var set
         wsUrl = `ws://localhost:8080/api/v1/ws?token=${token}`;
      }

      console.log('Connecting to Chat WS:', wsUrl);

      this.socket = new WebSocket(wsUrl);

      this.socket.onopen = () => {
        console.log('Chat Connected');
        this.isConnected = true;
        this.reconnectAttempts = 0;
      };

      this.socket.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          this.handleIncomingMessage(msg);
        } catch (e) {
          console.error('WS Parse Error:', e);
        }
      };

      this.socket.onclose = () => {
        console.log('Chat Disconnected');
        this.isConnected = false;
        
        // Auto Reconnect with backoff
        if (this.reconnectAttempts < 5) {
          const timeout = Math.pow(2, this.reconnectAttempts) * 1000;
          this.reconnectAttempts++;
          setTimeout(() => this.connect(), timeout);
        }
      };
      
      this.socket.onerror = (err) => {
        console.error('WS Error:', err);
      };
    },
    
    handleIncomingMessage(msg) {
      if (msg.type === 'chat_message') {
        const payload = msg.payload;
        // Identify conversation key
        const authStore = useAuthStore();
        const myId = authStore.user.id;
        
        let key = '';
        
        if (payload.group_id) {
          key = `group_${payload.group_id}`;
        } else {
          // It's a DM.
          // If I am sender, valid key is recipient.
          // If I am recipient, valid key is sender.
          const otherId = payload.sender_id === myId ? payload.recipient_id : payload.sender_id;
          key = `user_${otherId}`;
        }
        
        if (!this.messages[key]) {
          this.messages[key] = [];
        }
        
        this.messages[key].push(payload);

        // Determine the active conversation key
        const activeKey = this.activeConversation
          ? `${this.activeConversation.type}_${this.activeConversation.id}`
          : null;

        // Increment unread if:
        // 1. Message is not from me (don't count our own messages as unread), AND
        // 2. (Chat is not open, OR no active conversation, OR message from different conversation)
        const isMyMessage = payload.sender_id === myId;
        const shouldIncrement = !isMyMessage && (!this.isOpen || !activeKey || activeKey !== key);

        if (shouldIncrement) {
          if (!this.unreadCounts[key]) this.unreadCounts[key] = 0;
          this.unreadCounts[key]++;
        }
      } else if (msg.type === 'user_status') {
        // Handle online/offline status update
        // Find user in contacts and update status
        const userId = msg.payload.user_id;
        const isOnline = msg.payload.is_online;
        
        const user = this.contacts.users.find(u => u.id === userId);
        if (user) {
          user.is_online = isOnline;
        }
      }
    },

    sendMessage(content) {
      if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
        console.error('Socket not connected');
        return;
      }
      
      const toSend = {
        content,
        // Add recipients
      };
      
      if (this.activeConversation.type === 'user') {
        toSend.recipient_id = this.activeConversation.id;
      } else {
        toSend.group_id = this.activeConversation.id;
      }
      
      this.socket.send(JSON.stringify(toSend));
    },

    async deleteMessage(msgId) {
      try {
        await api.delete(`/chat/messages/${msgId}`);
        // Remove locally
        for (const key in this.messages) {
           this.messages[key] = this.messages[key].filter(m => m.id !== msgId);
        }
      } catch (error) {
        console.error('Error deleting message:', error);
      }
    },

    async clearHistory() {
      if (!this.activeConversation) return;

      const type = this.activeConversation.type;
      const id = this.activeConversation.id;
      const params = type === 'user' ? { target_id: id } : { group_id: id };

      try {
        await api.delete('/chat/history', { params });
        // Clear locally
        const key = type === 'user' ? `user_${id}` : `group_${id}`;
        this.messages[key] = [];
      } catch (error) {
        console.error('Error clearing history:', error);
      }
    },
    
    disconnect() {
      if (this.socket) {
        this.socket.close();
        this.socket = null;
        this.isConnected = false;
      }
    }
  }
});
