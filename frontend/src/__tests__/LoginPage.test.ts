import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import LoginPage from '@/views/LoginPage.vue'
import { createRouter, createWebHistory } from 'vue-router'

describe('LoginPage', () => {
  it('renders login form', () => {
    setActivePinia(createPinia())
    const router = createRouter({
      history: createWebHistory(),
      routes: [{ path: '/login', component: LoginPage }],
    })

    const wrapper = mount(LoginPage, {
      global: {
        plugins: [createPinia(), router],
      },
    })

    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.html()).toContain('Sign In')
  })

  it('shows error on empty submit', async () => {
    setActivePinia(createPinia())
    const router = createRouter({
      history: createWebHistory(),
      routes: [{ path: '/login', component: LoginPage }],
    })

    const wrapper = mount(LoginPage, {
      global: {
        plugins: [createPinia(), router],
      },
    })

    await wrapper.find('input#email').setValue('')
    await wrapper.find('input#password').setValue('')
    await wrapper.find('form').trigger('submit')
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Email and password are required')
  })
})
