import { defineNuxtRouteMiddleware, navigateTo } from '#app'
import { useAuthStore } from '~/stores/auth'

export default defineNuxtRouteMiddleware((to) => {
    const auth = useAuthStore()

    if (!auth.isLoggedIn && to.path !== '/account/login') {
        return navigateTo('/account/login')
    }

    if (auth.isLoggedIn && to.path === '/account/login') {
        return navigateTo('/test')
    }
})
