<template>
  <div class="relative w-full h-full min-h-[600px]">
    <!-- Loading overlay -->
    <div v-if="isLoading" class="absolute inset-0 bg-white bg-opacity-75 flex items-center justify-center z-50">
      <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-primary-500" />
    </div>

    <!-- Simple calendar placeholder -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-6">
      <div class="flex items-center justify-between mb-6">
        <div class="flex flex-col">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ $t('calendar.title') }}
          </h3>
          <div class="text-sm text-gray-600 dark:text-gray-400 mt-1">
            {{ currentPeriodDisplay }}
          </div>
        </div>
        <div class="flex gap-2">
          <!-- View Mode Selector -->
          <div class="flex gap-1 bg-gray-100 dark:bg-gray-700 rounded-lg p-1 mr-4">
            <button
              @click="setViewMode('month')"
              :class="[
                'px-2 py-1 rounded-md font-medium transition-colors text-xs',
                currentViewMode === 'month'
                  ? 'bg-white dark:bg-gray-800 text-primary-600 shadow-sm'
                  : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
              ]"
            >
              {{ $t('calendar.month') }}
            </button>
            <button
              @click="setViewMode('week')"
              :class="[
                'px-2 py-1 rounded-md font-medium transition-colors text-xs',
                currentViewMode === 'week'
                  ? 'bg-white dark:bg-gray-800 text-primary-600 shadow-sm'
                  : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
              ]"
            >
              {{ $t('calendar.week') }}
            </button>
            <button
              @click="setViewMode('day')"
              :class="[
                'px-2 py-1 rounded-md font-medium transition-colors text-xs',
                currentViewMode === 'day'
                  ? 'bg-white dark:bg-gray-800 text-primary-600 shadow-sm'
                  : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
              ]"
            >
              {{ $t('calendar.day') }}
            </button>
          </div>
          
          <!-- Navigation -->
          <button 
            @click="navigatePrevious"
            class="px-3 py-1 bg-gray-200 dark:bg-gray-700 rounded-md"
            :title="$t('calendar.previous')"
          >
            <Icon icon="mdi:chevron-left" class="h-4 w-4" />
          </button>
          <button 
            @click="goToToday"
            class="px-3 py-1 bg-gray-200 dark:bg-gray-700 rounded-md"
          >
            {{ $t('calendar.today') }}
          </button>
          <button 
            @click="navigateNext"
            class="px-3 py-1 bg-gray-200 dark:bg-gray-700 rounded-md"
            :title="$t('calendar.next')"
          >
            <Icon icon="mdi:chevron-right" class="h-4 w-4" />
          </button>
        </div>
      </div>

      <!-- Days of week header -->
      <div class="grid grid-cols-7 gap-1 mb-4">
        <div 
          v-for="(day, index) in daysOfWeek" 
          :key="`${day}-${index}`"
          class="text-center text-sm font-semibold text-gray-700 dark:text-gray-200 py-3 bg-gray-50 dark:bg-gray-700 border-b-2 border-gray-200 dark:border-gray-600"
        >
          {{ day }}
        </div>
      </div>

      <!-- Calendar content based on view mode -->
      <div v-if="currentViewMode === 'month'">
        <!-- Month View -->
        <div class="grid grid-cols-7 gap-px bg-gray-200 dark:bg-gray-700 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
          <div 
            v-for="day in calendarDays" 
            :key="`${day.date.getFullYear()}-${day.date.getMonth()}-${day.date.getDate()}`"
            :class="[
              'min-h-[100px] p-1 bg-white dark:bg-gray-800 relative transition-colors hover:bg-gray-50 dark:hover:bg-gray-750',
              !day.isCurrentMonth && 'bg-gray-50/50 dark:bg-gray-900/50 text-gray-400',
              day.isToday && 'bg-blue-50/30 dark:bg-blue-900/10'
            ]"

          >
            <!-- Date Header -->
            <div class="flex justify-between items-start mb-1">
              <span 
                :class="[
                  'text-xs font-medium w-6 h-6 flex items-center justify-center rounded-full',
                  day.isToday 
                    ? 'bg-blue-600 text-white' 
                    : 'text-gray-700 dark:text-gray-300'
                ]"
              >
                {{ day.date.getDate() }}
              </span>
            </div>

            <!-- Events List -->
            <div class="space-y-0.5">
              <div 
                v-for="event in day.events.slice(0, 4)" 
                :key="event.id"
                @click="showEvent(event)"
                :class="[
                  'text-[10px] px-1.5 py-0.5 truncate cursor-pointer hover:opacity-80 transition-opacity select-none',
                  // Styling for continuous feel
                  getDateString(new Date(event.start_date)) === getDateString(day.date) ? 'rounded-l-md' : 'rounded-l-none -ml-1 pl-2',
                  event.end_date && getDateString(new Date(event.end_date)) === getDateString(day.date) ? 'rounded-r-md' : 'rounded-r-none -mr-1 pr-2',
                  
                  // Default colors if not specified
                  !event.color && 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-100',
                  
                  // Event type/priority indicator (optional border)
                  'border-l-2'
                ]"
                :style="{
                  backgroundColor: event.color ? event.color + '20' : undefined,
                  color: event.color ? event.color : undefined,
                  borderLeftColor: event.color || '#3B82F6'
                }"
              >
                {{ event.title }}
              </div>
              
              <!-- More events indicator -->
              <div v-if="day.events.length > 4" class="text-[10px] text-gray-400 pl-1">
                +{{ day.events.length - 4 }} autres
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="currentViewMode === 'week'">
        <!-- Week View -->
        <div class="grid grid-cols-7 gap-1">
          <div 
            v-for="day in weekDays" 
            :key="`${day.date.getFullYear()}-${day.date.getMonth()}-${day.date.getDate()}`"
            :class="[
              'p-2 text-sm border border-gray-200 dark:border-gray-700 min-h-[200px] relative',
              'bg-white dark:bg-gray-800',
              day.isToday ? 'bg-blue-50 dark:bg-blue-900' : ''
            ]"
          >
            <div class="font-medium mb-2">{{ day.date.getDate() }} {{ day.date.toLocaleDateString(i18n.global.locale.value, { month: 'short' }) }}</div>
            <div v-if="day.events.length > 0" class="space-y-1">
              <div 
                v-for="event in day.events" 
                :key="event.id"
                :class="[
                  'text-xs p-2 rounded cursor-pointer group relative border-l-4',
                  event.color ? '' : 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                ]"
                :style="event.color ? { backgroundColor: event.color + '20', color: event.color, borderLeftColor: event.color } : { borderLeftColor: '#3B82F6' }"
                @click="showEvent(event)"
              >
                <div class="font-medium truncate">{{ event.title }}</div>
                <div v-if="event.start_date" class="text-xs opacity-75">
                  {{ formatEventTime(event.start_date) }}
                </div>
                <!-- View Icon Overlay -->
                <Icon 
                  icon="mdi:eye" 
                  class="absolute top-1 right-1 h-3 w-3 p-0.5 bg-white dark:bg-gray-800 rounded-full opacity-0 group-hover:opacity-100 transition-opacity hover:text-primary-600"
                  @click.stop="viewEvent(event)"
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="currentViewMode === 'day'">
        <!-- Day View -->
        <div class="max-w-2xl mx-auto">
          <div class="text-center mb-6">
            <h4 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ currentDate.toLocaleDateString(i18n.global.locale.value, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' }) }}
            </h4>
          </div>
          
          <div v-if="dayEvents.length > 0" class="space-y-3">
            <div 
              v-for="event in dayEvents" 
              :key="event.id"
              :class="[
                'p-4 rounded-lg cursor-pointer group relative border-l-4',
                event.color ? '' : 'bg-blue-50 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
              ]"
              :style="event.color ? { backgroundColor: event.color + '10', color: event.color, borderLeftColor: event.color } : { borderLeftColor: '#3B82F6' }"
              @click="showEvent(event)"
            >
              <div class="flex justify-between items-start">
                <div class="flex-1">
                  <h5 class="font-medium text-lg">{{ event.title }}</h5>
                  <div v-if="event.start_date" class="text-sm opacity-75 mt-1">
                    {{ formatEventDateTime(event.start_date) }}
                    <span v-if="event.end_date"> - {{ formatEventTime(event.end_date) }}</span>
                  </div>
                  <p v-if="event.location" class="text-sm opacity-75 mt-1">
                    <Icon icon="mdi:map-marker" class="h-4 w-4 inline mr-1" />
                    {{ event.location }}
                  </p>
                </div>
                <!-- View Icon -->
                <Icon 
                  icon="mdi:eye" 
                  class="h-5 w-5 p-1 rounded-full opacity-0 group-hover:opacity-100 transition-opacity hover:text-primary-600"
                  @click.stop="viewEvent(event)"
                />
              </div>
            </div>
          </div>
          
          <div v-else class="text-center py-12">
            <Icon icon="mdi:calendar-outline" class="h-16 w-16 mx-auto text-gray-400 mb-4" />
            <p class="text-gray-600 dark:text-gray-400 text-lg">
              {{ $t('calendar.noEventsForDay') }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Event Detail Modal -->
    <EventDetailModal
      v-if="selectedEvent"
      :event="selectedEvent"
      @close="selectedEvent = null"
      @updated="handleEventUpdated"
    />

    <!-- Empty state for calendar view -->
    <div v-if="!isLoading && events.length === 0" class="mt-6 text-center py-12">
      <Icon icon="mdi:calendar-outline" class="h-16 w-16 mx-auto text-gray-400 mb-4" />
      <p class="text-gray-600 dark:text-gray-400 text-lg">
        {{ $t('calendar.noEvents') }}
      </p>
      <p class="text-gray-500 dark:text-gray-500 text-sm mt-2">
        {{ $t('calendar.noEventsHelp') }}
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Icon } from '@iconify/vue'
import { useI18n } from 'vue-i18n'
import { i18n } from '@/i18n'
import { useEventsStore } from '@/stores/events'
import { useAuthStore } from '@/stores/auth'
import EventDetailModal from './EventDetailModal.vue'


