<template>
  <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <!-- Loading State -->
    <div v-if="eventsStore.isLoading" class="flex justify-center py-12">
      <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-primary-500" />
    </div>

    <!-- Event Content -->
    <article v-else-if="event" class="bg-white dark:bg-gray-800 rounded-lg shadow-sm overflow-hidden">
      <!-- Header -->
      <div class="p-8 border-b border-gray-200 dark:border-gray-700">
        <!-- Breadcrumb -->
        <div class="mb-4">
          <button
            @click="$router.back()"
            class="flex items-center gap-1 text-sm text-gray-600 dark:text-gray-400 hover:text-primary-500"
          >
            <Icon icon="mdi:arrow-left" class="h-4 w-4" />
            {{ $t('events.detail.back') }}
          </button>
        </div>

        <!-- Badges -->
        <div class="flex flex-wrap items-center gap-2 mb-4">
          <!-- All Day Badge -->
          <span
            v-if="event.is_all_day"
            class="px-3 py-1 rounded-full text-sm font-medium bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300"
          >
            <Icon icon="mdi:weather-sunny" class="inline h-4 w-4 mr-1" />
            {{ $t('events.badge.allDay') }}
          </span>

          <!-- Recurring Badge -->
          <span
            v-if="event.is_recurring"
            class="px-3 py-1 rounded-full text-sm font-medium bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
          >
            <Icon icon="mdi:repeat" class="inline h-4 w-4 mr-1" />
            {{ $t('events.badge.recurring') }}
          </span>

          <!-- Past Event Badge -->
          <span
            v-if="isPastEvent"
            class="px-3 py-1 rounded-full text-sm font-medium bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300"
          >
            <Icon icon="mdi:history" class="inline h-4 w-4 mr-1" />
            {{ $t('events.badge.past') }}
          </span>

          <!-- Priority Badge -->
          <span
            v-if="event.priority === 'urgent'"
            class="px-3 py-1 rounded-full text-sm font-medium bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300"
          >
            <Icon icon="mdi:alert" class="inline h-4 w-4 mr-1" />
            {{ $t('events.priority.urgent') }}
          </span>
          <span
            v-else-if="event.priority === 'important'"
            class="px-3 py-1 rounded-full text-sm font-medium bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300"
          >
            <Icon icon="mdi:star" class="inline h-4 w-4 mr-1" />
            {{ $t('events.priority.important') }}
          </span>

          <!-- Status Badge -->
          <span
            v-if="!event.is_published"
            class="px-3 py-1 rounded-full text-sm font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300"
          >
            <Icon icon="mdi:clock" class="inline h-4 w-4 mr-1" />
            {{ $t('events.status.draft') }}
          </span>
          <span
            v-else-if="event.published_at"
            class="px-3 py-1 rounded-full text-sm font-medium bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
          >
            <Icon icon="mdi:check-circle" class="inline h-4 w-4 mr-1" />
            {{ $t('events.status.published') }}
          </span>

          <!-- Visibility Badge -->
          <span
            v-if="!event.target_groups || event.target_groups.length === 0"
            class="px-3 py-1 rounded-full text-sm font-medium bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
          >
            <Icon icon="mdi:earth" class="inline h-4 w-4 mr-1" />
            {{ $t('events.visibility.public') }}
          </span>
          <span
            v-else
            class="px-3 py-1 rounded-full text-sm font-medium bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300"
            :title="getGroupsTooltip(event.target_groups)"
          >
            <Icon icon="mdi:lock" class="inline h-4 w-4 mr-1" />
            {{ getVisibilityLabel(event.target_groups) }}
          </span>

          <!-- Category -->
          <span
            v-if="event.category"
            class="px-3 py-1 rounded-full text-sm font-medium bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300"
          >
            <Icon :icon="event.category.icon || 'mdi:folder'" class="inline h-4 w-4 mr-1" :style="{ color: event.category.color }" />
            {{ event.category.name }}
          </span>
        </div>

        <!-- Title -->
        <h1 class="text-4xl font-bold text-gray-900 dark:text-white mb-4">
          {{ event.title }}
        </h1>

        <!-- Description -->
        <p v-if="event.description" class="text-xl text-gray-600 dark:text-gray-400 mb-6">
          {{ getSummary(event.description) }}
        </p>

        <!-- Event Details Card -->
        <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-6 mb-6">
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            <!-- Date -->
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 w-10 h-10 bg-primary-100 dark:bg-primary-900 rounded-lg flex items-center justify-center">
                <Icon icon="mdi:calendar" class="h-5 w-5 text-primary-600 dark:text-primary-400" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ $t('events.detail.date') }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  {{ formatDateRange(event.start_date, event.end_date) }}
                </p>
              </div>
            </div>

            <!-- Time -->
            <div v-if="!event.is_all_day" class="flex items-start gap-3">
              <div class="flex-shrink-0 w-10 h-10 bg-blue-100 dark:bg-blue-900 rounded-lg flex items-center justify-center">
                <Icon icon="mdi:clock" class="h-5 w-5 text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ $t('events.detail.time') }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  {{ formatTimeRange(event.start_date, event.end_date) }}
                </p>
              </div>
            </div>

            <!-- Location -->
            <div v-if="event.location" class="flex items-start gap-3">
              <div class="flex-shrink-0 w-10 h-10 bg-green-100 dark:bg-green-900 rounded-lg flex items-center justify-center">
                <Icon icon="mdi:map-marker" class="h-5 w-5 text-green-600 dark:text-green-400" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ $t('events.detail.location') }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  {{ event.location }}
                </p>
              </div>
            </div>

            <!-- Timezone -->
            <div v-if="event.timezone" class="flex items-start gap-3">
              <div class="flex-shrink-0 w-10 h-10 bg-purple-100 dark:bg-purple-900 rounded-lg flex items-center justify-center">
                <Icon icon="mdi:earth" class="h-5 w-5 text-purple-600 dark:text-purple-400" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ $t('events.detail.timezone') }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  {{ event.timezone }}
                </p>
              </div>
            </div>

            <!-- Recurrence Pattern -->
            <div v-if="event.is_recurring && event.recurrence_rule" class="flex items-start gap-3">
              <div class="flex-shrink-0 w-10 h-10 bg-orange-100 dark:bg-orange-900 rounded-lg flex items-center justify-center">
                <Icon icon="mdi:repeat" class="h-5 w-5 text-orange-600 dark:text-orange-400" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ $t('events.detail.recurrence') }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  {{ formatRecurrenceRule(event.recurrence_rule) }}
                </p>
              </div>
            </div>

            <!-- Views -->
            <div v-if="event.views" class="flex items-start gap-3">
              <div class="flex-shrink-0 w-10 h-10 bg-indigo-100 dark:bg-indigo-900 rounded-lg flex items-center justify-center">
                <Icon icon="mdi:eye" class="h-5 w-5 text-indigo-600 dark:text-indigo-400" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ $t('events.detail.views') }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  {{ event.views }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Metadata -->
        <div class="flex flex-wrap items-center gap-6 text-sm text-gray-600 dark:text-gray-400">
          <div class="flex items-center gap-2">
            <Icon icon="mdi:account" class="h-5 w-5" />
            <span>{{ event.author?.first_name }} {{ event.author?.last_name }}</span>
          </div>
          <div class="flex items-center gap-2">
            <Icon icon="mdi:calendar" class="h-5 w-5" />
            <span>{{ formatDate(event.created_at) }}</span>
          </div>
          <div v-if="event.published_at" class="flex items-center gap-2">
            <Icon icon="mdi:check-circle" class="h-5 w-5" />
            <span>{{ $t('events.detail.published') }} {{ formatRelativeTime(event.published_at) }}</span>
          </div>
        </div>

        <!-- Tags -->
        <div v-if="event.tags && event.tags.length > 0" class="flex flex-wrap gap-2 mt-4">
          <span
            v-for="tag in event.tags"
            :key="tag.id"
            class="px-3 py-1 rounded-full text-sm font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
            :style="{ backgroundColor: tag.color + '20', color: tag.color }"
          >
            #{{ tag.name }}
          </span>
        </div>
      </div>

      <!-- Content -->
      <div class="p-8">
        <div v-if="event.description" class="prose prose-lg dark:prose-invert max-w-none">
          <TiptapRenderer :content="event.description" />
        </div>
      </div>

      <!-- External Links -->
      <div v-if="hasExternalLinks" class="p-8 border-t border-gray-200 dark:border-gray-700">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          {{ $t('events.detail.links') }}
        </h3>
        <div class="space-y-2">
          <a
            v-for="link in externalLinks"
            :key="link.url"
            :href="link.url"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-2 text-primary-600 hover:text-primary-500 dark:text-primary-400"
          >
            <Icon icon="mdi:external-link" class="h-4 w-4" />
            <span>{{ link.title || link.url }}</span>
          </a>
        </div>
      </div>

      <!-- Edit Button (for editors/admins) -->
      <div v-if="canEdit" class="p-8 border-t border-gray-200 dark:border-gray-700">
        <button
          @click="editEvent"
          class="px-4 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors flex items-center gap-2"
        >
          <Icon icon="mdi:pencil" class="h-5 w-5" />
          {{ $t('events.detail.edit') }}
        </button>
      </div>
    </article>

    <!-- Error State -->
    <div v-else class="text-center py-12">
      <Icon icon="mdi:alert-circle" class="h-16 w-16 mx-auto text-red-500 mb-4" />
      <p class="text-gray-600 dark:text-gray-400 text-lg">
        {{ $t('events.detail.notFound') }}
      </p>
      <button
        @click="$router.push({ name: 'EventsCenter' })"
        class="mt-4 px-6 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
      >
        {{ $t('events.detail.backToList') }}
      </button>
    </div>

    <!-- Feedback and Comments Section -->
    <div v-if="event" class="mt-12 space-y-8">
      <!-- Feedback Widget -->
      <FeedbackWidget
        entity-type="event"
        :entity-id="event.id"
      />

      <!-- Comments Section -->
      <CommentSection
        entity-type="event"
        :entity-id="event.id"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Icon } from '@iconify/vue'
