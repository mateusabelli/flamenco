import { defineStore } from 'pinia';
import { FarmStatusReport } from '@/manager-api';

/**
 * Keep track of the farm status. This is updated from UpdateListener.vue.
 */
export const useFarmStatus = defineStore('farmStatus', {
  state: () => ({
    lastStatusReport: new FarmStatusReport(),
  }),

  actions: {
    status() {
      return this.lastStatusReport.status;
    },
  },
});
