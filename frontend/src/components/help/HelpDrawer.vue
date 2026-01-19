<template>
  <Teleport to="body">
    <!-- Backdrop -->
    <Transition name="fade">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-40 bg-black/50"
        @click="close"
      />
    </Transition>

    <!-- Modal -->
    <Transition name="modal">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <div class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-4xl max-h-[85vh] flex flex-col overflow-hidden">
          <!-- Header -->
          <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-blue-500 to-blue-600 flex-shrink-0">
            <div class="flex items-center gap-3">
              <div class="h-10 w-10 bg-white rounded-lg flex items-center justify-center">
                <Icon icon="mdi:help-circle" class="h-6 w-6 text-blue-600" />
              </div>
              <div>
                <h2 class="text-lg font-semibold text-white">{{ $t('help.title') }}</h2>
                <p class="text-sm text-blue-100">{{ $t('help.subtitle') }}</p>
              </div>
            </div>
            <button
              @click="close"
              class="text-white hover:text-gray-200 transition-colors p-2 rounded-lg hover:bg-white/10"
            >
              <Icon icon="mdi:close" class="h-6 w-6" />
            </button>
          </div>

          <!-- Navigation Tabs - Only show if multiple guides available -->
          <div v-if="availableGuides.length > 1" class="border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
            <nav class="flex overflow-x-auto px-4 -mb-px">
              <button
                v-for="guide in availableGuides"
                :key="guide.id"
                @click="selectedGuide = guide.id"
                :class="[
                  'flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 whitespace-nowrap transition-colors',
                  selectedGuide === guide.id
                    ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'
                ]"
              >
                <Icon :icon="guide.icon" class="h-4 w-4" />
                {{ guide.label }}
              </button>
            </nav>
          </div>

          <!-- Content -->
          <div class="flex-1 overflow-hidden flex min-h-0">
            <!-- Sidebar Navigation -->
            <div class="w-56 border-r border-gray-200 dark:border-gray-700 overflow-y-auto bg-gray-50 dark:bg-gray-900/50 flex-shrink-0">
              <div class="p-3">
                <div v-for="section in currentGuide?.sections" :key="section.id" class="mb-4">
                  <button
                    @click="toggleSection(section.id)"
                    class="w-full flex items-center justify-between px-3 py-2 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
                  >
                    <span>{{ section.title }}</span>
                    <Icon
                      :icon="expandedSections.includes(section.id) ? 'mdi:chevron-down' : 'mdi:chevron-right'"
                      class="h-4 w-4"
                    />
                  </button>
                  <Transition name="expand">
                    <ul v-if="expandedSections.includes(section.id)" class="mt-1 space-y-1">
                      <li v-for="item in section.items" :key="item.id">
                        <button
                          @click="selectPage(item)"
                          :class="[
                            'w-full text-left px-3 py-2 text-sm rounded-lg transition-colors',
                            selectedPage?.id === item.id
                              ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 font-medium'
                              : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800'
                          ]"
                        >
                          {{ item.title }}
                        </button>
                      </li>
                    </ul>
                  </Transition>
                </div>
              </div>
            </div>

            <!-- Content Area -->
            <div class="flex-1 overflow-y-auto">
              <div v-if="loading" class="flex items-center justify-center h-full">
                <div class="text-center">
                  <Icon icon="mdi:loading" class="h-8 w-8 text-blue-500 animate-spin mx-auto mb-2" />
                  <p class="text-gray-500 dark:text-gray-400">{{ $t('help.loading') }}</p>
                </div>
              </div>
              <div v-else-if="selectedPage" class="p-6">
                <!-- Page Header -->
                <div class="mb-6">
                  <h1 class="text-2xl font-bold text-gray-900 dark:text-white mb-2">
                    {{ selectedPage.title }}
                  </h1>
                  <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
                    <Icon :icon="currentGuide?.icon" class="h-4 w-4" />
                    <span>{{ currentGuide?.label }}</span>
                    <Icon icon="mdi:chevron-right" class="h-4 w-4" />
                    <span>{{ selectedPage.section }}</span>
                  </div>
                </div>

                <!-- Markdown Content -->
                <div
                  class="prose prose-blue dark:prose-invert max-w-none prose-headings:scroll-mt-4 prose-img:rounded-lg prose-img:shadow-lg prose-a:text-blue-600 dark:prose-a:text-blue-400"
                  v-html="renderedContent"
                />
              </div>
              <div v-else class="flex items-center justify-center h-full">
                <div class="text-center p-8">
                  <Icon icon="mdi:book-open-page-variant" class="h-16 w-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
                  <h3 class="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">
                    {{ $t('help.selectTopic') }}
                  </h3>
                  <p class="text-gray-500 dark:text-gray-400">
                    {{ $t('help.selectTopicDesc') }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="border-t border-gray-200 dark:border-gray-700 px-6 py-3 bg-gray-50 dark:bg-gray-900/50 flex-shrink-0">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-4">
                <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
                  <Icon icon="mdi:account" class="h-4 w-4" />
                  <span>{{ $t('help.yourRole') }}: <strong class="text-gray-700 dark:text-gray-300">{{ userRoleLabel }}</strong></span>
                </div>
              </div>
              <button
                @click="close"
                class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
              >
                {{ $t('common.close') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { marked } from 'marked'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue'])

const { locale, t } = useI18n()
const authStore = useAuthStore()

const loading = ref(false)
const selectedGuide = ref(null)
const selectedPage = ref(null)
const expandedSections = ref([])
const contentCache = ref({})

// User role label for display
const userRoleLabel = computed(() => {
  const role = authStore.user?.role
  if (role === 'admin') return t('users.role_admin')
  if (role === 'group_admin') return t('help.guides.groupAdmin')
  if (role === 'editor') return t('users.role_editor')
  return t('users.role_user')
})

// All guide definitions
const allGuides = computed(() => [
  {
    id: 'user',
    label: t('help.guides.user'),
    icon: 'mdi:account',
    roles: ['user', 'editor', 'group_admin', 'admin'], // Available to all
    sections: [
      {
        id: 'getting-started',
        title: t('help.sections.gettingStarted'),
        items: [
          { id: 'introduction', title: t('help.pages.introduction'), file: 'user-guide/index' }
        ]
      },
      {
        id: 'features',
        title: t('help.sections.features'),
        items: [
          { id: 'dashboard', title: t('help.pages.dashboard'), file: 'user-guide/dashboard' },
          { id: 'applications', title: t('help.pages.applications'), file: 'user-guide/applications' },
          { id: 'news-hub', title: t('help.pages.newsHub'), file: 'user-guide/news-hub' },
          { id: 'events', title: t('help.pages.events'), file: 'user-guide/events' },
          { id: 'polls', title: t('help.pages.polls'), file: 'user-guide/polls' },
          { id: 'profile', title: t('help.pages.profile'), file: 'user-guide/profile' }
        ]
      }
    ]
  },
  {
    id: 'editor',
    label: t('help.guides.editor'),
    icon: 'mdi:pencil',
    roles: ['editor', 'admin'], // Only for editors and admins
    sections: [
      {
        id: 'editor-basics',
        title: t('help.sections.basics'),
        items: [
          { id: 'editor-overview', title: t('help.pages.overview'), file: 'editor-guide/index' },
          { id: 'creating-news', title: t('help.pages.creatingNews'), file: 'editor-guide/creating-news' },
          { id: 'rich-text', title: t('help.pages.richTextEditor'), file: 'editor-guide/rich-text-editor' }
        ]
      },
      {
        id: 'editor-advanced',
        title: t('help.sections.advanced'),
        items: [
          { id: 'managing-tags', title: t('help.pages.managingTags'), file: 'editor-guide/managing-tags' },
          { id: 'best-practices', title: t('help.pages.bestPractices'), file: 'editor-guide/best-practices' }
        ]
      }
    ]
  },
  {
    id: 'group-admin',
    label: t('help.guides.groupAdmin'),
    icon: 'mdi:shield-account',
    roles: ['group_admin', 'admin'], // Only for group admins and admins
    sections: [
      {
        id: 'group-admin-basics',
        title: t('help.sections.basics'),
        items: [
          { id: 'ga-overview', title: t('help.pages.overview'), file: 'group-admin-guide/index' },
          { id: 'ga-dashboard', title: t('help.pages.dashboard'), file: 'group-admin-guide/dashboard' },
          { id: 'ga-permissions', title: t('help.pages.permissions'), file: 'group-admin-guide/permissions' }
        ]
      },
      {
        id: 'group-admin-management',
        title: t('help.sections.management'),
        items: [
          { id: 'managing-app-groups', title: t('help.pages.managingAppGroups'), file: 'group-admin-guide/managing-app-groups' },
          { id: 'managing-applications', title: t('help.pages.managingApplications'), file: 'group-admin-guide/managing-applications' },
          { id: 'group-members', title: t('help.pages.groupMembers'), file: 'group-admin-guide/group-members' }
        ]
      }
    ]
  },
  {
    id: 'admin',
    label: t('help.guides.admin'),
    icon: 'mdi:shield-crown',
    roles: ['admin'], // Only for admins
    sections: [
      {
        id: 'admin-installation',
        title: t('help.sections.installation'),
        items: [
          { id: 'installation', title: t('help.pages.installation'), file: 'getting-started/index' },
          { id: 'configuration', title: t('help.pages.configuration'), file: 'getting-started/configuration' },
          { id: 'first-steps', title: t('help.pages.firstSteps'), file: 'getting-started/first-steps' }
        ]
      },
      {
        id: 'admin-basics',
        title: t('help.sections.basics'),
        items: [
          { id: 'admin-overview', title: t('help.pages.overview'), file: 'admin-guide/index' },
          { id: 'user-management', title: t('help.pages.userManagement'), file: 'admin-guide/user-management' },
          { id: 'group-management', title: t('help.pages.groupManagement'), file: 'admin-guide/group-management' }
        ]
      },
      {
        id: 'admin-apps',
        title: t('help.sections.applications'),
        items: [
          { id: 'admin-app-groups', title: t('help.pages.appGroups'), file: 'admin-guide/app-groups' },
          { id: 'admin-applications', title: t('help.pages.applications'), file: 'admin-guide/applications' }
        ]
      },
      {
        id: 'admin-content',
        title: t('help.sections.content'),
        items: [
          { id: 'news-management', title: t('help.pages.newsManagement'), file: 'admin-guide/news-management' },
          { id: 'events-management', title: t('help.pages.eventsManagement'), file: 'admin-guide/events' },
          { id: 'polls-management', title: t('help.pages.pollsManagement'), file: 'admin-guide/polls' },
          { id: 'announcements', title: t('help.pages.announcements'), file: 'admin-guide/announcements' }
        ]
      },
      {
        id: 'admin-system',
        title: t('help.sections.system'),
        items: [
          { id: 'analytics', title: t('help.pages.analytics'), file: 'admin-guide/analytics' },
          { id: 'settings', title: t('help.pages.settings'), file: 'admin-guide/settings' }
        ]
      }
    ]
  }
])

// Filter guides based on user role
const availableGuides = computed(() => {
  const userRole = authStore.user?.role || 'user'
  return allGuides.value.filter(guide => guide.roles.includes(userRole))
})

const currentGuide = computed(() => availableGuides.value.find(g => g.id === selectedGuide.value))

const renderedContent = computed(() => {
  if (!selectedPage.value?.content) return ''

  // Configure marked options
  marked.setOptions({
    breaks: true,
    gfm: true
  })

  return marked(selectedPage.value.content)
})

// Get locale code for docs path
const docsLocale = computed(() => {
  const loc = locale.value
  const localeMap = {
    'fr': 'fr',
    'en': 'en',
    'es': 'es',
    'ar': 'ar'
  }
  return localeMap[loc] || 'en'
})

function close() {
  emit('update:modelValue', false)
}

function toggleSection(sectionId) {
  const index = expandedSections.value.indexOf(sectionId)
  if (index === -1) {
    expandedSections.value.push(sectionId)
  } else {
    expandedSections.value.splice(index, 1)
  }
}

async function selectPage(item) {
  const cacheKey = `${docsLocale.value}/${item.file}`

  // Check cache first
  if (contentCache.value[cacheKey]) {
    selectedPage.value = {
      ...item,
      content: contentCache.value[cacheKey],
      section: currentGuide.value?.sections.find(s => s.items.some(i => i.id === item.id))?.title
    }
    return
  }

  loading.value = true

  try {
    // Try to load from docs directory
    const response = await fetch(`/docs/${docsLocale.value}/${item.file}.md`)

    if (response.ok) {
      let content = await response.text()

      // Vérifier que le contenu ne contient pas de HTML (page d'erreur)
      // Rejeter si on trouve des balises HTML typiques d'une page d'erreur
      const htmlPatterns = [
        /^\s*<!DOCTYPE/i,
        /^\s*<html/i,
        /<link\s+rel=["']preconnect["']/i,
        /<script\s+type=["']module["']/i,
        /<link\s+rel=["']modulepreload["']/i
      ]

      const isHtmlContent = htmlPatterns.some(pattern => pattern.test(content))

      if (isHtmlContent) {
        console.warn(`HTML content detected instead of markdown for ${item.file}`)
        selectedPage.value = {
          ...item,
          content: t('help.contentNotAvailable'),
          section: currentGuide.value?.sections.find(s => s.items.some(i => i.id === item.id))?.title
        }
      } else {
        // Remove frontmatter if present
        content = content.replace(/^---[\s\S]*?---\n/, '')

        // Remove VitePress-specific elements
        content = content.replace(/<div class="role-badge[^"]*">[^<]*<\/div>/g, '')

        // Cache the content
        contentCache.value[cacheKey] = content

        selectedPage.value = {
          ...item,
          content,
          section: currentGuide.value?.sections.find(s => s.items.some(i => i.id === item.id))?.title
        }
      }
    } else {
      // Fallback content
      selectedPage.value = {
        ...item,
        content: t('help.contentNotAvailable'),
        section: currentGuide.value?.sections.find(s => s.items.some(i => i.id === item.id))?.title
      }
    }
  } catch (error) {
    console.error('Failed to load help content:', error)
    selectedPage.value = {
      ...item,
      content: t('help.loadError'),
      section: currentGuide.value?.sections.find(s => s.items.some(i => i.id === item.id))?.title
    }
  } finally {
    loading.value = false
  }
}

// Initialize selected guide based on user role
function initializeGuide() {
  if (availableGuides.value.length > 0) {
    // Select the most relevant guide for the user's role
    const userRole = authStore.user?.role || 'user'

    if (userRole === 'admin') {
      selectedGuide.value = 'admin'
    } else if (userRole === 'group_admin') {
      selectedGuide.value = 'group-admin'
    } else if (userRole === 'editor') {
      selectedGuide.value = 'editor'
    } else {
      selectedGuide.value = 'user'
    }

    // Expand first section
    if (currentGuide.value?.sections.length) {
      expandedSections.value = [currentGuide.value.sections[0].id]
    }
  }
}

// Watch for guide changes
watch(selectedGuide, () => {
  selectedPage.value = null
  if (currentGuide.value?.sections.length) {
    expandedSections.value = [currentGuide.value.sections[0].id]
  }
})

// Watch for modal open
watch(() => props.modelValue, (isOpen) => {
  if (isOpen) {
    initializeGuide()
  }
})

// Initialize on mount
onMounted(() => {
  if (props.modelValue) {
    initializeGuide()
  }
})
</script>

<style scoped>
/* Transitions */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
  transform: scale(0.95);
}

.expand-enter-active,
.expand-leave-active {
  transition: all 0.2s ease;
  overflow: hidden;
}

.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  max-height: 0;
}

.expand-enter-to,
.expand-leave-from {
  max-height: 500px;
}

/* Prose customization */
:deep(.prose) {
  @apply text-gray-700 dark:text-gray-300;
}

:deep(.prose h1) {
  @apply text-gray-900 dark:text-white;
}

:deep(.prose h2) {
  @apply text-gray-800 dark:text-gray-100 border-b border-gray-200 dark:border-gray-700 pb-2;
}

:deep(.prose h3) {
  @apply text-gray-800 dark:text-gray-200;
}

:deep(.prose code) {
  @apply bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded text-sm;
}

:deep(.prose pre) {
  @apply bg-gray-900 dark:bg-gray-950 rounded-lg;
}

:deep(.prose table) {
  @apply w-full;
}

:deep(.prose th) {
  @apply bg-gray-50 dark:bg-gray-800;
}

:deep(.prose td),
:deep(.prose th) {
  @apply border border-gray-200 dark:border-gray-700 px-3 py-2;
}

:deep(.prose img) {
  @apply mx-auto;
}

:deep(.prose blockquote) {
  @apply border-l-4 border-blue-500 bg-blue-50 dark:bg-blue-900/20 pl-4 py-2 italic;
}
</style>
