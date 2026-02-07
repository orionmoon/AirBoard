<template>
  <div class="home-page">
    <!-- Loading State -->
    <div v-if="isLoading" class="loading-state">
      <div class="loading-spinner">
        <Icon icon="mdi:loading" class="spinner-icon" />
        <p class="loading-text">{{ $t('home.loading') }}</p>
      </div>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-state">
      <div class="error-card">
        <Icon icon="mdi:alert-circle" class="error-icon" />
        <h3 class="error-title">{{ $t('home.error.title') }}</h3>
        <p class="error-message">{{ $t('home.error.message') }}</p>
        <button @click="loadHomeData" class="retry-btn">
          <Icon icon="mdi:refresh" class="btn-icon" />
          {{ $t('common.retry') }}
        </button>
      </div>
    </div>

    <!-- Home Content -->
    <div v-else class="home-content">
      <!-- Hero Section -->
      <HeroSection
        :stats="{
          apps: homeData.stats?.total_apps || homeData.new_apps?.length || 0,
          events: (homeData.today_events?.length || 0) + (homeData.upcoming_events?.length || 0),
          news: homeData.recent_news?.length || 0,
          polls: homeData.polls?.length || 0
        }"
        :show-quick-stats="true"
        :announcements="homeData.announcements || []"
        :hero-messages="homeData.hero_messages || []"
      />

      <!-- Optimized 3-Column Grid Layout -->
      <div class="content-layout">
        <!-- Column 1 (25% width) - Administrative & Personal -->
        <div class="column-1">
          <!-- Statistics Widget (Adaptive Height) - Admin/Group Admin Only -->
          <div v-if="showStats" class="bento-item stats-widget-container" data-aos="fade-up">
            <StatsWidget :stats="homeData.stats" :role="homeData.user_role" :managedGroupIds="homeData.managed_group_ids" />
          </div>

          <!-- Gamification Widget -->
          <div v-if="homeData.gamification" class="bento-item" data-aos="fade-up" data-aos-delay="100">
            <GamificationWidget :summary="homeData.gamification" />
          </div>

          <!-- Favorite Apps -->
          <div v-if="homeData.favorite_apps?.length > 0" class="bento-item" data-aos="fade-up" data-aos-delay="200">
            <FavoriteAppsWidget :apps="homeData.favorite_apps" />
          </div>

          <!-- New Apps (3 dernières apps ajoutées) -->
          <div v-if="homeData.new_apps?.length > 0" class="bento-item" data-aos="fade-up" data-aos-delay="250">
            <NewAppsWidget :apps="homeData.new_apps.slice(0, 3)" />
          </div>
        </div>

        <!-- Column 2 (50% width) - Content Hub -->
        <div class="column-2">
          <!-- Recent News (Medium) -->
          <div v-if="homeData.recent_news?.length > 0" class="bento-item" data-aos="fade-up" data-aos-delay="100">
            <RecentNewsWidget :news="homeData.recent_news" />
          </div>


        </div>

        <!-- Column 3 (25% width) - Time-based Information -->
        <div class="column-3">
          <!-- Today's Events -->
          <div v-if="homeData.today_events" class="bento-item" data-aos="fade-up" data-aos-delay="100">
            <TodayEventsWidget :events="homeData.today_events" />
          </div>

          <!-- Upcoming Events (limit to 3) -->
          <div v-if="homeData.upcoming_events" class="bento-item" data-aos="fade-up" data-aos-delay="150">
            <UpcomingEventsWidget :events="homeData.upcoming_events.slice(0, 3)" />
          </div>

          <!-- Polls Widget -->
          <div v-if="homeData.polls?.length > 0" class="bento-item" data-aos="fade-up" data-aos-delay="300">
            <PollsWidget :polls="homeData.polls" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { homeService } from '@/services/api'

// Components
import HeroSection from '@/components/home/HeroSection.vue'
import StatsWidget from '@/components/home/StatsWidget.vue'
import TodayEventsWidget from '@/components/home/TodayEventsWidget.vue'
import FavoriteAppsWidget from '@/components/home/FavoriteAppsWidget.vue'
import NewAppsWidget from '@/components/home/NewAppsWidget.vue'
import UpcomingEventsWidget from '@/components/home/UpcomingEventsWidget.vue'
import RecentNewsWidget from '@/components/home/RecentNewsWidget.vue'
import PollsWidget from '@/components/home/PollsWidget.vue'
import GamificationWidget from '@/components/home/GamificationWidget.vue'

const authStore = useAuthStore()

// State
const homeData = ref({})
const isLoading = ref(true)
const error = ref(null)

