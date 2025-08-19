<template>
  <div class="btn-bar jobs">
    <div class="btn-bar-popover" v-if="showDeleteJobPopup">
      <p v-if="shamanEnv">
        Delete {{ jobs.selectedJobs.length }} job(s), including Shaman checkout?
      </p>
      <p v-else>Delete {{ jobs.selectedJobs.length }} job(s)?</p>
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
    <button
      title="Select all jobs with updated timestamps equal to or less than a selected job's updated timestamp. Helpful for managing jobs updated before a certain timestamp."
      :disabled="!jobs.selectedJobs.length"
      v-on:click="onButtonMassSelect">
      Select Preceding Jobs
    </button>
  </div>
</template>

<script>
import { useJobs } from '@/stores/jobs';
import { useNotifs } from '@/stores/notifications';
import { getAPIClient, newBareAPIClient } from '@/api-client';
import { JobsApi, MetaApi } from '@/manager-api';

const cancelDescription = 'marked for cancellation';
const requeueDescription = 'marked for requeueing';
const pauseDescription = 'marked for pausing';
const deleteDescription = 'marked for deleting';
export default {
  name: 'JobActionsBar',
  props: ['activeJobID'],
  data: () => ({
    jobs: useJobs(),
    notifs: useNotifs(),
    jobsAPI: new JobsApi(getAPIClient()),
    metaAPI: new MetaApi(newBareAPIClient()),

    shamanEnv: null,
    showDeleteJobPopup: false,
  }),
  computed: {},
  watch: {
    activeJobID() {
      this._hideDeleteJobPopup();
    },
  },
  emits: ['mass-select'],
  methods: {
    onButtonMassSelect() {
      this.$emit('mass-select');
    },
    onButtonDelete() {
      this._startJobDeletionFlow();
    },
    onButtonDeleteConfirmed() {
      return this._handleJobActionPromise(this.jobs.deleteJobs(), deleteDescription);
    },
    onButtonCancel() {
      return this._handleJobActionPromise(this.jobs.cancelJobs(), cancelDescription);
    },
    onButtonRequeue() {
      return this._handleJobActionPromise(this.jobs.requeueJobs(), requeueDescription);
    },
    onButtonPause() {
      return this._handleJobActionPromise(this.jobs.pauseJobs(), pauseDescription);
    },

    _handleJobActionPromise(promise, description) {
      return promise
        .then((values) => {
          const { incompatibleJobs, compatibleJobs, totalJobCount } = values;

          // TODO: messages could be improved to specify the names of jobs that failed
          const failedMessage = `Could not apply ${description} status to ${incompatibleJobs.length} out of ${totalJobCount} job(s).`;
          const successMessage = `${compatibleJobs.length} job(s) successfully ${description}.`;

          this.notifs.add(
            `${compatibleJobs.length > 0 ? successMessage : ''}
             ${incompatibleJobs.length > 0 ? failedMessage : ''}`
          );
        })
        .catch((error) => {
          const errorMsg = JSON.stringify(error); // TODO: handle API errors better.
          this.notifs.add(`Error: ${errorMsg}`);
        })
        .finally(() => {
          this._hideDeleteJobPopup();
        });
    },

    _startJobDeletionFlow() {
      if (!this.jobs.selectedJobs.length) {
        this.notifs.add('No selected job(s), unable to delete anything');
        return;
      }

      this._showDeleteJobPopup();
    },

    /**
     * Shows the delete popup, and checks for Shaman to render the correct delete confirmation message.
     */
    _showDeleteJobPopup() {
      // Concurrently fetch:
      // 1) the first job's deletion info
      // 2) the environment configuration
      Promise.allSettled([
        this.jobsAPI
          .deleteJobWhatWouldItDo(this.jobs.selectedJobs[0].id)
          .catch((error) => console.error('Error fetching deleteJobWhatWouldItDo:', error)),
        this.metaAPI
          .getConfiguration()
          .catch((error) => console.error('Error getting configuration:', error)),
      ]).then((results) => {
        const [jobDeletionInfo, managerConfig] = results.map((result) => result.value);

        // If either have Shaman, render the message relevant to an enabled Shaman environment
        this.shamanEnv = jobDeletionInfo.shaman_checkout || managerConfig.shamanEnabled;
      });

      this.showDeleteJobPopup = true;
    },

    _hideDeleteJobPopup() {
      this.shamanEnv = null;
      this.showDeleteJobPopup = false;
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
