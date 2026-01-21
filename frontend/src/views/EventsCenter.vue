<template>
  <div class="w-full p-6">
    <!-- Header -->
    <div class="mb-6">
      <div class="flex justify-between items-start flex-wrap gap-4">
        <div class="flex-1 min-w-0">
          <h1 class="text-3xl font-bold text-gray-900 dark:text-white mb-2">
            {{ $t('agenda.title') }}
          </h1>
          <p class="text-gray-600 dark:text-gray-400">
            {{ $t('agenda.subtitle') }}
          </p>
        </div>
        <div class="flex gap-2 items-center flex-shrink-0">
          <!-- View Toggle Buttons -->
          <div class="flex gap-1 bg-gray-100 dark:bg-gray-700 rounded-lg p-1">
            <button
              @click="switchView('calendar')"
              :class="[
                'px-3 py-2 rounded-md font-medium transition-colors flex items-center gap-2 min-w-[100px] justify-center',
                currentView === 'calendar'
                  ? 'bg-white dark:bg-gray-800 text-primary-600 shadow-sm'
                  : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
              ]"
            >
              <Icon icon="mdi:calendar-month" class="h-4 w-4" />
              <span class="hidden xs:inline text-sm">{{ $t('agenda.calendar') }}</span>
            </button>
            <button
              @click="switchView('list')"
              :class="[
                'px-3 py-2 rounded-md font-medium transition-colors flex items-center gap-2 min-w-[80px] justify-center',
                currentView === 'list'
                  ? 'bg-white dark:bg-gray-800 text-primary-600 shadow-sm'
                  : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
              ]"
            >
              <Icon icon="mdi:format-list-bulleted" class="h-4 w-4" />
              <span class="hidden xs:inline text-sm">{{ $t('agenda.list') }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Calendar View -->
    <div v-if="currentView === 'calendar'" class="bg-white dark:bg-gray-800 rounded-lg shadow-sm">
      <EventCalendar ref="calendarComponent" />
    </div>

    <!-- List View -->
    <div v-else-if="currentView === 'list'" class="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-6">
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
        {{ $t('events.center.title') }} ({{ events.length }})
      </h3>
      
      <div v-if="isLoading" class="text-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500 mx-auto"></div>
        <p class="mt-2 text-gray-600 dark:text-gray-400">{{ $t('agenda.loading') }}</p>
      </div>

      <div v-else-if="getDisplayEvents().length > 0" class="space-y-4">
        <div 
          v-for="event in getDisplayEvents()" 
          :key="event.id"
          class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:shadow-md transition-shadow cursor-pointer"
          @click="viewEvent(event)"
        >
          <div class="flex justify-between items-start">
            <div class="flex-1">
              <h4 class="font-medium text-gray-900 dark:text-white">{{ event.title }}</h4>
              <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                {{ formatEventDate(event.start_date) }}
              </p>
              <p v-if="event.location" class="text-sm text-gray-500 dark:text-gray-400 mt-1">
                <Icon icon="mdi:map-marker" class="h-4 w-4 inline mr-1" />
                {{ event.location }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <span 
                v-if="event.category"
                class="inline-block px-2 py-1 text-xs bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 rounded-full"
              >
                {{ event.category.name }}
              </span>
              <span 
                :class="[
                  'inline-block px-2 py-1 text-xs rounded-full',
                  event.is_published
                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                    : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                ]"
              >
                {{ event.is_published ? $t('agenda.published') : $t('agenda.draft') }}
              </span>
            </div>
          </div>
          <p v-if="event.description" class="text-sm text-gray-600 dark:text-gray-400 mt-2 line-clamp-2">
            {{ extractTextFromTiptap(event.description) }}
          </p>
        </div>
      </div>

      <div v-else class="text-center py-8">
        <Icon icon="mdi:calendar-outline" class="h-12 w-12 mx-auto text-gray-400 mb-2" />
        <p class="text-gray-600 dark:text-gray-400">{{ $t('agenda.noEventsFound') }}</p>
        <button 
          @click="loadData" 
          class="mt-4 px-4 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600"
        >
          {{ $t('agenda.reload') }}
        </button>
      </div>
    </div>

    <!-- Category Filter -->
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-6 mt-6">
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
        {{ $t('agenda.filterByCategory') }}
      </h3>

      <div v-if="categories.length > 0" class="flex flex-wrap gap-3">
        <!-- All categories button -->
        <button
          @click="selectedCategoryId = null"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg border-2 transition-all font-medium',
            selectedCategoryId === null
              ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300'
              : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 text-gray-700 dark:text-gray-300'
          ]"
        >
          <Icon icon="mdi:calendar-multiple" class="h-5 w-5" />
          <span>{{ $t('agenda.allCategories') }}</span>
          <span class="ml-1 px-2 py-0.5 text-xs rounded-full bg-gray-200 dark:bg-gray-700">
            {{ getTotalEventsCount() }}
          </span>
        </button>

        <!-- Category buttons -->
        <button
          v-for="category in categories"
          :key="category.id"
          @click="selectedCategoryId = category.id"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg border-2 transition-all font-medium',
            selectedCategoryId === category.id
              ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300'
              : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 text-gray-700 dark:text-gray-300'
          ]"
        >
          <Icon :icon="category.icon || 'mdi:calendar'" class="h-5 w-5" :style="{ color: category.color }" />
          <span>{{ category.name }}</span>
          <span class="ml-1 px-2 py-0.5 text-xs rounded-full bg-gray-200 dark:bg-gray-700">
            {{ getCategoryEventsCount(category.id) }}
          </span>
        </button>

        <!-- Holidays filter button -->
        <button
          @click="toggleHolidaysFilter"
          :class="[
            'flex items-center gap-2 px-4 py-2 rounded-lg border-2 transition-all font-medium',
            showOnlyHolidays
              ? 'border-red-500 bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-300'
              : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 text-gray-700 dark:text-gray-300'
          ]"
        >
          <Icon icon="mdi:calendar-star" class="h-5 w-5 text-red-500" />
          <span>{{ $t('agenda.holidays') }}</span>
          <span class="ml-1 px-2 py-0.5 text-xs rounded-full bg-gray-200 dark:bg-gray-700">
            {{ getHolidaysCount() }}
          </span>
        </button>
      </div>

      <div v-else class="text-center py-4">
        <p class="text-gray-600 dark:text-gray-400">{{ $t('agenda.noCategories') }}</p>
      </div>
    </div>

    <!-- Working Days Calculator -->
    <WorkingDaysCalculator class="mt-6" />

    <!-- Event Detail Modal -->
    <EventDetailModal
      v-if="selectedEvent"
      :event="selectedEvent"
      @close="selectedEvent = null"
      @updated="handleEventUpdated"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@iconify/vue'
