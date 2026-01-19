<template>
  <div class="content-area">
    <!-- Header -->
    <div class="page-header">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="page-title">{{ $t('groupAdmin.appGroupsManagement.title') }}</h1>
          <p class="page-subtitle">{{ $t('groupAdmin.appGroupsManagement.subtitle') }}</p>
        </div>
        <button @click="openCreateModal" class="btn btn-primary" :disabled="managedGroups.length === 0">
          <Icon icon="mdi:plus" class="h-4 w-4 mr-2" />
          {{ $t('groupAdmin.appGroupsManagement.newGroup') }}
        </button>
      </div>
    </div>

    <!-- Info Alert -->
    <div v-if="managedGroups.length === 0" class="mb-6 p-4 bg-yellow-900/20 border border-yellow-600 rounded-lg">
      <div class="flex items-start">
        <Icon icon="mdi:information" class="h-5 w-5 text-yellow-500 mr-3 mt-0.5" />
        <div>
          <h3 class="text-yellow-500 font-medium">{{ $t('groupAdmin.appGroupsManagement.noManagedGroups') }}</h3>
          <p class="text-sm text-yellow-200 mt-1">
            {{ $t('groupAdmin.appGroupsManagement.noManagedGroupsHelp') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Filters -->
    <div class="filters-container">
      <div class="search-container flex-1">
        <Icon icon="mdi:magnify" class="search-icon" />
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="$t('groupAdmin.appGroupsManagement.searchPlaceholder')"
          class="form-input search-input"
        />
      </div>
      <select v-model="statusFilter" class="form-select w-full sm:w-auto">
        <option value="">{{ $t('groupAdmin.appGroupsManagement.status.all') }}</option>
        <option value="active">{{ $t('groupAdmin.appGroupsManagement.status.active') }}</option>
        <option value="inactive">{{ $t('groupAdmin.appGroupsManagement.status.inactive') }}</option>
      </select>
    </div>

    <!-- Content -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" class="h-8 w-8 animate-spin text-gray-400" />
    </div>

    <div v-else-if="filteredAppGroups.length > 0" class="grid-container grid-cols-auto">
      <div
        v-for="group in filteredAppGroups"
        :key="group.id"
        class="card hover:border-gray-600 transition-all duration-200 fade-in"
      >
        <div class="card-header">
          <div class="flex items-center space-x-3">
            <div
              class="h-10 w-10 rounded-lg flex items-center justify-center"
              :style="{ backgroundColor: group.color || '#10b981' }"
            >
              <Icon :icon="group.icon || 'mdi:folder'" class="h-5 w-5 text-white" />
            </div>
            <div>
              <h3 class="card-title">{{ group.name }}</h3>
              <p class="text-sm text-gray-400">{{ group.applications?.length || 0 }} {{ $t('groupAdmin.appGroupsManagement.applications') }}</p>
            </div>
          </div>

          <div class="flex space-x-2">
            <button @click="editGroup(group)" class="btn-ghost" :title="$t('groupAdmin.appGroupsManagement.actions.edit')">
              <Icon icon="mdi:pencil" class="h-4 w-4" />
            </button>
            <button @click="confirmDelete(group)" class="btn-ghost text-red-400 hover:text-red-300" :title="$t('groupAdmin.appGroupsManagement.actions.delete')">
              <Icon icon="mdi:delete" class="h-4 w-4" />
            </button>
          </div>
        </div>

        <div class="card-content">
          <p v-if="group.description" class="text-sm mb-4">{{ group.description }}</p>

          <div class="flex items-center justify-between text-sm mb-2">
            <span :class="group.is_active ? 'badge badge-success' : 'badge badge-secondary'">
              {{ group.is_active ? $t('groupAdmin.appGroupsManagement.statusBadge.active') : $t('groupAdmin.appGroupsManagement.statusBadge.inactive') }}
            </span>
            <span class="text-gray-500">{{ $t('groupAdmin.appGroupsManagement.order') }} {{ group.order || 0 }}</span>
          </div>

          <!-- Owner Group Badge -->
          <div v-if="group.is_private && group.owner_group" class="mt-2">
            <span class="owner-group-badge inline-flex items-center px-2 py-1 rounded text-xs">
              <Icon icon="mdi:shield-account" class="h-3 w-3 mr-1" />
              {{ $t('groupAdmin.appGroupsManagement.ownerGroup') }} {{ group.owner_group.name }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty States -->
    <div v-else-if="appGroups.length === 0" class="empty-state">
      <Icon icon="mdi:folder-multiple" class="empty-state-icon" />
      <h3 class="empty-state-title">{{ $t('groupAdmin.appGroupsManagement.empty.title') }}</h3>
      <p class="empty-state-description">{{ $t('groupAdmin.appGroupsManagement.empty.description') }}</p>
      <button
        @click="openCreateModal"
        class="btn btn-primary"
        :disabled="managedGroups.length === 0"
      >
        <Icon icon="mdi:plus" class="h-4 w-4 mr-2" />
        {{ $t('groupAdmin.appGroupsManagement.empty.createButton') }}
      </button>
    </div>

    <div v-else class="empty-state">
      <Icon icon="mdi:magnify" class="empty-state-icon" />
      <h3 class="empty-state-title">{{ $t('groupAdmin.appGroupsManagement.noResults.title') }}</h3>
      <p class="empty-state-description">{{ $t('groupAdmin.appGroupsManagement.noResults.description') }}</p>
    </div>

    <!-- Group Admin App Group Modal -->
    <GroupAdminAppGroupModal
      :show="showModal"
      :app-group="selectedAppGroup"
      :managed-groups="managedGroups"
      @close="closeModal"
      @submit="handleSubmit"
    />

    <!-- Delete Confirmation Modal -->
    <div v-if="showDeleteModal" class="modal-overlay">
      <div class="modal-container">
        <div class="modal-panel sm:max-w-lg sm:w-full">
          <div class="modal-header">
            <div class="flex items-start space-x-3">
              <div class="flex-shrink-0">
                <Icon icon="mdi:alert" class="h-6 w-6 text-red-500" />
              </div>
              <div>
                <h3 class="modal-title">{{ $t('groupAdmin.appGroupsManagement.deleteConfirm.title') }}</h3>
                <p class="modal-subtitle">
                  {{ $t('groupAdmin.appGroupsManagement.deleteConfirm.message', { name: appGroupToDelete?.name }) }}
                </p>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button @click="closeDeleteModal" class="btn btn-secondary w-full sm:w-auto">
              {{ $t('common.cancel') }}
            </button>
            <button
              @click="deleteGroup"
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
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@iconify/vue'
import { groupAdminService } from '@/services/api'
import { useAppStore } from '@/stores/app'
import GroupAdminAppGroupModal from '@/components/group-admin/AppGroupModal.vue'

const { t } = useI18n()
const appStore = useAppStore()

// State
const appGroups = ref([])
const managedGroups = ref([])
const loading = ref(false)
const showModal = ref(false)
const selectedAppGroup = ref(null)
const showDeleteModal = ref(false)
const appGroupToDelete = ref(null)
const deleteLoading = ref(false)
const searchQuery = ref('')
const statusFilter = ref('')

// Computed
const filteredAppGroups = computed(() => {
  let filtered = appGroups.value

  if (!Array.isArray(filtered)) {
    return []
  }

  // Filter by search query
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    filtered = filtered.filter(group =>
      group.name.toLowerCase().includes(query) ||
      group.description?.toLowerCase().includes(query)
    )
  }

  // Filter by status
  if (statusFilter.value === 'active') {
    filtered = filtered.filter(group => group.is_active)
  } else if (statusFilter.value === 'inactive') {
    filtered = filtered.filter(group => !group.is_active)
  }

  return filtered.sort((a, b) => (a.order || 0) - (b.order || 0))
})

// Methods
const loadAppGroups = async () => {
  try {
    loading.value = true
    const data = await groupAdminService.getAppGroups()
    appGroups.value = Array.isArray(data) ? data : (data.data || [])
  } catch (error) {
    console.error('Error loading app groups:', error)
    appGroups.value = []
  } finally {
    loading.value = false
  }
}

const loadManagedGroups = async () => {
  try {
    const data = await groupAdminService.getManagedGroups()
    managedGroups.value = Array.isArray(data) ? data : (data.data || [])
  } catch (error) {
    console.error('Error loading managed groups:', error)
    managedGroups.value = []
  }
}

const openCreateModal = () => {
  if (managedGroups.value.length === 0) {
    return
  }
  selectedAppGroup.value = null
  showModal.value = true
}

const editGroup = (group) => {
  selectedAppGroup.value = { ...group }
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  selectedAppGroup.value = null
}

const handleSubmit = async (formData) => {
  try {
    if (formData.id) {
      await groupAdminService.updateAppGroup(formData.id, formData)
    } else {
      await groupAdminService.createAppGroup(formData)
    }
    closeModal()
    await loadAppGroups()
  } catch (error) {
    console.error('Error saving app group:', error)
    throw error
  }
}

const confirmDelete = (group) => {
  appGroupToDelete.value = group
  showDeleteModal.value = true
}

const closeDeleteModal = () => {
  showDeleteModal.value = false
  appGroupToDelete.value = null
}

const deleteGroup = async () => {
  if (!appGroupToDelete.value) return

  try {
    deleteLoading.value = true
    await groupAdminService.deleteAppGroup(appGroupToDelete.value.id)
    closeDeleteModal()
    await loadAppGroups()
  } catch (error) {
    console.error('Error deleting app group:', error)
    // Could show a toast notification here instead of alert
    appStore.showError(t('groupAdmin.appGroupsManagement.deleteError'))
  } finally {
    deleteLoading.value = false
  }
}

onMounted(async () => {
  await loadManagedGroups()
  await loadAppGroups()
})
</script>
