import { ref } from 'vue'

const splashVisible = ref(true)
const MIN_DURATION = 200
let timer: ReturnType<typeof setTimeout> | undefined
let shownAt = 0

export function useSplash() {
  function showSplash() {
    if (timer) clearTimeout(timer)
    timer = undefined
    shownAt = Date.now()
    splashVisible.value = true
  }

  // Hides after the remainder of the 200ms minimum hold, so the splash keeps
  // covering the caller's work (e.g. a lazy route loading) however long it
  // takes, while never showing for less than the minimum.
  function hideSplash() {
    const remaining = MIN_DURATION - (Date.now() - shownAt)
    timer = setTimeout(() => {
      splashVisible.value = false
      timer = undefined
    }, Math.max(0, remaining))
  }

  return { splashVisible, showSplash, hideSplash }
}