const props = defineProps({
  events: {
    type: Array,
    default: () => []
  }
})

const router = useRouter()
const { t } = useI18n()
const eventsStore = useEventsStore()
const authStore = useAuthStore()

// Refs
const selectedEvent = ref(null)
const currentDate = ref(new Date())
const currentViewMode = ref('month') // 'month', 'week', 'day'
const daysOfWeek = ref(['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'])

// Computed
const isLoading = computed(() => eventsStore.isLoadingCalendar)

// Current period display
const currentPeriodDisplay = computed(() => {
  const options = {}
  const locale = i18n.global.locale.value
  
  switch (currentViewMode.value) {
    case 'day':
      return currentDate.value.toLocaleDateString(locale, { 
        weekday: 'long', 
        year: 'numeric', 
        month: 'long', 
        day: 'numeric' 
      })
    case 'week':
      const weekStart = getWeekStart(currentDate.value)
      const weekEnd = getWeekEnd(currentDate.value)
      return `${weekStart.toLocaleDateString(locale, { month: 'short', day: 'numeric' })} - ${weekEnd.toLocaleDateString(locale, { month: 'short', day: 'numeric', year: 'numeric' })}`
    case 'month':
    default:
      return currentDate.value.toLocaleDateString(locale, { month: 'long', year: 'numeric' })
  }
})

