<template>
  <v-app>
    <Topbar @toggle-drawer="drawer = !drawer" />
    <v-main style="max-width: 600px; margin: auto;">
      <slot />
    </v-main>
  </v-app>
</template>

<script setup>
import Topbar from '~/components/Topbar.vue'
import UserCard from '~/components/UserCard.vue'
import MenuGrid from '~/components/MenuGrid.vue'
const drawer = ref(false)
</script>
