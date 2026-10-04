<template>
  <v-bottom-navigation :elevation="8" grow>
    <v-btn value="recent">
      <v-icon>mdi-history</v-icon>

      <span>Recent</span>
    </v-btn>

    <v-btn value="favorites">
      <v-icon>mdi-heart</v-icon>

      <span>Favorites</span>
    </v-btn>

    <v-btn value="nearby">
      <v-icon>mdi-map-marker</v-icon>

      <span>Nearby</span>
    </v-btn>
  </v-bottom-navigation>
</template>

<script setup>
import { computed } from 'vue'
import { useAuthStore } from '~/stores/auth'
defineEmits(['toggle-drawer'])

const authStore = useAuthStore()

const userInitial = computed(() => {
  const email = authStore.user?.email || ''
  return email.charAt(0).toUpperCase()
})

const handleLogout = () => {
  authStore.logout()
}
</script>

<style scoped>
.logo-center {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  z-index: 1;
}
</style>
