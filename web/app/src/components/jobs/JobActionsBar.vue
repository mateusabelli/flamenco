<template>
  <div class="btn-bar jobs">
    <div class="btn-bar-popover" v-if="deleteInfo != null">
      <p v-if="deleteInfo.shaman_checkout">Delete job, including Shaman checkout?</p>
      <p v-else>Delete job? The job files will be kept.</p>
      <div class="inner-btn-bar">
        <button class="btn cancel" v-on:click="_hideDeleteJobPopup">Cancel</button>
        <button class="btn delete dangerous" v-on:click="onButtonDeleteConfirmed">Delete</button>
      </div>
    </div>
    <button class="btn pause" :disabled="!jobs.canPause" v-on:click="onButtonPause">
      Pause Job
    </button>
    <button class="btn cancel" :disabled="!jobs.canCancel" v-on:click="onButtonCancel">
      Cancel Job
    </button>
    <button class="btn requeue" :disabled="!jobs.canRequeue" v-on:click="onButtonRequeue">
      Requeue
    </button>
    <button
      class="action delete dangerous"
      title="Mark this job for deletion, after asking for a confirmation."
      :disabled="!jobs.canDelete"
      v-on:click="onButtonDelete">
      Delete...
    </button>
  </div>
</template>

<script>
import { useJobs } from '@/stores/jobs';
import { useNotifs } from '@/stores/notifications';
import { getAPIClient } from '@/api-client';
import { JobsApi } from '@/manager-api';
import { JobDeletionInfo } from '@/manager-api';

export default {
  name: 'JobActionsBar',
  props: ['activeJobID'],
  data: () => ({
    jobs: useJobs(),
    notifs: useNotifs(),
    jobsAPI: new JobsApi(getAPIClient()),

    deleteInfo: null,
  }),
  computed: {},
  watch: {
    activeJobID() {
      this._hideDeleteJobPopup();
    },
  },
  methods: {
    onButtonDelete() {
      this._startJobDeletionFlow();
    },
    onButtonDeleteConfirmed() {
      return this.jobs
        .deleteJobs()
        .then(() => {
          this.notifs.add('job marked for deletion');
        })
        .catch((error) => {
          const errorMsg = JSON.stringify(error); // TODO: handle API errors better.
          this.notifs.add(`Error: ${errorMsg}`);
        })
        .finally(this._hideDeleteJobPopup);
    },
    onButtonCancel() {
      return this._handleJobActionPromise(this.jobs.cancelJobs(), 'marked for cancellation');
    },
    onButtonRequeue() {
      return this._handleJobActionPromise(this.jobs.requeueJobs(), 'requeueing');
    },
    onButtonPause() {
      return this._handleJobActionPromise(this.jobs.pauseJobs(), 'marked for pausing');
    },

    _handleJobActionPromise(promise, description) {
      return promise.then(() => {
        // There used to be a call to `this.notifs.add(message)` here, but now
        // that job status changes are logged in the notifications anyway,
        // it's no longer necessary.
        // This function is still kept, in case we want to bring back the
        // notifications when multiple jobs can be selected. Then a summary
        // ("N jobs requeued") could be logged here.btn-bar-popover
      });
    },

    _startJobDeletionFlow() {
      if (!this.activeJobID) {
        this.notifs.add('No active job, unable to delete anything');
        return;
      }

      this.jobsAPI
        .deleteJobWhatWouldItDo(this.activeJobID)
        .then(this._showDeleteJobPopup)
        .catch((error) => {
          const errorMsg = JSON.stringify(error); // TODO: handle API errors better.
          this.notifs.add(`Error: ${errorMsg}`);
        });
    },

    /**
     * @param { JobDeletionInfo } deleteInfo
     */
    _showDeleteJobPopup(deleteInfo) {
      this.deleteInfo = deleteInfo;
    },

    _hideDeleteJobPopup() {
      this.deleteInfo = null;
    },
  },
};
</script>

<style scoped>
.btn-bar-popover {
  align-items: center;
  background-color: var(--color-background-popover);
  border-radius: var(--border-radius);
  border: var(--border-color) var(--border-width);
  color: var(--color-text);
  display: flex;
  height: 3.5em;
  left: 0;
  margin: 0;
  padding: 1rem 1rem;
  position: absolute;
  right: 0;
  top: 0;
  z-index: 1000;
}

.btn-bar-popover p {
  flex-grow: 1;
}

.btn-bar-popover .inner-btn-bar {
  flex-grow: 0;
}
</style>