// Computed properties for different views
const weekDays = computed(() => {
  const weekStart = getWeekStart(currentDate.value)
  const days = []
  
  for (let i = 0; i < 7; i++) {
    const date = new Date(weekStart)
    date.setDate(weekStart.getDate() + i)
    days.push({
      date,
      isCurrentMonth: date.getMonth() === currentDate.value.getMonth(),
      isToday: isToday(date),
      events: getEventsForDate(date)
    })
  }
  
  return days
})

const dayEvents = computed(() => {
  return getEventsForDate(currentDate.value)
})

// Methods
const isToday = (date) => {
  const today = new Date()
  return date.toDateString() === today.toDateString()
}

// Calendar computed for month view
const calendarDays = computed(() => {
  try {
    const year = currentDate.value.getFullYear()
    const month = currentDate.value.getMonth()
    
    // Validate current date
    if (isNaN(year) || isNaN(month)) {
      console.warn('Invalid current date:', currentDate.value)
      return []
    }
    
    const firstDay = new Date(year, month, 1)
    const lastDay = new Date(year, month + 1, 0)
    
    // Calculate the first day of the week (Monday = 0, Sunday = 6)
    // getDay() returns 0 for Sunday, 1 for Monday, etc.
    // We want Monday to be 0, so we adjust: if getDay() is 0 (Sunday), it becomes 6
    const firstDayOfWeek = firstDay.getDay() === 0 ? 6 : firstDay.getDay() - 1
    
    const days = []
    
    // Previous month days - calculate correctly
    const prevMonthLastDay = new Date(year, month, 0).getDate()
    for (let i = firstDayOfWeek - 1; i >= 0; i--) {
      const date = new Date(year, month - 1, prevMonthLastDay - i)
      days.push({
        date,
        isCurrentMonth: false,
        isToday: isToday(date),
        events: getEventsForDate(date)
      })
    }
    
    // Current month days
    for (let day = 1; day <= lastDay.getDate(); day++) {
      const date = new Date(year, month, day)
      days.push({
        date,
        isCurrentMonth: true,
        isToday: isToday(date),
        events: getEventsForDate(date)
      })
    }
    
    // Next month days to fill the grid (always show 6 weeks = 42 days)
    const remainingDays = 42 - days.length
    for (let day = 1; day <= remainingDays; day++) {
      const date = new Date(year, month + 1, day)
      days.push({
        date,
        isCurrentMonth: false,
        isToday: isToday(date),
        events: getEventsForDate(date)
      })
    }
    
    return days
  } catch (error) {
    console.error('Error generating calendar days:', error)
    return []
  }
})

