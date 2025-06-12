<template>
  <v-app>
    <UserCard />
    <MenuGrid />
  </v-app>
</template>

<script setup>
import { useAuthStore } from "~/stores/auth.js";

const authStore = useAuthStore()
const drawer = ref(false)
</script>
