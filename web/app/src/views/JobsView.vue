<template>
  <div class="col col-1">
    <jobs-table ref="jobsTable" :activeJobID="jobID" />
  </div>
  <div class="col col-2 job-details-column" id="col-job-details">
    <get-the-addon v-if="jobs.isJobless" />
    <template v-else>
      <job-details
        ref="jobDetails"
        :jobData="jobs.activeJob"
        @reshuffled="_recalcTasksTableHeight" />
      <tasks-table v-if="hasJobData" ref="tasksTable" :jobID="jobID" :taskID="taskID" />
    </template>
  </div>
  <div class="col col-3">
    <task-details
      v-if="hasJobData"
      :taskData="tasks.activeTask"
      @showTaskLogTail="showTaskLogTail" />
  </div>

  <footer class="app-footer" v-if="!showFooterPopup" @click="showFooterPopup = true">
    <notification-bar />
    <div class="app-footer-expand">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round">
        <line x1="12" y1="19" x2="12" y2="5"></line>
        <polyline points="5 12 12 5 19 12"></polyline>
      </svg>
    </div>
  </footer>
  <footer-popup v-if="showFooterPopup" ref="footerPopup" @clickClose="showFooterPopup = false" />

  <update-listener
    ref="updateListener"
    mainSubscription="allJobs"
    :subscribedJobID="jobID"
    :subscribedTaskID="taskID"
    @jobUpdate="onSioJobUpdate"
    @taskUpdate="onSioTaskUpdate"
    @taskLogUpdate="onSioTaskLogUpdate"
    @lastRenderedUpdate="onSioLastRenderedUpdate"
    @message="onChatMessage"
    @sioReconnected="onSIOReconnected"
    @sioDisconnected="onSIODisconnected" />
</template>

<script>
import { useJobs } from '@/stores/jobs';
import { useTasks } from '@/stores/tasks';
import { useNotifs } from '@/stores/notifications';
import { useTaskLog } from '@/stores/tasklog';

import FooterPopup from '@/components/footer/FooterPopup.vue';
import GetTheAddon from '@/components/GetTheAddon.vue';
import JobDetails from '@/components/jobs/JobDetails.vue';
import JobsTable from '@/components/jobs/JobsTable.vue';
import NotificationBar from '@/components/footer/NotificationBar.vue';
import TaskDetails from '@/components/jobs/TaskDetails.vue';
import TasksTable from '@/components/jobs/TasksTable.vue';
import UpdateListener from '@/components/UpdateListener.vue';

export default {
  name: 'JobsView',
  props: ['jobID', 'taskID'], // provided by Vue Router.
  components: {
    FooterPopup,
    GetTheAddon,
    JobDetails,
    JobsTable,
    NotificationBar,
    TaskDetails,
    TasksTable,
    UpdateListener,
  },
  data: () => ({
    messages: [],

    jobs: useJobs(),
    tasks: useTasks(),
    notifs: useNotifs(),
    taskLog: useTaskLog(),
    showFooterPopup: !!localStorage.getItem('footer-popover-visible'),
  }),
  computed: {
    hasJobData() {
      return !objectEmpty(this.jobs.activeJob);
    },
  },
  mounted() {
    window.jobsView = this;
    window.footerPopup = this.$refs.footerPopup;

    // Useful for debugging:
    // this.jobs.$subscribe((mutation, state) => {
    //   console.log("Pinia mutation:", mutation)
    //   console.log("Pinia state   :", state)
    // })

    window.addEventListener('resize', this._recalcTasksTableHeight);
  },
  unmounted() {
    window.removeEventListener('resize', this._recalcTasksTableHeight);
  },
  watch: {
    showFooterPopup(shown) {
      if (shown) localStorage.setItem('footer-popover-visible', 'true');
      else localStorage.removeItem('footer-popover-visible');
      this._recalcTasksTableHeight();
    },
  },
  methods: {
    showTaskLogTail() {
      this.showFooterPopup = true;
      this.$nextTick(() => {
        this.$refs.footerPopup.showTaskLogTail();
      });
    },

    // SocketIO data event handlers:
    onSioJobUpdate(jobUpdate) {
      this.notifs.addJobUpdate(jobUpdate);
      this.jobs.setIsJobless(false);

      if (this.$refs.jobsTable) {
        this.$refs.jobsTable.processJobUpdate(jobUpdate);
      }
      if (this.jobID != jobUpdate.id || jobUpdate.was_deleted) {
        return;
      }

      if (jobUpdate.refresh_tasks) {
        if (this.$refs.tasksTable) this.$refs.tasksTable.fetchTasks();
      }
    },

    /**
     * Event handler for SocketIO task updates.
     * @param {API.EventTaskUpdate} taskUpdate
     */
    onSioTaskUpdate(taskUpdate) {
      if (this.$refs.tasksTable) this.$refs.tasksTable.processTaskUpdate(taskUpdate);

      this.notifs.addTaskUpdate(taskUpdate);
    },

    /**
     * Event handler for SocketIO task log updates.
     * @param {API.EventTaskLogUpdate} taskLogUpdate
     */
    onSioTaskLogUpdate(taskLogUpdate) {
      this.taskLog.addTaskLogUpdate(taskLogUpdate);
    },

    /**
     * Event handler for SocketIO "last-rendered" updates.
     * @param {API.EventLastRenderedUpdate} lastRenderedUpdate
     */
    onSioLastRenderedUpdate(lastRenderedUpdate) {
      this.$refs.jobDetails.refreshLastRenderedImage(lastRenderedUpdate);
    },

    onChatMessage(message) {
      console.log('chat message received:', message);
      this.messages.push(`${message.text}`);
    },

    // SocketIO connection event handlers:
    onSIOReconnected() {
      this.$refs.jobsTable.onReconnected();
      if (this.$refs.tasksTable) this.$refs.tasksTable.onReconnected();
    },
    onSIODisconnected(reason) {},

    _recalcTasksTableHeight() {
      if (!this.$refs.tasksTable) return;
      // Any recalculation should be done after the DOM has updated.
      this.$nextTick(this.$refs.tasksTable.recalcTableHeight);
    },
  },
};
</script>

<style scoped>
.isFetching {
  opacity: 50%;
}
</style>
