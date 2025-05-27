<template>
  <div class="col col-tags-list">
    <h2 class="column-title">Available Tags</h2>

    <div class="action-buttons btn-bar">
      <form @submit="createTag">
        <div class="create-tag-container">
          <input
            type="text"
            name="newtagname"
            v-model="newTagName"
            placeholder="New Tag Name"
            class="create-tag-input" />
          <button id="submit-button" type="submit" :disabled="newTagName.trim() === ''">
            Create Tag
          </button>
        </div>
      </form>
    </div>

    <div id="tag-table-container"></div>
  </div>
  <div class="col col-tags-info">
    <h2 class="column-title">Information</h2>

    <p>
      Workers and jobs can be tagged. With these tags you can assign a job to a subset of your
      workers.
    </p>

    <h4>Job Perspective:</h4>
    <ul>
      <li>A job can have one tag, or no tag.</li>
      <li>A job <strong>with</strong> a tag will only be assigned to workers with that tag.</li>
      <li>A job <strong>without</strong> tag will be assigned to any worker.</li>
    </ul>

    <h4>Worker Perspective:</h4>
    <ul>
      <li>A worker can have any number of tags.</li>
      <li>
        A worker <strong>with</strong> one or more tags will work only on jobs with one those tags,
        and on tagless jobs.
      </li>
      <li>A worker <strong>without</strong> tags will only work on tagless jobs.</li>
    </ul>
  </div>
  <footer class="app-footer">
    <notification-bar />
    <update-listener
      ref="updateListener"
      mainSubscription="allWorkerTags"
      @workerTagUpdate="onSIOWorkerTagsUpdate"
      @sioReconnected="onSIOReconnected"
      @sioDisconnected="onSIODisconnected" />
  </footer>
</template>

<style>
.col-tags-list {
  grid-area: col-1;
}
.col-tags-info {
  grid-area: col-2;
  color: var(--color-text-muted);
}
.col-tags-info h4 {
  margin-bottom: 0.5em;
}
.col-tags-info ul {
  margin-top: 0.5em;
  padding-left: 2em;
}

.create-tag-container {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.create-tag-input {
  flex: 1;
  margin-right: 10px;
  height: 30px;
}
.placeholder {
  color: var(--color-text-hint);
}
</style>

<script>
import { TabulatorFull as Tabulator } from 'tabulator-tables';
import { useWorkers } from '@/stores/workers';
import { useNotifs } from '@/stores/notifications';
import { WorkerMgtApi } from '@/manager-api';
import { WorkerTag } from '@/manager-api';
import { getAPIClient } from '@/api-client';
import NotificationBar from '@/components/footer/NotificationBar.vue';
import UpdateListener from '@/components/UpdateListener.vue';

export default {
  components: {
    NotificationBar,
    UpdateListener,
  },
  data() {
    return {
      tags: [],
      selectedTag: null,
      newTagName: '',
      workers: useWorkers(),
      activeRowIndex: -1,
    };
  },

  mounted() {
    document.body.classList.add('is-two-columns');

    this.fetchTags();

    const tag_options = {
      columns: [
        {
          title: 'Name',
          field: 'name',
          sorter: 'string',
          editor: 'input',
          vertAlign: 'center',
          widthGrow: 1,
        },
        {
          title: 'Description',
          field: 'description',
          sorter: 'string',
          editor: 'input',
          vertAlign: 'center',
          widthGrow: 2,

          formatter(cell) {
            const cellValue = cell.getData().description;
            if (!cellValue) {
              return '<span class="placeholder">click to set a description</span>';
            }
            return cellValue;
          },
        },
        {
          title: '',
          headerHozAlign: 'right',
          width: 60,

          formatter: (cell) => {
            const button = document.createElement('button');
            button.classList.add('remove-tag-button');
            button.innerHTML = '❌';
            button.addEventListener('click', () => {
              const tag = cell.getRow().getData();
              this.deleteTag(tag);
            });

            return button;
          },
          headerSort: false,
          vertAlign: 'center',
        },
      ],
      layout: 'fitColumns',
      layoutColumnsOnNewData: true,
      height: '82%',
      selectableRows: false,
    };

    this.tabulator = new Tabulator('#tag-table-container', tag_options);
    this.tabulator.on('tableBuilt', () => {
      this.fetchTags();
    });
    this.tabulator.on('cellEdited', (cell) => {
      const editedTag = cell.getRow().getData();
      this.updateTagInAPI(editedTag);
    });
  },
  unmounted() {
    document.body.classList.remove('is-two-columns');
  },

  methods: {
    _onTableBuilt() {
      this.fetchTags();
    },

    fetchTags() {
      this.workers
        .refreshTags()
        .then(() => {
          this.tags = this.workers.tags;
          this.tabulator.setData(this.tags);
        })
        .catch((error) => {
          const errorMsg = JSON.stringify(error);
          useNotifs().add(`Error: ${errorMsg}`);
        });
    },

    createTag(event) {
      event.preventDefault();

      const api = new WorkerMgtApi(getAPIClient());
      const newTag = new WorkerTag(this.newTagName);
      api
        .createWorkerTag(newTag)
        .then(() => {
          this.fetchTags(); // Refresh table data
          this.newTagName = '';
        })
        .catch((error) => {
          const errorMsg = JSON.stringify(error);
          useNotifs().add(`Error: ${errorMsg}`);
        });
    },

    updateTagInAPI(tag) {
      const { id: tagId, ...updatedTagData } = tag;
      const api = new WorkerMgtApi(getAPIClient());

      api
        .updateWorkerTag(tagId, updatedTagData)
        .then(() => {
          console.log('Tag updated successfully');
        })
        .catch((error) => {
          const errorMsg = JSON.stringify(error);
          useNotifs().add(`Error: ${errorMsg}`);
        });
    },

    deleteTag(tag) {
      const api = new WorkerMgtApi(getAPIClient());
      api
        .deleteWorkerTag(tag.id)
        .then(() => {
          this.tabulator.getRows().forEach((row) => {
            if (row.getData().id === tag.id) {
              row.delete();
            }
          });
        })
        .catch((error) => {
          const errorMsg = JSON.stringify(error);
          useNotifs().add(`Error: ${errorMsg}`);
        });
    },

    // SocketIO connection event handlers:
    onSIOReconnected() {
      this.fetchTags();
    },
    onSIODisconnected(reason) {},
    onSIOWorkerTagsUpdate(workerTagsUpdate) {
      this.fetchTags();
    },
  },
};
</script>
