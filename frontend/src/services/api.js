import axios from 'axios'

// Configuration de base d'Axios
const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  }
})

// Variables pour gérer le refresh token de manière thread-safe
let isRefreshing = false
let failedQueue = []

const processQueue = (error, token = null) => {
  failedQueue.forEach(prom => {
    if (error) {
      prom.reject(error)
    } else {
      prom.resolve(token)
    }
  })
  failedQueue = []
}

// Logs pour le développement
if (import.meta.env.DEV) {
  api.interceptors.request.use(
    (config) => {
      console.log('🚀', config.method?.toUpperCase(), config.url, {
        data: config.data,
        params: config.params
      })
      return config
    },
    (error) => {
      console.error('❌', 'Request Error:', error)
      return Promise.reject(error)
    }
  )

  api.interceptors.response.use(
    (response) => {
      console.log('✅', response.config.method?.toUpperCase(), response.config.url, response.data)
      return response
    },
    (error) => {
      console.error('❌', error.config?.method?.toUpperCase(), error.config?.url, error.response?.data || error.message)
      return Promise.reject(error)
    }
  )
}

// Configuration des intercepteurs avec authentification
export function setupInterceptors(router, logoutCallback) {
  // Intercepteur de requête pour ajouter le token
  api.interceptors.request.use(
    (config) => {
      const token = localStorage.getItem('airboard_token')
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
        console.log('🔑 Ajout du token Authorization:', config.url)
      } else {
        console.log('⚠️ Aucun token trouvé pour:', config.url)
      }
      return config
    },
    (error) => {
      console.error('❌ Request interceptor error:', error)
      return Promise.reject(error)
    }
  )

  // Intercepteur de réponse pour gérer l'authentification
  api.interceptors.response.use(
    (response) => {
      console.log('✅ Réponse réussie:', response.config.method?.toUpperCase(), response.config.url, response.status)
      return response
    },
    async (error) => {
      const originalRequest = error.config

      console.log('❌ Erreur API:', error.response?.status, originalRequest?.url, error.response?.data)

      // Si erreur 401 et pas déjà une tentative de refresh
      if (error.response?.status === 401 && !originalRequest._retry) {
        // Ignorer les requêtes de refresh qui échouent
        if (originalRequest.url === '/auth/refresh') {
          return Promise.reject(error)
        }

        const refreshToken = localStorage.getItem('airboard_refresh_token')

        // Si pas de refresh token, rediriger vers login
        if (!refreshToken) {
          console.log('⚠️ Pas de refresh token disponible')
          if (logoutCallback) logoutCallback()
          
          if (!window.location.pathname.includes('/auth/')) {
            router.push('/auth/login')
          }
          return Promise.reject(error)
        }

        // Si un refresh est déjà en cours, mettre la requête en queue
        if (isRefreshing) {
          console.log('🔄 Refresh en cours, mise en queue de:', originalRequest.url)
          return new Promise((resolve, reject) => {
            failedQueue.push({ resolve, reject })
          }).then(token => {
            originalRequest.headers.Authorization = `Bearer ${token}`
            return api(originalRequest)
          }).catch(err => {
            return Promise.reject(err)
          })
        }

        originalRequest._retry = true
        isRefreshing = true
        console.log('🔄 Tentative de refresh du token...')

        try {
          console.log('🚀 Appel du refresh token...')
          const response = await api.post('/auth/refresh', {
            refresh_token: refreshToken
          })

          const { token, refresh_token } = response.data
          localStorage.setItem('airboard_token', token)
          localStorage.setItem('airboard_refresh_token', refresh_token)

          console.log('🔄 Token rafraîchi avec succès')

          // Traiter les requêtes en queue
          processQueue(null, token)

          // Refaire la requête originale avec le nouveau token
          originalRequest.headers.Authorization = `Bearer ${token}`
          return api(originalRequest)
        } catch (refreshError) {
          console.error('❌ Refresh token échoué:', refreshError.response?.data || refreshError.message)

          // Rejeter toutes les requêtes en queue
          processQueue(refreshError, null)

          // Refresh failed, redirect to login
          if (logoutCallback) logoutCallback()
          localStorage.removeItem('airboard_token')
          localStorage.removeItem('airboard_refresh_token')
          localStorage.removeItem('airboard_user')

          // Ne rediriger que si on n'est pas déjà sur une page d'auth
          if (!window.location.pathname.includes('/auth/')) {
            router.push('/auth/login')
          }
          return Promise.reject(refreshError)
        } finally {
          isRefreshing = false
        }
      }

      return Promise.reject(error)
    }
  )
}

