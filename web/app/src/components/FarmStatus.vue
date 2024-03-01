<template>
  <span id="farmstatus" v-if="status"
    >Farm status: <span :class="'farm-status-' + status">{{ status }}</span></span
  >
</template>

<script setup>
const props = defineProps(['status']);
</script>

<style>
span#farmstatus {
  margin: 0 auto;
}
.farm-status-starting {
  color: var(--color-farm-status-starting);
}
.farm-status-active {
  color: var(--color-farm-status-active);
}
.farm-status-idle {
  color: var(--color-farm-status-idle);
}
.farm-status-waiting {
  color: var(--color-farm-status-waiting);
}
.farm-status-asleep {
  color: var(--color-farm-status-asleep);
}
.farm-status-inoperative,
.farm-status-unknown {
  color: var(--color-farm-status-inoperative);
  font-weight: bold;
}
</style>
