import { defineStore } from 'pinia';

import * as API from '@/manager-api';
import { getAPIClient } from '@/api-client';
import { useJobs } from '@/stores/jobs';

const jobsAPI = new API.JobsApi(getAPIClient());

const taskStatusCanCancel = ['active', 'queued', 'soft-failed'];
const taskStatusCanRequeue = ['canceled', 'completed', 'failed'];
// 'use' prefix is idiomatic for Pinia stores.
// See https://pinia.vuejs.org/core-concepts/
export const useTasks = defineStore('tasks', {
  state: () => ({
    /** @type {API.Task} */
    activeTask: null,
    /**
     * ID of the active task. Easier to query than `activeTask ? activeTask.id : ""`.
     * @type {string}
     */
    activeTaskID: '',
    selectedTasks: [],
  }),
  getters: {
    canCancel() {
      const jobs = useJobs();
      const activeJob = jobs.activeJob;

      if (!activeJob) {
        console.warn('no active job, unable to determine whether the task(s) is cancellable');
        return false;
      }

      if (activeJob.status == 'pause-requested') {
        // Cancelling task(s) should not be possible while the job is being paused.
        // In the future this might be supported, see issue #104315.
        return false;
      }
      // Allow cancellation for specified task statuses.
      return this._anyTaskWithStatus(taskStatusCanCancel);
    },
    canRequeue() {
      return this._anyTaskWithStatus(taskStatusCanRequeue);
    },
  },
  actions: {
    setSelectedTasks(tasks) {
      this.$patch({
        selectedTasks: tasks,
      });
    },
    clearSelectedTasks() {
      this.$patch({
        selectedTasks: [],
      });
    },
    setActiveTaskID(taskID) {
      this.$patch({
        activeTask: { id: taskID },
        activeTaskID: taskID,
      });
    },
    setActiveTask(task) {
      this.$patch({
        activeTask: task,
        activeTaskID: task.id,
      });
    },
    clearActiveTask() {
      this.$patch({
        activeTask: null,
        activeTaskID: '',
      });
    },

    /**
     * Actions on the selected tasks.
     *
     * All the action functions return a promise that resolves when the action has been performed.
     */
    cancelTasks() {
      return this._setTaskStatus('canceled', taskStatusCanCancel);
    },
    requeueTasks() {
      return this._setTaskStatus('queued', taskStatusCanRequeue);
    },

    // Internal methods.

    /**
     *
     * @param {string[]} task_statuses
     * @returns bool indicating whether there is a selected task with any of the given statuses.
     */
    _anyTaskWithStatus(task_statuses) {
      if (this.selectedTasks.length) {
        return this.selectedTasks.some((task) => task_statuses.includes(task.status));
      }
      return false;
    },

    /**
     * Transition the selected task(s) to the new status.
     * @param {string} newStatus
     * @param {string[]} task_statuses The task statuses compatible with the transition to new status
     * @returns a Promise for the API request(s).
     */
    _setTaskStatus(newStatus, task_statuses) {
      const totalTaskCount = this.selectedTasks.length;

      if (!totalTaskCount) {
        console.warn(`_setTaskStatus(${newStatus}) impossible, no selected tasks`);
        return;
      }

      const { compatibleTasks, incompatibleTasks } = this.selectedTasks.reduce(
        (result, task) => {
          if (task_statuses.includes(task.status)) {
            result.compatibleTasks.push(task);
          } else {
            result.incompatibleTasks.push(task);
          }
          return result;
        },
        { compatibleTasks: [], incompatibleTasks: [] }
      );

      const statuschange = new API.TaskStatusChange(newStatus, 'requested from web interface');
      const setTaskStatusPromises = compatibleTasks.map((task) =>
        jobsAPI.setTaskStatus(task.id, statuschange)
      );

      return Promise.allSettled(setTaskStatusPromises).then((results) => ({
        compatibleTasks: results,
        incompatibleTasks,
        totalTaskCount,
      }));
    },
  },
});