// Computed
const showStats = computed(() => {
  return homeData.value.stats && (homeData.value.user_role === 'admin' || homeData.value.managed_group_ids?.length > 0)
})



// Helper function to get welcome message with fallback
const getWelcomeMessage = () => {
  return homeData.value.app_settings?.welcome_message || 
         homeData.value.app_settings?.WelcomeMessage || 
         ''
}

// Methods
const loadHomeData = async () => {
  try {
    isLoading.value = true
    error.value = null
    homeData.value = await homeService.getHomeData()
  } catch (err) {
    console.error('Failed to load home data:', err)
    error.value = err
  } finally {
    isLoading.value = false
  }
}

// Lifecycle
onMounted(() => {
  loadHomeData()

  // Initialize AOS (Animate On Scroll) if available
  if (typeof AOS !== 'undefined') {
    AOS.init({
      duration: 800,
      easing: 'ease-out-cubic',
      once: true,
      offset: 50
    })
  }
})
</script>

<style scoped>
.home-page {
  min-height: 100vh;
  background: linear-gradient(135deg, #f5f7fa 0%, #e9ecef 100%);
  padding: 0;
  margin: 0;
  overflow-x: hidden;
  width: 100%;
  max-width: 100vw;
  box-sizing: border-box;
}

.dark .home-page {
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%);
}

/* Loading State */
.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}

.loading-spinner {
  text-align: center;
}

.spinner-icon {
  font-size: 3rem;
  color: #667eea;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.loading-text {
  margin-top: 1rem;
  color: #6b7280;
  font-size: 1rem;
}

.dark .loading-text {
  color: #9ca3af;
}

/* Error State */
.error-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
  padding: 2rem;
}

.error-card {
  background: white;
  border-radius: 24px;
  padding: 3rem 2rem;
  text-align: center;
  max-width: 400px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.1);
}

.dark .error-card {
  background: #1e293b;
}

.error-icon {
  font-size: 4rem;
  color: #ef4444;
  margin-bottom: 1rem;
}

.error-title {
  font-size: 1.5rem;
  font-weight: 700;
  color: #1f2937;
  margin-bottom: 0.5rem;
}

.dark .error-title {
  color: white;
}

.error-message {
  color: #6b7280;
  margin-bottom: 1.5rem;
}

.dark .error-message {
  color: #9ca3af;
}

.retry-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  border: none;
  border-radius: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
}

.retry-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(102, 126, 234, 0.4);
}

.btn-icon {
  font-size: 1.25rem;
}

