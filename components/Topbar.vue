<template>
  <v-app-bar app color="black" dark flat height="64" class="d-flex align-center">

    <div class="logo-center">
      <img src="~/assets/images/425logo.jpeg" alt="Logo" style="height: 70px;cursor: pointer" @click="$router.push('/')" />
    </div>

    <v-spacer />

    <v-menu offset-y>
      <template #activator="{ props }">
        <v-btn icon v-bind="props">
          <v-avatar size="32" color="white" class="text-uppercase font-weight-bold">
            {{ userInitial }}
          </v-avatar>
        </v-btn>
      </template>
      <v-list>
        <v-list-item>
          <v-list-item-title>Profile</v-list-item-title>
        </v-list-item>
        <v-list-item @click="handleLogout">
          <v-list-item-title>Logout</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-menu>

  </v-app-bar>
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
