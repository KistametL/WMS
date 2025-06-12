<!-- components/MenuGrid.vue -->
<template>
  <v-container>
    <v-row justify="center" align="center">
      <v-col
          v-for="(menu, index) in menus"
          :key="index"
          cols="3"
          class="d-flex justify-center"
      >
        <MenuItem
            :icon="menu.icon"
            :label="menu.label"
            :to="menu.to"
        />
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import MenuItem from './MenuItem.vue'
import addIcon from '@/assets/images/add.png'
import moveIcon from '@/assets/images/move.png'
import viewIcon from '@/assets/images/view.png'
import removeIcon from '@/assets/images/remove.png'
import countLocationIcon from '@/assets/images/count-location.png'
import transferSendIcon from '@/assets/images/transfer-send.png'
import clearBasketIcon from '@/assets/images/clear-basket.png'
import historyIcon from '@/assets/images/history.png'
import countSkuIcon from '@/assets/images/count-sku.png'
import transferReceivedIcon from '@/assets/images/transfer-received.png'

const menus = [
  { icon: addIcon, label: 'Add', to: '/add' },
  { icon: moveIcon, label: 'Move', action: () => console.log('Move') },
  { icon: removeIcon, label: 'Remove', action: () => console.log('Remove') },
  { icon: viewIcon, label: 'View', action: () => console.log('View') },
  { icon: countLocationIcon, label: 'Count Location', action: () => console.log('Count Location') },
  { icon: transferSendIcon, label: 'Transfer Send', action: () => console.log('Transfer Send') },
  { icon: clearBasketIcon, label: 'Clear Basket', action: () => console.log('Clear Basket') },
  { icon: historyIcon, label: 'History', action: () => console.log('History') },
  { icon: countSkuIcon, label: 'Count SKU', action: () => console.log('Count SKU') },
  { icon: transferReceivedIcon, label: 'Transfer Received', action: () => console.log('Transfer Received') }
]
</script>