// Week utility functions
const getWeekStart = (date) => {
  const start = new Date(date)
  const day = start.getDay()
  const diff = start.getDate() - day + (day === 0 ? -6 : 1) // adjust when day is sunday
  start.setDate(diff)
  start.setHours(0, 0, 0, 0)
  return start
}

const getWeekEnd = (date) => {
  const end = getWeekStart(date)
  end.setDate(end.getDate() + 6)
  end.setHours(23, 59, 59, 999)
  return end
}

// View mode management
const setViewMode = (mode) => {
  currentViewMode.value = mode
  // Adjust current date when switching views to ensure we have valid data
  if (mode === 'day' && !currentDate.value) {
    currentDate.value = new Date()
  }
}

// Navigation methods
const navigatePrevious = () => {
  switch (currentViewMode.value) {
    case 'day':
      currentDate.value = new Date(currentDate.value)
      currentDate.value.setDate(currentDate.value.getDate() - 1)
      break
    case 'week':
      currentDate.value = new Date(currentDate.value)
      currentDate.value.setDate(currentDate.value.getDate() - 7)
      break
    case 'month':
    default:
      currentDate.value = new Date(currentDate.value.getFullYear(), currentDate.value.getMonth() - 1, 1)
      break
  }
  // No need to load events, handled by parent
}

const navigateNext = () => {
  switch (currentViewMode.value) {
    case 'day':
      currentDate.value = new Date(currentDate.value)
      currentDate.value.setDate(currentDate.value.getDate() + 1)
      break
    case 'week':
      currentDate.value = new Date(currentDate.value)
      currentDate.value.setDate(currentDate.value.getDate() + 7)
      break
    case 'month':
    default:
      currentDate.value = new Date(currentDate.value.getFullYear(), currentDate.value.getMonth() + 1, 1)
      break
  }
  // No need to load events, handled by parent
}

