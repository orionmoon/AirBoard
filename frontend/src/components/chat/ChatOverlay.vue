<template>
  <div class="chat-overlay">
    <!-- Minimized Bubble -->
    <div
      v-if="!chatStore.isOpen"
      class="chat-bubble shadow-lg hover:shadow-xl transition-all cursor-pointer bg-blue-600 text-white rounded-full w-14 h-14 flex items-center justify-center relative z-50"
      :class="{ 'animate-pulse-slow': totalUnread > 0 }"
      @click="chatStore.toggleChat()"
    >
      <Icon icon="mdi:chat" class="text-2xl" />
      <span
        v-if="totalUnread > 0"
        class="absolute -top-1 -right-1 bg-red-500 text-white text-xs font-bold rounded-full w-5 h-5 flex items-center justify-center border-2 border-white animate-bounce"
      >
        {{ totalUnread > 99 ? '99+' : totalUnread }}
      </span>
    </div>

    <!-- Expanded Window -->
    <div v-else class="chat-window shadow-2xl rounded-t-lg bg-white dark:bg-gray-900 border dark:border-gray-800 flex flex-col z-50">
      <!-- Header -->
      <div class="chat-header bg-blue-600 text-white p-3 rounded-t-lg flex justify-between items-center cursor-pointer" @click="toggleMinimize">
        <div class="flex items-center gap-2">
           <Icon icon="mdi:chat" class="text-xl" />
           <span class="font-bold">Messagerie</span>
           <span v-if="!chatStore.isConnected" class="text-xs bg-red-500 px-1 rounded">Offline</span>
        </div>
        <button @click.stop="chatStore.toggleChat()" class="hover:bg-blue-700 rounded p-1">
          <Icon icon="mdi:close" />
        </button>
      </div>

      <div class="flex flex-1 overflow-hidden h-full">
        <!-- Sidebar (Contacts) -->
        <div v-if="!isMobile || !chatStore.activeConversation" class="w-full md:w-1/3 border-r dark:border-gray-700 flex flex-col bg-gray-50 dark:bg-gray-800 h-full">
           <!-- Search Bar -->
           <div class="px-3 py-2 border-b dark:border-gray-700">
              <div class="relative">
                 <Icon icon="mdi:magnify" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 text-sm" />
                 <input 
                   v-model="searchQuery" 
                   type="text" 
                   placeholder="Rechercher..." 
                   class="w-full pl-8 pr-4 py-1.5 bg-gray-100 dark:bg-gray-700 border-none rounded-lg text-sm focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-white"
                 >
                 <button v-if="searchQuery" @click="searchQuery = ''" class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600">
                   <Icon icon="mdi:close-circle" class="text-xs" />
                 </button>
              </div>
           </div>
           
           <div class="overflow-y-auto flex-1 p-2 space-y-2">
               <!-- 1. UNREAD SECTION (Always first if not empty) -->
               <div v-if="hasUnreadUsers" class="mb-4">
                 <h3 class="text-[10px] font-bold text-blue-500 uppercase px-2 mb-1 tracking-wider">Messages non lus</h3>
                 <div v-for="user in unreadUsers" :key="'unread_'+user.id" 
                      @click="chatStore.openConversation('user', user)"
                      class="flex items-center gap-2 p-2 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors"
                      :class="{'bg-blue-100 dark:bg-blue-900/40': chatStore.activeConversation?.id === user.id && chatStore.activeConversation?.type === 'user'}"
                 >
                    <div class="relative flex-shrink-0">
                       <img 
                         :src="user.avatar_url || `https://ui-avatars.com/api/?name=${user.first_name}+${user.last_name}&background=random`" 
                         class="w-8 h-8 rounded-full object-cover"
                       >
                       <!-- Online Status -->
                       <span v-if="user.is_online" class="absolute bottom-0 right-0 w-2.5 h-2.5 bg-green-500 border-2 border-white dark:border-gray-800 rounded-full"></span>
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="font-medium text-sm truncate text-gray-900 dark:text-gray-100">{{ user.first_name }} {{ user.last_name }}</div>
                      <div class="text-[10px] text-gray-500 dark:text-gray-400 truncate">{{ user.job_title }}</div>
                    </div>
                    <span v-if="chatStore.unreadCounts['user_'+user.id]" class="bg-blue-500 text-white text-[10px] font-bold px-1.5 py-0.5 rounded-full">
                      {{ chatStore.unreadCounts['user_'+user.id] }}
                    </span>
                 </div>
               </div>

               <!-- 2. GROUPS -->
               <div v-if="filteredGroups.length > 0" class="mb-4">
                 <h3 class="text-[10px] font-bold text-gray-400 uppercase px-2 mb-1 tracking-wider">Groupes de discussion</h3>
                 <div 
                   v-for="group in filteredGroups" 
                   :key="group.id"
                   @click="chatStore.openConversation('group', group)"
                   class="flex items-center gap-2 p-2 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors"
                   :class="{'bg-blue-100 dark:bg-blue-900/40': chatStore.activeConversation?.id === group.id && chatStore.activeConversation?.type === 'group'}"
                 >
                    <div class="w-8 h-8 rounded bg-gray-200 dark:bg-gray-700 flex items-center justify-center text-gray-500 dark:text-gray-400">
                      <Icon icon="mdi:account-group" class="text-lg" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="font-medium text-sm truncate text-gray-900 dark:text-gray-100">{{ group.name }}</div>
                    </div>
                    <span v-if="chatStore.unreadCounts['group_'+group.id]" class="bg-blue-500 text-white text-[10px] font-bold px-1.5 py-0.5 rounded-full">
                      {{ chatStore.unreadCounts['group_'+group.id] }}
                    </span>
                 </div>
               </div>

               <!-- 3. USERS BY APP GROUPS -->
               <div v-for="(groupData, groupName) in groupedUsers" :key="groupName" class="mb-4">
                 <h3 class="text-[10px] font-bold text-gray-400 uppercase px-2 mb-1 tracking-wider">{{ groupName }}</h3>
                 <div 
                   v-for="user in groupData.users" 
                   :key="user.id"
                   @click="chatStore.openConversation('user', user)"
                   class="flex items-center gap-2 p-2 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors"
                   :class="{'bg-blue-100 dark:bg-blue-900/40': chatStore.activeConversation?.id === user.id && chatStore.activeConversation?.type === 'user'}"
                 >
                    <div class="relative flex-shrink-0">
                       <img 
                         :src="user.avatar_url || `https://ui-avatars.com/api/?name=${user.first_name}+${user.last_name}&background=random`" 
                         class="w-8 h-8 rounded-full object-cover"
                       >
                       <!-- Online Status -->
                       <span v-if="user.is_online" class="absolute bottom-0 right-0 w-2.5 h-2.5 bg-green-500 border-2 border-white dark:border-gray-800 rounded-full"></span>
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="font-medium text-sm truncate text-gray-900 dark:text-gray-100">{{ user.first_name }} {{ user.last_name }}</div>
                      <div class="text-[10px] text-gray-500 dark:text-gray-400 truncate">{{ user.job_title }}</div>
                    </div>
                    <span v-if="chatStore.unreadCounts['user_'+user.id]" class="bg-blue-500 text-white text-[10px] font-bold px-1.5 py-0.5 rounded-full">
                      {{ chatStore.unreadCounts['user_'+user.id] }}
                    </span>
                 </div>
               </div>

               <!-- 4. OTHERS (Users without group) -->
               <div v-if="otherUsers.length > 0" class="mb-2">
                 <h3 class="text-[10px] font-bold text-gray-400 uppercase px-2 mb-1 tracking-wider">Autres contacts</h3>
                 <div 
                   v-for="user in otherUsers" 
                   :key="user.id"
                   @click="chatStore.openConversation('user', user)"
                   class="flex items-center gap-2 p-2 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors"
                   :class="{'bg-blue-100 dark:bg-blue-900/40': chatStore.activeConversation?.id === user.id && chatStore.activeConversation?.type === 'user'}"
                 >
                    <div class="relative flex-shrink-0">
                       <img 
                         :src="user.avatar_url || `https://ui-avatars.com/api/?name=${user.first_name}+${user.last_name}&background=random`" 
                         class="w-8 h-8 rounded-full object-cover"
                       >
                       <!-- Online Status -->
                       <span v-if="user.is_online" class="absolute bottom-0 right-0 w-2.5 h-2.5 bg-green-500 border-2 border-white dark:border-gray-800 rounded-full"></span>
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="font-medium text-sm truncate text-gray-900 dark:text-gray-100">{{ user.first_name }} {{ user.last_name }}</div>
                      <div class="text-[10px] text-gray-500 dark:text-gray-400 truncate">{{ user.job_title }}</div>
                    </div>
                    <span v-if="chatStore.unreadCounts['user_'+user.id]" class="bg-blue-500 text-white text-[10px] font-bold px-1.5 py-0.5 rounded-full">
                      {{ chatStore.unreadCounts['user_'+user.id] }}
                    </span>
                 </div>
               </div>

               <!-- Empty Search -->
               <div v-if="filteredGroups.length === 0 && Object.keys(groupedUsers).length === 0 && otherUsers.length === 0" class="text-center py-8 text-gray-400 text-xs">
                 <Icon icon="mdi:account-search" class="text-3xl mb-2 opacity-20" />
                 <p>Aucun contact trouvé</p>
               </div>
           </div>
        </div>

        <!-- Conversation View -->
        <div v-if="chatStore.activeConversation" class="flex-1 flex flex-col h-full bg-white dark:bg-gray-900" :class="{'hidden md:flex': !isMobile && !chatStore.activeConversation}">
           <!-- Active Header -->
           <div class="p-3 border-b dark:border-gray-700 flex justify-between items-center bg-white dark:bg-gray-900">
              <div class="flex items-center gap-2">
                 <button v-if="isMobile" @click="chatStore.closeConversation()" class="md:hidden mr-1 text-gray-600 dark:text-gray-400"><Icon icon="mdi:arrow-left" /></button>
                 <span class="font-bold truncate text-gray-900 dark:text-gray-100">{{ chatStore.activeConversation.name }}</span>
              </div>
              
              <div class="relative">
                 <button @click.stop="toggleMenu" class="text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 p-1 rounded transition-colors flex items-center justify-center">
                    <Icon icon="mdi:dots-vertical" class="text-xl" />
                 </button>
                 <div v-if="showMenu" class="absolute right-0 top-full mt-1 bg-white dark:bg-gray-800 border dark:border-gray-700 rounded shadow-lg w-56 z-20 py-1 text-gray-800 dark:text-gray-200">
                    <!-- Sound Toggle -->
                    <button 
                      @click="toggleNotifications"
                      class="w-full text-left px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2 border-b dark:border-gray-700 mb-1"
                    >
                      <Icon :icon="notificationsEnabled ? 'mdi:bell-ring-outline' : 'mdi:bell-off-outline'" :class="notificationsEnabled ? 'text-green-500' : 'text-gray-400'" />
                      <span class="flex-1 text-xs">Sons notifications</span>
                      <span class="text-[9px] uppercase font-bold px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700" :class="notificationsEnabled ? 'text-green-500' : 'text-gray-500'">
                         {{ notificationsEnabled ? 'On' : 'Off' }}
                      </span>
                    </button>

                    <button 
                      @click="confirmClearHistory"
                      class="w-full text-left px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 flex items-center gap-2"
                    >
                      <Icon icon="mdi:delete-sweep" /> Vider l'historique
                    </button>
                 </div>
              </div>
           </div>

           <!-- Messages Area -->
           <div class="flex-1 overflow-y-auto p-4 space-y-3 flex flex-col" ref="messagesContainer" @click="showMenu = false">
              <!-- Reversed flex-col to stick to bottom -->
               <!-- Note: Data is usually chronological (old -> new). To use flex-col-reverse, we need new -> old order in DOM. 
                    Let's stick to standard flex-col and scroll to bottom for simplicity first. -->
               <div v-for="msg in currentMessages" :key="msg.id" 
                    class="flex flex-col max-w-[80%]"
                    :class="msg.sender_id == myId ? 'self-end items-end' : 'self-start items-start'"
               >
                  <span v-if="chatStore.activeConversation?.type === 'group' && msg.sender_id != myId" class="text-[10px] font-bold text-gray-500 dark:text-gray-400 mb-0.5 px-1 truncate max-w-full">
                     {{ msg.sender?.first_name }} {{ msg.sender?.last_name || msg.sender?.username }}
                  </span>
                  <div 
                    class="px-3 py-2 rounded-lg break-words text-sm group flex items-start gap-2"
                    :class="msg.sender_id == myId ? 'bg-blue-600 text-white rounded-tr-none' : 'bg-gray-100 dark:bg-gray-800 text-gray-800 dark:text-gray-200 rounded-tl-none border dark:border-gray-700'"
                  >
                    <span class="flex-1 message-content" v-html="formatMessage(msg.content)"></span>
                    
                    <!-- Delete Button (Inline) -->
                    <button 
                      v-if="msg.sender_id == myId"
                      @click.stop="deleteMessage(msg.id)"
                      class="delete-btn p-1 rounded transition-colors flex items-center justify-center"
                      :class="msg.sender_id == myId ? 'text-white/60 hover:text-white hover:bg-red-500' : 'text-gray-400 hover:text-red-500 hover:bg-red-50'"
                      title="Supprimer"
                    >
                      <Icon icon="mdi:delete" class="text-sm" />
                    </button>
                  </div>
                  <span class="text-[10px] text-gray-400 mt-1">
                    {{ formatTime(msg.created_at) }}
                  </span>
               </div>
               
               <div v-if="currentMessages.length === 0" class="text-center text-gray-400 text-sm mt-4">
                  Début de la conversation
               </div>
           </div>

           <!-- Input Area -->
           <div class="relative">
              <!-- Emoji Picker -->
              <div v-if="showEmojiPicker" class="absolute bottom-full left-0 mb-2 bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-lg shadow-xl p-2 z-50 w-64">
                 <div class="grid grid-cols-8 gap-1 h-40 overflow-y-auto p-1 scrollbar-thin">
                    <button 
                      v-for="emoji in commonEmojis" 
                      :key="emoji" 
                      @click="addEmoji(emoji)"
                      class="hover:bg-gray-100 dark:hover:bg-gray-700 p-1 rounded text-xl flex items-center justify-center transition-transform hover:scale-125"
                    >
                       {{ emoji }}
                    </button>
                 </div>
              </div>
           </div>

           <div class="p-3 border-t dark:border-gray-700 flex gap-2 bg-white dark:bg-gray-900 items-center">
              <button 
                @click="showEmojiPicker = !showEmojiPicker"
                class="text-gray-500 hover:text-blue-500 transition-colors p-1"
                :class="{'text-blue-500': showEmojiPicker}"
                title="Ajouter un emoji"
              >
                <Icon icon="mdi:emoticon-outline" class="text-xl" />
              </button>

              <input 
                v-model="newMessage" 
                @keyup.enter="sendMessage"
                @focus="showEmojiPicker = false"
                type="text" 
                placeholder="Écrivez un message..." 
                class="flex-1 border dark:border-gray-700 rounded-full px-4 py-2 text-sm focus:outline-none focus:border-blue-500 bg-gray-50 dark:bg-gray-800 text-gray-900 dark:text-white"
              >
              <button 
                @click="sendMessage"
                class="bg-blue-600 text-white rounded-full w-9 h-9 flex items-center justify-center hover:bg-blue-700 disabled:opacity-50 flex-shrink-0"
                :disabled="!newMessage.trim()"
              >
                <Icon icon="mdi:send" class="text-sm" />
              </button>
           </div>
        </div>
        
        <!-- Empty State (Desktop) -->
        <div v-else-if="!isMobile" class="flex-1 flex flex-col items-center justify-center text-gray-400 p-4">
           <span class="mdi mdi-message-outline text-4xl mb-2"></span>
           <p>Sélectionnez une conversation</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, nextTick } from 'vue';
