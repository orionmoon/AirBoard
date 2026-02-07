<template>
  <!-- 3 Columns Grid for Categories -->
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 auto-flow-dense">
    <div
      v-for="appGroup in appGroups"
      :key="appGroup.id"
      class="app-group-container"
    >
      <!-- Group Header -->
      <div
        class="app-group-header"
        @click="toggleGroup(appGroup.id)"
      >
        <div class="flex items-center gap-2 flex-1 min-w-0">
          <div
            class="h-7 w-7 rounded-lg flex items-center justify-center shadow-sm flex-shrink-0"
            :style="{ backgroundColor: appGroup.color || '#10b981' }"
          >
            <Icon :icon="appGroup.icon || 'mdi:folder'" class="h-4 w-4 text-white" />
          </div>
          <div class="flex items-baseline gap-2 flex-1 min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white truncate">
              {{ appGroup.name }}
            </h2>
            <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
              ({{ appGroup.applications?.length || 0 }})
            </span>
          </div>
        </div>
        <Icon
          :icon="isGroupCollapsed(appGroup.id) ? 'mdi:chevron-down' : 'mdi:chevron-up'"
          class="h-4 w-4 text-gray-500 dark:text-gray-400 transition-transform duration-300 flex-shrink-0"
        />
      </div>

      <!-- Applications List -->
      <transition name="collapse">
        <div v-show="!isGroupCollapsed(appGroup.id)" class="app-group-content">
          <div class="space-y-2">
            <div
              v-for="app in appGroup.applications"
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
                    <div class="flex items-center">
                      <button 
                        @click.stop="emit('show-details', app)"
                        class="p-0.5 rounded-full hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors mr-1"
                        :title="$t('common.details')"
                      >
                       <Icon icon="mdi:information-outline" class="h-3.5 w-3.5 text-blue-500" />
                      </button>
                      <Icon
                        v-if="app.open_in_new_tab"
                        icon="mdi:open-in-new"
                        class="h-3 w-3 text-gray-400 dark:text-gray-500 flex-shrink-0"
                      />
                    </div>
                  </div>
                <p v-if="app.description" class="text-xs text-gray-600 dark:text-gray-400 truncate mt-0.5">
                  {{ app.description }}
                </p>
              </div>

              <!-- Favorite Button -->
              <button
                @click.stop="toggleFavorite(app)"
                class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex-shrink-0"
                :title="isFavorite(app.id) ? $t('common.removeFromFavorites') : $t('common.addToFavorites')"
              >
                <Icon
                  :icon="isFavorite(app.id) ? 'mdi:star' : 'mdi:star-outline'"
                  class="h-4 w-4"
                  :class="isFavorite(app.id) ? 'text-yellow-500' : 'text-gray-400 dark:text-gray-500'"
                />
              </button>
            </div>
          </div>
        </div>
      </transition>
    </div>
  </div>
</template>

<script setup>
import { Icon } from '@iconify/vue'
import { useFavoritesStore } from '@/stores/favorites'
import { analyticsService } from '@/services/api'

const props = defineProps({
  appGroups: {
    type: Array,
    required: true
  },
  collapsedGroups: {
    type: Set,
    required: true
  }
})

const emit = defineEmits(['toggle-group'])

const favoritesStore = useFavoritesStore()

// Methods
const isGroupCollapsed = (groupId) => {
  return props.collapsedGroups.has(groupId)
}

const toggleGroup = (groupId) => {
  emit('toggle-group', groupId)
}

const isFavorite = (appId) => {
  return favoritesStore.isFavorite(appId)
}

const toggleFavorite = async (app) => {
  try {
    await favoritesStore.toggleFavorite(app.id)
  } catch (error) {
    console.error('Error toggling favorite:', error)
  }
}

const openApplication = async (app) => {
  // Track click for analytics
  analyticsService.trackClick(app.id)

  // Open application
  if (app.open_in_new_tab) {
    window.open(app.url, '_blank', 'noopener,noreferrer')
  } else {
    window.location.href = app.url
  }

  console.log(`Application opened: ${app.name} - ${app.url}`)
}
</script>

<style scoped>
/* Group Container */
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

/* Group Header */
.app-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 1rem;
  cursor: pointer;
  transition: background-color 0.2s;
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

/* Group Content */
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

/* Collapse Transition */
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
</style>