import { useRouter } from 'vue-router'
import { useEventsStore } from '@/stores/events'
import { useAuthStore } from '@/stores/auth'
import EventCalendar from '@/components/events/EventCalendar.vue'
import EventDetailModal from '@/components/events/EventDetailModal.vue'
import WorkingDaysCalculator from '@/components/events/WorkingDaysCalculator.vue'

const router = useRouter()
const eventsStore = useEventsStore()
const authStore = useAuthStore()
const { locale } = useI18n()

// Refs
const isLoading = ref(false)
const currentView = ref('calendar')
const calendarComponent = ref(null)
const selectedEvent = ref(null) // Pour le modal de détail d'événement
const selectedCategoryId = ref(null) // Filtre par catégorie
const showOnlyHolidays = ref(false) // Filtre jours fériés

// Computed
const events = computed(() => eventsStore.events)
const categories = computed(() => eventsStore.categories)

// Methods
const loadData = async () => {
  try {
    isLoading.value = true

    // Load both regular events and calendar events for consistency
    const currentDate = new Date()
    const currentYear = currentDate.getFullYear()
    const startDate = `${currentYear}-01-01`
    const endDate = `${currentYear}-12-31`

    await Promise.all([
      eventsStore.fetchEvents(),
      eventsStore.fetchCategories(),
      eventsStore.fetchCalendarEvents(startDate, endDate)
    ])
  } catch (error) {
    console.error('Error loading data:', error)
  } finally {
    isLoading.value = false
  }
}

const switchView = (view) => {
  currentView.value = view
  if (view === 'calendar' && calendarComponent.value) {
    // Refresh calendar when switching to it
    calendarComponent.value.refreshCalendar()
  } else if (view === 'list') {
    // Ensure we have events data for list view
    if (events.value.length === 0) {
      const currentDate = new Date()
      const currentYear = currentDate.getFullYear()
      eventsStore.fetchCalendarEvents(`${currentYear}-01-01`, `${currentYear}-12-31`)
    }
  }
}

const refreshCalendar = () => {
  if (calendarComponent.value) {
    calendarComponent.value.refreshCalendar()
  }
}