import { useChatStore } from '@/stores/chat';
import { useAuthStore } from '@/stores/auth';
import { storeToRefs } from 'pinia';
import { Icon } from '@iconify/vue';
import DOMPurify from 'dompurify';

const chatStore = useChatStore();
const authStore = useAuthStore();
const { activeConversation, messages } = storeToRefs(chatStore);

const newMessage = ref('');
const searchQuery = ref('');
const showEmojiPicker = ref(false);
const messagesContainer = ref(null);
const isMobile = ref(window.innerWidth < 768);
const showMenu = ref(false);

const commonEmojis = [
  '😀', '😂', '🤣', '😊', '😍', '😘', '😉', '😎', 
  '🤔', '🤨', '😐', '🙄', '😏', '😮', '😴', '😌', 
  '😛', '😜', '🤤', '😒', '😓', '😔', '😕', '🙃', 
  '😲', '☹️', '😤', '😢', '😭', '🤯', '😬', '😰', 
  '😱', '🥵', '🥶', '😳', '🤪', '😵', '😡', '😠', 
  '😇', '🥳', '🥴', '🥺', '🤫', '🧐', '🤓', '😈', 
  '💀', '👻', '👽', '🤖', '💩', '😺', '😸', '😻', 
  '👍', '👎', '👌', '✌️', '🤞', '🤝', '👏', '🙌', 
  '🔥', '✨', '⭐', '❤️', '💯', '🎉', '🎁', '🚀'
];

