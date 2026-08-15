import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LayoutShell from '@/components/layout/LayoutShell.vue'

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
    path: '/change-password',
    name: 'ChangePassword',
    component: () => import('@/views/ChangePasswordPage.vue'),
    meta: { title: 'Change Password' },
  },
  {
    path: '/',
    component: LayoutShell,
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/DashboardPage.vue'),
        meta: { title: 'Dashboard' },
      },
      {
        path: 'contacts/:id',
        name: 'ContactDetail',
        component: () => import('@/views/ContactDetailPage.vue'),
        meta: { title: 'Contact' },
      },
      {
        path: 'contacts',
        name: 'Contacts',
        component: () => import('@/views/ContactsPage.vue'),
        meta: { title: 'Contacts' },
      },
      {
        path: 'leads',
        name: 'Leads',
        component: () => import('@/views/LeadsPage.vue'),
        meta: { title: 'Leads' },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/SettingsPage.vue'),
        meta: { title: 'Settings' },
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/views/ProfilePage.vue'),
        meta: { title: 'Profile' },
      },
      {
        path: 'reminders',
        name: 'Reminders',
        component: () => import('@/views/RemindersPage.vue'),
        meta: { title: 'Reminders' },
      },
      {
        path: 'activities',
        name: 'Activities',
        component: () => import('@/views/ActivitiesPage.vue'),
        meta: { title: 'Activities' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  // Reset scroll when moving between pages so a tall page (e.g. kanban) does
  // not leave the next page mid-scroll; query-only navigation keeps position.
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.path !== from.path) return { top: 0 }
    return undefined
  },
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.isAuthenticated) {
    return next({ name: 'Login' })
  }
  if (to.meta.public && auth.isAuthenticated) {
    if (auth.mustChangePassword) {
      return next({ name: 'ChangePassword' })
    }
    return next({ name: 'Dashboard' })
  }
  if (
    to.name !== 'ChangePassword' &&
    auth.isAuthenticated &&
    auth.mustChangePassword
  ) {
    return next({ name: 'ChangePassword' })
  }
  next()
})

export default router
