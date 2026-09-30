import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AuthCard from '@/components/auth/AuthCard.vue'

function mountCard(props: Record<string, unknown> = {}) {
  return mount(AuthCard, {
    props: { title: 'Sign in', submitLabel: 'Sign in', ...props },
    slots: { default: '<input id="email" />' },
  })
}

describe('AuthCard error announcement', () => {
  it('announces the error to assistive technology', () => {
    const wrapper = mountCard({ error: 'Invalid email or password' })

    const alert = wrapper.get('[role="alert"]')
    expect(alert.text()).toContain('Invalid email or password')
  })

  it('renders no alert without an error', () => {
    const wrapper = mountCard()

    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
})