const addEmoji = (emoji) => {
  newMessage.value += emoji;
};

const myId = computed(() => authStore.user?.id);

// Filtering logic
const filteredGroups = computed(() => {
  if (!searchQuery.value) return chatStore.contacts.groups;
  const q = searchQuery.value.toLowerCase();
  return chatStore.contacts.groups.filter(g => g.name.toLowerCase().includes(q));
});

const filteredUsers = computed(() => {
  let users = chatStore.contacts.users;
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase();
    users = users.filter(u => 
      `${u.first_name} ${u.last_name}`.toLowerCase().includes(q) || 
      u.username.toLowerCase().includes(q)
    );
  }
  return users;
});

const unreadUsers = computed(() => {
  return filteredUsers.value.filter(u => chatStore.unreadCounts[`user_${u.id}`] > 0);
});

const hasUnreadUsers = computed(() => unreadUsers.value.length > 0);

const groupedUsers = computed(() => {
  const groups = {};
  filteredUsers.value.forEach(user => {
    // If user has unread messages, we might show them in a special section already, 
    // but here we group them by their actual application groups.
    if (user.groups && user.groups.length > 0) {
      user.groups.forEach(g => {
        if (!groups[g.name]) groups[g.name] = { color: g.color, users: [] };
        groups[g.name].users.push(user);
      });
    }
  });
  return groups;
});

