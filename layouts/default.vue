<template>
  <v-app style="max-width: 600px; margin: auto;">
    <Topbar v-if="!hideTopbar" @toggle-drawer="drawer = !drawer" />
    <v-main class="mt-2" :style="mainStyle">
      <slot />
    </v-main>
    <BottomNavigation v-if="!hideTopbar" />
  </v-app>
</template>

<script setup>
import {useRoute} from 'vue-router'
import Topbar from '~/components/Topbar.vue'
import BottomNavigation from '~/components/BottomNavigation.vue'

const drawer = ref(false)
const route = useRoute()

const hideTopbar = computed(() => route.path === '/account/login')
const mainStyle = computed(() =>
    route.path === '/account/login' ? '' : 'padding: 72px 4px;'
)
</script>

<style>
.v-application__wrap {
  min-height: 60dvh !important;
}
</style>
