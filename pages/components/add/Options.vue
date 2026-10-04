<template>
  <v-app>
    <h2 class="d-flex align-center">
      <v-icon size="32" class="mr-1">mdi-basket</v-icon>
      Options
    </h2>

    <div class="d-flex align-center mt-2">
      <v-icon size="24">mdi-map-marker</v-icon>
      {{ addStore.suggestionLocation[0]?.location[0]?.location || 'No location' }}
    </div>

    <div style="padding: 0 12px ; margin-top: 8px">
      <v-divider color="black"></v-divider>
    </div>

    <v-container
        fluid
        class="d-flex flex-column align-center justify-center"
        style="min-height: 60vh; gap: 20px;"
    >
      <ButtonOptions
          icon="mdi-basket-plus"
          iconColor="#22C55E"
          btnColor="#101010"
          label="New Basket"
          :onClick="goToNextStep"
      />

      <ButtonOptions
        icon="mdi-basket-remove"
        iconColor="#FF2626"
        btnColor="#101010"
        label="Without Basket"
        :onClick="goToNextStep"
      />
    </v-container>
  </v-app>
</template>


<script setup>
import inputCode from '~/components/inputCode.vue'
import {useAddApi} from "~/composables/api/useAddApi.ts";
import { useAddStore } from "~/stores/add.js";
import ButtonOptions from '~/components/ButtonOptions.vue'

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
  emit('next', 'add_product')
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

<style scoped>


</style>