const otherUsers = computed(() => {
  return filteredUsers.value.filter(u => !u.groups || u.groups.length === 0);
});

const notificationsEnabled = ref(localStorage.getItem('chat_notifications') !== 'false');

const toggleNotifications = () => {
  notificationsEnabled.value = !notificationsEnabled.value;
  localStorage.setItem('chat_notifications', notificationsEnabled.value);
};

const toggleMenu = () => {
  showMenu.value = !showMenu.value;
};

const playNotificationSound = async () => {
  if (!notificationsEnabled.value) return;

  try {
    const audioCtx = new (window.AudioContext || window.webkitAudioContext)();

    // Resume audio context if suspended (browser autoplay policy)
    if (audioCtx.state === 'suspended') {
      await audioCtx.resume();
    }

    const oscillator = audioCtx.createOscillator();
    const gainNode = audioCtx.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(audioCtx.destination);

    oscillator.type = 'sine';
    oscillator.frequency.setValueAtTime(880, audioCtx.currentTime); // A5

    gainNode.gain.setValueAtTime(0, audioCtx.currentTime);
    gainNode.gain.linearRampToValueAtTime(0.15, audioCtx.currentTime + 0.01);
    gainNode.gain.exponentialRampToValueAtTime(0.01, audioCtx.currentTime + 0.4);

    oscillator.start();
    oscillator.stop(audioCtx.currentTime + 0.4);

    // Auto-close context after sound finishes to save resources
    setTimeout(() => {
      audioCtx.close();
    }, 500);
  } catch (e) {
    console.warn('Audio notification error:', e);
  }
};

