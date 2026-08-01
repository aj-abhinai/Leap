import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin } from '@tanstack/vue-query'
import App from './App.vue'
import router from './router'
import './style.css'
import { useAuthStore } from '@/stores/auth'

const pinia = createPinia()
const auth = useAuthStore(pinia)
await auth.bootstrap()

const app = createApp(App)
app.use(pinia)
app.use(router)
app.use(VueQueryPlugin)
app.mount('#app')
