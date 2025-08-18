import { defineStore } from 'pinia';

import { WorkerMgtApi } from '@/manager-api';
import { getAPIClient } from '@/api-client';

// 'use' prefix is idiomatic for Pinia stores.
// See https://pinia.vuejs.org/core-concepts/
export const useWorkers = defineStore('workers', {
  state: () => ({
    /** @type {API.Worker} */
    activeWorker: null,
    /**
     * ID of the active worker. Easier to query than `activeWorker ? activeWorker.id : ""`.
     * @type {string}
     */
    activeWorkerID: '',

    /** @type {API.WorkerTag[]} */
    tags: [],

    /* Mapping from tag UUID to API.WorkerTag. */
    tagsByID: {},

    /** @type {API.Worker[]} */
    selectedWorkers: [],
  }),
  actions: {
    setActiveWorkerID(workerID) {
      this.$patch({
        activeWorker: { id: workerID, settings: {}, metadata: {} },
        activeWorkerID: workerID,
      });
    },
    setActiveWorker(worker) {
      // The "function" form of $patch is necessary here, as otherwise it'll
      // merge `worker` into `state.activeWorker`. As a result, it won't touch missing
      // keys, which means that metadata fields that existed on the previous worker
      // but not on the new one will still linger around. By passing a function
      // to `$patch` this is resolved.
      this.$patch((state) => {
        state.activeWorker = worker;
        state.activeWorkerID = worker.id;
        state.hasChanged = true;
      });
    },
    clearActiveWorker() {
      this.$patch({
        activeWorker: null,
        activeWorkerID: '',
      });
    },
    setSelectedWorkers(workers) {
      this.$patch({
        selectedWorkers: workers,
      });
    },
    clearSelectedWorkers() {
      this.$patch({
        selectedWorkers: [],
      });
    },
    /**
     * Fetch the available worker tags from the Manager.
     *
     * @returns a promise.
     */
    refreshTags() {
      const api = new WorkerMgtApi(getAPIClient());
      return api.fetchWorkerTags().then((resp) => {
        this.tags = resp.tags;

        let tagsByID = {};
        for (let tag of this.tags) {
          tagsByID[tag.id] = tag;
        }
        this.tagsByID = tagsByID;
      });
    },

    /**
     * @returns {bool} whether atleast one selected worker understands how to get restarted.
     */
    canRestart() {
      if (this.selectedWorkers.length) {
        return this.selectedWorkers.some((worker) => worker.can_restart);
      }
      return false;
    },
  },
});
