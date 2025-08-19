<template>
  <h3 class="sub-title">Tasks</h3>
  <div class="btn-bar-group">
    <task-actions-bar />
    <status-filter-bar
      :availableStatuses="availableStatuses"
      :activeStatuses="shownStatuses"
      @click="toggleStatusFilter" />
  </div>
  <div>
    <div
      @keydown="disableSorting"
      @keyup="enableSorting"
      class="task-list with-clickable-row"
      id="flamenco_task_list"></div>
  </div>
</template>

<script>
import { TabulatorFull as Tabulator } from 'tabulator-tables';
import * as datetime from '@/datetime';
import * as API from '@/manager-api';
import { indicator } from '@/statusindicator';
import { getAPIClient } from '@/api-client';
import { useTasks } from '@/stores/tasks';

import TaskActionsBar from '@/components/jobs/TaskActionsBar.vue';
import StatusFilterBar from '@/components/StatusFilterBar.vue';

export default {
  props: [
    'jobID', // ID of the job of which the tasks are shown here.
    'taskID', // The active task.
  ],
  components: {
    TaskActionsBar,
    StatusFilterBar,
  },
  data: () => {
    return {
      tasks: useTasks(),
      shownStatuses: [],
      availableStatuses: [], // Will be filled after data is loaded from the backend.
      lastSelectedTaskPosition: null,
      sortable: true,
    };
  },
  mounted() {
    // Allow testing from the JS console:
    // tasksTableVue.processTaskUpdate({id: "ad0a5a00-5cb8-4e31-860a-8a405e75910e", status: "heyy", updated: DateTime.local().toISO(), previous_status: "uuuuh", name: "Updated manually"});
    // tasksTableVue.processTaskUpdate({id: "ad0a5a00-5cb8-4e31-860a-8a405e75910e", status: "heyy", updated: DateTime.local().toISO()});
    window.tasksTableVue = this;

    const vueComponent = this;
    const options = {
      // See pkg/api/flamenco-openapi.yaml, schemas Task and TaskUpdate.
      columns: [
        { title: 'Num', field: 'index_in_job' },
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
        { title: 'Name', field: 'name', sorter: 'string', minWidth: 104 },
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
        {
          title: 'Worker',
          field: 'worker.name',
          sorter: 'string',
          sorterParams: { alignEmptyValues: 'bottom' },
          formatter: (cell) => {
            const worker = cell.getData().worker;
            if (!worker) return '';
            return `<a href="/app/workers/${worker.id}">${worker.name}</a>`;
          },
          minWidth: 100,
          widthGrow: 1,
        },
      ],
      rowFormatter(row) {
        const data = row.getData();
        const isActive = data.id === vueComponent.taskID;
        row.getElement().classList.toggle('active-row', isActive);
      },
      initialSort: [{ column: 'updated', dir: 'desc' }],
      layout: 'fitDataFill',
      layoutColumnsOnNewData: true,
      height: '100%', // Must be set in order for the virtual DOM to function correctly.
      maxHeight: '100%',
      data: [], // Will be filled via a Flamenco API request.
      selectableRows: false, // The active and selected tasks are tracked by custom click events.
    };

    this.tabulator = new Tabulator('#flamenco_task_list', options);
    this.tabulator.on('rowClick', this.onRowClick);
    this.tabulator.on('tableBuilt', this._onTableBuilt);

    window.addEventListener('resize', this.recalcTableHeight);
  },
  unmounted() {
    window.removeEventListener('resize', this.recalcTableHeight);
  },
  watch: {
    jobID() {
      this.fetchTasks();
    },
    taskID(newID, oldID) {
      this._reformatRow(oldID);
      this._reformatRow(newID);
    },
    availableStatuses() {
      // Statuses changed, so the filter bar could have gone from "no statuses"
      // to "any statuses" (or one row of filtering stuff to two, I don't know)
      // and changed height.
      this.$nextTick(this.recalcTableHeight);
    },
  },
  methods: {
    onReconnected() {
      // If the connection to the backend was lost, we have likely missed some
      // updates. Just fetch the data and start from scratch.
      this.fetchTasks();
    },
    /**
     * @param {string} taskID task ID to navigate to within this job, can be
     * empty string for "no active task".
     */
    _routeToTask(taskID) {
      const route = { name: 'jobs', params: { jobID: this.jobID, taskID: taskID } };
      this.$router.push(route);
    },
    sortData() {
      if (!this.sortable) return;
      const tab = this.tabulator;
      tab.setSort(tab.getSorters()); // This triggers re-sorting.
    },
    _onTableBuilt() {
      this.tabulator.setFilter(this._filterByStatus);
      this.fetchTasks();
    },
    /**
     * Fetch task info and set the active task once it's received.
     */
    fetchActiveTask() {
      // If there's no active task, reset the state and Pinia stores
      if (!this.taskID) {
        this.tasks.clearActiveTask();
        this.tasks.clearSelectedTasks();
        this.lastSelectedTaskPosition = null;
        return;
      }

      // Otherwise, set the state and Pinia stores
      const jobsApi = new API.JobsApi(getAPIClient()); // init the API
      jobsApi.fetchTask(this.taskID).then((task) => {
        this.tasks.setActiveTask(task);
        this.tasks.setSelectedTasks([task]);

        const activeRow = this.tabulator.getRow(this.taskID);
        // If the page is reloaded, re-initialize the last selected task (or active task) position, allowing the user to multi-select from that task.
        this.lastSelectedTaskPosition = activeRow.getPosition();
        // Make sure the active row on tabulator has the selected status toggled as well
        this.tabulator.selectRow(activeRow);
      });
    },
    /**
     * Fetch all tasks and set the Tabulator data
     */
    fetchTasks() {
      // No active job
      if (!this.jobID) {
        this.tabulator.setData([]);
        return;
      }

      // Deselect all rows before setting new task data. This prevents the error caused by trying to deselect rows that don't exist on the new data.
      this.tabulator.deselectRow();

      const jobsApi = new API.JobsApi(getAPIClient()); // init the API

      jobsApi.fetchJobTasks(this.jobID).then(
        (data) => {
          this.tabulator.setData(data.tasks);
          this._refreshAvailableStatuses();
          this.recalcTableHeight();

          this.fetchActiveTask();
        },
        (error) => {
          // TODO: error handling.
          console.error(error);
        }
      );
    },
    enableSorting(event) {
      if (event.key === 'Shift') {
        this.sortable = true;
      }
    },
    disableSorting(event) {
      if (event.key === 'Shift') {
        this.sortable = false;
      }
    },
    processTaskUpdate(taskUpdate) {
      // Any updates to tasks i.e. status changes will need to reflect its changes to the rows on Tabulator here.
      // updateData() will only overwrite properties that are actually set on
      // taskUpdate, and leave the rest as-is.
      if (this.tabulator.initialized) {
        this.tabulator
          .updateData([taskUpdate])
          .then(this.sortData)
          .then(() => {
            this.tabulator.redraw();
          }); // Resize columns based on new data.
      }
      this.tasks.setSelectedTasks(this.getSelectedTasks()); // Update Pinia stores
      this._refreshAvailableStatuses();
    },
    getSelectedTasks() {
      return this.tabulator.getSelectedData();
    },
    handleMultiSelect(event, row, tabulator) {
      const position = row.getPosition();

      // Manage the click event and Tabulator row selection
      if (event.shiftKey && this.lastSelectedTaskPosition) {
        // Shift + Click - selects a range of rows
        let start = Math.min(position, this.lastSelectedTaskPosition);
        let end = Math.max(position, this.lastSelectedTaskPosition);
        const rowsToSelect = [];

        for (let i = start; i <= end; i++) {
          const currRow = this.tabulator.getRowFromPosition(i);
          rowsToSelect.push(currRow);
        }
        tabulator.selectRow(rowsToSelect);

        // Remove text-selection that occurs during Shift + Click
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
    onRowClick(event, row) {
      // Handles Shift + Click, Ctrl + Click, and regular Click
      this.handleMultiSelect(event, row, this.tabulator);

      // Update the app route, Pinia store, and component state
      if (this.tabulator.getSelectedRows().includes(row)) {
        // The row was toggled -> selected
        const rowData = row.getData();
        this._routeToTask(rowData.id);

        const jobsApi = new API.JobsApi(getAPIClient()); // init the API
        jobsApi.fetchTask(rowData.id).then((task) => {
          // row.getData() will return the API.TaskSummary data, while tasks.setActiveTask() needs the entire API.Task
          this.tasks.setActiveTask(task);
        });
        this.lastSelectedTaskPosition = row.getPosition();
      } else {
        // The row was toggled -> de-selected
        this._routeToTask('');
        this.tasks.clearActiveTask();
        this.lastSelectedTaskPosition = null;
      }

      this.tasks.setSelectedTasks(this.getSelectedTasks()); // Set the selected tasks according to tabulator's selected rows
    },
    toggleStatusFilter(status) {
      const asSet = new Set(this.shownStatuses);
      if (!asSet.delete(status)) {
        asSet.add(status);
      }
      this.shownStatuses = Array.from(asSet).sort();
      this.tabulator.refreshFilter();
    },
    _filterByStatus(tableItem) {
      if (this.shownStatuses.length == 0) {
        return true;
      }
      return this.shownStatuses.indexOf(tableItem.status) >= 0;
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
          'TaskTable.recalcTableHeight() only works when the offset parent is the real parent of the element.'
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
