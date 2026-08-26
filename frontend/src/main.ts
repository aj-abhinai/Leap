import { createApp } from 'vue'
import { createPinia } from 'pinia'
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
app.mount('#app')

// The app is ready: fade out the static splash from index.html. The Vue
// splash overlay takes over visually for the 200ms hold, so the crossfade
// is seamless.
const splash = document.getElementById('splash')
if (splash) {
  splash.classList.add('splash-hide')
  window.setTimeout(() => splash.remove(), 350)
}