const formatMessage = (content) => {
  if (!content) return '';
  
  // URL detection regex
  const urlRegex = /(https?:\/\/[^\s]+)/g;
  
  // Replace URLs with <a> tags
  let formatted = content.replace(urlRegex, (url) => {
    return `<a href="${url}" target="_blank" rel="noopener noreferrer" class="text-blue-200 dark:text-blue-400 underline hover:text-white dark:hover:text-blue-300 transition-colors break-all cursor-pointer">${url}</a>`;
  });
  
  return DOMPurify.sanitize(formatted, {
    ALLOWED_TAGS: ['a'],
    ALLOWED_ATTR: ['href', 'target', 'rel', 'class']
  });
};

const totalUnread = computed(() => {
  return Object.values(chatStore.unreadCounts).reduce((a, b) => a + b, 0);
});

const currentMessages = computed(() => {
  if (!activeConversation.value) return [];
  const key = activeConversation.value.type === 'user' 
     ? `user_${activeConversation.value.id}` 
     : `group_${activeConversation.value.id}`;
  return messages.value[key] || [];
});

const scrollToBottom = () => {
  nextTick(() => {
    // If using flex-col standard
    if (messagesContainer.value) {
        // However, in the template I used flex-col-reverse logic in comments but standard div order.
        // Actually, let's fix the template to strictly follow one logic.
        // Standard chat: div order = chronological. Scroll Top = Max.
        if (messagesContainer.value) {
            messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight;
        }
    }
  });
};

