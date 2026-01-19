<template>
  <div class="rich-text-editor">
    <!-- Toolbar -->
    <div v-if="editor" class="editor-toolbar">
      <!-- Text formatting -->
      <div class="toolbar-group">
        <button
          type="button"
          @click="editor.chain().focus().toggleBold().run()"
          :class="{ 'is-active': editor.isActive('bold') }"
          class="toolbar-button"
          title="Bold (Ctrl+B)"
        >
          <Icon icon="mdi:format-bold" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleItalic().run()"
          :class="{ 'is-active': editor.isActive('italic') }"
          class="toolbar-button"
          title="Italic (Ctrl+I)"
        >
          <Icon icon="mdi:format-italic" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleStrike().run()"
          :class="{ 'is-active': editor.isActive('strike') }"
          class="toolbar-button"
          title="Strikethrough"
        >
          <Icon icon="mdi:format-strikethrough" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleCode().run()"
          :class="{ 'is-active': editor.isActive('code') }"
          class="toolbar-button"
          title="Inline Code"
        >
          <Icon icon="mdi:code-tags" class="h-5 w-5" />
        </button>
      </div>

      <!-- Headings -->
      <div class="toolbar-group">
        <button
          type="button"
          @click="editor.chain().focus().toggleHeading({ level: 1 }).run()"
          :class="{ 'is-active': editor.isActive('heading', { level: 1 }) }"
          class="toolbar-button"
          title="Heading 1"
        >
          <Icon icon="mdi:format-header-1" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleHeading({ level: 2 }).run()"
          :class="{ 'is-active': editor.isActive('heading', { level: 2 }) }"
          class="toolbar-button"
          title="Heading 2"
        >
          <Icon icon="mdi:format-header-2" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleHeading({ level: 3 }).run()"
          :class="{ 'is-active': editor.isActive('heading', { level: 3 }) }"
          class="toolbar-button"
          title="Heading 3"
        >
          <Icon icon="mdi:format-header-3" class="h-5 w-5" />
        </button>
      </div>

      <!-- Lists -->
      <div class="toolbar-group">
        <button
          type="button"
          @click="editor.chain().focus().toggleBulletList().run()"
          :class="{ 'is-active': editor.isActive('bulletList') }"
          class="toolbar-button"
          title="Bullet List"
        >
          <Icon icon="mdi:format-list-bulleted" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleOrderedList().run()"
          :class="{ 'is-active': editor.isActive('orderedList') }"
          class="toolbar-button"
          title="Ordered List"
        >
          <Icon icon="mdi:format-list-numbered" class="h-5 w-5" />
        </button>
      </div>

      <!-- Blocks -->
      <div class="toolbar-group">
        <button
          type="button"
          @click="editor.chain().focus().toggleCodeBlock().run()"
          :class="{ 'is-active': editor.isActive('codeBlock') }"
          class="toolbar-button"
          title="Code Block"
        >
          <Icon icon="mdi:code-braces" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleBlockquote().run()"
          :class="{ 'is-active': editor.isActive('blockquote') }"
          class="toolbar-button"
          title="Blockquote"
        >
          <Icon icon="mdi:format-quote-close" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().setHorizontalRule().run()"
          class="toolbar-button"
          title="Horizontal Rule"
        >
          <Icon icon="mdi:minus" class="h-5 w-5" />
        </button>
      </div>

      <!-- Link -->
      <div class="toolbar-group">
        <button
          type="button"
          @click="setLink"
          :class="{ 'is-active': editor.isActive('link') }"
          class="toolbar-button"
          title="Add Link"
        >
          <Icon icon="mdi:link-variant" class="h-5 w-5" />
        </button>
        <button
          v-if="editor.isActive('link')"
          type="button"
          @click="editor.chain().focus().unsetLink().run()"
          class="toolbar-button"
          title="Remove Link"
        >
          <Icon icon="mdi:link-variant-off" class="h-5 w-5" />
        </button>
      </div>

      <!-- Media -->
      <div class="toolbar-group">
        <button
          type="button"
          @click="showMediaModal = true"
          class="toolbar-button"
          title="Insert Image"
        >
          <Icon icon="mdi:image-plus" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="showFileModal = true"
          class="toolbar-button"
          title="Attach File"
        >
          <Icon icon="mdi:file-plus" class="h-5 w-5" />
        </button>
      </div>

      <!-- Table (for later when we want to add tables) -->
      <!-- <div class="toolbar-group">
        <button
          type="button"
          @click="editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()"
          class="toolbar-button"
          title="Insert Table"
        >
          <Icon icon="mdi:table" class="h-5 w-5" />
        </button>
      </div> -->

      <!-- Undo/Redo -->
      <div class="toolbar-group ml-auto">
        <button
          type="button"
          @click="editor.chain().focus().undo().run()"
          :disabled="!editor.can().undo()"
          class="toolbar-button"
          title="Undo (Ctrl+Z)"
        >
          <Icon icon="mdi:undo" class="h-5 w-5" />
        </button>
        <button
          type="button"
          @click="editor.chain().focus().redo().run()"
          :disabled="!editor.can().redo()"
          class="toolbar-button"
          title="Redo (Ctrl+Y)"
        >
          <Icon icon="mdi:redo" class="h-5 w-5" />
        </button>
      </div>
    </div>

    <!-- Editor content -->
    <EditorContent :editor="editor" class="editor-content" />

    <!-- Media Modal -->
    <teleport to="body">
      <div v-if="showMediaModal" class="modal-overlay" @click.self="showMediaModal = false">
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">Insert Image</h3>
            <button @click="showMediaModal = false" class="modal-close">
              <Icon icon="mdi:close" class="h-6 w-6" />
            </button>
          </div>
          <div class="modal-body">
            <div class="tabs">
              <button
                @click="mediaTab = 'upload'"
                :class="{ 'active': mediaTab === 'upload' }"
                class="tab-button"
              >
                <Icon icon="mdi:upload" class="h-5 w-5" />
                Upload New
              </button>
              <button
                @click="mediaTab = 'gallery'"
                :class="{ 'active': mediaTab === 'gallery' }"
                class="tab-button"
              >
                <Icon icon="mdi:image-multiple" class="h-5 w-5" />
                Media Library
              </button>
            </div>

            <div v-if="mediaTab === 'upload'" class="tab-content">
              <MediaUploader
                accept="image/*"
                :max-size-m-b="5"
                @upload-success="handleMediaUpload"
              />
            </div>
            <div v-else class="tab-content">
              <MediaGallery
                @select="insertImage"
                @cancel="showMediaModal = false"
              />
            </div>
          </div>
        </div>
      </div>
    </teleport>

    <!-- File Modal -->
    <teleport to="body">
      <div v-if="showFileModal" class="modal-overlay" @click.self="showFileModal = false">
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">Attach File</h3>
            <button @click="showFileModal = false" class="modal-close">
              <Icon icon="mdi:close" class="h-6 w-6" />
            </button>
          </div>
          <div class="modal-body">
            <div class="tabs">
              <button
                @click="fileTab = 'upload'"
                :class="{ 'active': fileTab === 'upload' }"
                class="tab-button"
              >
                <Icon icon="mdi:upload" class="h-5 w-5" />
                Upload New
              </button>
              <button
                @click="fileTab = 'gallery'"
                :class="{ 'active': fileTab === 'gallery' }"
                class="tab-button"
              >
                <Icon icon="mdi:file-multiple" class="h-5 w-5" />
                Media Library
              </button>
            </div>

            <div v-if="fileTab === 'upload'" class="tab-content">
              <MediaUploader
                accept="application/pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.zip,.rar"
                :max-size-m-b="10"
                @upload-success="handleFileUpload"
              />
            </div>
            <div v-else class="tab-content">
              <MediaGallery
                @select="insertFile"
                @cancel="showFileModal = false"
              />
            </div>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import Image from '@tiptap/extension-image'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableCell } from '@tiptap/extension-table-cell'
