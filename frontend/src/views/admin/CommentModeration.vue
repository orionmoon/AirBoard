<template>
  <div class="comment-moderation p-6">
    <!-- Header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white mb-2">
        {{ $t('moderation.title') }}
      </h1>
      <p class="text-gray-600 dark:text-gray-400">
        {{ $t('moderation.description') }}
      </p>
    </div>

    <!-- Tabs -->
    <div class="mb-6 border-b border-gray-200 dark:border-gray-700">
      <nav class="-mb-px flex space-x-8">
        <button
          :class="[
            'py-4 px-1 border-b-2 font-medium text-sm',
            activeTab === 'pending'
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
          ]"
          @click="activeTab = 'pending'"
        >
          {{ $t('moderation.pending') }}
          <span
            v-if="pendingCount > 0"
            class="ml-2 px-2 py-1 rounded-full bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300 text-xs"
          >
            {{ pendingCount }}
          </span>
        </button>
        <button
          :class="[
            'py-4 px-1 border-b-2 font-medium text-sm',
            activeTab === 'settings'
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
          ]"
          @click="activeTab = 'settings'"
        >
          {{ $t('moderation.settings') }}
        </button>
      </nav>
    </div>

    <!-- Pending Comments Tab -->
    <div v-if="activeTab === 'pending'">
      <div v-if="loading" class="text-center py-12">
        <Icon icon="mdi:loading" class="animate-spin h-12 w-12 mx-auto text-blue-600" />
      </div>

      <div v-else-if="pendingComments.length === 0" class="text-center py-12">
        <Icon icon="mdi:check-circle-outline" class="h-16 w-16 mx-auto text-green-500 mb-4" />
        <p class="text-gray-600 dark:text-gray-400">
          {{ $t('moderation.no_pending') }}
        </p>
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="comment in pendingComments"
          :key="comment.id"
          class="bg-white dark:bg-gray-800 rounded-lg p-6 border border-gray-200 dark:border-gray-700"
        >
          <!-- Comment Info -->
          <div class="flex items-start justify-between mb-4">
            <div>
              <p class="font-medium text-gray-900 dark:text-white">
                {{ getUserName(comment.user) }}
              </p>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ formatDate(comment.created_at) }} •
                <span class="capitalize">{{ comment.entity_type }}</span> #{{ comment.entity_id }}
              </p>
            </div>
            <span
              v-if="comment.is_flagged"
              class="px-3 py-1 rounded-full bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 text-sm"
            >
              <Icon icon="mdi:flag" class="inline h-4 w-4 mr-1" />
              {{ $t('moderation.flagged') }}
            </span>
          </div>

          <!-- Comment Content -->
          <div class="mb-4 p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <p class="text-gray-700 dark:text-gray-300 whitespace-pre-wrap">
              {{ comment.content }}
            </p>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-3">
            <button
              class="flex items-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg
                     hover:bg-green-700 transition-colors"
              @click="moderateComment(comment.id, true, false)"
            >
              <Icon icon="mdi:check" class="h-5 w-5" />
              {{ $t('moderation.approve') }}
            </button>
            <button
              class="flex items-center gap-2 px-4 py-2 bg-yellow-600 text-white rounded-lg
                     hover:bg-yellow-700 transition-colors"
              @click="moderateComment(comment.id, false, true)"
            >
              <Icon icon="mdi:flag" class="h-5 w-5" />
              {{ $t('moderation.flag') }}
            </button>
            <button
              class="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg
                     hover:bg-red-700 transition-colors"
              @click="deleteComment(comment.id)"
            >
              <Icon icon="mdi:delete" class="h-5 w-5" />
              {{ $t('moderation.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Settings Tab -->
    <div v-if="activeTab === 'settings'">
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 border border-gray-200 dark:border-gray-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-6">
          {{ $t('moderation.settings_title') }}
        </h2>

        <div class="space-y-6">
          <!-- Global Comments Toggle -->
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-gray-900 dark:text-white">
                {{ $t('moderation.enable_comments') }}
              </label>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {{ $t('moderation.enable_comments_desc') }}
              </p>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                v-model="settings.comments_enabled"
                type="checkbox"
                class="sr-only peer"
              >
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300
                          dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700
                          peer-checked:after:translate-x-full peer-checked:after:border-white
                          after:content-[''] after:absolute after:top-[2px] after:left-[2px]
                          after:bg-white after:border-gray-300 after:border after:rounded-full
                          after:h-5 after:w-5 after:transition-all dark:border-gray-600
                          peer-checked:bg-blue-600"></div>
            </label>
          </div>

          <hr class="border-gray-200 dark:border-gray-700">

          <!-- News Comments -->
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-gray-900 dark:text-white">
                {{ $t('moderation.news_comments') }}
              </label>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {{ $t('moderation.news_comments_desc') }}
              </p>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                v-model="settings.news_comments_enabled"
                type="checkbox"
                class="sr-only peer"
                :disabled="!settings.comments_enabled"
              >
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300
                          dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700
                          peer-checked:after:translate-x-full peer-checked:after:border-white
                          after:content-[''] after:absolute after:top-[2px] after:left-[2px]
                          after:bg-white after:border-gray-300 after:border after:rounded-full
                          after:h-5 after:w-5 after:transition-all dark:border-gray-600
                          peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
            </label>
          </div>

          <!-- App Comments -->
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-gray-900 dark:text-white">
                {{ $t('moderation.app_comments') }}
              </label>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {{ $t('moderation.app_comments_desc') }}
              </p>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                v-model="settings.app_comments_enabled"
                type="checkbox"
                class="sr-only peer"
                :disabled="!settings.comments_enabled"
              >
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300
                          dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700
                          peer-checked:after:translate-x-full peer-checked:after:border-white
                          after:content-[''] after:absolute after:top-[2px] after:left-[2px]
                          after:bg-white after:border-gray-300 after:border after:rounded-full
                          after:h-5 after:w-5 after:transition-all dark:border-gray-600
                          peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
            </label>
          </div>

          <!-- Event Comments -->
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-gray-900 dark:text-white">
                {{ $t('moderation.event_comments') }}
              </label>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {{ $t('moderation.event_comments_desc') }}
              </p>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                v-model="settings.event_comments_enabled"
                type="checkbox"
                class="sr-only peer"
                :disabled="!settings.comments_enabled"
              >
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300
                          dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700
                          peer-checked:after:translate-x-full peer-checked:after:border-white
                          after:content-[''] after:absolute after:top-[2px] after:left-[2px]
                          after:bg-white after:border-gray-300 after:border after:rounded-full
                          after:h-5 after:w-5 after:transition-all dark:border-gray-600
                          peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
            </label>
          </div>

          <hr class="border-gray-200 dark:border-gray-700">

          <!-- Require Moderation -->
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-gray-900 dark:text-white">
                {{ $t('moderation.require_moderation') }}
              </label>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {{ $t('moderation.require_moderation_desc') }}
              </p>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                v-model="settings.require_moderation"
                type="checkbox"
                class="sr-only peer"
              >
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300
                          dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700
                          peer-checked:after:translate-x-full peer-checked:after:border-white
                          after:content-[''] after:absolute after:top-[2px] after:left-[2px]
                          after:bg-white after:border-gray-300 after:border after:rounded-full
                          after:h-5 after:w-5 after:transition-all dark:border-gray-600
                          peer-checked:bg-blue-600"></div>
            </label>
          </div>

          <!-- Max Comment Length -->
          <div>
            <label class="font-medium text-gray-900 dark:text-white block mb-2">
              {{ $t('moderation.max_length') }}
            </label>
            <input
              v-model.number="settings.max_comment_length"
              type="number"
              min="100"
              max="5000"
              class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg
                     focus:ring-2 focus:ring-blue-500 focus:border-transparent
                     bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
            >
            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
              {{ $t('moderation.max_length_desc') }}
            </p>
          </div>
        </div>

        <!-- Save Button -->
        <div class="mt-6">
          <button
            class="w-full px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700
                   transition-colors font-medium"
            @click="saveSettings"
          >
            {{ $t('moderation.save_settings') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { commentsService } from '@/services/api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const activeTab = ref('pending')
const loading = ref(false)
const pendingComments = ref([])
const settings = ref({
  comments_enabled: true,
  news_comments_enabled: true,
  app_comments_enabled: false,
  event_comments_enabled: true,
  require_moderation: false,
  max_comment_length: 1000
})

const pendingCount = computed(() => pendingComments.value.length)

const getUserName = (user) => {
  if (!user) return t('comments.anonymous')
  return user.first_name && user.last_name
    ? `${user.first_name} ${user.last_name}`
    : user.username || user.email
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleDateString('fr-FR', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const loadPendingComments = async () => {
  loading.value = true
  try {
    const response = await commentsService.getPendingComments()
    pendingComments.value = response.comments || []
  } catch (error) {
    console.error('Erreur lors du chargement des commentaires en attente:', error)
  } finally {
    loading.value = false
  }
}

const loadSettings = async () => {
  try {
    const response = await commentsService.getSettings()
    settings.value = response
  } catch (error) {
    console.error('Erreur lors du chargement des paramètres:', error)
  }
}

const moderateComment = async (commentId, isApproved, isFlagged) => {
  try {
    await commentsService.moderateComment({
      comment_id: commentId,
      is_approved: isApproved,
      is_flagged: isFlagged
    })
    await loadPendingComments()
  } catch (error) {
    console.error('Erreur lors de la modération du commentaire:', error)
    alert('Erreur lors de la modération du commentaire')
  }
}

const deleteComment = async (commentId) => {
  if (!confirm('Êtes-vous sûr de vouloir supprimer ce commentaire ?')) return

  try {
    await commentsService.deleteComment(commentId)
    await loadPendingComments()
  } catch (error) {
    console.error('Erreur lors de la suppression du commentaire:', error)
    alert('Erreur lors de la suppression du commentaire')
  }
}

const saveSettings = async () => {
  try {
    await commentsService.updateSettings(settings.value)
    alert('Paramètres mis à jour avec succès')
  } catch (error) {
    console.error('Erreur lors de la mise à jour des paramètres:', error)
    alert('Erreur lors de la mise à jour des paramètres')
  }
}

onMounted(() => {
  loadPendingComments()
  loadSettings()
})
</script>