// Services API

// Auth Service
export const authService = {
  async login(credentials) {
    const response = await api.post('/auth/login', credentials)
    return response.data
  },

  async register(userData) {
    const response = await api.post('/auth/register', userData)
    return response.data
  },

  async refreshToken(refreshToken) {
    const response = await api.post('/auth/refresh', { refresh_token: refreshToken })
    return response.data
  },

  async getProfile() {
    const response = await api.get('/auth/profile')
    return response.data
  },

  async changePassword(oldPassword, newPassword) {
    const response = await api.post('/auth/change-password', {
      old_password: oldPassword,
      new_password: newPassword
    })
    return response.data
  },

  async updateProfile(profileData) {
    const response = await api.put('/auth/profile', profileData)
    return response.data
  },

  async uploadAvatar(file) {
    const formData = new FormData()
    formData.append('avatar', file)
    const response = await api.post('/auth/avatar', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
    return response.data
  },

  async deleteAvatar() {
    const response = await api.delete('/auth/avatar')
    return response.data
  },

  async getSignupStatus() {
    const response = await api.get('/auth/signup/status')
    return response.data
  },

  logout() {
    localStorage.removeItem('airboard_token')
    localStorage.removeItem('airboard_refresh_token')
    localStorage.removeItem('airboard_user')
  },

  async ssoAutoLogin() {
    const response = await api.get('/auth/sso/auto-login')
    return response.data
  }
}

// Dashboard Service
export const dashboardService = {
  async getDashboard() {
    const response = await api.get('/dashboard')
    return response.data
  }
}

// Home Service
export const homeService = {
  async getHomeData() {
    const response = await api.get('/home')
    return response.data
  }
}

// Admin Services
export const adminService = {
  // App Groups
  async getAppGroups(params = {}) {
    const response = await api.get('/admin/app-groups', { params })
    return response.data.data || response.data
  },

  async createAppGroup(data) {
    const response = await api.post('/admin/app-groups', data)
    return response.data
  },

  async updateAppGroup(id, data) {
    const response = await api.put(`/admin/app-groups/${id}`, data)
    return response.data
  },

  async deleteAppGroup(id) {
    const response = await api.delete(`/admin/app-groups/${id}`)
    return response.data
  },

  // Applications
  async getApplications(params = {}) {
    const response = await api.get('/admin/applications', { params })
    return response.data.data || response.data
  },

  async createApplication(data) {
    const response = await api.post('/admin/applications', data)
    return response.data
  },

  async updateApplication(id, data) {
    const response = await api.put(`/admin/applications/${id}`, data)
    return response.data
  },

  async deleteApplication(id) {
    const response = await api.delete(`/admin/applications/${id}`)
    return response.data
  },

  // Users
  async getUsers() {
    const response = await api.get('/admin/users')
    return response.data.data || response.data
  },

  async createUser(data) {
    const response = await api.post('/admin/users', data)
    return response.data
  },

  async updateUser(id, data) {
    const response = await api.put(`/admin/users/${id}`, data)
    return response.data
  },

  async deleteUser(id) {
    const response = await api.delete(`/admin/users/${id}`)
    return response.data
  },

  // Groups
  async getGroups() {
    const response = await api.get('/admin/groups')
    return response.data.data || response.data
  },

  async createGroup(data) {
    const response = await api.post('/admin/groups', data)
    return response.data
  },

  async updateGroup(id, data) {
    const response = await api.put(`/admin/groups/${id}`, data)
    return response.data
  },

  async deleteGroup(id) {
    const response = await api.delete(`/admin/groups/${id}`)
    return response.data
  },

  async getGroupAdmins(groupId) {
    const response = await api.get(`/admin/groups/${groupId}/admins`)
    return response.data
  },

  async assignGroupAdmins(groupId, userIds) {
    const response = await api.put(`/admin/groups/${groupId}/admins`, { user_ids: userIds })
    return response.data
  },

  // App Settings
  async getAppSettings() {
    const response = await api.get('/admin/settings')
    return response.data.data || response.data
  },

  async updateAppSettings(data) {
    const response = await api.put('/admin/settings', data)
    return response.data.data || response.data
  },

  async resetAppSettings() {
    const response = await api.post('/admin/settings/reset')
    return response.data.data || response.data
  },

  // Hero Messages
  async getHeroMessages() {
    const response = await api.get('/admin/settings/hero-messages')
    return response.data.data || response.data
  },

  async createHeroMessage(data) {
    const response = await api.post('/admin/settings/hero-messages', data)
    return response.data
  },

  async updateHeroMessage(id, data) {
    const response = await api.put(`/admin/settings/hero-messages/${id}`, data)
    return response.data
  },

  async deleteHeroMessage(id) {
    const response = await api.delete(`/admin/settings/hero-messages/${id}`)
    return response.data
  },

  // Database
  async resetDatabase() {
    const response = await api.post('/admin/database/reset')
    return response.data
  },

  // OAuth Providers
  async getOAuthProviders() {
    const response = await api.get('/admin/oauth/providers')
    return response.data
  },

  async updateOAuthProvider(id, data) {
    const response = await api.put(`/admin/oauth/providers/${id}`, data)
    return response.data
  },

  // Deleted Users Management
  async getDeletedUsers() {
    const response = await api.get('/admin/users/deleted')
    return response.data
  },

  async restoreUser(id) {
    const response = await api.post(`/admin/users/${id}/restore`)
    return response.data
  },

  async permanentlyDeleteUser(id) {
    const response = await api.delete(`/admin/users/${id}/permanent`)
    return response.data
  }
}

// OAuth Service (public)
export const oauthService = {
  async getEnabledProviders() {
    const response = await api.get('/auth/oauth/providers')
    return response.data
  },

  async initiateOAuth(providerName) {
    const response = await api.get(`/auth/oauth/${providerName}/initiate`)
    return response.data
  },

  async handleCallback(providerName, code, state, nonce) {
    // Utiliser POST au lieu de GET pour éviter les problèmes de proxy/cache en production
    const response = await api.post(`/auth/oauth/${providerName}/callback`, {
      code,
      state,
      nonce
    })
    return response.data
  }
}

// Favorites Service
export const favoritesService = {
  async getFavorites() {
    const response = await api.get('/user/favorites')
    return response.data
  },

  async addFavorite(applicationId) {
    const response = await api.post('/user/favorites', { application_id: applicationId })
    return response.data
  },

  async removeFavorite(applicationId) {
    const response = await api.delete(`/user/favorites/${applicationId}`)
    return response.data
  },

  async isFavorite(applicationId) {
    const response = await api.get(`/user/favorites/${applicationId}/check`)
    return response.data
  }
}

// Analytics Service
export const analyticsService = {
  async trackClick(applicationId) {
    try {
      const response = await api.post('/analytics/track', { application_id: applicationId })
      return response.data
    } catch (error) {
      // Ne pas bloquer l'application si le tracking échoue
      console.error('Error tracking click:', error)
      return null
    }
  },

  async getDashboard() {
    const response = await api.get('/admin/analytics/dashboard')
    return response.data
  },

  async getApplicationStats(applicationId) {
    const response = await api.get(`/admin/analytics/applications/${applicationId}`)
    return response.data
  },

  async getUserStats(userId) {
    const response = await api.get(`/admin/analytics/users/${userId}`)
    return response.data
  }
}

// Announcements Service
export const announcementsService = {
  // Public - Get active announcements for users
  async getActiveAnnouncements() {
    const response = await api.get('/announcements')
    return response.data
  },

  // Admin - Get all announcements
  async getAllAnnouncements() {
    const response = await api.get('/admin/announcements')
    return response.data
  },

  // Admin - Get single announcement
  async getAnnouncement(id) {
    const response = await api.get(`/admin/announcements/${id}`)
    return response.data
  },

  // Admin - Create announcement
  async createAnnouncement(data) {
    const response = await api.post('/admin/announcements', data)
    return response.data
  },

  // Admin - Update announcement
  async updateAnnouncement(id, data) {
    const response = await api.put(`/admin/announcements/${id}`, data)
    return response.data
  },

  // Admin - Delete announcement
  async deleteAnnouncement(id) {
    const response = await api.delete(`/admin/announcements/${id}`)
    return response.data
  }
}

// News Hub Service
export const newsService = {
  // User - Get news with filters and pagination
  async getNews(params = {}) {
    const response = await api.get('/news', { params })
    return response.data
  },

  // User - Get news by slug
  async getNewsBySlug(slug) {
    const response = await api.get(`/news/article/${slug}`)
    return response.data
  },

  // User - Increment view count
  async incrementView(id) {
    const response = await api.post(`/news/${id}/view`)
    return response.data
  },

  // User - Get reactions for a news
  async getReactions(id) {
    const response = await api.get(`/news/${id}/reactions`)
    return response.data
  },

  // User - Add reaction
  async addReaction(id, reactionType) {
    const response = await api.post(`/news/${id}/react`, { reaction_type: reactionType })
    return response.data
  },

  // User - Remove reaction
  async removeReaction(id) {
    const response = await api.delete(`/news/${id}/react`)
    return response.data
  },

  // User - Get unread count
  async getUnreadCount() {
    const response = await api.get('/news/unread/count')
    return response.data
  },

  // User - Get categories
  async getCategories() {
    const response = await api.get('/news/categories')
    return response.data
  },

  // User - Get tags
  async getTags() {
    const response = await api.get('/news/tags')
    return response.data
  },

  // User - Get unread count
  async getUnreadCount() {
    const response = await api.get('/news/unread/count')
    return response.data
  },

  // Editor - Create news
  async createNews(data) {
    const response = await api.post('/editor/news', data)
    return response.data
  },

  // Editor - Update news
  async updateNews(id, data) {
    const response = await api.put(`/editor/news/${id}`, data)
    return response.data
  },

  // Editor - Delete news
  async deleteNews(id) {
    const response = await api.delete(`/editor/news/${id}`)
    return response.data
  },

  // Editor - Create tag
  async createTag(data) {
    const response = await api.post('/editor/news/tags', data)
    return response.data
  },

  // Editor - Update tag
  async updateTag(id, data) {
    const response = await api.put(`/editor/news/tags/${id}`, data)
    return response.data
  },

  // Editor - Delete tag
  async deleteTag(id) {
    const response = await api.delete(`/editor/news/tags/${id}`)
    return response.data
  },

  // Admin - Create category
  async createCategory(data) {
    const response = await api.post('/admin/news/categories', data)
    return response.data
  },

  // Admin - Update category
  async updateCategory(id, data) {
    const response = await api.put(`/admin/news/categories/${id}`, data)
    return response.data
  },

  // Admin - Delete category
  async deleteCategory(id) {
    const response = await api.delete(`/admin/news/categories/${id}`)
    return response.data
  },

  // Admin - Toggle pin
  async togglePin(id) {
    const response = await api.post(`/admin/news/${id}/pin`)
    return response.data
  },

  // Admin - Get analytics
  async getAnalytics() {
    const response = await api.get('/admin/news/analytics')
    return response.data
  }
}

// Group Admin Service
export const groupAdminService = {
  // AppGroups (scoped to managed groups)
  async getAppGroups(params = {}) {
    const response = await api.get('/group-admin/app-groups', { params })
    return response.data.data || response.data
  },

  async createAppGroup(data) {
    const response = await api.post('/group-admin/app-groups', data)
    return response.data
  },

  async updateAppGroup(id, data) {
    const response = await api.put(`/group-admin/app-groups/${id}`, data)
    return response.data
  },

  async deleteAppGroup(id) {
    const response = await api.delete(`/group-admin/app-groups/${id}`)
    return response.data
  },

  // Applications (scoped to managed groups)
  async getApplications(params = {}) {
    const response = await api.get('/group-admin/applications', { params })
    return response.data.data || response.data
  },

  async createApplication(data) {
    const response = await api.post('/group-admin/applications', data)
    return response.data
  },

  async updateApplication(id, data) {
    const response = await api.put(`/group-admin/applications/${id}`, data)
    return response.data
  },

  async deleteApplication(id) {
    const response = await api.delete(`/group-admin/applications/${id}`)
    return response.data
  },

  // News (scoped to managed groups)
  async getNews(params = {}) {
    const response = await api.get('/group-admin/news', { params })
    return response.data
  },

  async createNews(data) {
    const response = await api.post('/group-admin/news', data)
    return response.data
  },

  async updateNews(slugOrId, data) {
    const response = await api.put(`/group-admin/news/${slugOrId}`, data)
    return response.data
  },

  async deleteNews(id) {
    const response = await api.delete(`/group-admin/news/${id}`)
    return response.data
  },

  // Tags
  async createTag(data) {
    const response = await api.post('/group-admin/news/tags', data)
    return response.data
  },

  async updateTag(id, data) {
    const response = await api.put(`/group-admin/news/tags/${id}`, data)
    return response.data
  },

  async deleteTag(id) {
    const response = await api.delete(`/group-admin/news/tags/${id}`)
    return response.data
  },

  // Categories (group admin peut créer/modifier des catégories)
  async createCategory(data) {
    const response = await api.post('/group-admin/news/categories', data)
    return response.data
  },

  async updateCategory(id, data) {
    const response = await api.put(`/group-admin/news/categories/${id}`, data)
    return response.data
  },

  async deleteCategory(id) {
    const response = await api.delete(`/group-admin/news/categories/${id}`)
    return response.data
  },

  // Managed Groups
  async getManagedGroups() {
    const response = await api.get('/group-admin/managed-groups')
    return response.data
  }
}

// Events Service (public routes)
export const eventsService = {
  async getEvents(params = {}) {
    const response = await api.get('/events', { params })
    return response.data
  },

  async getCalendarView(startDate, endDate, params = {}) {
    const response = await api.get('/events/calendar', {
      params: { start_date: startDate, end_date: endDate, ...params }
    })
    return response.data
  },

  async getEventBySlug(slug) {
    const response = await api.get(`/events/${slug}`)
    return response.data
  },

  async getCategories() {
    const response = await api.get('/events/categories')
    return response.data
  }
}

// Gamification Service
export const gamificationService = {
  async getProfile() {
    const response = await api.get('/gamification/profile')
    return response.data
  },

  async getMyAchievements() {
    const response = await api.get('/gamification/achievements')
    return response.data
  },

  async getAllAchievements() {
    const response = await api.get('/gamification/achievements/all')
    return response.data
  },

  async getLeaderboard() {
    const response = await api.get('/gamification/leaderboard')
    return response.data
  },
  
  async getTransactions() {
    const response = await api.get('/gamification/transactions')
    return response.data
  }
}


// Admin Events Service
export const adminEventsService = {
  async getEvents(params = {}) {
    const response = await api.get('/admin/events', { params })
    // API returns EventListResponse structure directly: { events: [], total: 20, page: 1, page_size: 10, total_pages: 2 }
    return response.data
  },

  async createEvent(data) {
    const response = await api.post('/admin/events', data)
    return response.data
  },

  async updateEvent(slug, data) {
    const response = await api.put(`/admin/events/${slug}`, data)
    return response.data
  },

  async deleteEvent(id) {
    const response = await api.delete(`/admin/events/${id}`)
    return response.data
  },

  async getAnalytics() {
    const response = await api.get('/admin/events/analytics')
    return response.data
  },

  // Categories
  async createCategory(data) {
    const response = await api.post('/admin/events/categories', data)
    return response.data
  },

  async updateCategory(id, data) {
    const response = await api.put(`/admin/events/categories/${id}`, data)
    return response.data
  },

  async deleteCategory(id) {
    const response = await api.delete(`/admin/events/categories/${id}`)
    return response.data
  },

  // Holidays
  async getAvailableCountries() {
    const response = await api.get('/admin/events/holidays/countries')
    return response.data
  },

  async previewHolidays(countryCode, year) {
    const response = await api.get('/admin/events/holidays/preview', {
      params: { country_code: countryCode, year }
    })
    return response.data
  },

  async importHolidays(countryCode, year, categoryId = null) {
    const response = await api.post('/admin/events/holidays/import', {
      country_code: countryCode,
      year,
      category_id: categoryId
    })
    return response.data
  },

  async deleteHolidays(countryCode, year) {
    const response = await api.delete('/admin/events/holidays', {
      params: { country_code: countryCode, year }
    })
    return response.data
  }
}

// Editor Events Service
export const editorEventsService = {
  async createEvent(data) {
    const response = await api.post('/editor/events', data)
    return response.data
  },

  async updateEvent(slug, data) {
    const response = await api.put(`/editor/events/${slug}`, data)
    return response.data
  },

  async deleteEvent(id) {
    const response = await api.delete(`/editor/events/${id}`)
    return response.data
  }
}

// Group Admin Events Service
export const groupAdminEventsService = {
  async getEvents(params = {}) {
    const response = await api.get('/group-admin/events', { params })
    return response.data
  },

  async createEvent(data) {
    const response = await api.post('/group-admin/events', data)
    return response.data
  },

  async updateEvent(slug, data) {
    const response = await api.put(`/group-admin/events/${slug}`, data)
    return response.data
  },

  async deleteEvent(id) {
    const response = await api.delete(`/group-admin/events/${id}`)
    return response.data
  }
}

// Email Settings Service
export const emailService = {
  // SMTP Configuration
  async getSMTPConfig() {
    const response = await api.get('/admin/email/smtp')
    return response.data
  },

  async updateSMTPConfig(data) {
    const response = await api.put('/admin/email/smtp', data)
    return response.data
  },

  async testSMTPConfig(toEmail) {
    const response = await api.post('/admin/email/smtp/test', { to_email: toEmail })
    return response.data
  },

  // Email Templates
  async getTemplates() {
    const response = await api.get('/admin/email/templates')
    return response.data
  },

  async getTemplate(type) {
    const response = await api.get(`/admin/email/templates/${type}`)
    return response.data
  },

  async updateTemplate(type, data) {
    const response = await api.put(`/admin/email/templates/${type}`, data)
    return response.data
  },

  async resetTemplate(type) {
    const response = await api.post(`/admin/email/templates/${type}/reset`)
    return response.data
  },

  async previewTemplate(type) {
    const response = await api.get(`/admin/email/templates/${type}/preview`)
    return response.data
  },

  async getTemplateVariables() {
    const response = await api.get('/admin/email/templates/variables')
    return response.data
  },

  // Logs
  async getLogs() {
    const response = await api.get('/admin/email/logs')
    return response.data
  },

  // OAuth 2.0 Configuration
  async getOAuthConfig() {
    const response = await api.get('/admin/email/oauth')
    return response.data
  },

  async updateOAuthConfig(data) {
    const response = await api.put('/admin/email/oauth', data)
    return response.data
  },

  async testOAuthConnection(toEmail) {
    const response = await api.post('/admin/email/oauth/test', { to_email: toEmail })
    return response.data
  },

  async refreshOAuthToken() {
    const response = await api.post('/admin/email/oauth/refresh')
    return response.data
  },

  // Health Status (diagnostic)
  async getHealthStatus() {
    const response = await api.get('/admin/email/health')
    return response.data
  }
}

// Comments Service
export const getComments = async (entityType, entityId) => {
  const response = await api.get('/comments', {
    params: { entity_type: entityType, entity_id: entityId }
  })
  return response.data
}

export const createComment = async (data) => {
  const response = await api.post('/comments', data)
  return response.data
}

export const updateComment = async (id, data) => {
  const response = await api.put(`/comments/${id}`, data)
  return response.data
}

export const deleteComment = async (id) => {
  const response = await api.delete(`/comments/${id}`)
  return response.data
}

export const getCommentSettings = async () => {
  const response = await api.get('/comments/settings')
  return response.data
}

export const getPendingComments = async () => {
  const response = await api.get('/admin/comments/pending')
  return response.data
}

export const moderateComment = async (data) => {
  const response = await api.post('/admin/comments/moderate', data)
  return response.data
}

export const updateCommentSettings = async (data) => {
  const response = await api.put('/admin/comments/settings', data)
  return response.data
}

// Feedback Service
export const getFeedbackStats = async (entityType, entityId) => {
  const response = await api.get('/feedback/stats', {
    params: { entity_type: entityType, entity_id: entityId }
  })
  return response.data
}

export const addFeedback = async (data) => {
  const response = await api.post('/feedback', data)
  return response.data
}

export const removeFeedback = async (entityType, entityId) => {
  const response = await api.delete('/feedback', {
    params: { entity_type: entityType, entity_id: entityId }
  })
  return response.data
}

export const getAllFeedback = async (entityType, entityId) => {
  const response = await api.get('/admin/feedback/all', {
    params: { entity_type: entityType, entity_id: entityId }
  })
  return response.data
}

// ===== Notifications Service =====
export const notificationService = {
  // Récupérer les notifications
  async getNotifications(params = {}) {
    const response = await api.get('/notifications', { params })
    return response.data
  },

  // Récupérer le nombre de notifications non lues
  async getUnreadCount() {
    const response = await api.get('/notifications/unread/count')
    return response.data
  },

  // Récupérer les statistiques
  async getStats() {
    const response = await api.get('/notifications/stats')
    return response.data
  },

  // Marquer une notification comme lue
  async markAsRead(id) {
    const response = await api.put(`/notifications/${id}/read`)
    return response.data
  },

  // Marquer toutes les notifications comme lues
  async markAllAsRead() {
    const response = await api.put('/notifications/read-all')
    return response.data
  },

  // Supprimer une notification
  async deleteNotification(id) {
    const response = await api.delete(`/notifications/${id}`)
    return response.data
  },

  // Supprimer toutes les notifications lues
  async deleteAllRead() {
    const response = await api.delete('/notifications/read/all')
    return response.data
  }
}

// ===== Polls Service =====
export const pollsService = {
  // Récupérer les sondages (avec filtres et pagination)
  async getPolls(params = {}) {
    const response = await api.get('/polls', { params })
    return response.data
  },

  // Récupérer un sondage par ID
  async getPollById(id) {
    const response = await api.get(`/polls/${id}`)
    return response.data
  },

  // Créer un sondage (admin/editor)
  async createPoll(data) {
    const response = await api.post('/admin/polls', data)
    return response.data
  },

  // Créer un sondage (editor)
  async createPollAsEditor(data) {
    const response = await api.post('/editor/polls', data)
    return response.data
  },

  // Créer un sondage (admin de groupe)
  async createPollAsGroupAdmin(data) {
    const response = await api.post('/group-admin/polls', data)
    return response.data
  },

  // Mettre à jour un sondage
  async updatePoll(id, data) {
    const response = await api.put(`/admin/polls/${id}`, data)
    return response.data
  },

  // Mettre à jour un sondage (editor)
  async updatePollAsEditor(id, data) {
    const response = await api.put(`/editor/polls/${id}`, data)
    return response.data
  },

  // Mettre à jour un sondage (admin de groupe)
  async updatePollAsGroupAdmin(id, data) {
    const response = await api.put(`/group-admin/polls/${id}`, data)
    return response.data
  },

  // Supprimer un sondage
  async deletePoll(id) {
    const response = await api.delete(`/admin/polls/${id}`)
    return response.data
  },

  // Supprimer un sondage (editor)
  async deletePollAsEditor(id) {
    const response = await api.delete(`/editor/polls/${id}`)
    return response.data
  },

  // Supprimer un sondage (admin de groupe)
  async deletePollAsGroupAdmin(id) {
    const response = await api.delete(`/group-admin/polls/${id}`)
    return response.data
  },

  // Fermer un sondage (désactiver)
  async closePoll(id) {
    const response = await api.post(`/admin/polls/${id}/close`)
    return response.data
  },

  // Fermer un sondage (admin de groupe)
  async closePollAsGroupAdmin(id) {
    const response = await api.post(`/group-admin/polls/${id}/close`)
    return response.data
  },

  // Voter pour un sondage
  async vote(id, pollOptionIds) {
    const response = await api.post(`/polls/${id}/vote`, {
      poll_option_ids: pollOptionIds
    })
    return response.data
  },

  // Récupérer les résultats d'un sondage
  async getResults(id) {
    const response = await api.get(`/polls/${id}/results`)
    return response.data
  },

  // Récupérer les statistiques des sondages (admin)
  async getAnalytics() {
    const response = await api.get('/admin/polls/analytics')
    return response.data
  },

  // Récupérer les sondages pour group admin
  async getPollsAsGroupAdmin(params = {}) {
    const response = await api.get('/group-admin/polls', { params })
    return response.data
  }
}

// ===== Comments Service =====
export const commentsService = {
  // Public/Protected
  async getComments(params = {}) {
    const response = await api.get('/comments', { params })
    return response.data
  },

  async createComment(data) {
    const response = await api.post('/comments', data)
    return response.data
  },

  async updateComment(id, data) {
    const response = await api.put(`/comments/${id}`, data)
    return response.data
  },

  async deleteComment(id) {
    const response = await api.delete(`/comments/${id}`)
    return response.data
  },

  async getSettings() {
    const response = await api.get('/comments/settings')
    return response.data
  },

  // Admin
  async getPendingComments() {
    const response = await api.get('/admin/comments/pending')
    return response.data
  },

  async moderateComment(data) {
    const response = await api.post('/admin/comments/moderate', data)
    return response.data
  },

  async updateSettings(data) {
    const response = await api.put('/admin/comments/settings', data)
    return response.data
  }
}

// ===== Media Service =====
export const mediaService = {
  // Upload un fichier média (editor/admin de groupe/admin)
  async uploadMedia(file, onUploadProgress) {
    const formData = new FormData()
    formData.append('file', file)

    const response = await api.post('/editor/media/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress,
    })
    return response.data
  },

  // Récupérer la liste des médias avec pagination et filtres
  async getMediaList(params = {}) {
    const response = await api.get('/media', { params })
    return response.data
  },

  // Récupérer un média par ID
  async getMediaById(id) {
    const response = await api.get(`/media/${id}`)
    return response.data
  },

  // Supprimer un média
  async deleteMedia(id) {
    const response = await api.delete(`/media/${id}`)
    return response.data
  },

  // Upload média pour group admin
  async uploadMediaAsGroupAdmin(file, onUploadProgress) {
    const formData = new FormData()
    formData.append('file', file)

    const response = await api.post('/group-admin/media/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress,
    })
    return response.data
  }
}

// ===== Admin Media Service =====
export const getAdminMediaList = (params = {}) => api.get('/admin/media', { params })
export const getAdminMedia = (id) => api.get(`/admin/media/${id}`)
export const uploadAdminMedia = (formData, onUploadProgress) => {
  return api.post('/admin/media/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
    onUploadProgress,
  })
}
export const updateAdminMedia = (id, data) => api.put(`/admin/media/${id}`, data)
export const deleteAdminMedia = (id) => api.delete(`/admin/media/${id}`)

export default api