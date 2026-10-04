<template>
  <v-btn
      stacked
      :color="btnColor"
      :ripple="false"
      @click="onClick"
      style="width: 100%; height: 100px; font-size: 15px;"
  >
    <template #prepend>
      <v-icon :size="iconSize" :color="iconColor">{{ icon }}</v-icon>
    </template>
    <span class="text-body-3" style="font-size: 15px">{{ label }}</span>
  </v-btn>
</template>

<script setup>
defineProps({
  icon: {
    type: String,
    default: 'mdi-basket-remove',
  },
  iconSize: {
    type: [Number, String],
    default: 36,
  },
  iconColor: {
    type: String,
    default: '#FF2626',
  },
  btnColor: {
    type: String,
    default: '#101010',
  },
  label: {
    type: String,
    default: '',
  },
  onClick: {
    type: Function,
    required: true,
  },
})
</script>