const goToToday = () => {
  currentDate.value = new Date()
}

// Date and time formatting
const formatEventTime = (dateString) => {
  const date = new Date(dateString)
  return date.toLocaleTimeString(i18n.global.locale.value, { 
    hour: '2-digit', 
    minute: '2-digit' 
  })
}

const formatEventDateTime = (dateString) => {
  const date = new Date(dateString)
  return date.toLocaleDateString(i18n.global.locale.value, { 
    weekday: 'short',
    month: 'short', 
    day: 'numeric',
    hour: '2-digit', 
    minute: '2-digit'
  })
}

const initializeTranslations = () => {
  try {
    // Use global i18n instance directly
    const days = i18n.global.t('calendar.daysOfWeek')

    if (Array.isArray(days) && days.length === 7) {
      daysOfWeek.value = days
    } else {
      // Try using the global messages directly
      const currentLocale = i18n.global.locale.value
      const messages = i18n.global.messages.value

      if (messages[currentLocale]?.calendar?.daysOfWeek) {
        daysOfWeek.value = messages[currentLocale].calendar.daysOfWeek
      }
    }
  } catch (error) {
    console.error('Error translating days of week:', error)
  }
}

// Utility function to get date string in YYYY-MM-DD format (local time)
const getDateString = (date) => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const getEventsForDate = (date) => {
  const targetDateStr = getDateString(date)
  const targetDate = new Date(targetDateStr)
  
  // Create end of target date for comparison
  const targetDateEnd = new Date(targetDate)
  targetDateEnd.setHours(23, 59, 59, 999)

  // Use props.events instead of store
  const allEvents = props.events || []

  const matchingEvents = allEvents.filter(event => {
    // Validate event has required properties
    if (!event || !event.start_date) {
      return false
    }

    try {
      const startDate = new Date(event.start_date)
      // Check if the date is valid
      if (isNaN(startDate.getTime())) {
        return false
      }
      
      const startDateStr = getDateString(startDate)
      
      // If it's a single day event
      if (!event.end_date || getDateString(new Date(event.end_date)) === startDateStr) {
         return startDateStr === targetDateStr && (event.is_published !== false)
      }
      
      // Multi-day event
      const endDate = new Date(event.end_date)
      const startDateOnly = new Date(getDateString(startDate))
      const endDateOnly = new Date(getDateString(endDate))
      
      // Check if target date is within range [start, end]
      // We compare just dates (ignoring time for the range check relative to days)
      const isWithinRange = targetDate >= startDateOnly && targetDate <= endDateOnly
      
      return isWithinRange && (event.is_published !== false)
    } catch (error) {
      return false
    }
  })

  // Sort events chronologically by start time
  const sortedEvents = matchingEvents.sort((a, b) => {
    try {
      const dateA = new Date(a.start_date)
      const dateB = new Date(b.start_date)
      return dateA.getTime() - dateB.getTime()
    } catch (error) {
      return 0
    }
  })

  return sortedEvents
}

const changeMonth = (direction) => {
  if (direction === 0) {
    goToToday()
  } else {
    currentDate.value = new Date(currentDate.value.getFullYear(), currentDate.value.getMonth() + direction, 1)
  }
}

const showEvent = (event) => {
  selectedEvent.value = event
}

const handleEventUpdated = (updatedEvent) => {
  // Emit update to parent to reload data
  // emit('updated') - not defined yet but events store updates should propagate
}

// Expose methods
defineExpose({
  refreshCalendar: () => {}, // No-op, parent controls data
  setViewMode
})

// Lifecycle
onMounted(() => {
  initializeTranslations()
})

// Add cleanup to prevent lifecycle warnings
onUnmounted(() => {
  // Clear any pending operations or timers here if needed
})
</script>

<style scoped>
/* Removed unused calendar-day CSS rule */
</style>