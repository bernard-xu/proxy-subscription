import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { requiresAuth: true }
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue')
    },
    {
      path: '/subscriptions',
      name: 'Subscriptions',
      component: () => import('@/views/SubscriptionView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/proxies',
      name: 'proxies',
      component: () => import('@/views/ProxyView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/SettingsView.vue'),
      meta: { requiresAuth: true }
    }
  ]
})

// 导航守卫
router.beforeEach((to, from) => {
  const authStore = useAuthStore()
  
  // 如果需要登录且未登录，重定向到登录页
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return {
      path: '/login',
      query: { redirect: to.fullPath }
    }
  }
  
  // 如果已登录，尝试访问登录页，重定向到首页
  if (to.name === 'login' && authStore.isAuthenticated) {
    return { path: '/' }
  }
})

export default router
