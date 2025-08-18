<template>
  <div class="col col-workers-list">
    <workers-table ref="workersTable" :activeWorkerID="workerID" />
  </div>
  <div class="col col-workers-details">
    <worker-details :workerData="workers.activeWorker" />
  </div>
  <footer class="app-footer">
    <notification-bar />
    <update-listener
      ref="updateListener"
      mainSubscription="allWorkers"
      extraSubscription="allWorkerTags"
      @workerUpdate="onSIOWorkerUpdate"
      @workerTagUpdate="onSIOWorkerTagsUpdate"
      @sioReconnected="onSIOReconnected"
      @sioDisconnected="onSIODisconnected" />
  </footer>
</template>

<style scoped>
.col-workers-list {
  grid-area: col-1;
}

.col-workers-details {
  grid-area: col-2;
}
</style>

<script>
import { useNotifs } from '@/stores/notifications';
import { useWorkers } from '@/stores/workers';

import NotificationBar from '@/components/footer/NotificationBar.vue';
import UpdateListener from '@/components/UpdateListener.vue';
import WorkerDetails from '@/components/workers/WorkerDetails.vue';
import WorkersTable from '@/components/workers/WorkersTable.vue';

export default {
  name: 'WorkersView',
  props: ['workerID'], // provided by Vue Router.
  components: {
    NotificationBar,
    UpdateListener,
    WorkerDetails,
    WorkersTable,
  },
  data: () => ({
    workers: useWorkers(),
    notifs: useNotifs(),
  }),
  mounted() {
    window.workersView = this;

    document.body.classList.add('is-two-columns');
  },
  unmounted() {
    document.body.classList.remove('is-two-columns');
  },
  methods: {
    // SocketIO connection event handlers:
    onSIOReconnected() {
      this.$refs.workersTable.onReconnected();
    },
    onSIODisconnected(reason) {},
    onSIOWorkerUpdate(workerUpdate) {
      this.notifs.addWorkerUpdate(workerUpdate);

      if (this.$refs.workersTable) {
        this.$refs.workersTable.processWorkerUpdate(workerUpdate);
      }
    },
    onSIOWorkerTagsUpdate(workerTagsUpdate) {
      if (!this.workerID) {
        this.workers.clearActiveWorker();
        return;
      }

      this.workers
        .refreshTags()
        .then(() =>
          this.api.fetchWorker(this.workerID).then((worker) => this.workers.setActiveWorker(worker))
        );
    },
  },
};
</script>
