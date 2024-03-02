<template>
  <span id="farmstatus" v-if="status"
    >Farm status:
    <span :class="'farm-status-' + status" :title="explanation">{{ status }}</span></span
  >
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps(['status']);

const expanations = {
  active: 'Actively working on a job.',
  idle: 'Workers are awake and ready for work.',
  waiting: 'Work has been queued, but all workers are asleep.',
  asleep: 'All workers are asleep, and no work has been queued.',
  inoperative: 'Cannot work: there are no workers, or all are offline.',
  starting: 'Farm is starting up.',
};

const explanation = computed(() => {
  return expanations[props.status] || '';
});
</script>

<style>
span#farmstatus {
  cursor: default;
}
span#farmstatus span {
  cursor: help;
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
