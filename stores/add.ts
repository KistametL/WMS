import { defineStore } from 'pinia'

export const useAddStore = defineStore('add', {
    state: () => ({
        dsn: null as string | null,
        location: null as any,
        locationSuggestions: [] as any[],
        isLoading: false,
        error: null as string | null
    }),
    persist: true,
    actions: {
        setDsn(dsn: string) {
            this.dsn = dsn
        },
        setLocation(location: any) {
            this.location = location
        },
        setLocationSuggestions(suggestions: any[]) {
            this.locationSuggestions = suggestions
        },
        setLoading(loading: boolean) {
            this.isLoading = loading
        },
        setError(error: string | null) {
            this.error = error
        }
    },
    getters: {
        hasDsn: (state) => !!state.dsn,
        hasLocation: (state) => !!state.location
    }
})