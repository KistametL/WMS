<template>
  <v-container style="padding: 0 8px">
    <component :is="stepComponent" @next="currentStep = $event" />
  </v-container>
</template>

<script setup>
import SuggestionLocation from "~/pages/components/add/SuggestionLocation.vue";
import LocationDestination from "~/pages/components/add/LocationDestination.vue";
import Option from "~/pages/components/add/Options.vue";
import AddProducts from "~/pages/components/add/AddProducts.vue";

const currentStep = ref('suggestion')

const stepComponent = computed(() => {
  switch (currentStep.value) {
    case 'suggestion': return SuggestionLocation
    case 'location': return LocationDestination
    case 'options': return Option
    case 'add_product': return AddProducts
  }
})
</script>
