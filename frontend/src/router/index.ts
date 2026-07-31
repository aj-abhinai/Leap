import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    public?: boolean
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/LoginPage.vue'),
    meta: { public: true, title: 'Sign In' },
  },
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('@/views/DashboardPage.vue'),
    meta: { title: 'Dashboard' },
  },
  {
    path: '/contacts/:id',
    name: 'ContactDetail',
    component: () => import('@/views/ContactDetailPage.vue'),
    meta: { title: 'Contact' },
  },
  {
    path: '/contacts',
    name: 'Contacts',
    component: () => import('@/views/ContactsPage.vue'),
    meta: { title: 'Contacts' },
  },
  {
    path: '/leads',
    name: 'Leads',
    component: () => import('@/views/LeadsPage.vue'),
    meta: { title: 'Leads' },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/SettingsPage.vue'),
    meta: { title: 'Settings' },
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/ProfilePage.vue'),
    meta: { title: 'Profile' },
  },
  {
    path: '/reminders',
    name: 'Reminders',
    component: () => import('@/views/RemindersPage.vue'),
    meta: { title: 'Reminders' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.isAuthenticated) {
    return next({ name: 'Login' })
  }
  if (to.meta.public && auth.isAuthenticated) {
    return next({ name: 'Dashboard' })
  }
  next()
})

export default router