import { TableHeader } from '@tiptap/extension-table-header'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import TextAlign from '@tiptap/extension-text-align'
import Placeholder from '@tiptap/extension-placeholder'
import { createLowlight } from 'lowlight'
import { Icon } from '@iconify/vue'
import MediaUploader from '@/components/media/MediaUploader.vue'
import MediaGallery from '@/components/media/MediaGallery.vue'

// Import common languages for syntax highlighting
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import python from 'highlight.js/lib/languages/python'
import java from 'highlight.js/lib/languages/java'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import xml from 'highlight.js/lib/languages/xml'
import css from 'highlight.js/lib/languages/css'

// Create lowlight instance and register languages
const lowlight = createLowlight()
lowlight.register('javascript', javascript)
lowlight.register('typescript', typescript)
lowlight.register('python', python)
lowlight.register('java', java)
lowlight.register('bash', bash)
lowlight.register('json', json)
lowlight.register('xml', xml)
lowlight.register('css', css)

const props = defineProps({
  modelValue: {
    type: [String, Object],
    default: ''
  },
  placeholder: {
    type: String,
    default: 'Start writing...'
  },
  editable: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['update:modelValue'])

const showMediaModal = ref(false)
const showFileModal = ref(false)
const mediaTab = ref('upload')
const fileTab = ref('upload')

const editor = useEditor({
  content: props.modelValue,
  editable: props.editable,
  extensions: [
    StarterKit.configure({
      codeBlock: false, // Disable default code block, we use lowlight
      link: false, // Disable default link, we configure it separately
    }),
    Link.configure({
      openOnClick: false,
      HTMLAttributes: {
        class: 'text-blue-600 dark:text-blue-400 underline hover:text-blue-800 dark:hover:text-blue-300',
      },
    }),
    Image.configure({
      inline: false,
      HTMLAttributes: {
        class: 'rounded-lg max-w-full h-auto',
      },
    }),
    Table.configure({
      resizable: true,
    }),
    TableRow,
    TableCell,
    TableHeader,
    CodeBlockLowlight.configure({
      lowlight,
    }),
    TextAlign.configure({
      types: ['heading', 'paragraph'],
    }),
    Placeholder.configure({
      placeholder: props.placeholder,
    }),
  ],
  onUpdate: ({ editor }) => {
    emit('update:modelValue', editor.getJSON())
  },
  editorProps: {
    attributes: {
      class: 'prose prose-sm sm:prose lg:prose-lg xl:prose-xl dark:prose-invert max-w-none focus:outline-none min-h-[300px] p-4',
    },
  },
})

// Set link
const setLink = () => {
  const previousUrl = editor.value.getAttributes('link').href
  const url = window.prompt('URL', previousUrl)

  if (url === null) {
    return
  }

  if (url === '') {
    editor.value.chain().focus().extendMarkRange('link').unsetLink().run()
    return
  }

  editor.value.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
}

// Watch for external content changes
watch(() => props.modelValue, (value) => {
  if (editor.value) {
    const isSame = JSON.stringify(editor.value.getJSON()) === JSON.stringify(value)
    if (!isSame) {
      editor.value.commands.setContent(value, false)
    }
  }
})

// Watch for editable changes
watch(() => props.editable, (value) => {
  if (editor.value) {
    editor.value.setEditable(value)
  }
})

// Media handlers
const handleMediaUpload = (media) => {
  // Automatically insert uploaded image
  if (media.mime_type.startsWith('image/')) {
    insertImage(media)
  }
}

const insertImage = (media) => {
  if (editor.value) {
    editor.value.chain().focus().setImage({ src: media.url, alt: media.filename }).run()
    showMediaModal.value = false
  }
}

const handleFileUpload = (media) => {
  // Insert file as a link
  insertFile(media)
}

const insertFile = (media) => {
  if (editor.value) {
    // Insert file as a formatted link with icon
    const fileIcon = getFileIcon(media.mime_type)
    const fileLink = `📎 ${media.filename}`
    editor.value.chain().focus().insertContent(`<p><a href="${media.url}" target="_blank" class="file-attachment">${fileLink}</a></p>`).run()
    showFileModal.value = false
  }
}

const getFileIcon = (mimeType) => {
  if (mimeType === 'application/pdf') return '📄'
  if (mimeType.includes('word') || mimeType.includes('document')) return '📝'
  if (mimeType.includes('sheet') || mimeType.includes('excel')) return '📊'
  if (mimeType.includes('presentation') || mimeType.includes('powerpoint')) return '📽️'
  if (mimeType.includes('zip') || mimeType.includes('rar')) return '🗜️'
  return '📎'
}

// Cleanup
onBeforeUnmount(() => {
  if (editor.value) {
    editor.value.destroy()
  }
})
</script>

<style scoped>
.rich-text-editor {
  @apply border border-gray-300 dark:border-gray-600 rounded-lg overflow-hidden bg-white dark:bg-gray-800;
}

.editor-toolbar {
  @apply flex flex-wrap items-center gap-1 p-2 bg-gray-50 dark:bg-gray-900 border-b border-gray-300 dark:border-gray-600;
}

.toolbar-group {
  @apply flex items-center gap-1 border-r border-gray-300 dark:border-gray-600 pr-2 mr-2;
}

.toolbar-group:last-child {
  @apply border-r-0 mr-0 pr-0;
}

.toolbar-button {
  @apply p-2 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-gray-700 dark:text-gray-300;
}

.toolbar-button.is-active {
  @apply bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-400;
}

.editor-content {
  @apply min-h-[300px];
}

/* Tiptap editor styles */
.editor-content :deep(.ProseMirror) {
  @apply focus:outline-none;
}

.editor-content :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  @apply text-gray-400 dark:text-gray-500 float-left h-0 pointer-events-none;
}

