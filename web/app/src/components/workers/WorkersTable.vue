<template>
  <h2 class="column-title">Workers</h2>

  <div class="btn-bar-group">
    <worker-actions-bar />
    <status-filter-bar
      :availableStatuses="availableStatuses"
      :activeStatuses="shownStatuses"
      classPrefix="worker-"
      @click="toggleStatusFilter" />
  </div>

  <div>
    <div class="workers-list with-clickable-row" id="flamenco_workers_list"></div>
  </div>
</template>

<script>
import { TabulatorFull as Tabulator } from 'tabulator-tables';
import { WorkerMgtApi } from '@/manager-api';
import { indicator, workerStatus } from '@/statusindicator';
import { getAPIClient } from '@/api-client';
import { useWorkers } from '@/stores/workers';

import StatusFilterBar from '@/components/StatusFilterBar.vue';
import WorkerActionsBar from '@/components/workers/WorkerActionsBar.vue';

export default {
  name: 'WorkersTable',
  props: ['activeWorkerID'],
  emits: ['tableRowClicked'],
  components: {
    StatusFilterBar,
    WorkerActionsBar,
  },
  data: () => {
    return {
      workers: useWorkers(),
      api: new WorkerMgtApi(getAPIClient()),

      shownStatuses: [],
      availableStatuses: [], // Will be filled after data is loaded from the backend.
      lastSelectedWorkerPosition: null,
    };
  },
  mounted() {
    window.workersTableVue = this;

    const vueComponent = this;
    const options = {
      // See pkg/api/flamenco-openapi.yaml, schemas WorkerSummary and EventWorkerUpdate.
      columns: [
        // Useful for debugging when there are many similar workers:
        // { title: "ID", field: "id", headerSort: false, formatter: (cell) => cell.getData().id.substr(0, 8), },
        {
          title: 'Status',
          field: 'status',
          sorter: 'string',
          formatter: (cell) => {
            const data = cell.getData();
            const dot = indicator(data.status, 'worker-');
            const asString = workerStatus(data);
            return `${dot} ${asString}`;
          },
        },
        { title: 'Name', field: 'name', sorter: 'string' },
        { title: 'Version', field: 'version', sorter: 'string' },
      ],
      rowFormatter(row) {
        const data = row.getData();
        const isActive = data.id === vueComponent.activeWorkerID;
        row.getElement().classList.toggle('active-row', isActive);
      },
      initialSort: [{ column: 'name', dir: 'asc' }],
      layout: 'fitDataFill',
      layoutColumnsOnNewData: true,
      height: '360px', // Must be set in order for the virtual DOM to function correctly.
      data: [], // Will be filled via a Flamenco API request.
      selectableRows: false, // The active worker is tracked by click events, not row selection.
    };
    this.tabulator = new Tabulator('#flamenco_workers_list', options);
    this.tabulator.on('rowClick', this.onRowClick);
    this.tabulator.on('tableBuilt', this._onTableBuilt);

    window.addEventListener('resize', this.recalcTableHeight);
  },
  unmounted() {
    window.removeEventListener('resize', this.recalcTableHeight);
  },
  watch: {
    activeWorkerID(newWorkerID, oldWorkerID) {
      this._reformatRow(oldWorkerID);
      this._reformatRow(newWorkerID);
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
     * @param {string} workerID worker ID to navigate to, can be empty string for "no active worker".
     */
    _routeToWorker(workerID) {
      const route = { name: 'workers', params: { workerID: workerID } };
      this.$router.push(route);
    },
    async onReconnected() {
      // If the connection to the backend was lost, we have likely missed some
      // updates. Just re-initialize the data and start from scratch.
      await this.initAllWorkers();
      await this.initActiveWorker();
    },
    sortData() {
      const tab = this.tabulator;
      tab.setSort(tab.getSorters()); // This triggers re-sorting.
    },
    async _onTableBuilt() {
      this.tabulator.setFilter(this._filterByStatus);
      await this.initAllWorkers();
      await this.initActiveWorker();
    },
    /**
     * Initializes the active worker. Updates pinia stores and state accordingly.
     */
    async initActiveWorker() {
      // If there's no active Worker, reset the state and Pinia stores
      if (!this.activeWorkerID) {
        this.workers.clearActiveWorker();
        this.workers.clearSelectedWorkers();
        this.lastSelectedWorkerPosition = null;
        return;
      }

      try {
        const worker = await this.fetchWorker(this.activeWorkerID);

        this.workers.setActiveWorker(worker);
        this.workers.setSelectedWorkers([worker]);

        const activeRow = this.tabulator.getRow(this.activeWorkerID);
        // If the page is reloaded, re-initialize the last selected worker (or active worker)
        // position, allowing the user to multi-select from that worker.
        this.lastSelectedWorkerPosition = activeRow.getPosition();
        // Make sure the active row on tabulator has the selected status toggled as well
        this.tabulator.selectRow(activeRow);
      } catch (e) {
        console.error(e);
      }
    },
    /**
     * Fetch a Worker based on ID
     */
    fetchWorker(workerID) {
      return this.api
        .fetchWorker(workerID)
        .then((worker) => worker)
        .catch((err) => {
          throw new Error(`Unable to fetch worker with ID ${workerID}:`, err);
        });
    },
    /**
     * Initializes all workers and sets the Tabulator data. Updates pinia stores and state accordingly.
     */
    async initAllWorkers() {
      try {
        const workers = await this.fetchAllWorkers();
        this.tabulator.setData(workers);
        this._refreshAvailableStatuses();
        this.recalcTableHeight();
      } catch (e) {
        console.error(e);
      }
    },
    /**
     * Fetch all workers
     */
    fetchAllWorkers() {
      return this.api
        .fetchWorkers()
        .then((data) => data.workers)
        .catch((e) => {
          throw new Error('Unable to fetch all workers:', e);
        });
    },
    async processWorkerUpdate(workerUpdate) {
      if (!this.tabulator.initialized) return;

      try {
        // Contrary to tabulator.getRow(), rowManager.findRow() doesn't log a
        // warning when the row cannot be found,
        const existingRow = this.tabulator.rowManager.findRow(workerUpdate.id);

        // Delete the row
        if (existingRow && workerUpdate.deleted_at) {
          // Prevents the issue where deleted rows persist on Tabulator's selectedData
          this.tabulator.deselectRow(workerUpdate.id);
          await existingRow.delete();

          // If the deleted worker was active, route to /workers
          if (workerUpdate.id === this.activeWorkerID) {
            this._routeToWorker('');
            // Update Pinia Stores
            this.workers.clearActiveWorker();
          }
          // Update Pinia Stores
          this.workers.setSelectedWorkers(this.getSelectedWorkers());

          return;
        }

        if (existingRow) {
          // Prepare to update an existing row.
          // Tabulator doesn't update ommitted fields, but if `status_change`
          // is ommitted it means "no status change requested"; this should still
          // force an update of the `status_change` field.
          workerUpdate.status_change = workerUpdate.status_change || null;

          // Update the existing row.
          // Tabulator doesn't know we're using 'status_change' in the 'status'
          // column, so it also won't know to redraw when that field changes.
          await this.tabulator.updateData([workerUpdate]);
          existingRow.reinitialize(true);
        } else {
          await this.tabulator.addData([workerUpdate]); // Add a new row.
        }

        this.sortData();
        await this.tabulator.redraw();
        this._refreshAvailableStatuses(); // Resize columns based on new data.

        // Update Pinia stores
        this.workers.setSelectedWorkers(this.getSelectedWorkers());
        if (workerUpdate.id === this.activeWorkerID) {
          const worker = await this.fetchWorker(this.activeWorkerID);
          this.workers.setActiveWorker(worker);
        }

        // TODO: this should also resize the columns, as the status column can
        // change sizes considerably.
      } catch (e) {
        console.error(e);
      }
    },
    handleMultiSelect(event, row, tabulator) {
      const position = row.getPosition();

      // Manage the click event and Tabulator row selection
      if (event.shiftKey && this.lastSelectedWorkerPosition) {
        // Shift + Click - selects a range of rows
        let start = Math.min(position, this.lastSelectedWorkerPosition);
        let end = Math.max(position, this.lastSelectedWorkerPosition);
        const rowsToSelect = [];

        for (let i = start; i <= end; i++) {
          const currRow = this.tabulator.getRowFromPosition(i);
          rowsToSelect.push(currRow);
        }
        tabulator.selectRow(rowsToSelect);

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
    async onRowClick(event, row) {
      // Take a copy of the data, so that it's decoupled from the tabulator data
      // store. There were some issues where navigating to another worker would
      // overwrite the old worker's ID, and this prevents that.

      this.handleMultiSelect(event, row, this.tabulator);

      // Update the app route, Pinia store, and component state
      if (this.tabulator.getSelectedRows().includes(row)) {
        // The row was toggled -> selected
        const rowData = row.getData();
        this._routeToWorker(rowData.id);

        const worker = await this.fetchWorker(rowData.id);
        this.workers.setActiveWorker(worker);
        this.lastSelectedWorkerPosition = row.getPosition();
      } else {
        // The row was toggled -> de-selected
        this._routeToWorker('');
        this.workers.clearActiveWorker();
        this.lastSelectedWorkerPosition = null;
      }

      // Set the selected jobs according to tabulator's selected rows
      this.workers.setSelectedWorkers(this.getSelectedWorkers());
    },
    getSelectedWorkers() {
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
    _filterByStatus(worker) {
      if (this.shownStatuses.length == 0) {
        return true;
      }
      return this.shownStatuses.indexOf(worker.status) >= 0;
    },
    _refreshAvailableStatuses() {
      const statuses = new Set();
      for (let row of this.tabulator.getData()) {
        statuses.add(row.status);
      }
      this.availableStatuses = Array.from(statuses).sort();
    },

    _reformatRow(workerID) {
      // Use tab.rowManager.findRow() instead of `tab.getRow()` as the latter
      // logs a warning when the row cannot be found.
      const row = this.tabulator.rowManager.findRow(workerID);
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
      this.tabulator.setHeight(tableHeight);
    },
  },
};
</script>
