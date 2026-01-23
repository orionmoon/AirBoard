<template>
  <div class="content-area">
    <!-- Header -->
    <div class="page-header">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="page-title">{{ $t('groupAdmin.eventsManagement.title') }}</h1>
          <p class="page-subtitle">{{ $t('groupAdmin.eventsManagement.subtitle') }}</p>
        </div>
        <router-link to="/group-admin/events/new" class="btn btn-primary">
          <Icon icon="mdi:plus" class="h-4 w-4 mr-2" />
          {{ $t('groupAdmin.eventsManagement.newEvent') }}
        </router-link>
      </div>
    </div>

    <!-- Info Alert -->
    <div class="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-300 dark:border-blue-600 rounded-lg">
      <div class="flex items-start">
        <Icon icon="mdi:information" class="h-5 w-5 text-blue-600 dark:text-blue-500 mr-3 mt-0.5" />
        <div>
          <h3 class="text-blue-800 dark:text-blue-500 font-medium">{{ $t('groupAdmin.eventsManagement.infoTitle') }}</h3>
          <p class="text-sm text-blue-700 dark:text-blue-200 mt-1">
            {{ $t('groupAdmin.eventsManagement.infoDescription') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Filters -->
    <div class="card mb-6">
      <div class="flex flex-col sm:flex-row gap-4">
        <!-- Search -->
        <div class="flex-1 min-w-[200px]">
          <input
            v-model="filters.search"
            type="text"
            :placeholder="$t('groupAdmin.eventsManagement.searchPlaceholder')"
            class="input"
          />
        </div>

        <!-- Category filter -->
        <select v-model="filters.category_id" class="input w-full sm:w-48">
          <option value="">{{ $t('groupAdmin.eventsManagement.categoryFilter.all') }}</option>
          <option v-for="cat in categories" :key="cat.id" :value="cat.id">
            {{ cat.name }}
          </option>
        </select>

        <!-- Status filter -->
        <select v-model="filters.status" class="input w-full sm:w-40">
          <option value="">{{ $t('groupAdmin.eventsManagement.statusFilter.all') }}</option>
          <option value="published">{{ $t('groupAdmin.eventsManagement.statusFilter.published') }}</option>
          <option value="draft">{{ $t('groupAdmin.eventsManagement.statusFilter.draft') }}</option>
        </select>

        <!-- Event Type filter -->
        <select v-model="filters.event_type" class="input w-full sm:w-40">
          <option value="">{{ $t('groupAdmin.eventsManagement.typeFilter.all') }}</option>
          <option value="recurring">{{ $t('groupAdmin.eventsManagement.typeFilter.recurring') }}</option>
          <option value="all_day">{{ $t('groupAdmin.eventsManagement.typeFilter.allDay') }}</option>
          <option value="upcoming">{{ $t('groupAdmin.eventsManagement.typeFilter.upcoming') }}</option>
          <option value="past">{{ $t('groupAdmin.eventsManagement.typeFilter.past') }}</option>
        </select>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="isLoading" class="flex justify-center py-12">
      <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-gray-400" />
    </div>

    <!-- Events List -->
    <div v-else-if="eventsList.length > 0" class="space-y-4">
      <div
        v-for="event in eventsList"
        :key="event.id"
        class="card hover:shadow-lg transition-shadow"
      >
        <div class="flex items-start gap-4">
          <!-- Status indicator -->
          <div class="flex-shrink-0 pt-1">
            <div
              :class="event.is_published ? 'bg-green-500' : 'bg-gray-400'"
              class="h-3 w-3 rounded-full"
              :title="event.is_published ? $t('groupAdmin.eventsManagement.status.published') : $t('groupAdmin.eventsManagement.status.draft')"
            ></div>
          </div>

          <!-- Content -->
          <div class="flex-1 min-w-0">
            <div class="flex items-start justify-between gap-4">
              <div class="flex-1">
                <div class="flex items-center gap-2 mb-1">
                  <!-- Recurring indicator -->
                  <Icon
                    v-if="event.is_recurring"
                    icon="mdi:repeat"
                    class="h-4 w-4 text-green-500"
                    :title="$t('groupAdmin.eventsManagement.indicators.recurring')"
                  />

                  <!-- All day indicator -->
                  <Icon
                    v-if="event.is_all_day"
                    icon="mdi:weather-sunny"
                    class="h-4 w-4 text-blue-500"
                    :title="$t('groupAdmin.eventsManagement.indicators.allDay')"
                  />

                  <h3
                    class="text-lg font-semibold text-gray-900 dark:text-white cursor-pointer hover:text-primary-600 dark:hover:text-primary-400 transition-colors"
                    @click="viewEvent(event)"
                  >
                    {{ event.title }}
                  </h3>

                  <!-- Priority badge -->
                  <span
                    v-if="event.priority && event.priority !== 'normal'"
                    :class="{
                      'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200': event.priority === 'urgent',
                      'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200': event.priority === 'important'
                    }"
                    class="px-2 py-0.5 rounded text-xs font-medium"
                  >
                    {{ $t(`groupAdmin.eventsManagement.priority.${event.priority}`) }}
                  </span>
                </div>

                <p v-if="event.description" class="text-sm text-gray-600 dark:text-gray-400 mb-2 line-clamp-2">
                  {{ extractTextFromTiptap(event.description) }}
                </p>

                <div class="flex flex-wrap items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
                  <!-- Category -->
                  <div v-if="event.category" class="flex items-center gap-1">
                    <Icon :icon="event.category.icon || 'mdi:folder'" class="h-4 w-4" />
                    <span>{{ event.category.name }}</span>
                  </div>

                  <!-- Author -->
                  <div class="flex items-center gap-1">
                    <Icon icon="mdi:account" class="h-4 w-4" />
                    <span>{{ event.author?.first_name || event.author?.username || $t('groupAdmin.eventsManagement.author.unknown') }} {{ event.author?.last_name || '' }}</span>
                  </div>

                  <!-- Date Range -->
                  <div class="flex items-center gap-1">
                    <Icon icon="mdi:calendar" class="h-4 w-4" />
                    <span>{{ formatDateRange(event.start_date, event.end_date) }}</span>
                  </div>

                  <!-- Time -->
                  <div v-if="!event.is_all_day" class="flex items-center gap-1">
                    <Icon icon="mdi:clock" class="h-4 w-4" />
                    <span>{{ formatTimeRange(event.start_date, event.end_date) }}</span>
                  </div>

                  <!-- Location -->
                  <div v-if="event.location" class="flex items-center gap-1">
                    <Icon icon="mdi:map-marker" class="h-4 w-4" />
                    <span class="line-clamp-1">{{ event.location }}</span>
                  </div>

                  <!-- Target Groups -->
                  <div v-if="event.target_groups && event.target_groups.length > 0" class="flex items-center gap-1">
                    <Icon icon="mdi:account-group" class="h-4 w-4" />
                    <div class="flex gap-1">
                      <span
                        v-for="group in event.target_groups.slice(0, 2)"
                        :key="group.id"
                        class="px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 rounded text-xs"
                      >
                        {{ group.name }}
                      </span>
                      <span v-if="event.target_groups.length > 2" class="text-xs">
                        +{{ event.target_groups.length - 2 }}
                      </span>
                    </div>
                  </div>
                  <div v-else class="flex items-center gap-1">
                    <Icon icon="mdi:earth" class="h-4 w-4" />
                    <span>{{ $t('groupAdmin.eventsManagement.visibility.public') }}</span>
                  </div>
                </div>
              </div>

              <!-- Actions -->
              <div class="flex items-center gap-2">
                <router-link
                  :to="`/group-admin/events/${event.slug}/edit`"
                  class="btn btn-secondary btn-sm"
                  :title="$t('groupAdmin.eventsManagement.actions.edit')"
                >
                  <Icon icon="mdi:pencil" class="h-4 w-4" />
                </router-link>

                <button
                  @click="togglePublish(event)"
                  class="btn btn-secondary btn-sm"
                  :title="event.is_published ? $t('groupAdmin.eventsManagement.actions.unpublish') : $t('groupAdmin.eventsManagement.actions.publish')"
                >
                  <Icon :icon="event.is_published ? 'mdi:eye-off' : 'mdi:eye'" class="h-4 w-4" />
                </button>

                <button
                  @click="confirmDelete(event)"
                  class="btn btn-danger btn-sm"
                  :title="$t('groupAdmin.eventsManagement.actions.delete')"
                >
                  <Icon icon="mdi:delete" class="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Pagination -->
      <div v-if="pagination.total_pages > 1" class="flex justify-center gap-2 mt-6">
        <button
          @click="changePage(pagination.page - 1)"
          :disabled="pagination.page === 1"
          class="btn btn-secondary btn-sm"
        >
          <Icon icon="mdi:chevron-left" class="h-4 w-4" />
        </button>

        <span class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300">
          {{ $t('groupAdmin.eventsManagement.pagination.page', { page: pagination.page, pages: pagination.total_pages }) }}
        </span>

        <button
          @click="changePage(pagination.page + 1)"
          :disabled="pagination.page === pagination.total_pages"
          class="btn btn-secondary btn-sm"
        >
          <Icon icon="mdi:chevron-right" class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="empty-state">
      <Icon icon="mdi:calendar" class="empty-state-icon" />
      <h3 class="empty-state-title">{{ $t('groupAdmin.eventsManagement.empty.title') }}</h3>
      <p class="empty-state-description">
        {{ filters.search || filters.category_id || filters.status || filters.event_type
          ? $t('groupAdmin.eventsManagement.empty.noResults')
          : $t('groupAdmin.eventsManagement.empty.createFirst')
        }}
      </p>
      <router-link v-if="!filters.search && !filters.category_id" to="/group-admin/events/new" class="btn btn-primary">
        <Icon icon="mdi:plus" class="h-4 w-4 mr-2" />
        {{ $t('groupAdmin.eventsManagement.empty.createButton') }}
      </router-link>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="showDeleteModal" class="modal-overlay">
      <div class="modal-container">
        <div class="modal-panel sm:max-w-lg sm:w-full">
          <div class="modal-header">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0">
                <Icon icon="mdi:alert" class="h-6 w-6 text-red-500" />
              </div>
              <div>
                <h3 class="modal-title">{{ $t('groupAdmin.eventsManagement.deleteConfirm.title') }}</h3>
                <p class="modal-subtitle">
                  {{ $t('groupAdmin.eventsManagement.deleteConfirm.message', { title: eventToDelete?.title }) }}
                </p>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button @click="closeDeleteModal" class="btn btn-secondary w-full sm:w-auto">
              {{ $t('common.cancel') }}
            </button>
            <button
              @click="deleteEvent"
              :disabled="deleteLoading"
              class="btn btn-danger w-full sm:w-auto"
            >
              <Icon v-if="deleteLoading" icon="mdi:loading" class="animate-spin h-4 w-4 mr-2" />
              {{ $t('common.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, getCurrentInstance } from 'vue'
import { useRouter } from 'vue-router'
import { Icon } from '@iconify/vue'
import { groupAdminEventsService, eventsService } from '@/services/api'
import { useAppStore } from '@/stores/app'

const { proxy } = getCurrentInstance()
const router = useRouter()
const appStore = useAppStore()

// State
const eventsList = ref([])
const categories = ref([])
const isLoading = ref(false)
const showDeleteModal = ref(false)
const eventToDelete = ref(null)
const deleteLoading = ref(false)

const filters = ref({
  search: '',
  category_id: '',
  status: '',
  event_type: ''
})

const pagination = ref({
  page: 1,
  limit: 20,
  total: 0,
  total_pages: 0
})

// Methods
const loadEvents = async () => {
  try {
    isLoading.value = true
    const params = {
      page: pagination.value.page,
      page_size: pagination.value.limit,
      search: filters.value.search || undefined,
      category_id: filters.value.category_id || undefined
    }

    // Handle status filter
    if (filters.value.status === 'published') {
      params.is_published = true
    } else if (filters.value.status === 'draft') {
      params.is_published = false
    }

    // Handle event type filter
    if (filters.value.event_type === 'recurring') {
      params.is_recurring = true
    } else if (filters.value.event_type === 'all_day') {
      params.is_all_day = true
    } else if (filters.value.event_type === 'upcoming') {
      params.upcoming = true
    } else if (filters.value.event_type === 'past') {
      params.past = true
    }

    const response = await groupAdminEventsService.getEvents(params)

    // Handle response structure
    eventsList.value = response.events || response.data || []
    pagination.value = {
      ...pagination.value,
      page: response.page || 1,
      total: response.total || 0,
      total_pages: response.total_pages || 1
    }
  } catch (error) {
    console.error('Error loading events:', error)
    eventsList.value = []
    appStore.showError(proxy.$t('groupAdmin.eventsManagement.loadError'))
  } finally {
    isLoading.value = false
  }
}

const loadCategories = async () => {
  try {
    const data = await eventsService.getCategories()
    categories.value = Array.isArray(data) ? data : (data.data || [])
  } catch (error) {
    console.error('Error loading categories:', error)
    categories.value = []
  }
}

const formatDateRange = (startDate, endDate) => {
  if (!startDate) return 'N/A'
  const start = new Date(startDate)
  const end = endDate ? new Date(endDate) : null

  const options = {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  }

  if (end && start.toDateString() !== end.toDateString()) {
    return `${start.toLocaleDateString('fr-FR', options)} - ${end.toLocaleDateString('fr-FR', options)}`
  }

  return start.toLocaleDateString('fr-FR', options)
}

const formatTimeRange = (startDate, endDate) => {
  if (!startDate) return ''
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

const extractTextFromTiptap = (content) => {
  if (!content) return ''

  let contentObj = content

  // If content is a string, try to parse it as JSON
  if (typeof content === 'string') {
    try {
      contentObj = JSON.parse(content)
    } catch (e) {
      // If parsing fails, return the content as is (plain text)
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
        // Add space between paragraphs
        if (child.type === 'paragraph') {
          text += ' '
        }
      }
    }

    return text
  }

  return extractText(contentObj).trim()
}

const viewEvent = (event) => {
  if (event && event.slug) {
    router.push({ name: 'EventDetail', params: { slug: event.slug } })
  }
}

const togglePublish = async (event) => {
  try {
    const updatedEvent = { ...event, is_published: !event.is_published }
    await groupAdminEventsService.updateEvent(event.slug, updatedEvent)

    // Update local state
    const index = eventsList.value.findIndex(e => e.id === event.id)
    if (index !== -1) {
      eventsList.value[index].is_published = !event.is_published
    }

    appStore.showSuccess(
      event.is_published
        ? proxy.$t('groupAdmin.eventsManagement.unpublishedSuccess')
        : proxy.$t('groupAdmin.eventsManagement.publishedSuccess')
    )
  } catch (error) {
    console.error('Error toggling publish status:', error)
    appStore.showError(proxy.$t('groupAdmin.eventsManagement.updateError'))
  }
}

const changePage = (page) => {
  if (page >= 1 && page <= pagination.value.total_pages) {
    pagination.value.page = page
    loadEvents()
  }
}

const confirmDelete = (event) => {
  eventToDelete.value = event
  showDeleteModal.value = true
}

const closeDeleteModal = () => {
  showDeleteModal.value = false
  eventToDelete.value = null
}

const deleteEvent = async () => {
  if (!eventToDelete.value) return

  try {
    deleteLoading.value = true
    console.log('[DEBUG Events] Deleting event with ID:', eventToDelete.value.id)
    const deleteResponse = await groupAdminEventsService.deleteEvent(eventToDelete.value.id)
    console.log('[DEBUG Events] Delete response:', deleteResponse)
    closeDeleteModal()
    console.log('[DEBUG Events] Reloading events list...')
    await loadEvents()
    console.log('[DEBUG Events] Events list after reload:', eventsList.value.map(e => ({ id: e.id, title: e.title })))
    appStore.showSuccess(proxy.$t('groupAdmin.eventsManagement.deleteSuccess'))
  } catch (error) {
    console.error('[ERROR Events] Error deleting event:', error)
    appStore.showError(proxy.$t('groupAdmin.eventsManagement.deleteError'))
  } finally {
    deleteLoading.value = false
  }
}

// Watch filters
watch(
  () => [filters.value.search, filters.value.category_id, filters.value.status, filters.value.event_type],
  () => {
    pagination.value.page = 1
    loadEvents()
  }
)

onMounted(async () => {
  await loadCategories()
  await loadEvents()
})
</script>

<style scoped>
.line-clamp-2 {
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}

.line-clamp-1 {
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
  line-clamp: 1;
}
</style>