/* List styling */
.editor-content :deep(.ProseMirror ul) {
  @apply list-disc ml-6 mb-4 space-y-1;
}

.editor-content :deep(.ProseMirror ol) {
  @apply list-decimal ml-6 mb-4 space-y-1;
}

.editor-content :deep(.ProseMirror li) {
  @apply ml-2;
}

.editor-content :deep(.ProseMirror ul ul),
.editor-content :deep(.ProseMirror ol ol) {
  @apply ml-6 mb-0 mt-1;
}

/* Code block styling */
.editor-content :deep(.ProseMirror pre) {
  @apply bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto;
}

.editor-content :deep(.ProseMirror code) {
  @apply bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded text-sm;
}

/* Link styling */
.editor-content :deep(.ProseMirror a) {
  @apply text-blue-600 dark:text-blue-400 underline hover:text-blue-800 dark:hover:text-blue-300;
}

/* Table styling */
.editor-content :deep(.ProseMirror table) {
  @apply border-collapse table-auto w-full;
}

.editor-content :deep(.ProseMirror th),
.editor-content :deep(.ProseMirror td) {
  @apply border border-gray-300 dark:border-gray-600 px-3 py-2;
}

.editor-content :deep(.ProseMirror th) {
  @apply bg-gray-100 dark:bg-gray-800 font-semibold;
}

