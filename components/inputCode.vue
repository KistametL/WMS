<template>
  <div class="w-100 pa-3">
    <v-text-field
        ref="inputRef"
        v-model="inputValue"
        :placeholder="placeholder"
        :inputmode="inputMode"
        variant="outlined"
        hide-details
        density="comfortable"
        @keyup.enter="handleInput"
        append-inner-icon="mdi-magnify"
        @click:append-inner="handleInput"
        :loading="loading"
        clearable
    />
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import Swal from 'sweetalert2'
import { useRuntimeConfig, useNuxtApp } from '#app'

const props = defineProps({
  modelValue: { type: String, default: '' },
  inputMode: { type: String, default: 'text' },
  placeholder: { type: String, default: 'Search' },
  sourceComponent: { type: String, default: 'UnknownComponent' },
  loading: { type: Boolean, default: false }
})

const emit = defineEmits(['update:searchBasket', 'update:searchDSN', 'update:searchLocation', 'update:searchSKU'])

const inputValue = ref(props.modelValue)
const inputRef = ref(null)
const parsedInput = ref('')
const config = useRuntimeConfig()
const { $axios } = useNuxtApp()
const userInfo = ref({ username: 'anonymous' })

watch(() => props.modelValue, (newVal) => {
  inputValue.value = newVal
})

const handleInput = async () => {
  const trimmedInput = inputValue.value.trim()
  inputValue.value = ''
  if (trimmedInput === '') {
    return showEmptyAlert()
  }

  const matchPattern = /\/([A-Za-z0-9-]+)$/
  const validPattern = /^[A-Za-z0-9-]+$/
  const match = trimmedInput.match(matchPattern)
  parsedInput.value = match ? match[1] : trimmedInput

  if (!validPattern.test(parsedInput.value)) {
    return showInvalidAlert()
  }

  if (parsedInput.value.length <= 4 && !parsedInput.value.includes('-')) {
    emit('update:searchBasket', parsedInput.value)
  } else if (parsedInput.value.length > 4 && parsedInput.value.length <= 9 && !parsedInput.value.includes('-')) {
    emit('update:searchDSN', parsedInput.value)
  } else if (parsedInput.value.includes('-') && parsedInput.value.length === 12) {
    emit('update:searchLocation', parsedInput.value)
  } else if (parsedInput.value.includes('-') && parsedInput.value.length >= 5 && parsedInput.value.length <= 10) {
    emit('update:searchSKU', parsedInput.value)
  } else {
    await Swal.fire({
      title: 'Error',
      text: 'Invalid format.',
      icon: 'error',
      confirmButtonText: 'OK',
      confirmButtonColor: '#22252f',
      allowOutsideClick: false,
      allowEscapeKey: false,
      allowEnterKey: false
    })
  }
}

const showInvalidAlert = () => {
  const audio = new Audio(require('@/assets/warning-sound.mp3'))
  audio.play()
  Swal.fire({
    title: 'Invalid input',
    text: 'Please enter only English letters and numbers.',
    icon: 'error',
    confirmButtonText: 'OK',
    confirmButtonColor: '#22252f',
    allowOutsideClick: false,
    allowEscapeKey: false,
    allowEnterKey: false
  })
}

const showEmptyAlert = () => {
  const audio = new Audio(require('@/assets/warning-sound.mp3'))
  audio.play()
  Swal.fire({
    icon: 'error',
    title: 'Error',
    text: 'Please enter the input',
    confirmButtonText: 'OK',
    confirmButtonColor: '#22252f',
    allowOutsideClick: false,
    allowEscapeKey: false,
    allowEnterKey: false
  })
}

const focusInput = () => {
  inputRef.value?.focus()
}
</script>

<style scoped>
.custom-placeholder::placeholder {
  color: #CBCBCB;
  font-size: 25px;
}
</style>