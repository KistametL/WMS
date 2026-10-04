<template>
  <v-app>
    <h2 class="d-flex align-center">
      <v-icon size="32">mdi-map-marker</v-icon>
      Suggestion Location
    </h2>
    <inputCode
        placeholder="Scan DSN"
        sourceComponent="IndexPage"
        :loading="loading"
        @update:searchDSN="suggestionLocation"
        @update:searchSKU="handleSearchSKU"
        @update:searchLocation="handleSearchLocation"
        @update:searchBasket="handleSearchBasket"
    />
    <div style="padding: 0 12px">
      <v-divider color="black"></v-divider>
    </div>
  </v-app>
</template>

<script setup>
import inputCode from '~/components/inputCode.vue'
import {useAddApi} from "~/composables/api/useAddApi.ts";
import { useAddStore } from "~/stores/add.js";

const { getSuggestionLocation } = useAddApi()
const loading = ref(false)
const emit = defineEmits(['next'])
const addStore = useAddStore()

const suggestionLocation = async (dsn) => {
  loading.value = true
  const res = await getSuggestionLocation('6JXKJ0LYC')
  loading.value = false
  if (res) {
    addStore.suggestionLocation = res.data
    goToNextStep()
  } else {
    console.error('Error fetching suggestion location:', res.message)
  }
}

const goToNextStep = () => {
  emit('next', 'location')
}
const handleSearchDSN = (dsn) => {
  console.log('DSN:', dsn)
}

const handleSearchSKU = (sku) => {
  console.log('SKU:', sku)
}

const handleSearchLocation = (loc) => {
  console.log('Location:', loc)
}

const handleSearchBasket = (basket) => {
  console.log('Basket:', basket)
}
</script>

