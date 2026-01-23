<template>
  <div class="content-area">
    <!-- Header -->
    <div class="page-header">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="page-title">{{ $t('groupAdmin.newsManagement.title') }}</h1>
          <p class="page-subtitle">{{ $t('groupAdmin.newsManagement.subtitle') }}</p>
        </div>
        <router-link to="/group-admin/news/new" class="btn btn-primary">
          <Icon icon="mdi:plus" class="h-4 w-4 mr-2" />
          {{ $t('groupAdmin.newsManagement.newArticle') }}
        </router-link>
      </div>
    </div>

    <!-- Info Alert -->
    <div class="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-300 dark:border-blue-600 rounded-lg">
      <div class="flex items-start">
        <Icon icon="mdi:information" class="h-5 w-5 text-blue-600 dark:text-blue-500 mr-3 mt-0.5" />
        <div>
          <h3 class="text-blue-800 dark:text-blue-500 font-medium">{{ $t('groupAdmin.newsManagement.infoTitle') }}</h3>
          <p class="text-sm text-blue-700 dark:text-blue-200 mt-1">
            {{ $t('groupAdmin.newsManagement.infoDescription') }}
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
            :placeholder="$t('groupAdmin.newsManagement.searchPlaceholder')"
            class="input"
          />
        </div>

        <!-- Category filter -->
        <select v-model="filters.category" class="input w-full sm:w-48">
          <option value="">{{ $t('groupAdmin.newsManagement.categoryFilter.all') }}</option>
          <option v-for="cat in categories" :key="cat.id" :value="cat.id">
            {{ cat.name }}
          </option>
        </select>

        <!-- Status filter -->
        <select v-model="filters.status" class="input w-full sm:w-40">
          <option value="">{{ $t('groupAdmin.newsManagement.statusFilter.all') }}</option>
          <option value="published">{{ $t('groupAdmin.newsManagement.statusFilter.published') }}</option>
          <option value="draft">{{ $t('groupAdmin.newsManagement.statusFilter.draft') }}</option>
        </select>

        <!-- Priority filter -->
        <select v-model="filters.priority" class="input w-full sm:w-40">
          <option value="">{{ $t('groupAdmin.newsManagement.priorityFilter.all') }}</option>
          <option value="urgent">{{ $t('groupAdmin.newsManagement.priorityFilter.urgent') }}</option>
          <option value="important">{{ $t('groupAdmin.newsManagement.priorityFilter.important') }}</option>
          <option value="normal">{{ $t('groupAdmin.newsManagement.priorityFilter.normal') }}</option>
        </select>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="isLoading" class="flex justify-center py-12">
      <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-gray-400" />
    </div>

    <!-- News List -->
    <div v-else-if="newsList.length > 0" class="space-y-4">
      <div
        v-for="news in newsList"
        :key="news.id"
        class="card hover:shadow-lg transition-shadow"
      >
        <div class="flex items-start gap-4">
          <!-- Status indicator -->
          <div class="flex-shrink-0 pt-1">
            <div
              :class="news.is_published ? 'bg-green-500' : 'bg-gray-400'"
              class="h-3 w-3 rounded-full"
              :title="news.is_published ? $t('groupAdmin.newsManagement.status.published') : $t('groupAdmin.newsManagement.status.draft')"
            ></div>
          </div>

          <!-- Content -->
          <div class="flex-1 min-w-0">
            <div class="flex items-start justify-between gap-4">
              <div class="flex-1">
                <div class="flex items-center gap-2 mb-1">
                  <!-- Pin indicator -->
                  <Icon
                    v-if="news.is_pinned"
                    icon="mdi:pin"
                    class="h-4 w-4 text-yellow-500"
                  />

