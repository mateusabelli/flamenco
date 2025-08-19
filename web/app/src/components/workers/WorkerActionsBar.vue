<template>
  <div class="btn-bar workers">
    <select v-model="selectedAction">
      <option value="" selected>
        <template v-if="!workers.selectedWorkers.length">Select a Worker</template>
        <template v-else>Choose an action...</template>
      </option>
      <template v-for="(action, key) in WORKER_ACTIONS">
        <option :key="action.label" :value="key" v-if="action.condition()">
          {{ action.label }}
        </option>
      </template>
    </select>
    <button :disabled="!canPerformAction" class="btn" @click.prevent="performWorkerAction">
      Apply
    </button>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useWorkers } from '@/stores/workers';
import { useNotifs } from '@/stores/notifications';
import { WorkerMgtApi, WorkerStatusChangeRequest } from '@/manager-api';
import { getAPIClient } from '@/api-client';

/* Freeze to prevent Vue.js from creating getters & setters all over this object.
 * We don't need it to be tracked, as it won't be changed anyway. */
const WORKER_ACTIONS = Object.freeze({
  offline_lazy: {
    label: 'Shut Down (after task is finished)',
    icon: '✝',
    title:
      'Shut down the worker after the current task finishes. The worker may automatically restart.',
    target_status: 'offline',
    lazy: true,
    condition: () => true,
  },
  offline_immediate: {
    label: 'Shut Down (immediately)',
    icon: '✝!',
    title: 'Immediately shut down the worker. It may automatically restart.',
    target_status: 'offline',
    lazy: false,
    condition: () => true,
  },
  restart_lazy: {
    label: 'Restart (after task is finished)',
    icon: '✝',
    title: 'Restart the worker after the current task finishes.',
    target_status: 'restart',
    lazy: true,
    condition: () => workers.canRestart(),
  },
  restart_immediate: {
    label: 'Restart (immediately)',
    icon: '✝!',
    title: 'Immediately restart the worker.',
    target_status: 'restart',
    lazy: false,
    condition: () => workers.canRestart(),
  },
  asleep_lazy: {
    label: 'Send to Sleep (after task is finished)',
    icon: '😴',
    title: 'Let the worker sleep after finishing this task.',
    target_status: 'asleep',
    lazy: true,
    condition: () => true,
  },
  asleep_immediate: {
    label: 'Send to Sleep (immediately)',
    icon: '😴!',
    title: 'Let the worker sleep immediately.',
    target_status: 'asleep',
    lazy: false,
    condition: () => true,
  },
  wakeup: {
    label: 'Wake Up',
    icon: '😃',
    title: 'Wake the worker up. A sleeping worker can take a minute to respond.',
    target_status: 'awake',
    lazy: false,
    condition: () => true,
  },
});

const selectedAction = ref('');
const workers = useWorkers();
const canPerformAction = computed(() => workers.selectedWorkers.length && !!selectedAction.value);
const notifs = useNotifs();

function performWorkerAction() {
  if (!workers.selectedWorkers.length) {
    notifs.add('Select at least one Worker before applying an action.');
    return;
  }

  const api = new WorkerMgtApi(getAPIClient()); // Init the api
  const action = WORKER_ACTIONS[selectedAction.value];
  const statuschange = new WorkerStatusChangeRequest(action.target_status, action.lazy);

  const promises = workers.selectedWorkers.map((worker) =>
    api
      .requestWorkerStatusChange(worker.id, statuschange)
      .then(() =>
        notifs.add(`Worker ${worker.name} status change to ${action.target_status} confirmed.`)
      )
      .catch((error) =>
        notifs.add(`Error requesting worker ${worker.name} status change: ${error.body.message}`)
      )
  );

  Promise.allSettled(promises);
}
</script>
