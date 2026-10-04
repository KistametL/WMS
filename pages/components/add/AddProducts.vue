<template>
  <v-app>
    <h2 class="d-flex align-center">
      <v-icon size="32">mdi-map-marker</v-icon>
      Goods Receipt
    </h2>
    <inputCode
        placeholder="Scan DSN"
        sourceComponent="IndexPage"
        :loading="loading"
        @update:searchDSN="suggestionLocation"
        @update:searchSKU="handleSearchSKU"
        @update:searchLocation="handleSearchLocation"
        @update:searchBasket="handleSearchBasket"
    />
    <div style="padding: 0 12px">
      <v-divider color="black"></v-divider>
    </div>
  </v-app>
</template>