import { useEventsStore } from '@/stores/events'
import { useAuthStore } from '@/stores/auth'
import TiptapRenderer from '@/components/news/TiptapRenderer.vue'
import FeedbackWidget from '@/components/feedback/FeedbackWidget.vue'
import CommentSection from '@/components/comments/CommentSection.vue'

const route = useRoute()
const router = useRouter()
const eventsStore = useEventsStore()
const authStore = useAuthStore()

const { t } = useI18n()

// Computed
const event = computed(() => eventsStore.currentEvent)
const canEdit = computed(() => {
  const user = authStore.user
  return user?.role === 'admin' ||
         user?.role === 'editor' ||
         (user?.admin_of_groups && user.admin_of_groups.length > 0)
})

const isPastEvent = computed(() => {
  if (!event.value) return false
  const eventDate = new Date(event.value.start_date)
  const now = new Date()
  return eventDate < now
})

const hasExternalLinks = computed(() => {
  if (!event.value?.external_links) return false
  try {
    const links = JSON.parse(event.value.external_links)
    return Array.isArray(links) && links.length > 0
  } catch {
    return false
  }
})

const externalLinks = computed(() => {
  if (!event.value?.external_links) return []
  try {
    return JSON.parse(event.value.external_links)
  } catch {
    return []
  }
})