/* Home Content */
.home-content {
  animation: fadeIn 0.6s ease-out;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Optimized 3-Column Content Layout: 25% - 50% - 25% */
.content-layout {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.125rem;
  padding: 0 1rem 1.5rem;
  max-width: 1400px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
  overflow-x: hidden;
}

/* Optimisation spécifique pour 1280x720 */
@media (width: 1280px) and (height: 720px) {
  .content-layout {
    max-width: 1100px;
    padding: 0 0 1.25rem;
    gap: 1rem;
  }
  
  .bento-item {
    padding: 1rem;
    border-radius: 16px;
  }
  
  /* Réduire la hauteur minimum des éléments pour éviter le défilement */
  .main-column .bento-tall {
    min-height: 400px;
  }
  
  /* Optimiser les espacements internes */
  .bento-item :deep(.widget-header) {
    margin-bottom: 0.5rem !important;
    padding-bottom: 0.375rem !important;
  }
  
  .bento-item :deep(.header-icon-wrapper) {
    width: 28px !important;
    height: 28px !important;
  }
  
  .bento-item :deep(.header-icon) {
    font-size: 1rem !important;
  }
  
  .bento-item :deep(.widget-title) {
    font-size: 0.85rem !important;
  }
  
  .bento-item :deep(.app-card-inner),
  .bento-item :deep(.event-card) {
    padding: 0.75rem !important;
  }
  
  .bento-item :deep(.app-icon) {
    width: 36px !important;
    height: 36px !important;
    margin-bottom: 0.375rem !important;
  }
  
  .bento-item :deep(.icon) {
    font-size: 1.125rem !important;
  }
  
  .bento-item :deep(.app-name),
  .bento-item :deep(.event-title) {
    font-size: 0.75rem !important;
    margin-bottom: 0.2rem !important;
  }
  
  .bento-item :deep(.app-description),
  .bento-item :deep(.event-description),
  .bento-item :deep(.event-time) {
    font-size: 0.65rem !important;
  }
}

/* 3-Column Grid System */
.column-1,
.column-2,
.column-3 {
  display: flex;
  flex-direction: column;
  gap: 1.125rem;
  min-width: 0; /* Prevent overflow */
  width: 100%;
}

/* Desktop layout: 25% - 50% - 25% */
@media (min-width: 1024px) {
  .content-layout {
    grid-template-columns: 1fr 2fr 1fr;
    gap: 1.5rem;
    padding: 0 0 1.5rem;
    width: 100%;
    max-width: 1400px;
  }
  
  .column-1 {
    display: flex;
    flex-direction: column;
    gap: 1.125rem;
    min-width: 0;
    width: 100%;
  }
  
  .column-2 {
    display: flex;
    flex-direction: column;
    gap: 1.125rem;
    min-width: 0;
    width: 100%;
  }
  
  .column-3 {
    display: flex;
    flex-direction: column;
    gap: 1.125rem;
    min-width: 0;
    width: 100%;
  }
  
  /* Ensure bento items don't overflow */
  .bento-item {
    width: 100%;
    box-sizing: border-box;
    overflow: hidden;
  }
  
  /* Ensure the grid items respect their column boundaries */
  .column-1,
  .column-2,
  .column-3 {
    overflow: hidden;
  }
}

/* Optimisation pour 1280x720 et résolutions similaires */
@media (max-width: 1366px) {
  .content-layout {
    max-width: 1200px;
    padding: 0 0 1.5rem;
  }
}

.bento-item {
  background: white;
  border-radius: 20px;
  padding: 1.125rem;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
}

/* Réduire les espacements internes de tous les widgets */
.bento-item :deep(.widget-header) {
  margin-bottom: 0.5rem !important;
  padding-bottom: 0.375rem !important;
}

.bento-item :deep(.header-icon-wrapper) {
  width: 28px !important;
  height: 28px !important;
}

.bento-item :deep(.header-icon) {
  font-size: 1rem !important;
}

.bento-item :deep(.widget-title) {
  font-size: 0.875rem !important;
  font-weight: 600 !important;
}

.bento-item :deep(.apps-grid),
.bento-item :deep(.events-list),
.bento-item :deep(.polls-list),
.bento-item :deep(.news-list) {
  gap: 0.25rem !important;
}

.bento-item :deep(.app-card-inner),
.bento-item :deep(.event-card),
.bento-item :deep(.poll-item),
.bento-item :deep(.news-list-item) {
  padding: 0.375rem 0.5rem !important;
}

.bento-item :deep(.app-icon) {
  width: 32px !important;
  height: 32px !important;
  margin-bottom: 0.375rem !important;
}

.bento-item :deep(.icon) {
  font-size: 1rem !important;
}

.bento-item :deep(.app-name),
.bento-item :deep(.event-title) {
  font-size: 0.75rem !important;
  margin-bottom: 0.2rem !important;
}

.bento-item :deep(.app-description),
.bento-item :deep(.event-description),
.bento-item :deep(.event-time) {
  font-size: 0.65rem !important;
}

/* Styles spécifiques pour le widget statistiques */
.bento-item :deep(.stats-widget) {
  height: auto !important;
}

.bento-item.stats-widget-container {
  height: auto !important;
  min-height: auto !important;
  max-height: none !important;
}

.bento-item.stats-widget-container :deep(.stats-widget) {
  height: auto !important;
}

.bento-item.stats-widget-container :deep(.stats-list) {
  flex: none !important;
  max-height: none !important;
}

.bento-item :deep(.stats-widget .widget-header) {
  margin-bottom: 0.375rem !important;
  padding-bottom: 0.375rem !important;
}

.bento-item :deep(.stats-widget .header-icon) {
  font-size: 1rem !important;
}

.bento-item :deep(.stats-widget .widget-title) {
  font-size: 0.875rem !important;
  font-weight: 600 !important;
}

.bento-item :deep(.stats-widget .stats-list) {
  gap: 0.25rem !important;
}

.bento-item :deep(.stats-widget .stat-row) {
  padding: 0.3rem 0.4rem !important;
  gap: 0.375rem !important;
  border-radius: 6px !important;
}

.bento-item :deep(.stats-widget .stat-icon) {
  font-size: 1.125rem !important;
}

.bento-item :deep(.stats-widget .stat-value) {
  font-size: 1rem !important;
  min-width: 2rem !important;
  font-weight: 600 !important;
}

.bento-item :deep(.stats-widget .stat-label) {
  font-size: 0.6875rem !important;
}

.bento-item :deep(.stats-widget .stat-arrow) {
  font-size: 0.875rem !important;
}

/* Optimisations générales pour tous les widgets */
.bento-item :deep(.event-item) {
  padding: 0.25rem !important;
}

.bento-item :deep(.event-date-badge) {
  width: 36px !important;
  height: 36px !important;
}

.bento-item :deep(.event-date-badge .text-xs) {
  font-size: 0.625rem !important;
}

.bento-item :deep(.event-date-badge .text-lg) {
  font-size: 0.875rem !important;
}

.bento-item :deep(.news-list-item .h-9) {
  width: 28px !important;
  height: 28px !important;
}

.bento-item :deep(.news-list-item .h-4) {
  font-size: 0.875rem !important;
}

.bento-item :deep(.badge-stat) {
  padding: 0.0625rem 0.25rem !important;
  font-size: 0.625rem !important;
}

.bento-item :deep(.header-icon-wrapper) {
  width: 36px !important;
  height: 36px !important;
}

.bento-item :deep(.header-icon) {
  font-size: 1.25rem !important;
}

.bento-item :deep(.widget-title) {
  font-size: 1rem !important;
}

.bento-item :deep(.apps-grid),
.bento-item :deep(.events-list) {
  gap: 0.75rem !important;
}

.bento-item :deep(.app-card-inner),
.bento-item :deep(.event-card) {
  padding: 1rem !important;
}

.bento-item :deep(.app-icon) {
  width: 48px !important;
  height: 48px !important;
  margin-bottom: 0.75rem !important;
}

.bento-item :deep(.icon) {
  font-size: 1.5rem !important;
}

.bento-item :deep(.app-name),
.bento-item :deep(.event-title) {
  font-size: 0.875rem !important;
  margin-bottom: 0.25rem !important;
}

.bento-item :deep(.app-description),
.bento-item :deep(.event-description),
.bento-item :deep(.event-time) {
  font-size: 0.75rem !important;
}

.dark .bento-item {
  background: #1e293b;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
}

.bento-item::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: linear-gradient(90deg, #667eea 0%, #764ba2 50%, #f093fb 100%);
  transform: scaleX(0);
  transition: transform 0.4s ease;
}

.bento-item:hover {
  transform: translateY(-8px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
}

.dark .bento-item:hover {
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
}

.bento-item:hover::before {
  transform: scaleX(1);
}



/* Special styling for tall items */
.column-1 .bento-tall,
.column-2 .bento-tall,
.column-3 .bento-tall {
  min-height: 500px;
}

/* Tablet layout: 2-column stack */
@media (max-width: 1023px) and (min-width: 769px) {
  .home-page {
    margin: 0;
  }

  .content-layout {
    padding: 0 1rem 1.25rem;
    gap: 1rem;
    max-width: 900px;
    grid-template-columns: 1fr 1fr; /* 2-column layout for tablets */
    width: 100%;
  }

  /* Column 1 and 2 on top row, Column 3 on bottom row */
  .column-1 {
    grid-column: 1;
  }
  
  .column-2 {
    grid-column: 2;
  }
  
  .column-3 {
    grid-column: 1 / -1; /* Span full width */
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  
  .column-3 .bento-item {
    margin: 0;
  }

  .bento-item {
    border-radius: 18px;
    padding: 1.125rem;
    width: 100%;
    box-sizing: border-box;
  }
}

@media (max-width: 768px) {
  .home-page {
    margin: 0;
  }

  .content-layout {
    padding: 0 1rem 1rem;
    gap: 0.875rem;
    max-width: 100%;
    width: 100%;
  }

  .sidebar-column,
  .main-column {
    gap: 0.875rem;
  }

  .bento-item {
    border-radius: 16px;
    padding: 0.875rem;
  }

  .main-column .bento-tall {
    min-height: 400px;
  }
}

@media (max-width: 480px) {
  .home-page {
    margin: 0;
  }

  .content-layout {
    padding: 0 0.75rem 0.875rem;
    gap: 0.75rem;
    max-width: 100%;
    width: 100%;
  }

  .sidebar-column,
  .main-column {
    gap: 0.75rem;
  }

  .bento-item {
    border-radius: 14px;
    padding: 0.75rem;
  }

  .main-column .bento-tall {
    min-height: 350px;
  }

  /* Réduire encore plus les espacements internes */
  .bento-item :deep(.widget-header) {
    margin-bottom: 0.75rem !important;
  }

  .bento-item :deep(.header-icon-wrapper) {
    width: 32px !important;
    height: 32px !important;
  }

  .bento-item :deep(.header-icon) {
    font-size: 1.1rem !important;
  }

  .bento-item :deep(.widget-title) {
    font-size: 0.875rem !important;
  }
}
</style>
