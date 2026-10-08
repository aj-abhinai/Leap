import { watchEffect } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { localEnv } from '@/tasks'
import { provideTasks } from '@/tasks/vue'

// useOrgTasks installs the app's task module: one env, provided to every
// descendant, whose reminder lead follows the settings store. Until the org
// value arrives the env keeps the module default, which mirrors the server
// default. Call once, from the authenticated shell.
export function useOrgTasks() {
  const settings = useSettingsStore()
  const env = localEnv()
  watchEffect(() => {
    const minutes = settings.nudgeLeadMinutes
    if (minutes !== null) env.defaults.nudgeLeadMinutes = minutes
  })
  provideTasks({ env })
  void settings.fetchNudgeLead()
  return env
}