// Methods
const fetchEvent = async () => {
  try {
    const slug = route.params.slug
    await eventsStore.fetchEventBySlug(slug)
  } catch (error) {
    console.error('Error fetching event:', error)
  }
}

const editEvent = () => {
  const user = authStore.user
  if (user?.role === 'admin' || user?.role === 'editor') {
    router.push({ name: 'AdminEventEdit', params: { slug: event.value.slug } })
  } else if (user?.admin_of_groups && user.admin_of_groups.length > 0) {
    router.push({ name: 'GroupAdminEventEdit', params: { slug: event.value.slug } })
  }
}

const formatDateRange = (startDate, endDate) => {
  const start = new Date(startDate)
  const end = endDate ? new Date(endDate) : null
  
  const options = { 
    year: 'numeric', 
    month: 'long', 
    day: 'numeric' 
  }
  
  if (end && start.toDateString() !== end.toDateString()) {
    return `${start.toLocaleDateString('fr-FR', options)} - ${end.toLocaleDateString('fr-FR', options)}`
  }
  
  return start.toLocaleDateString('fr-FR', options)
}

const formatTimeRange = (startDate, endDate) => {
  const start = new Date(startDate)
  const end = endDate ? new Date(endDate) : null
  
  const timeOptions = { 
    hour: '2-digit', 
    minute: '2-digit' 
  }
  
  if (end) {
    return `${start.toLocaleTimeString('fr-FR', timeOptions)} - ${end.toLocaleTimeString('fr-FR', timeOptions)}`
  }
  
  return start.toLocaleTimeString('fr-FR', timeOptions)
}

