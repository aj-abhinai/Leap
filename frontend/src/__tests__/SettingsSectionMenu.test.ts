import { describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SettingsSectionMenu from '@/components/settings/SettingsSectionMenu.vue'

const sections = [
  { value: 'first', label: 'First' },
  { value: 'second', label: 'Second' },
]

function mountMenu() {
  return mount(SettingsSectionMenu, {
    props: { sections },
    slots: {
      first: '<p>first panel</p>',
      second: '<p>second panel</p>',
    },
  })
}

function switchTo(wrapper: ReturnType<typeof mountMenu>, value: string) {
  wrapper.findComponent({ name: 'Tabs' }).vm.$emit('update:model-value', value)
  return flushPromises()
}

describe('SettingsSectionMenu', () => {
  beforeEach(() => localStorage.clear())

  it('opens the first available section with every name visible', () => {
    const wrapper = mountMenu()

    expect(wrapper.text()).toContain('First')
    expect(wrapper.text()).toContain('Second')
    expect(wrapper.text()).toContain('first panel')
    expect(wrapper.text()).not.toContain('second panel')
  })

  it('shows one section at a time when the selection changes', async () => {
    const wrapper = mountMenu()

    await switchTo(wrapper, 'second')

    expect(wrapper.text()).toContain('second panel')
    expect(wrapper.text()).not.toContain('first panel')
  })

  it('starts at the first section again on a new visit and persists nothing', async () => {
    const wrapper = mountMenu()
    await switchTo(wrapper, 'second')
    expect(wrapper.text()).toContain('second panel')
    wrapper.unmount()

    expect(localStorage.length).toBe(0)

    const second = mountMenu()
    expect(second.text()).toContain('first panel')
    expect(second.text()).not.toContain('second panel')
  })
})
