import { computed, inject, provide, unref, type ComputedRef, type InjectionKey, type MaybeRef } from 'vue'
import {
  localEnv,
  present,
  resolveTask,
  type TaskEnv,
  type TaskPresentation,
  type TaskSource,
  type views,
} from './index'

// The Vue layer: one task module per app (or per test), shared through
// provide/inject. The module stays pure; only the env lives here.

export interface TaskModule {
  env: TaskEnv
}

export const taskModuleKey: InjectionKey<TaskModule> = Symbol('crm-tasks')

// provideTasks shares a configured module with every descendant.
export function provideTasks(module: TaskModule): TaskModule {
  provide(taskModuleKey, module)
  return module
}

let fallback: TaskModule | null = null

// useTasks returns the injected module, or one lazily created local env.
export function useTasks(): TaskModule {
  const injected = inject(taskModuleKey, null)
  if (injected) return injected
  if (!fallback) fallback = { env: localEnv() }
  return fallback
}

// useTaskPresentation resolves a reactive source into display segments.
export function useTaskPresentation(
  t: MaybeRef<TaskSource>,
  view: keyof typeof views = 'full',
): ComputedRef<TaskPresentation> {
  const tasks = useTasks()
  return computed(() => present(resolveTask(unref(t), tasks.env), view, tasks.env))
}
