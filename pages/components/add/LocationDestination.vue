<template>
  <v-app>
    <h2 class="d-flex align-center">
      <v-icon size="32">mdi-map-marker</v-icon>
      Location Destination
    </h2>
    <inputCode
        placeholder="Scan Location"
        sourceComponent="IndexPage"
        :loading="loading"
        @update:searchDSN="handleSearchDSN"
        @update:searchSKU="handleSearchSKU"
        @update:searchLocation="locationDestination"
        @update:searchBasket="handleSearchBasket"
    />
    <div style="padding: 0 12px">
      <v-divider color="black"></v-divider>
    </div>

    <v-card class="ma-2 pa-2" rounded="lg" variant="tonal" v-for="(item, index) in addStore.suggestionLocation">
      <v-card-title class="text-subtitle-1 font-weight-bold d-flex align-center">
        <v-icon class="me-2" size="20">mdi-map-marker</v-icon>
        {{ item.location[0]?.location || 'No location' }}
      </v-card-title>

      <v-card-text>
        <v-row dense class="mb-1">
          <v-col cols="6" class="text-body-2 text-grey">SKU:</v-col>
          <v-col cols="6" class="text-body-2 font-weight-medium text-right">
            {{ item.sku }}
          </v-col>
        </v-row>

        <v-row dense>
          <v-col cols="6" class="text-body-2 text-grey">Quantity:</v-col>
          <v-col cols="6" class="text-body-2 font-weight-medium text-right">
            {{ item.location[0]?.instock }} pieces
          </v-col>
        </v-row>

        <v-row dense class="mb-1">
          <v-col cols="6" class="text-body-2 text-grey">Name:</v-col>
          <v-col cols="12" class="text-body-2 font-weight-medium">
            {{ item.name }}
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>

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

const locationDestination = async (dsn) => {
  console.log('DSN:', dsn)
  loading.value = true
  goToNextStep()
  loading.value = false
}

const goToNextStep = () => {
  emit('next', 'options')
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

