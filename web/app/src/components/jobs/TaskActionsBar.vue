<template>
  <section class="btn-bar tasks">
    <button class="btn cancel" :disabled="!tasks.canCancel" v-on:click="onButtonCancel">
      Cancel Task
    </button>
    <button class="btn requeue" :disabled="!tasks.canRequeue" v-on:click="onButtonRequeue">
      Requeue
    </button>
  </section>
</template>

<script>
import { useTasks } from '@/stores/tasks';
import { useNotifs } from '@/stores/notifications';

export default {
  name: 'TaskActionsBar',
  data: () => ({
    tasks: useTasks(),
    notifs: useNotifs(),
  }),
  computed: {},
  methods: {
    onButtonCancel() {
      return this._handleTaskActionPromise(this.tasks.cancelTasks(), 'cancelled');
    },
    onButtonRequeue() {
      return this._handleTaskActionPromise(this.tasks.requeueTasks(), 'requeueing');
    },

    _handleTaskActionPromise(promise, description) {
      return promise
        .then((values) => {
          const { incompatibleTasks, compatibleTasks, totalTaskCount } = values;

          // TODO: messages could be improved to specify the names of tasks that failed
          const failedMessage = `Could not apply ${description} status to ${incompatibleTasks.length} out of ${totalTaskCount} task(s).`;
          const successMessage = `${compatibleTasks.length} task(s) successfully ${description}.`;

          this.notifs.add(
            `${compatibleTasks.length > 0 ? successMessage : ''}
            ${incompatibleTasks.length > 0 ? failedMessage : ''}`
          );
        })
        .catch((error) => {
          const errorMsg = JSON.stringify(error); // TODO: handle API errors better.
          this.notifs.add(`Error: ${errorMsg}`);
        });
    },
  },
};
</script>