const getDisplayEvents = () => {
  // Use regular events first, fallback to calendar events if needed
  const regularEvents = events.value || []
  const calendarEvents = eventsStore.calendarEvents || []

  let eventsList = []
  if (regularEvents.length > 0) {
    eventsList = regularEvents
  } else if (calendarEvents.length > 0) {
    eventsList = calendarEvents
  }

  // Apply category filter
  if (selectedCategoryId.value !== null) {
    eventsList = eventsList.filter(event => event.category_id === selectedCategoryId.value)
  }

  // Apply holidays filter
  if (showOnlyHolidays.value) {
    eventsList = eventsList.filter(event => event.is_holiday === true)
  }

  return eventsList
}

// Category filter methods
const getTotalEventsCount = () => {
  const regularEvents = events.value || []
  const calendarEvents = eventsStore.calendarEvents || []
  return regularEvents.length > 0 ? regularEvents.length : calendarEvents.length
}

const getCategoryEventsCount = (categoryId) => {
  const regularEvents = events.value || []
  const calendarEvents = eventsStore.calendarEvents || []
  const eventsList = regularEvents.length > 0 ? regularEvents : calendarEvents
  return eventsList.filter(event => event.category_id === categoryId).length
}

const getHolidaysCount = () => {
  const regularEvents = events.value || []
  const calendarEvents = eventsStore.calendarEvents || []
  const eventsList = regularEvents.length > 0 ? regularEvents : calendarEvents
  return eventsList.filter(event => event.is_holiday === true).length
}

const toggleHolidaysFilter = () => {
  showOnlyHolidays.value = !showOnlyHolidays.value
  if (showOnlyHolidays.value) {
    selectedCategoryId.value = null // Reset category filter when showing holidays
  }
}

const viewEvent = (event) => {
  // Vérifier que l'événement a un slug valide
  if (!event.slug) {
    console.error('Événement sans slug:', event)
    // Afficher une notification ou utiliser un modal comme fallback
    showEventModal(event)
    return
  }
  
  // Vérifier que le slug n'est pas vide
  if (event.slug.trim() === '') {
    console.error('Événement avec slug vide:', event)
    showEventModal(event)
    return
  }

  router.push(`/events/${event.slug}`)
}

const showEventModal = (event) => {
  // Créer un objet événement avec les propriétés nécessaires pour le modal
  const eventForModal = {
    ...event,
    // S'assurer que toutes les propriétés nécessaires sont présentes
    id: event.id || Math.random(),
    title: event.title || 'Événement sans titre',
    description: event.description || '',
    start_date: event.start_date,
    end_date: event.end_date,
    is_all_day: event.is_all_day || false,
    location: event.location || '',
    category: event.category || null,
    author: event.author || { first_name: 'Inconnu', last_name: '' },
    color: event.color || '#3B82F6',
    priority: event.priority || 'normal',
    status: event.status || 'confirmed',
    created_at: event.created_at || new Date().toISOString(),
    external_links: event.external_links || null,
    is_recurring: event.is_recurring || false,
    recurrence_rule: event.recurrence_rule || null,
    is_published: event.is_published !== false // Par défaut publié sauf indication contraire
  }
  
  selectedEvent.value = eventForModal
}

const handleEventUpdated = (updatedEvent) => {
  // Recharger les événements quand un événement est mis à jour via le modal
  loadData()
  selectedEvent.value = null
}

// Utility function to extract text from Tiptap JSON format
const extractTextFromTiptap = (content) => {
  if (!content) return ''
  
  let contentObj = content
  
  // If content is a string, try to parse it as JSON
  if (typeof content === 'string') {
    try {
      contentObj = JSON.parse(content)
    } catch (e) {
      // If parsing fails, return the content as is
      return content
    }
  }
  
  // Extract text from Tiptap JSON structure
  const extractText = (node) => {
    if (!node) return ''
    
    let text = ''
    
    if (node.type === 'text' && node.text) {
      text += node.text
    }
    
    if (node.content && Array.isArray(node.content)) {
      for (const child of node.content) {
        text += extractText(child)
      }
    }
    
    return text
  }
  
  return extractText(contentObj)
}

const formatEventDate = (dateString) => {
  const date = new Date(dateString)
  const locale = getCurrentLocale()
  
  return date.toLocaleDateString(locale, { 
    weekday: 'long', 
    year: 'numeric', 
    month: 'long', 
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const getCurrentLocale = () => {
  // Map locale codes to locale strings for date formatting
  const localeMap = {
    'fr': 'fr-FR',
    'en': 'en-US',
    'ar': 'ar-SA',
    'es': 'es-ES'
  }
  
  return localeMap[locale.value] || 'fr-FR'
}

// Lifecycle
onMounted(() => {
  loadData()
})

// Add cleanup to prevent lifecycle warnings
onUnmounted(() => {
  // Clear any pending operations or timers here if needed
})
</script>

<style scoped>
/* Pas de styles spéciaux pour l'instant */
</style>