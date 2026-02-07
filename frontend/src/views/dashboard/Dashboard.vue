<template>
  <div class="content-area">
    <!-- Header avec paramètres dynamiques -->
    <div class="page-header">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="page-title">{{ appSettings.dashboard_title || 'Dashboard' }}</h1>
          <p class="page-subtitle">
            {{ appSettings.welcome_message || 'Welcome to your application portal' }}
          </p>
        </div>

        <!-- Controls -->
        <div class="flex items-center gap-3">
          <!-- View Mode Selector (hidden on mobile) -->
          <div class="hidden md:block">
            <ViewModeSelector v-model="viewMode" />
          </div>

          <!-- Collapse/Expand All Button -->
          <button
            v-if="dashboard?.app_groups?.length > 0"
            @click="toggleAllGroups"
            class="hidden sm:flex items-center gap-2 px-3 py-2 rounded-lg bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors text-sm font-medium text-gray-700 dark:text-gray-300"
            :title="allGroupsCollapsed ? $t('dashboard.expandAll') : $t('dashboard.collapseAll')"
          >
            <Icon
              :icon="allGroupsCollapsed ? 'mdi:unfold-more-horizontal' : 'mdi:unfold-less-horizontal'"
              class="h-5 w-5"
            />
            <span class="hidden lg:inline">
              {{ allGroupsCollapsed ? $t('dashboard.expandAll') : $t('dashboard.collapseAll') }}
            </span>
          </button>

          <!-- Bouton menu mobile -->
          <button
            @click="appStore.toggleSidebar()"
            class="lg:hidden btn btn-secondary"
          >
            <Icon icon="mdi:menu" class="h-5 w-5" />
          </button>
        </div>
      </div>

      <!-- View Mode Selector Mobile (below header) -->
      <div class="md:hidden mt-4">
        <ViewModeSelector v-model="viewMode" />
      </div>
    </div>

    <!-- Statistiques (Admin seulement) -->
    <div v-if="authStore.isAdmin && dashboard?.stats" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div class="card hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
        <div class="flex items-center">
          <div class="flex-shrink-0">
            <div class="h-10 w-10 bg-green-500 dark:bg-green-900 rounded-lg flex items-center justify-center">
              <Icon icon="mdi:folder-multiple" class="h-5 w-5 text-white" />
            </div>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">App Groups</p>
            <p class="text-2xl font-bold text-gray-900 dark:text-white">
              {{ dashboard.stats.total_app_groups }}
            </p>
          </div>
        </div>
      </div>

      <div class="card hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
        <div class="flex items-center">
          <div class="flex-shrink-0">
            <div class="h-10 w-10 bg-blue-500 dark:bg-blue-900 rounded-lg flex items-center justify-center">
              <Icon icon="mdi:application" class="h-5 w-5 text-white" />
            </div>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Applications</p>
            <p class="text-2xl font-bold text-gray-900 dark:text-white">
              {{ dashboard.stats.total_applications }}
            </p>
          </div>
        </div>
      </div>

      <div class="card hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
        <div class="flex items-center">
          <div class="flex-shrink-0">
            <div class="h-10 w-10 bg-purple-500 dark:bg-purple-900 rounded-lg flex items-center justify-center">
              <Icon icon="mdi:account-multiple" class="h-5 w-5 text-white" />
            </div>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Users</p>
            <p class="text-2xl font-bold text-gray-900 dark:text-white">
              {{ dashboard.stats.total_users }}
            </p>
          </div>
        </div>
      </div>

      <div class="card hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
        <div class="flex items-center">
          <div class="flex-shrink-0">
            <div class="h-10 w-10 bg-yellow-500 dark:bg-yellow-900 rounded-lg flex items-center justify-center">
              <Icon icon="mdi:account-group" class="h-5 w-5 text-white" />
            </div>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Groups</p>
            <p class="text-2xl font-bold text-gray-900 dark:text-white">
              {{ dashboard.stats.total_groups }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading state -->
    <div v-if="isLoading" class="flex justify-center py-12">
      <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-gray-400" />
    </div>

    <!-- Empty state -->
    <div v-else-if="!dashboard?.app_groups?.length" class="empty-state">
      <Icon icon="mdi:application" class="empty-state-icon" />
      <h3 class="empty-state-title">No Applications Available</h3>
      <p class="empty-state-description">
        {{ authStore.isAdmin 
          ? 'Start by creating application groups and applications.' 
          : 'Contact your administrator to get access to applications.' 
        }}
      </p>
      <div v-if="authStore.isAdmin" class="mt-6">
        <router-link to="/admin/app-groups" class="btn btn-primary">
          <Icon icon="mdi:plus" class="h-4 w-4 mr-2" />
          Create App Group
        </router-link>
      </div>
    </div>

    <!-- Groupes d'applications -->
    <div v-else class="space-y-6">
      <!-- Announcements Carousel -->
      <div v-if="activeAnnouncements.length > 0" class="relative">
        <div class="announcement-carousel">
          <!-- Carousel Container -->
          <div class="relative overflow-hidden" @mouseenter="pauseAutoRotation" @mouseleave="resumeAutoRotation">
            <!-- Announcement Slide -->
            <transition :name="slideDirection" mode="out-in">
              <div
                :key="currentAnnouncementIndex"
                :class="getAnnouncementClass(currentAnnouncement.type)"
                class="rounded-lg p-4 border-l-4"
              >
                <div class="flex items-start gap-3">
                  <Icon :icon="getAnnouncementIcon(currentAnnouncement.type)" class="h-5 w-5 flex-shrink-0 mt-0.5" />
                  <div class="flex-1">
                    <h3 class="font-semibold text-sm mb-1">{{ currentAnnouncement.title }}</h3>
                    <p v-if="currentAnnouncement.content" class="text-sm whitespace-pre-wrap line-clamp-3">
                      {{ currentAnnouncement.content }}
                    </p>
                  </div>
                </div>
              </div>
            </transition>

            <!-- Navigation Buttons -->
            <button
              v-if="activeAnnouncements.length > 1"
              @click="previousAnnouncement"
              class="absolute left-2 top-1/2 -translate-y-1/2 p-2 rounded-full bg-white/90 dark:bg-gray-700/90 hover:bg-white dark:hover:bg-gray-600 shadow-md transition-all z-10"
              :title="'Previous'"
            >
              <Icon icon="mdi:chevron-left" class="h-5 w-5 text-gray-700 dark:text-gray-300" />
            </button>
            <button
              v-if="activeAnnouncements.length > 1"
              @click="nextAnnouncement"
              class="absolute right-2 top-1/2 -translate-y-1/2 p-2 rounded-full bg-white/90 dark:bg-gray-700/90 hover:bg-white dark:hover:bg-gray-600 shadow-md transition-all z-10"
              :title="'Next'"
            >
              <Icon icon="mdi:chevron-right" class="h-5 w-5 text-gray-700 dark:text-gray-300" />
            </button>
          </div>

          <!-- Controls Bar -->
          <div v-if="activeAnnouncements.length > 1" class="flex items-center justify-between mt-3 px-2">
            <!-- Pagination Dots -->
            <div class="flex items-center gap-2">
              <button
                v-for="(announcement, index) in activeAnnouncements"
                :key="index"
                @click="goToAnnouncement(index)"
                class="transition-all duration-200"
                :class="index === currentAnnouncementIndex
                  ? 'w-6 h-2 rounded-full bg-blue-600 dark:bg-blue-400'
                  : 'w-2 h-2 rounded-full bg-gray-300 dark:bg-gray-600 hover:bg-gray-400 dark:hover:bg-gray-500'"
                :title="`Announcement ${index + 1}`"
              ></button>
            </div>

            <!-- Counter and Auto-play Control -->
            <div class="flex items-center gap-3">
              <span class="text-xs text-gray-600 dark:text-gray-400 font-medium">
                {{ currentAnnouncementIndex + 1 }}/{{ activeAnnouncements.length }}
              </span>
              <button
                @click="toggleAutoRotation"
                class="p-1.5 rounded-md hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
                :title="isAutoRotating ? 'Pause' : 'Play'"
              >
                <Icon
                  :icon="isAutoRotating ? 'mdi:pause' : 'mdi:play'"
                  class="h-4 w-4 text-gray-600 dark:text-gray-400"
                />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Mes Favoris Section -->
      <div v-if="favoriteApps.length > 0" class="app-group-container fade-in" style="margin-bottom: 1.5rem;">
        <div 
          class="app-group-header bg-gradient-to-r from-yellow-50 to-orange-50 dark:from-yellow-900/20 dark:to-orange-900/20"
          @click="toggleGroup('favorites')"
        >
          <div class="flex items-center gap-2 flex-1">
            <div
              class="h-8 w-8 rounded-lg flex items-center justify-center shadow-sm flex-shrink-0 bg-gradient-to-br from-yellow-400 to-orange-500"
            >
              <Icon icon="mdi:star" class="h-4 w-4 text-white" />
            </div>
            <div class="flex items-baseline gap-2 flex-1 min-w-0">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white truncate">
                {{ $t('common.myFavorites') }}
              </h2>
              <span class="text-xs text-gray-500 dark:text-gray-400 truncate">
                - {{ favoriteApps.length }} {{ favoriteApps.length > 1 ? $t('common.applications').toLowerCase() : $t('common.applications').toLowerCase().slice(0, -1) }}
              </span>
            </div>
          </div>
          <Icon
            :icon="isGroupCollapsed('favorites') ? 'mdi:chevron-down' : 'mdi:chevron-up'"
            class="h-5 w-5 text-gray-500 dark:text-gray-400 transition-transform duration-300 flex-shrink-0"
          />
        </div>

        <transition name="collapse">
          <div v-show="!isGroupCollapsed('favorites')" class="app-group-content">
            <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2">
              <div
                v-for="app in favoriteApps"
                :key="app.id"
                class="app-list-item"
                @click="openApplication(app)"
              >
                <!-- App Icon -->
                <div
                  class="h-9 w-9 rounded-lg flex items-center justify-center flex-shrink-0 shadow-sm"
                  :style="{ backgroundColor: app.color || '#6366f1' }"
                >
                  <Icon :icon="app.icon || 'mdi:application'" class="h-4 w-4 text-white" />
                </div>

                <!-- App Info -->
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-1">
                    <h3 class="font-semibold text-xs text-gray-900 dark:text-white truncate">
                      {{ app.name }}
                    </h3>
                    <Icon
                      v-if="app.open_in_new_tab"
                      icon="mdi:open-in-new"
                      class="h-3 w-3 text-gray-400 dark:text-gray-500 flex-shrink-0"
                    />
                  </div>
                  <p v-if="app.description" class="text-xs text-gray-600 dark:text-gray-400 truncate mt-0.5">
                    {{ app.description }}
                  </p>
                </div>

                <!-- Favorite Button -->
                <button
                  @click.stop="toggleFavorite($event, app)"
                  class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex-shrink-0"
                  :title="$t('common.removeFromFavorites')"
                >
                  <Icon icon="mdi:star" class="h-4 w-4 text-yellow-500" />
                </button>
              </div>
            </div>
          </div>
        </transition>
      </div>

      <!-- Grid View (formerly List) -->
      <div v-if="viewMode === 'grid'">
        <GridView
          :app-groups="dashboard.app_groups"
          :collapsed-groups="collapsedGroups"
          @toggle-group="toggleGroup"
          @show-details="openAppDetails"
        />
      </div>

      <!-- Table View -->
      <div v-else-if="viewMode === 'table'">
        <TableView
          :app-groups="dashboard.app_groups"
        />
      </div>

      <!-- List View (formerly Grid) -->
      <div v-else-if="viewMode === 'list'">
        <ListView
          :app-groups="dashboard.app_groups"
          :collapsed-groups="collapsedGroups"
          @toggle-group="toggleGroup"
          @show-details="openAppDetails"
        />
      </div>
    </div>

    <!-- Application Details Modal -->
    <ApplicationDetailModal
      v-if="showDetailsModal && selectedApp"
      :app="selectedApp"
      @close="showDetailsModal = false"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { Icon } from '@iconify/vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useFavoritesStore } from '@/stores/favorites'
import { dashboardService, adminService, analyticsService, announcementsService } from '@/services/api'
import ViewModeSelector from '@/components/dashboard/ViewModeSelector.vue'
import TableView from '@/components/dashboard/TableView.vue'
import ListView from '@/components/dashboard/ListView.vue'
import GridView from '@/components/dashboard/GridView.vue'
import ApplicationDetailModal from '@/components/dashboard/ApplicationDetailModal.vue'

const authStore = useAuthStore()
const appStore = useAppStore()
const favoritesStore = useFavoritesStore()

// État local
const dashboard = ref(null)
const appSettings = ref({})
const isLoading = ref(false)
const collapsedGroups = ref(new Set())
const activeAnnouncements = ref([])

// Application Details Modal
const showDetailsModal = ref(false)
const selectedApp = ref(null)

const openAppDetails = (app) => {
  selectedApp.value = app
  showDetailsModal.value = true
}

// View mode state
const viewMode = ref(localStorage.getItem('dashboard-view-mode') || 'grid')
const allGroupsCollapsed = ref(false)

// Carousel state
const currentAnnouncementIndex = ref(0)
const isAutoRotating = ref(true)
const slideDirection = ref('slide-left')
const autoRotationInterval = ref(null)

// Computed property for current announcement
const currentAnnouncement = computed(() => {
  return activeAnnouncements.value[currentAnnouncementIndex.value] || {}
})

// Watch view mode changes and save to localStorage
watch(viewMode, (newMode) => {
  localStorage.setItem('dashboard-view-mode', newMode)
})

// Carousel navigation methods
const previousAnnouncement = () => {
  slideDirection.value = 'slide-right'
  if (currentAnnouncementIndex.value === 0) {
    currentAnnouncementIndex.value = activeAnnouncements.value.length - 1
  } else {
    currentAnnouncementIndex.value--
  }
}

const nextAnnouncement = () => {
  slideDirection.value = 'slide-left'
  if (currentAnnouncementIndex.value === activeAnnouncements.value.length - 1) {
    currentAnnouncementIndex.value = 0
  } else {
    currentAnnouncementIndex.value++
  }
}

const goToAnnouncement = (index) => {
  if (index === currentAnnouncementIndex.value) return
  slideDirection.value = index > currentAnnouncementIndex.value ? 'slide-left' : 'slide-right'
  currentAnnouncementIndex.value = index
}

// Auto-rotation methods
const startAutoRotation = () => {
  if (autoRotationInterval.value) {
    clearInterval(autoRotationInterval.value)
  }
  if (activeAnnouncements.value.length > 1) {
    autoRotationInterval.value = setInterval(() => {
      nextAnnouncement()
    }, 5000) // 5 seconds
  }
}

const stopAutoRotation = () => {
  if (autoRotationInterval.value) {
    clearInterval(autoRotationInterval.value)
    autoRotationInterval.value = null
  }
}

const toggleAutoRotation = () => {
  isAutoRotating.value = !isAutoRotating.value
  if (isAutoRotating.value) {
    startAutoRotation()
  } else {
    stopAutoRotation()
  }
}

const pauseAutoRotation = () => {
  if (isAutoRotating.value) {
    stopAutoRotation()
  }
}

const resumeAutoRotation = () => {
  if (isAutoRotating.value) {
    startAutoRotation()
  }
}

// Watch for announcements changes to reset carousel
watch(activeAnnouncements, (newVal) => {
  if (newVal.length > 0) {
    currentAnnouncementIndex.value = 0
    if (isAutoRotating.value) {
      startAutoRotation()
    }
  } else {
    stopAutoRotation()
  }
}, { immediate: true })

// Cleanup on unmount
onUnmounted(() => {
  stopAutoRotation()
})

// Charger les groupes effondrés depuis le localStorage
const loadCollapsedGroups = () => {
  try {
    const saved = localStorage.getItem('airboard_collapsed_groups')
    if (saved) {
      collapsedGroups.value = new Set(JSON.parse(saved))
    }
  } catch (error) {
    console.error('Error loading collapsed groups:', error)
  }
}

// Sauvegarder les groupes effondrés dans le localStorage
const saveCollapsedGroups = () => {
  try {
    localStorage.setItem('airboard_collapsed_groups', JSON.stringify([...collapsedGroups.value]))
  } catch (error) {
    console.error('Error saving collapsed groups:', error)
  }
}

// Toggle un groupe
const toggleGroup = (groupId) => {
  if (collapsedGroups.value.has(groupId)) {
    collapsedGroups.value.delete(groupId)
  } else {
    collapsedGroups.value.add(groupId)
  }
  saveCollapsedGroups()
}

// Vérifier si un groupe est effondré
const isGroupCollapsed = (groupId) => {
  return collapsedGroups.value.has(groupId)
}

// Collapse all groups
const collapseAll = () => {
  if (!dashboard.value?.app_groups) return

  dashboard.value.app_groups.forEach(group => {
    collapsedGroups.value.add(group.id)
  })
  allGroupsCollapsed.value = true
  saveCollapsedGroups()
}

// Expand all groups
const expandAll = () => {
  collapsedGroups.value.clear()
  allGroupsCollapsed.value = false
  saveCollapsedGroups()
}

// Toggle all groups
const toggleAllGroups = () => {
  if (allGroupsCollapsed.value) {
    expandAll()
  } else {
    collapseAll()
  }
}

// Fonctions
const uniqueById = (items) => {
  if (!Array.isArray(items)) return []
  const seen = new Set()
  const out = []
  for (const item of items) {
    const id = item?.id ?? JSON.stringify(item)
    if (!seen.has(id)) {
      seen.add(id)
      out.push(item)
    }
  }
  return out
}

const loadDashboard = async () => {
  try {
    isLoading.value = true
    const data = await dashboardService.getDashboard()
    // Déduplication côté client par sécurité
    if (data?.app_groups) {
      data.app_groups = uniqueById(data.app_groups).map(g => ({
        ...g,
        applications: uniqueById(g.applications || [])
      }))
    }
    dashboard.value = data
  } catch (error) {
    console.error('Error loading dashboard:', error)
    appStore.showError('Failed to load dashboard')
  } finally {
    isLoading.value = false
  }
}

const loadAppSettings = async () => {
  try {
    // Utiliser le store si dispo
    if (appStore.appSettings) {
      appSettings.value = appStore.appSettings
      return
    }
    // Ne pas appeler l'API si l'utilisateur n'est pas admin (évite 403)
    if (!authStore.isAdmin) {
      appSettings.value = {
        dashboard_title: 'Dashboard',
        welcome_message: 'Welcome to your application portal'
      }
      return
    }
    const settings = await adminService.getAppSettings()
    appSettings.value = settings
    appStore.setAppSettings(settings)
  } catch (error) {
    console.error('Error loading app settings:', error)
    // Use defaults if settings can't be loaded
    appSettings.value = {
      dashboard_title: 'Dashboard',
      welcome_message: 'Welcome to your application portal'
    }
  }
}

const openApplication = async (app) => {
  // Track click for analytics (non-bloquant)
  analyticsService.trackClick(app.id)

  // Ouvrir l'application
  if (app.open_in_new_tab) {
    window.open(app.url, '_blank', 'noopener,noreferrer')
  } else {
    window.location.href = app.url
  }

  // Analytics ou logging (optionnel)
  console.log(`Application opened: ${app.name} - ${app.url}`)
}

// Toggle favorite
const toggleFavorite = async (event, app) => {
  event.stopPropagation() // Empêcher l'ouverture de l'application
  await favoritesStore.toggleFavorite(app.id)
}

// Get favorite applications
const favoriteApps = computed(() => {
  if (!dashboard.value?.app_groups) return []

  const allApps = []
  dashboard.value.app_groups.forEach(group => {
    group.applications?.forEach(app => {
      if (favoritesStore.isFavorite(app.id)) {
        allApps.push(app)
      }
    })
  })

  return allApps
})

// Load active announcements
const loadAnnouncements = async () => {
  try {
    activeAnnouncements.value = await announcementsService.getActiveAnnouncements()
  } catch (error) {
    console.error('Error loading announcements:', error)
  }
}

// Get announcement styling
const getAnnouncementClass = (type) => {
  const classes = {
    info: 'bg-blue-50 dark:bg-blue-900/20 border-blue-500 text-blue-900 dark:text-blue-100',
    warning: 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-500 text-yellow-900 dark:text-yellow-100',
    success: 'bg-green-50 dark:bg-green-900/20 border-green-500 text-green-900 dark:text-green-100',
    error: 'bg-red-50 dark:bg-red-900/20 border-red-500 text-red-900 dark:text-red-100'
  }
  return classes[type] || classes.info
}

// Get announcement icon
const getAnnouncementIcon = (type) => {
  const icons = {
    info: 'mdi:information',
    warning: 'mdi:alert',
    success: 'mdi:check-circle',
    error: 'mdi:alert-circle'
  }
  return icons[type] || icons.info
}

// 🔧 FIX: Watcher pour les changements de settings
watch(() => appStore.settingsLastUpdated, async () => {
  console.log('Settings updated, reloading dashboard settings...')
  await loadAppSettings()
}, { immediate: false })

// Lifecycle
onMounted(async () => {
  loadCollapsedGroups()
  await Promise.all([
    loadDashboard(),
    loadAppSettings(),
    favoritesStore.loadFavorites(),
    loadAnnouncements()
  ])
})
</script>

<style scoped>
/* Conteneur de groupe d'applications */
.app-group-container {
  background-color: rgb(255 255 255);
  border: 2px solid rgb(229 231 235);
  border-radius: 0.75rem;
  box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.05);
  overflow: hidden;
  margin-bottom: 1.5rem;
}