const sendMessage = () => {
  if (!newMessage.value.trim()) return;
  chatStore.sendMessage(newMessage.value);
  newMessage.value = '';
  scrollToBottom();
};

const formatTime = (dateStr) => {
  const date = new Date(dateStr);
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

// Listeners
onMounted(() => {
    window.addEventListener('resize', () => {
        isMobile.value = window.innerWidth < 768;
    });

    // Auto-connect if logged in
    if(authStore.token) {
        chatStore.connect();
    }

    // Initialize previous unread count to prevent sound on first load
    previousTotalUnread.value = totalUnread.value;
});

watch(currentMessages, () => {
   scrollToBottom();
}, { deep: true });

watch(activeConversation, (newVal) => {
    if(newVal) scrollToBottom();
});

// Watch for unread messages to play sound
const previousTotalUnread = ref(0);
watch(totalUnread, (newTotal, oldTotal) => {
  // Play sound if unread count increased
  // Sound will play if:
  // 1. Chat is closed, OR
  // 2. Chat is open but the new message is from a different conversation
  if (newTotal > oldTotal && newTotal > 0) {
    playNotificationSound();
  }
  previousTotalUnread.value = newTotal;
});

const toggleMinimize = () => {
   // Currently clicking header closes chat, logic matches "toggleChat" which toggles isOpen.
   // Minimizing behavior (keeping state but hiding window content) logic is handled by isOpen=false.
   // So toggleChat is fine for now as a simple open/close.
   chatStore.toggleChat();
};

const deleteMessage = (msgId) => {
  if (confirm('Supprimer ce message ?')) {
    chatStore.deleteMessage(msgId);
  }
};

const confirmClearHistory = () => {
   if (confirm('Voulez-vous vraiment effacer tout l\'historique de cette conversation ?')) {
      chatStore.clearHistory();
   }
};

</script>

<style scoped>
.chat-overlay {
  position: fixed;
  bottom: 20px;
  right: 20px;
  z-index: 9999;
}

.chat-window {
  width: 350px;
  height: 500px;
  max-width: calc(100vw - 40px);
  max-height: calc(100vh - 100px);
  display: flex;
  flex-direction: column;
}

@media (min-width: 768px) {
    .chat-window {
        width: 600px; /* Wider on desktop for sidebar + chat */
    }
}

/* Animation pour la bulle avec messages non lus */
@keyframes pulse-slow {
  0%, 100% {
    opacity: 1;
    box-shadow: 0 10px 15px -3px rgba(59, 130, 246, 0.5), 0 4px 6px -2px rgba(59, 130, 246, 0.5);
  }
  50% {
    opacity: 0.85;
    box-shadow: 0 20px 25px -5px rgba(59, 130, 246, 0.7), 0 10px 10px -5px rgba(59, 130, 246, 0.7);
  }
}

.animate-pulse-slow {
  animation: pulse-slow 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
</style>