const formatDate = (dateString) => {
  if (!dateString) return ''
  const date = new Date(dateString)
  return date.toLocaleDateString('fr-FR', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const formatRelativeTime = (dateString) => {
  const date = new Date(dateString)
  const now = new Date()
  const diffInSeconds = Math.floor((now - date) / 1000)
  
  if (diffInSeconds < 60) return t('time.justNow')
  if (diffInSeconds < 3600) return t('time.minutesAgo', { count: Math.floor(diffInSeconds / 60) })
  if (diffInSeconds < 86400) return t('time.hoursAgo', { count: Math.floor(diffInSeconds / 3600) })
  if (diffInSeconds < 2592000) return t('time.daysAgo', { count: Math.floor(diffInSeconds / 86400) })
  
  return date.toLocaleDateString('fr-FR', { year: 'numeric', month: 'short', day: 'numeric' })
}

const formatRecurrenceRule = (recurrenceRule) => {
  try {
    const rule = JSON.parse(recurrenceRule)
    const type = rule.type || 'custom'
    const interval = rule.interval || 1
    
    const typeLabels = {
      daily: t('events.recurrence.daily'),
      weekly: t('events.recurrence.weekly'),
      monthly: t('events.recurrence.monthly'),
      yearly: t('events.recurrence.yearly')
    }
    
    const typeLabel = typeLabels[type] || type
    const intervalLabel = interval > 1 ? t('events.recurrence.every', { interval }) : ''
    
    return `${intervalLabel} ${typeLabel}`.trim()
  } catch {
    return t('events.recurrence.custom')
  }
}

const getSummary = (content) => {
  if (!content) return ''
  
  try {
    // Try to parse as JSON (Tiptap format)
    const json = typeof content === 'string' ? JSON.parse(content) : content
    if (json && json.type === 'doc') {
      let text = ''
      const traverse = (node) => {
        if (node.type === 'text') {
          text += node.text
        }
        if (node.content) {
          node.content.forEach(traverse)
        }
      }
      traverse(json)
      return text
    }
  } catch (e) {
    // Not valid JSON, fall back to HTML stripping
  }

  // Use DOMParser instead of innerHTML for security
  const parser = new DOMParser()
  const doc = parser.parseFromString(content, 'text/html')
  return doc.body.textContent || doc.body.innerText || ''
}

const getVisibilityLabel = (targetGroups) => {
  if (!targetGroups || targetGroups.length === 0) {
    return t('events.visibility.public')
  }
  if (targetGroups.length === 1) {
    return targetGroups[0].name
  }
  return t('events.visibility.groupsCount', { count: targetGroups.length })
}

const getGroupsTooltip = (targetGroups) => {
  if (!targetGroups || targetGroups.length === 0) {
    return t('events.visibility.tooltip.public')
  }
  const groupNames = targetGroups.map(g => g.name).join(', ')
  return t('events.visibility.tooltip.groups', { groups: groupNames })
}

// Lifecycle
onMounted(() => {
  fetchEvent()
})
</script>