import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', {
    state: () => ({
        token: null as string | null,
        user: null as any
    }),
    persist: true,
    actions: {
        setToken(token: string) {
            this.token = token
        },
        setUser(user: any) {
            this.user = user
        },
        logout() {
            this.token = null
            this.user = null
        }
    },
    getters: {
        isLoggedIn: (state) => !!state.token
    }
})
