import { localEnv, type OrgDefaults, type TaskEnv } from './index'

// fixedEnv pins the clock at one instant so module tests and component tests
// are deterministic; the org defaults default to the production values.
export function fixedEnv(now: string, defaults: Partial<OrgDefaults> = {}): TaskEnv {
  const at = new Date(now)
  return { ...localEnv(defaults), clock: { now: () => new Date(at) } }
}