<h3
                    class="text-lg font-semibold text-gray-900 dark:text-white cursor-pointer hover:text-primary-600 dark:hover:text-primary-400 transition-colors"
                    @click="viewNews(news)"
                  >
                    {{ news.title }}
                  </h3>

                  <!-- Priority badge -->
                  <span
                    v-if="news.priority !== 'normal'"
                    :class="{
                      'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200': news.priority === 'urgent',
                      'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200': news.priority === 'important'
                    }"
                    class="px-2 py-0.5 rounded text-xs font-medium"
                  >
                    {{ $t(`groupAdmin.newsManagement.priority.${news.priority}`) }}
                  </span>
                </div>

                <p v-if="news.summary" class="text-sm text-gray-600 dark:text-gray-400 mb-2">
                  {{ news.summary }}
                </p>

                <div class="flex flex-wrap items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
                  <!-- Category -->
                  <div v-if="news.category" class="flex items-center gap-1">
                    <Icon :icon="news.category.icon" class="h-4 w-4" />
                    <span>{{ news.category.name }}</span>
                  </div>

                  <!-- Author -->
                  <div class="flex items-center gap-1">
                    <Icon icon="mdi:account" class="h-4 w-4" />
                    <span>{{ news.author?.username || $t('groupAdmin.newsManagement.author.unknown') }}</span>
                  </div>

                  <!-- Date -->
                  <div class="flex items-center gap-1">
                    <Icon icon="mdi:calendar" class="h-4 w-4" />
                    <span>{{ formatDate(news.published_at || news.created_at) }}</span>
                  </div>

                  <!-- Views -->
                  <div class="flex items-center gap-1">
                    <Icon icon="mdi:eye" class="h-4 w-4" />
                    <span>{{ news.view_count }}</span>
                  </div>

                  <!-- Target Groups -->
                  <div v-if="news.target_groups && news.target_groups.length > 0" class="flex items-center gap-1">
                    <Icon icon="mdi:account-group" class="h-4 w-4" />
                    <div class="flex gap-1">
                      <span
                        v-for="group in news.target_groups.slice(0, 2)"
                        :key="group.id"
                        class="px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 rounded text-xs"
                      >
                        {{ group.name }}
                      </span>
                      <span v-if="news.target_groups.length > 2" class="text-xs">
                        +{{ news.target_groups.length - 2 }}
                      </span>
                    </div>
                  </div>

                  <!-- Tags -->
                  <div v-if="news.tags && news.tags.length > 0" class="flex items-center gap-1">
                    <Icon icon="mdi:tag" class="h-4 w-4" />
                    <div class="flex gap-1">
                      <span
                        v-for="tag in news.tags.slice(0, 2)"
                        :key="tag.id"
                        class="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-xs"
                      >
                        {{ tag.name }}
                      </span>
                      <span v-if="news.tags.length > 2" class="text-xs">
                        +{{ news.tags.length - 2 }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Actions -->
              <div class="flex items-center gap-2">
                <router-link
                  :to="`/group-admin/news/${news.slug}/edit`"
                  class="btn btn-secondary btn-sm"
                  :title="$t('groupAdmin.newsManagement.actions.edit')"
                >
                  <Icon icon="mdi:pencil" class="h-4 w-4" />
                </router-link>

                <button
                  @click="confirmDelete(news)"
                  class="btn btn-danger btn-sm"
                  :title="$t('groupAdmin.newsManagement.actions.delete')"
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
          {{ $t('groupAdmin.newsManagement.pagination.page', { page: pagination.page, pages: pagination.total_pages }) }}
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
      <Icon icon="mdi:newspaper-variant" class="empty-state-icon" />
      <h3 class="empty-state-title">{{ $t('groupAdmin.newsManagement.empty.title') }}</h3>
      <p class="empty-state-description">
        {{ filters.search || filters.category || filters.status || filters.priority
          ? $t('groupAdmin.newsManagement.empty.noResults')
          : $t('groupAdmin.newsManagement.empty.createFirst')
        }}
      </p>
      <router-link v-if="!filters.search && !filters.category" to="/group-admin/news/new" class="btn btn-primary">
        <Icon icon="mdi:plus" class="h-4 w-4 mr-2" />
        {{ $t('groupAdmin.newsManagement.empty.createButton') }}
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
                <h3 class="modal-title">{{ $t('groupAdmin.newsManagement.deleteConfirm.title') }}</h3>
                <p class="modal-subtitle">
                  {{ $t('groupAdmin.newsManagement.deleteConfirm.message', { title: newsToDelete?.title }) }}
                </p>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button @click="closeDeleteModal" class="btn btn-secondary w-full sm:w-auto">
              {{ $t('common.cancel') }}
            </button>
            <button
              @click="deleteNews"
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
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Icon } from '@iconify/vue'
import { newsService, groupAdminService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

// State
const newsList = ref([])
const categories = ref([])
const isLoading = ref(false)
const showDeleteModal = ref(false)
const newsToDelete = ref(null)
const deleteLoading = ref(false)

const filters = ref({
  search: '',
  category: '',
  status: '',
  priority: ''
})

const pagination = ref({
  page: 1,
  limit: 20,
  total: 0,
  total_pages: 0
})

// Methods
const loadNews = async () => {
  try {
    isLoading.value = true
    const params = {
      page: pagination.value.page,
      page_size: pagination.value.limit,
      search: filters.value.search || undefined,
      category_id: filters.value.category || undefined,
      priority: filters.value.priority || undefined
    }

    console.log('[DEBUG] Loading news with params:', params)
    const response = await groupAdminService.getNews(params)
    console.log('[DEBUG] News response:', response)
    console.log('[DEBUG] News count:', response.news?.length || 0)
    newsList.value = response.news || []
    pagination.value = {
      ...pagination.value,
      page: response.page || 1,
      total: response.total || 0,
      total_pages: response.total_pages || 1
    }
  } catch (error) {
    console.error('[ERROR] Error loading news:', error)
    newsList.value = []
  } finally {
    isLoading.value = false
  }
}

const loadCategories = async () => {
  try {
    const data = await newsService.getCategories()
    categories.value = Array.isArray(data) ? data : (data.data || [])
  } catch (error) {
    console.error('Error loading categories:', error)
    categories.value = []
  }
}

const formatDate = (dateString) => {
  if (!dateString) return 'N/A'
  const date = new Date(dateString)
  return date.toLocaleDateString('fr-FR', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

// View news detail
const viewNews = (news) => {
  if (news && news.slug) {
    router.push({ name: 'NewsDetail', params: { slug: news.slug } })
  }
}

const changePage = (page) => {
  if (page >= 1 && page <= pagination.value.total_pages) {
    pagination.value.page = page
    loadNews()
  }
}

const confirmDelete = (news) => {
  newsToDelete.value = news
  showDeleteModal.value = true
}

const closeDeleteModal = () => {
  showDeleteModal.value = false
  newsToDelete.value = null
}

const deleteNews = async () => {
  if (!newsToDelete.value) return

  try {
    deleteLoading.value = true
    console.log('[DEBUG] Deleting news with ID:', newsToDelete.value.id)
    const deleteResponse = await groupAdminService.deleteNews(newsToDelete.value.id)
    console.log('[DEBUG] Delete response:', deleteResponse)
    closeDeleteModal()
    console.log('[DEBUG] Reloading news list...')
    await loadNews()
    console.log('[DEBUG] News list after reload:', newsList.value.map(n => ({ id: n.id, title: n.title })))
    appStore.showSuccess($t('groupAdmin.newsManagement.deleteSuccess'))
  } catch (error) {
    console.error('[ERROR] Error deleting news:', error)
    appStore.showError($t('groupAdmin.newsManagement.deleteError'))
  } finally {
    deleteLoading.value = false
  }
}

// Watch filters
watch(
  () => [filters.value.search, filters.value.category, filters.value.status, filters.value.priority],
  () => {
    pagination.value.page = 1
    loadNews()
  }
)

onMounted(async () => {
  await loadCategories()
  await loadNews()
})
</script>
