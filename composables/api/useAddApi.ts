import { useApi } from '@/composables/useApi'

export const useAddApi = () => {
    const { get } = useApi()

    const getSuggestionLocation = (dsn: string) => {
        return get(`/api/suggest/locationV2/${dsn}`)
    }

    return {
        getSuggestionLocation,
    }
}
