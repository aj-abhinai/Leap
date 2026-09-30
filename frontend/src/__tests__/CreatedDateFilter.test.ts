import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import CreatedDateFilter from '@/components/leads/CreatedDateFilter.vue'

// The real popover renders through reka-ui's portal, which unmounts when
// closed — jsdom cannot open it. Stub the popover module so its content
// renders inline, keeping the real range calendar under test.
vi.mock('@/components/ui/popover', () => ({
  Popover: { template: '<div><slot /></div>' },
  PopoverTrigger: { template: '<div data-testid="trigger"><slot /></div>' },
  PopoverContent: { template: '<div data-testid="content"><slot /></div>' },
}))

function mountFilter(props: Record<string, unknown> = {}) {
  return mount(CreatedDateFilter, {
    props: { preset: 'all', from: '', to: '' as string, ...props },
  })
}

describe('CreatedDateFilter', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    // Tuesday 15 September 2026, noon.
    vi.setSystemTime(new Date(2026, 8, 15, 12, 0, 0))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('labels the trigger with the active preset', () => {
    const wrapper = mountFilter({ preset: 'week' })

    expect(wrapper.find('[data-testid="trigger"]').text()).toContain('This week')
  })

  it('labels a custom range with its days', () => {
    const wrapper = mountFilter({ preset: 'custom', from: '2026-09-01', to: '2026-09-03' })

    const label = wrapper.find('[data-testid="trigger"]').text()
    expect(label).not.toContain('All dates')
    expect(label).toContain('Sep')
    expect(label).toContain('1')
    expect(label).toContain('3')
  })

  it('falls back to the custom-range label when the days are unusable', () => {
    const wrapper = mountFilter({ preset: 'custom', from: '', to: '' })

    expect(wrapper.find('[data-testid="trigger"]').text()).toContain('Custom range')
  })

  it('emits the picked preset', async () => {
    const wrapper = mountFilter()

    const today = wrapper.findAll('button').find((b) => b.text() === 'Today')
    expect(today).toBeTruthy()
    await today!.trigger('click')

    expect(wrapper.emitted('select-preset')).toEqual([['today']])
  })

  it('emits a range only once both days are picked', async () => {
    const wrapper = mountFilter()

    const start = wrapper.findAll('[data-value="2026-09-10"]')[0]
    const end = wrapper.findAll('[data-value="2026-09-12"]')[0]
    expect(start).toBeTruthy()
    expect(end).toBeTruthy()

    // The calendar tracks the hovered day before a click, as a pointer does.
    await start!.trigger('mouseenter')
    await start!.trigger('click')
    expect(wrapper.emitted('select-range')).toBeUndefined()

    await end!.trigger('mouseenter')
    await end!.trigger('click')
    expect(wrapper.emitted('select-range')).toEqual([['2026-09-10', '2026-09-12']])
  })
})
