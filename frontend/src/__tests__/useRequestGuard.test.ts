import { describe, it, expect } from 'vitest'
import { createRequestGuard } from '@/composables/useRequestGuard'

describe('request guard', () => {
  it('keeps a token current until a newer request starts', () => {
    const guard = createRequestGuard()

    expect(guard.start().isCurrent()).toBe(true)
  })

  it('discards the older token when its response lands after the newer request started', async () => {
    const guard = createRequestGuard()
    const first = guard.start()
    const firstResponseLands = (async () => {
      await Promise.resolve() // the first response arrives only now
      return first.isCurrent()
    })()
    const second = guard.start()

    expect(await firstResponseLands).toBe(false)
    expect(second.isCurrent()).toBe(true)
  })

  it('adopts the latest request without invalidating an in-flight one', () => {
    const guard = createRequestGuard()
    const inFlight = guard.start()

    const adopted = guard.current()

    expect(inFlight.isCurrent()).toBe(true)
    expect(adopted.isCurrent()).toBe(true)
  })

  it('makes an adopted token stale when a newer request starts', () => {
    const guard = createRequestGuard()
    const adopted = guard.current()

    guard.start()

    expect(adopted.isCurrent()).toBe(false)
  })

  it('keeps only the last of many starts current', () => {
    const guard = createRequestGuard()

    const tokens = [guard.start(), guard.start(), guard.start()]

    expect(tokens.map((t) => t.isCurrent())).toEqual([false, false, true])
  })
})