/* Image styling */
.editor-content :deep(.ProseMirror img) {
  @apply rounded-lg max-w-full h-auto my-4 cursor-pointer;
}

/* File attachment styling */
.editor-content :deep(.ProseMirror a.file-attachment) {
  @apply inline-flex items-center gap-2 px-3 py-2 bg-gray-100 dark:bg-gray-800 rounded-lg no-underline hover:bg-gray-200 dark:hover:bg-gray-700;
}

/* Modal styles */
.modal-overlay {
  @apply fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center p-4;
}

.modal-content {
  @apply bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col;
}

.modal-header {
  @apply flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700;
}

.modal-title {
  @apply text-lg font-semibold text-gray-900 dark:text-white;
}

.modal-close {
  @apply p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors text-gray-500 dark:text-gray-400;
}

.modal-body {
  @apply p-4 overflow-y-auto flex-1;
}

.tabs {
  @apply flex gap-2 mb-4 border-b border-gray-200 dark:border-gray-700;
}

.tab-button {
  @apply flex items-center gap-2 px-4 py-2 border-b-2 border-transparent text-gray-600 dark:text-gray-400;
  @apply hover:text-gray-900 dark:hover:text-white transition-colors;
}

.tab-button.active {
  @apply border-blue-500 text-blue-600 dark:text-blue-400 font-medium;
}

.tab-content {
  @apply py-4;
}
</style>
