import { defineStore } from 'pinia';

import * as API from '@/manager-api';
import { getAPIClient } from '@/api-client';

const jobsAPI = new API.JobsApi(getAPIClient());

const JOB_ACTIONS = Object.freeze({
  CANCEL: {
    status: 'cancel-requested',
    prerequisiteStatuses: ['active', 'paused', 'failed', 'queued', 'pause-requested'],
  },
  PAUSE: {
    status: 'pause-requested',
    prerequisiteStatuses: ['active', 'canceled', 'queued'],
  },
  REQUEUE: {
    status: 'requeueing',
    prerequisiteStatuses: ['canceled', 'completed', 'failed', 'paused'],
  },
  DELETE: {
    status: 'delete-requested',
    prerequisiteStatuses: ['canceled', 'completed', 'failed', 'paused', 'queued'],
  },
});

// 'use' prefix is idiomatic for Pinia stores.
// See https://pinia.vuejs.org/core-concepts/
export const useJobs = defineStore('jobs', {
  state: () => ({
    /** @type {API.Job} */
    activeJob: null,
    /**
     * ID of the active job. Easier to query than `activeJob ? activeJob.id : ""`.
     * @type {string}
     */
    activeJobID: '',
    /**
     * Set to true when it is known that there are no jobs at all in the system.
     * This is written by the JobsTable.vue component.
     * @type {bool}
     */
    isJobless: false,
    /** @type {API.Job[]} */
    selectedJobs: [],
  }),
  getters: {
    canDelete() {
      return this._anyJobWithStatus(JOB_ACTIONS.DELETE.prerequisiteStatuses);
    },
    canCancel() {
      return this._anyJobWithStatus(JOB_ACTIONS.CANCEL.prerequisiteStatuses);
    },
    canRequeue() {
      return this._anyJobWithStatus(JOB_ACTIONS.REQUEUE.prerequisiteStatuses);
    },
    canPause() {
      return this._anyJobWithStatus(JOB_ACTIONS.PAUSE.prerequisiteStatuses);
    },
  },
  actions: {
    setIsJobless(isJobless) {
      this.$patch({ isJobless: isJobless });
    },
    setActiveJobID(jobID) {
      this.$patch({
        activeJob: { id: jobID, settings: {}, metadata: {} },
        activeJobID: jobID,
      });
    },
    setActiveJob(job) {
      // The "function" form of $patch is necessary here, as otherwise it'll
      // merge `job` into `state.activeJob`. As a result, it won't touch missing
      // keys, which means that metadata fields that existed on the previous job
      // but not on the new one will still linger around. By passing a function
      // to `$patch` this is resolved.
      this.$patch((state) => {
        state.activeJob = job;
        state.activeJobID = job.id;
        state.hasChanged = true;
      });
    },
    clearActiveJob() {
      this.$patch({
        activeJob: null,
        activeJobID: '',
      });
    },
    setSelectedJobs(jobs) {
      this.$patch({
        selectedJobs: jobs,
      });
    },
    clearSelectedJobs() {
      this.$patch({
        selectedJobs: [],
      });
    },

    /**
     * Actions on the selected jobs.
     *
     * All the action functions return a promise that resolves when the action has been performed.
     */
    cancelJobs() {
      return this._setJobStatus(JOB_ACTIONS.CANCEL.status, JOB_ACTIONS.CANCEL.prerequisiteStatuses);
    },
    pauseJobs() {
      return this._setJobStatus(JOB_ACTIONS.PAUSE.status, JOB_ACTIONS.PAUSE.prerequisiteStatuses);
    },
    requeueJobs() {
      return this._setJobStatus(
        JOB_ACTIONS.REQUEUE.status,
        JOB_ACTIONS.REQUEUE.prerequisiteStatuses
      );
    },
    deleteJobs() {
      return this._setJobStatus(JOB_ACTIONS.DELETE.status, JOB_ACTIONS.DELETE.prerequisiteStatuses);
    },

    // Internal methods.

    /**
     *
     * @param {string[]} statuses
     * @returns bool indicating whether there is a selected job with any of the given statuses.
     */
    _anyJobWithStatus(job_statuses) {
      if (this.selectedJobs.length) {
        return this.selectedJobs.some((job) => job_statuses.includes(job.status));
      }
      return false;
    },

    /**
     * Transition the selected job(s) to the new status.
     * @param {string} newStatus
     * @param {string[]} job_statuses The job statuses compatible with the transition to new status
     * @returns a Promise for the API request(s).
     */
    _setJobStatus(newStatus, job_statuses) {
      const totalJobCount = this.selectedJobs.length;

      if (!totalJobCount) {
        console.warn(`_setJobStatus(${newStatus}) impossible, no selected job(s).`);
        return;
      }

      const { compatibleJobs, incompatibleJobs } = this.selectedJobs.reduce(
        (result, job) => {
          if (job_statuses.includes(job.status)) {
            result.compatibleJobs.push(job);
          } else {
            result.incompatibleJobs.push(job);
          }
          return result;
        },
        { compatibleJobs: [], incompatibleJobs: [] }
      );

      let setJobStatusPromises = [];

      if (newStatus === JOB_ACTIONS.DELETE.status) {
        setJobStatusPromises = compatibleJobs.map((job) => jobsAPI.deleteJob(job.id));
      } else {
        const statuschange = new API.JobStatusChange(newStatus, 'requested from web interface');

        setJobStatusPromises = compatibleJobs.map((job) =>
          jobsAPI.setJobStatus(job.id, statuschange)
        );
      }

      return Promise.allSettled(setJobStatusPromises).then((results) => ({
        compatibleJobs: results,
        incompatibleJobs,
        totalJobCount,
      }));
    },
  },
});
