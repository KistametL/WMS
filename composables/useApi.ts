import axios from 'axios'
import { useRuntimeConfig } from '#app'

export const useApi = () => {
    const config = useRuntimeConfig()

    const token = localStorage.getItem('access_token')

    const instance = axios.create({
        baseURL: config.public.backend,
        headers: {
            ...(token ? { Authorization: `JWT ${token}` } : {})
        }
    })

    const get = (url: string, options = {}) => instance.get(url, options)
    const post = (url: string, data = {}, options = {}) => instance.post(url, data, options)

    return { get, post }
}
