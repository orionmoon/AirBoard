<template>
  <div :class="['collapsible-menu', { 'nested': nested }]">
    <!-- Menu Header (clickable to toggle) -->
    <button
      @click="toggle"
      :class="[
        'collapsible-menu-header',
        { 'is-open': isOpen, 'has-active-child': hasActiveChild }
      ]"
    >
      <div class="flex items-center gap-2 flex-1 min-w-0">
        <Icon :icon="icon" class="h-4 w-4 flex-shrink-0" />
        <span class="truncate">{{ title }}</span>
      </div>
      <Icon
        :icon="isOpen ? 'mdi:chevron-up' : 'mdi:chevron-down'"
        class="h-4 w-4 flex-shrink-0 transition-transform duration-200"
      />
    </button>

    <!-- Menu Content (collapsible) -->
    <Transition
      enter-active-class="transition-all duration-200 ease-out"
      enter-from-class="opacity-0 max-h-0"
      enter-to-class="opacity-100 max-h-[500px]"
      leave-active-class="transition-all duration-150 ease-in"
      leave-from-class="opacity-100 max-h-[500px]"
      leave-to-class="opacity-0 max-h-0"
    >
      <div v-show="isOpen" class="collapsible-menu-content">
        <slot></slot>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed, useSlots } from 'vue'
import { useRoute } from 'vue-router'
import { Icon } from '@iconify/vue'

const props = defineProps({
  title: {
    type: String,
    required: true
  },
  icon: {
    type: String,
    default: 'mdi:folder'
  },
  storageKey: {
    type: String,
    default: null
  },
  defaultOpen: {
    type: Boolean,
    default: false
  },
  nested: {
    type: Boolean,
    default: false
  }
})

const route = useRoute()
const isOpen = ref(false)

// Check if any child route is active
const hasActiveChild = computed(() => {
  return props.defaultOpen
})

// Load state from localStorage on mount
onMounted(() => {
  if (props.storageKey) {
    const saved = localStorage.getItem(props.storageKey)
    if (saved !== null) {
      isOpen.value = saved === 'true'
    } else {
      isOpen.value = props.defaultOpen
    }
  } else {
    isOpen.value = props.defaultOpen
  }

  // If a child is active, force open
  if (props.defaultOpen) {
    isOpen.value = true
  }
})

// Watch for route changes to auto-open if child becomes active
watch(() => props.defaultOpen, (newVal) => {
  if (newVal) {
    isOpen.value = true
  }
})

// Toggle function
const toggle = () => {
  isOpen.value = !isOpen.value

  // Save state to localStorage
  if (props.storageKey) {
    localStorage.setItem(props.storageKey, isOpen.value.toString())
  }
}
</script>

<style scoped>
.collapsible-menu {
  @apply mb-1;
}

.collapsible-menu.nested {
  @apply ml-0;
}

.collapsible-menu-header {
  @apply w-full flex items-center justify-between gap-2 px-3 py-2 text-sm font-medium rounded-lg transition-colors;
  @apply text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700;
  @apply hover:text-gray-900 dark:hover:text-white;
}

.collapsible-menu-header.is-open {
  @apply text-gray-900 dark:text-white;
}

.collapsible-menu-header.has-active-child {
  @apply text-primary-600 dark:text-primary-400;
}

.collapsible-menu.nested .collapsible-menu-header {
  @apply text-xs py-1.5 pl-2;
}

.collapsible-menu-content {
  @apply overflow-hidden;
}

.collapsible-menu.nested .collapsible-menu-content {
  @apply pl-2 border-l-2 border-gray-200 dark:border-gray-700 ml-3 mt-1;
}

.collapsible-menu-content :deep(.nav-link) {
  @apply pl-3 ml-3;
}

.collapsible-menu.nested .collapsible-menu-content :deep(.nav-link) {
  @apply pl-4 text-xs py-1.5;
}
</style>