.dark .app-group-container {
  background-color: rgb(31 41 55);
  border-color: rgb(55 65 81);
}

/* En-tête de groupe (cliquable) */
.app-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 1rem;
  cursor: pointer;
  transition: color 0.2s, background-color 0.2s;
  border-bottom: 1px solid rgb(229 231 235);
}

.app-group-header:hover {
  background-color: rgb(249 250 251);
}

.dark .app-group-header {
  border-bottom-color: rgb(55 65 81);
}

.dark .app-group-header:hover {
  background-color: rgb(55 65 81 / 0.5);
}

/* Contenu du groupe */
.app-group-content {
  padding: 1rem;
}

/* List Item - Compact Horizontal Design */
.app-list-item {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.5rem 0.625rem;
  background-color: rgb(255 255 255);
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  cursor: pointer;
  transition: all 0.15s ease;
}

.app-list-item:hover {
  background-color: rgb(249 250 251);
  border-color: rgb(209 213 219);
  box-shadow: 0 2px 4px 0 rgb(0 0 0 / 0.05);
  transform: translateX(2px);
}

.dark .app-list-item {
  background-color: rgb(31 41 55);
  border-color: rgb(55 65 81);
}

.dark .app-list-item:hover {
  background-color: rgb(55 65 81);
  border-color: rgb(75 85 99);
}

/* Transitions pour le collapse */
.collapse-enter-active,
.collapse-leave-active {
  transition: all 0.3s ease;
  max-height: 2000px;
  overflow: hidden;
}

.collapse-enter-from,
.collapse-leave-to {
  max-height: 0;
  opacity: 0;
}

/* Animation d'apparition */
.fade-in {
  animation: fadeIn 0.5s ease-in-out;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Style pour les descriptions tronquées */
.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  line-clamp: 2;
  overflow: hidden;
}

.line-clamp-3 {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  line-clamp: 3;
  overflow: hidden;
}

/* Carousel slide transitions */
.slide-left-enter-active,
.slide-left-leave-active,
.slide-right-enter-active,
.slide-right-leave-active {
  transition: all 0.4s ease-in-out;
}

.slide-left-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.slide-left-leave-to {
  opacity: 0;
  transform: translateX(-100%);
}

.slide-right-enter-from {
  opacity: 0;
  transform: translateX(-100%);
}

.slide-right-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>