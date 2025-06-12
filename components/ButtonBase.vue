<template>
  <div :class="$style.buttonBase">
    <v-btn
        :loading="loading"
        :disabled="disabled"
        :class="$style.buttonBaseButton"
        variant="flat"
        @click="handleClick"
    >
      <span :class="$style.buttonBaseLabel">{{ label }}</span>
    </v-btn>
  </div>
</template>

<script setup>
import { defineProps, defineEmits } from 'vue';

const props = defineProps({
  label: { type: String, default: '' },
  loading: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
});

const emit = defineEmits(['click']);

const handleClick = () => {
  if (!props.disabled) {
    emit('click');
  }
};
</script>

<style module>
.buttonBase {
  width: 100%;
  padding: 8px 0 0;
  display: flex;
  justify-content: center;
  align-items: center;
}

.buttonBaseButton {
  width: 100%;
  border-radius: 24px !important;
  background-color: #101010 !important;
  color: #fff !important;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px 16px;
  box-sizing: border-box;
  text-align: center;
  font-size: 14px !important;
  transition: background 0.3s ease !important;
  height: 40px !important;
  letter-spacing: 0 !important;
  -webkit-font-smoothing: subpixel-antialiased;
}

.buttonBaseLabel {
  line-height: 24px;
  text-transform: none;
}

</style>
