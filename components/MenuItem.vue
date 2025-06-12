<template>
  <div class="d-flex flex-column align-center text-center" style="width: 80px;">
    <v-card
        @click="navigate"
        variant="tonal"
        class="d-flex flex-column align-center justify-center"
        style="width: 60px; height: 60px; border-radius: 10px; cursor: pointer; background-color: #f5f5f5;"
    >
      <v-img :src="icon" width="28" height="28" contain />
    </v-card>
    <span class="text-caption mt-1 font-weight-bold" style="font-size: 11px;color: #999999">{{ label }}</span>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'

const props = defineProps({
  icon: String,
  label: String,
  to: String,
})

const router = useRouter()

const navigate = () => {
  if (props.to) {
    router.push(props.to)
  }
}
</script>
