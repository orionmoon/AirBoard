<template>
  <div class="fixed inset-0 z-50 overflow-y-auto" aria-labelledby="modal-title" role="dialog" aria-modal="true">
    <!-- Backdrop -->
    <div class="flex items-end justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
      <div
        class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
        aria-hidden="true"
        @click="$emit('close')"
      ></div>

      <span class="hidden sm:inline-block sm:align-middle sm:h-screen" aria-hidden="true">&#8203;</span>

      <!-- Modal Panel -->
      <div
        class="inline-block align-bottom bg-white dark:bg-gray-800 rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-2xl sm:w-full"
      >
        <!-- Header -->
        <div class="bg-white dark:bg-gray-800 px-4 pt-5 pb-4 sm:p-6 sm:pb-4 border-b border-gray-200 dark:border-gray-700">
          <div class="sm:flex sm:items-start justify-between">
            <div class="flex items-center gap-4">
              <div
                class="mx-auto flex-shrink-0 flex items-center justify-center h-12 w-12 rounded-lg sm:mx-0 sm:h-12 sm:w-12 shadow-sm"
                :style="{ backgroundColor: app.color || '#6366f1' }"
              >
                <Icon :icon="app.icon || 'mdi:application'" class="h-6 w-6 text-white" />
              </div>
              <div class="mt-3 text-center sm:mt-0 sm:text-left">
                <h3 class="text-xl leading-6 font-semibold text-gray-900 dark:text-white" id="modal-title">
                  {{ app.name }}
                </h3>
                <div class="mt-1">
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ app.description || $t('applications.noDescription') }}
                  </p>
                </div>
              </div>
            </div>
            <button
              @click="$emit('close')"
              class="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300 focus:outline-none"
            >
              <Icon icon="mdi:close" class="h-6 w-6" />
            </button>
          </div>
        </div>

        <!-- Body -->
        <div class="px-4 py-5 sm:p-6">
          <!-- Action Buttons -->
          <div class="flex space-x-3 mb-8">
            <a
              :href="app.url"
              :target="app.open_in_new_tab ? '_blank' : '_self'"
              class="flex-1 flex items-center justify-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              <Icon icon="mdi:open-in-new" class="mr-2 h-5 w-5" />
              {{ $t('common.open') }}
            </a>
            <button
              @click="toggleFavorite"
              class="flex items-center justify-center px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              <Icon
                :icon="isFavorite ? 'mdi:star' : 'mdi:star-outline'"
                class="mr-2 h-5 w-5"
                :class="isFavorite ? 'text-yellow-500' : 'text-gray-400 dark:text-gray-500'"
              />
              {{ isFavorite ? $t('common.removeFromFavorites') : $t('common.addToFavorites') }}
            </button>
          </div>

          <!-- Comments Section -->
          <div class="border-t border-gray-200 dark:border-gray-700 pt-6">
            <CommentSection
              entity-type="application"
              :entity-id="app.id"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Icon } from '@iconify/vue'
import { useFavoritesStore } from '@/stores/favorites'
import CommentSection from '@/components/comments/CommentSection.vue'

const props = defineProps({
  app: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['close'])

const favoritesStore = useFavoritesStore()

const isFavorite = computed(() => {
  return favoritesStore.isFavorite(props.app.id)
})

const toggleFavorite = async () => {
  await favoritesStore.toggleFavorite(props.app.id)
}
</script>
