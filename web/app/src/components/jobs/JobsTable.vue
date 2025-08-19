<template>
  <h2 class="column-title">Jobs</h2>
  <div class="btn-bar-group">
    <job-actions-bar :activeJobID="jobs.activeJobID" @mass-select="handleMassSelect" />
    <status-filter-bar
      :availableStatuses="availableStatuses"
      :activeStatuses="shownStatuses"
      @click="toggleStatusFilter" />
  </div>
  <div>
    <div class="job-list with-clickable-row" id="flamenco_job_list"></div>
  </div>
</template>

<script>
import { TabulatorFull as Tabulator } from 'tabulator-tables';
import * as datetime from '@/datetime';
import * as API from '@/manager-api';
import { indicator } from '@/statusindicator';
import { getAPIClient } from '@/api-client';
import { useJobs } from '@/stores/jobs';

import JobActionsBar from '@/components/jobs/JobActionsBar.vue';
import StatusFilterBar from '@/components/StatusFilterBar.vue';

export default {
  name: 'JobsTable',
  props: ['activeJobID'],
  components: {
    JobActionsBar,
    StatusFilterBar,
  },
  data: () => {
    return {
      shownStatuses: [],
      availableStatuses: [], // Will be filled after data is loaded from the backend.
      jobs: useJobs(),
      lastSelectedJobPosition: null,
    };
  },
  mounted() {
    // Allow testing from the JS console:
    // jobsTableVue.processJobUpdate({id: "ad0a5a00-5cb8-4e31-860a-8a405e75910e", status: "heyy", updated: DateTime.local().toISO(), previous_status: "uuuuh", name: "Updated manually"});
    // jobsTableVue.processJobUpdate({id: "ad0a5a00-5cb8-4e31-860a-8a405e75910e", status: "heyy", updated: DateTime.local().toISO()});
    window.jobsTableVue = this;

    const vueComponent = this;
    const options = {
      // See pkg/api/flamenco-openapi.yaml, schemas Job and EventJobUpdate.
      columns: [
        // Useful for debugging when there are many similar jobs:
        // { title: "ID", field: "id", headerSort: false, formatter: (cell) => cell.getData().id.substr(0, 8), },
        {
          title: 'Status',
          field: 'status',
          sorter: 'string',
          formatter: (cell) => {
            const status = cell.getData().status;
            const dot = indicator(status);
            return `${dot} ${status}`;
          },
        },
        { title: 'Name', field: 'name', sorter: 'string' },
        {
          title: 'Updated',
          field: 'updated',
          sorter: 'alphanum',
          sorterParams: { alignEmptyValues: 'top' },
          formatter(cell) {
            const cellValue = cell.getData().updated;
            // TODO: if any "{amount} {units} ago" shown, the table should be
            // refreshed every few {units}, so that it doesn't show any stale "4
            // seconds ago" for days.
            return datetime.relativeTime(cellValue);
          },
        },
        { title: 'Prio', field: 'priority', sorter: 'number' },
        { title: 'Type', field: 'type', sorter: 'string' },
      ],
      rowFormatter(row) {
        const data = row.getData();
        const isActive = data.id === vueComponent.activeJobID;
        const classList = row.getElement().classList;
        classList.toggle('active-row', isActive);
        classList.toggle('deletion-requested', !!data.delete_requested_at);
      },
      initialSort: [{ column: 'updated', dir: 'desc' }],
      layout: 'fitDataFill',
      layoutColumnsOnNewData: true,
      height: '720px', // Must be set in order for the virtual DOM to function correctly.
      data: [], // Will be filled via a Flamenco API request.
      selectableRows: false, // The active job is tracked by click events, not row selection.
    };
    this.tabulator = new Tabulator('#flamenco_job_list', options);
    this.tabulator.on('rowClick', this.onRowClick);
    this.tabulator.on('tableBuilt', this._onTableBuilt);

    window.addEventListener('resize', this.recalcTableHeight);
  },
  unmounted() {
    window.removeEventListener('resize', this.recalcTableHeight);
  },
  watch: {
    activeJobID(newJobID, oldJobID) {
      this._reformatRow(oldJobID);
      this._reformatRow(newJobID);
    },
    availableStatuses() {
      // Statuses changed, so the filter bar could have gone from "no statuses"
      // to "any statuses" (or one row of filtering stuff to two, I don't know)
      // and changed height.
      this.$nextTick(this.recalcTableHeight);
    },
  },
  methods: {
    /**
     * Send to the job overview page, i.e. job view without active job.
     */
    _routeToJobOverview() {
      const route = { name: 'jobs' };
      this.$router.push(route);
    },
    /**
     * @param {string} jobID job ID to navigate to, can be empty string for "no active job".
     */
    _routeToJob(jobID) {
      const route = { name: 'jobs', params: { jobID: jobID } };
      this.$router.push(route);
    },
    async onReconnected() {
      // If the connection to the backend was lost, we have likely missed some
      // updates. Just fetch the data and start from scratch.
      await this.initAllJobs();
      await this.initActiveJob();
    },
    sortData() {
      const tab = this.tabulator;
      tab.setSort(tab.getSorters()); // This triggers re-sorting.
    },
    async _onTableBuilt() {
      this.tabulator.setFilter(this._filterByStatus);
      await this.initAllJobs();
      await this.initActiveJob();
    },
    async fetchAllJobs() {
      const jobsApi = new API.JobsApi(getAPIClient());
      return jobsApi
        .fetchJobs()
        .then((data) => data.jobs)
        .catch((e) => {
          throw new Error('Unable to fetch all jobs:', e);
        });
    },
    /**
     * Initializes all jobs and sets the Tabulator data. Updates pinia stores and state accordingly.
     */
    async initAllJobs() {
      try {
        this.jobs.isJobless = false;
        const jobs = await this.fetchAllJobs();

        // Update Tabulator
        this.tabulator.setData(jobs);
        this._refreshAvailableStatuses();
        this.recalcTableHeight();

        // Update  Pinia stores
        const hasJobs = jobs && jobs.length > 0;
        this.jobs.isJobless = !hasJobs;
      } catch (e) {
        console.error(e);
      }
    },
    /**
     * Initializes the active job. Updates pinia stores and state accordingly.
     */
    async initActiveJob() {
      // If there's no active job, reset the state and Pinia stores
      if (!this.activeJobID) {
        this.jobs.clearActiveJob();
        this.jobs.clearSelectedJobs();
        this.lastSelectedJobPosition = null;
        return;
      }

      // Otherwise, set the state and Pinia stores
      try {
        const job = await this.fetchJob(this.activeJobID);
        this.jobs.setActiveJob(job);
        this.processJobUpdate(job);
        this.jobs.setSelectedJobs([job]);

        const activeRow = this.tabulator.getRow(this.activeJobID);
        // If the page is reloaded, re-initialize the last selected job (or active job) position, allowing the user to multi-select from that job.
        this.lastSelectedJobPosition = activeRow.getPosition();
        // Make sure the active row on tabulator has the selected status toggled as well
        this.tabulator.selectRow(activeRow);
      } catch (error) {
        console.error(error);
      }
    },
    /**
     * Fetch a Job based on ID
     */
    fetchJob(jobID) {
      const jobsApi = new API.JobsApi(getAPIClient());
      return jobsApi
        .fetchJob(jobID)
        .then((job) => job)
        .catch((err) => {
          throw new Error(`Unable to fetch job with ID ${jobID}:`, err);
        });
    },
    async processJobUpdate(jobUpdate) {
      // updateData() will only overwrite properties that are actually set on
      // jobUpdate, and leave the rest as-is.
      if (!this.tabulator.initialized) {
        return;
      }

      try {
        const row = this.tabulator.rowManager.findRow(jobUpdate.id);
        // If the row update is for deletion, delete the row and route to /jobs
        if (jobUpdate.was_deleted && row) {
          // Prevents the issue where deleted rows persist on Tabulator's selectedData
          // (this should technically not happen -- need to investigate more)
          this.tabulator.deselectRow(jobUpdate.id);
          row.delete().then(() => {
            if (jobUpdate.id === this.activeJobID) {
              this._routeToJobOverview();

              // Update Pinia Stores
              this.jobs.clearActiveJob();
            }
            // Update Pinia Stores
            this.jobs.setSelectedJobs(this.getSelectedJobs());
          });
          return;
        }

        if (row) {
          await this.tabulator.updateData([jobUpdate]); // Update existing row
        } else {
          await this.tabulator.addData([jobUpdate]); // Add new row
        }
        this.sortData();
        await this.tabulator.redraw(); // Resize columns based on new data.
        this._refreshAvailableStatuses();

        if (jobUpdate.id === this.activeJobID && row) {
          const job = await this.fetchJob(jobUpdate.id);
          this.jobs.setActiveJob(job);
        }
        this.jobs.setSelectedJobs(this.tabulator.getSelectedData()); // Update Pinia stores
      } catch (e) {
        console.error(e);
      }
    },
    /**
     * Selects all jobs whose updated timestamp precedes the selected job(s) updated timestamp(s).
     * Handles the section of rows on Tabulator AND the Pinia store.
     */
    handleMassSelect() {
      // Find the most recent updated timestamp from selected jobs
      const mostRecentlyUpdatedJob = this.jobs.selectedJobs.reduce((mostRecent, current) => {
        return current.updated > mostRecent.updated ? current : mostRecent;
      });

      // Find the job rows whose updated timestamp is less than or equal to the most recent updated timestamp
      const rowsToSelect = this.tabulator.searchRows(
        'updated',
        '<=',
        mostRecentlyUpdatedJob.updated
      );

      this.tabulator.selectRow(rowsToSelect);
      // Unlike handleMultiSelect, this function takes responsibility of updating the Pinia store since it functions independent of row clicks.
      this.jobs.setSelectedJobs(this.getSelectedJobs()); // Set the selected jobs according to tabulator's selected rows
    },
    /**
     * A helper function for onRowClick.
     * It handles Shift + left-click and Ctrl/Cmd + left-click events, and the selection of rows on Tabulator.
     * @param event listen for keyboard events
     * @param row the row that was clicked
     * @param tabulator the tabulator to be modified
     */
    handleMultiSelect(event, row, tabulator) {
      const position = row.getPosition();

      // Manage the click event and Tabulator row selection
      if (event.shiftKey && this.lastSelectedJobPosition) {
        // Shift + Click - selects a range of rows
        let start = Math.min(position, this.lastSelectedJobPosition);
        let end = Math.max(position, this.lastSelectedJobPosition);
        const rowsToSelect = [];

        for (let i = start; i <= end; i++) {
          const currRow = this.tabulator.getRowFromPosition(i);
          rowsToSelect.push(currRow);
        }
        tabulator.selectRow(rowsToSelect);

        // Remove the text selection that occurs
        document.getSelection().removeAllRanges();
      } else if (event.ctrlKey || event.metaKey) {
        // Supports Cmd key on MacOS
        // Ctrl + Click - toggles additional rows
        if (tabulator.getSelectedRows().includes(row)) {
          tabulator.deselectRow(row);
        } else {
          tabulator.selectRow(row);
        }
      } else if (!event.ctrlKey && !event.metaKey) {
        // Regular Click - resets the selection to one row
        tabulator.deselectRow(); // De-select all rows
        tabulator.selectRow(row);
      }
    },
    /**
     * Handles Tabulator row click events, routes to the active job ID, and updates the Pinia store accordingly for active and selected jobs.
     * @param event listen for keyboard events
     * @param row the row that was clicked
     */
    async onRowClick(event, row) {
      // Handles Shift + Click, Ctrl + Click, and regular Click
      this.handleMultiSelect(event, row, this.tabulator);

      // Update the app route, Pinia store, and component state
      if (row.isSelected()) {
        // The row was toggled -> selected
        const rowData = row.getData();
        this._routeToJob(rowData.id);

        const job = await this.fetchJob(rowData.id);
        this.jobs.setActiveJob(job);
        this.lastSelectedJobPosition = row.getPosition();
      } else {
        // The row was toggled -> de-selected
        this._routeToJob('');
        this.jobs.clearActiveJob();
        this.lastSelectedJobPosition = null;
      }

      this.jobs.setSelectedJobs(this.getSelectedJobs()); // Set the selected jobs according to tabulator's selected rows
    },
    getSelectedJobs() {
      return this.tabulator.getSelectedData();
    },
    toggleStatusFilter(status) {
      const asSet = new Set(this.shownStatuses);
      if (!asSet.delete(status)) {
        asSet.add(status);
      }
      this.shownStatuses = Array.from(asSet).sort();
      this.tabulator.refreshFilter();
    },
    _filterByStatus(job) {
      if (this.shownStatuses.length == 0) {
        return true;
      }
      return this.shownStatuses.indexOf(job.status) >= 0;
    },
    _refreshAvailableStatuses() {
      const statuses = new Set();
      for (let row of this.tabulator.getData()) {
        statuses.add(row.status);
      }
      this.availableStatuses = Array.from(statuses).sort();
    },

    _reformatRow(jobID) {
      // Use tab.rowManager.findRow() instead of `tab.getRow()` as the latter
      // logs a warning when the row cannot be found.
      const row = this.tabulator.rowManager.findRow(jobID);
      if (!row) return;
      if (row.reformat) row.reformat();
      else if (row.reinitialize) row.reinitialize(true);
    },

    /**
     * Recalculate the appropriate table height to fit in the column without making that scroll.
     */
    recalcTableHeight() {
      if (!this.tabulator.initialized) {
        // Sometimes this function is called too early, before the table was initialised.
        // After the table is initialised it gets resized anyway, so this call can be ignored.
        return;
      }
      const table = this.tabulator.element;
      const tableContainer = table.parentElement;
      const outerContainer = tableContainer.parentElement;
      if (!outerContainer) {
        // This can happen when the component was removed before the function is
        // called. This is possible due to the use of Vue's `nextTick()`
        // function.
        return;
      }

      const availableHeight = outerContainer.clientHeight - 12; // TODO: figure out where the -12 comes from.

      if (tableContainer.offsetParent != tableContainer.parentElement) {
        // `offsetParent` is assumed to be the actual column in the 3-column
        // view. To ensure this, it's given `position: relative` in the CSS
        // styling.
        console.warn(
          'JobsTable.recalcTableHeight() only works when the offset parent is the real parent of the element.'
        );
        return;
      }

      const tableHeight = availableHeight - tableContainer.offsetTop;
      if (this.tabulator.element.clientHeight == tableHeight) {
        // Setting the height on a tabulator triggers all kinds of things, so
        // don't do if it not necessary.
        return;
      }

      this.tabulator.setHeight(tableHeight);
    },
  },
};
</script>
