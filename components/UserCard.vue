<template>
  <v-card class="mb-3 pa-4 ma-2" style="border: 2px solid #343a40; border-radius: 10px;">
    <v-row class="align-center px-2">
      <v-col cols="3" class="d-flex justify-center pr-2">
        <v-avatar size="50" class="bg-white" style="border: 2px solid #ffb932">
          <v-icon size="32">mdi-account</v-icon>
        </v-avatar>
      </v-col>
      <v-col cols="9" class="pl-2">
        <h3 class="text-h7 font-weight-bold">{{ authStore.user?.username || 'Guest' }}</h3>
        <p class="text-subtitle-2 text-grey">
          {{ authStore.user?.email || 'No email provided' }}
        </p>
      </v-col>
    </v-row>
  </v-card>
</template>

<script setup>
import { useAuthStore } from '~/stores/auth'
const authStore = useAuthStore()
</script>
