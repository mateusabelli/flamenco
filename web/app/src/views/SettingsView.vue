<script>
import NotificationBar from '@/components/footer/NotificationBar.vue';
import UpdateListener from '@/components/UpdateListener.vue';
import FormInputDropdownSelect from '@/components/settings/FormInputDropdownSelect.vue';
import FormInputSwitchCheckbox from '@/components/settings/FormInputSwitchCheckbox.vue';
import FormInputText from '@/components/settings/FormInputText.vue';
import FormInputNumber from '@/components/settings/FormInputNumber.vue';
import { MetaApi } from '@/manager-api';
import { getAPIClient } from '@/api-client';

const timeDurationOptions = {
  '0s': 'Zero',
  '1m0s': '1 Minute', // worker timeout
  '5m0s': '5 Minutes',
  '10m0s': '10 Minutes', // task timeout, DB check period
  '30m0s': '30 Minutes',
  '1h0m0s': '1 Hour',
  '24h0m0s': '1 Day', // GC period
  '168h0m0s': '1 Week',
  '744h0m0s': '1 Month', // GC maxAge
};

const platformOptions = {
  darwin: 'Darwin (MacOS)',
  windows: 'Windows',
  linux: 'Linux',
  all: 'All Operating Systems',
};

const audienceOptions = {
  all: 'All',
  users: 'Users',
  workers: 'Workers',
};

// The type determines which form component will be rendered and used to modify a value
const inputTypes = {
  string: 'string', // input type=string
  timeDuration: 'timeDuration', // dropdown
  boolean: 'boolean', // switch checkbox
  number: 'number', // input type=number
  platform: 'Platform', // dropdown
  audience: 'Audience', // dropdown
};

const categories = [
  {
    id: 'core-settings',
    label: 'Core Settings',
    settings: ['manager_name', 'database', 'database_check_period', 'listen', 'autodiscoverable'],
  },
  {
    id: 'storage',
    label: 'Storage',
    settings: ['local_manager_storage_path', 'shared_storage_path', 'shaman'],
  },
  {
    id: 'timeout-failures',
    label: 'Timeout & Failures',
    settings: [
      'task_timeout',
      'worker_timeout',
      'blocklist_threshold',
      'task_fail_after_softfail_count',
    ],
  },
  {
    id: 'mqtt',
    label: 'MQTT',
    settings: ['mqtt'],
  },
  { id: 'variables', label: 'Variables' },
];

// the initialFormValues object matches the hierarchy from flamenco-manager.yaml, making it easy to override and import
// For each of the sections: core, storage, timeout-failures, mqtt, and variables:
// - type is the expected input type that determines which input component to render
// - label is what's displayed on the user interface
// - value is the setting's input value
const initialFormValues = {
  _meta: {
    version: 3,
  },
  // Core
  manager_name: {
    type: inputTypes.string,
    label: 'Name',
    value: null,
    description: `The name of the Flamenco Manager.`,
  },
  database: {
    type: inputTypes.string,
    label: 'Database',
    value: null,
    description: `The file path for the SQLite database.`,
    required: true,
  },
  database_check_period: {
    type: inputTypes.timeDuration,
    label: 'Database Check Period',
    value: null,
    description: `How frequently the database is checked for internal consistency.\n\nThis check always happens at startup of Flamenco Manager. By setting this to a non-zero duration, the check is also performed while Flamenco Manager is running.\n\nIt is not typically necessary to set this; it was implemented to help find a bug, which has been fixed in Flamenco 3.6. The setting may be removed in the future.`,
    required: true,
  },
  listen: {
    type: inputTypes.string,
    label: 'Listening IP and Port Number',
    value: null,
    description: `The IP and port (e.g., :8080, 192.168.0.1:8080, or [::]:8080) Flamenco Manager will listen on.\n\nThis is the only port that is needed for Flamenco Manager, and will be used for the web interface, the API, and file submission via the Shaman system.`,
    required: true,
  },
  autodiscoverable: {
    type: inputTypes.boolean,
    label: 'Auto Discoverable',
    value: null,
    description: `This enables the autodiscovery. The manager uses UPnP/SSDP to broadcast its location on the network so it can be discovered by workers. Enabled by default.`,
  },

  // Storage
  local_manager_storage_path: {
    type: inputTypes.string,
    label: 'Local Manager Storage Path',
    value: null,
    description: `The path where the Manager stores local files (e.g., logs, last-rendered images, etc.).\n\nThese files are only necessary for the manager. Workers never need to access this directly, as the files are accessible via the web interface.`,
  },
  shared_storage_path: {
    type: inputTypes.string,
    label: 'Shared Storage Path',
    value: null,
    description: `The Shared Storage path where files shared between Manager and Worker(s) live (e.g., rendered output files, or the .blend files of render jobs).`,
    required: true,
  },
  shaman: {
    enabled: {
      type: inputTypes.boolean,
      label: 'Enable Shaman Storage',
      value: null,
      description: `Shaman is a file storage server built into Flamenco Manager. It accepts uploaded files via HTTP, and stores them based on their SHA256-checksum and their file length. It can recreate directory structures by symlinking those files. Effectively, it ensures that when you create a new render job, you only have to upload files that are new or have changed.\n\nNote that Shaman uses symlinking, and thus is incompatible with platforms or storage systems that do not support symbolic links.\n\n`,
      moreInfoText: `For more information, see`,
      moreInfoLinkUrl: `https://flamenco.blender.org/usage/shared-storage/shaman/`,
      moreInfoLinkLabel: `Shaman Storage System`,
    },
    garbageCollect: {
      period: {
        type: inputTypes.timeDuration,
        label: 'Period',
        value: null,
        description: `The period of time determining the frequency of garbage collection performed on file store.`,
        required: true,
      },
      maxAge: {
        type: inputTypes.timeDuration,
        label: 'Max Age',
        value: null,
        description: `The minimum lifespan of files required in order to be garbage collected.`,
        required: true,
      },
    },
  },

  // Timeout Failures
  task_timeout: {
    type: inputTypes.timeDuration,
    label: 'Task Timeout',
    value: null,
    description: `The Manager will consider a Worker to be “problematic” if it hasn't heard anything from that Worker for this amount of time. When that happens, the Worker will be shown on the Manager in error status.`,
    required: true,
  },
  worker_timeout: {
    type: inputTypes.timeDuration,
    label: 'Worker Timeout',
    value: null,
    description: `The amount of time since the worker's last sign of life (e.g., asking for a task to perform, or checking if it's allowed to perform its current task) before getting marked “timed out” and sent to error status`,
    required: true,
  },
  blocklist_threshold: {
    type: inputTypes.number,
    label: 'Blocklist Threshold',
    value: null,
    description: `The number of failures allowed on a type of task per job before banning a worker from that task type on that job.\n\nFor example, when a worker fails multiple blender tasks on one job, it's concluded that the job is too heavy for its hardware, and thus it gets blocked from doing more of those. It is then still allowed to do file management, video encoding tasks, or blender tasks on another job.`,
    required: true,
  },
  task_fail_after_softfail_count: {
    type: inputTypes.number,
    label: 'Task Fail after Soft Fail Count',
    value: null,
    description: `The number of workers allowed to have failed a task before hard-failing the task.`,
    required: true,
  },

  // MQTT
  mqtt: {
    enabled: {
      type: inputTypes.boolean,
      label: 'Enable MQTT Client',
      value: null,
      description: `Flamenco Manager can send its internal events to an MQTT broker. Other MQTT clients can listen to those events, in order to respond to what happens on the render farm.\n\n`,
      moreInfoText: 'For more information about the built-in MQTT client, see',
      moreInfoLinkUrl: 'https://flamenco.blender.org/usage/manager-configuration/mqtt/',
      moreInfoLinkLabel: `Manager Configuration: MQTT`,
    },
    client: {
      broker: {
        type: inputTypes.string,
        label: 'Broker',
        value: null,
        description: `The URL for the MQTT server.`,
      },
      clientID: {
        type: inputTypes.string,
        label: 'Client ID',
        value: null,
        description: `An identifier that each MQTT client uses to identify itself.`,
      },
      topic_prefix: {
        type: inputTypes.string,
        label: 'Topic Prefix',
        value: null,
        description: `The word to prefix each topic (e.g., flamenco).`,
      },
      username: {
        type: inputTypes.string,
        label: 'Username',
        value: null,
        description: `The username of the broker/client.`,
      },
      password: {
        type: inputTypes.string,
        label: 'Password',
        value: null,
        description: `The password of the broker/client.`,
      },
    },
  },

  // Variables
  variables: {},
};

export default {
  name: 'ConfigurationSettingsView',
  components: {
    NotificationBar,
    UpdateListener,
    FormInputText,
    FormInputNumber,
    FormInputSwitchCheckbox,
    FormInputDropdownSelect,
  },

  data: () => ({
    // Make a deep copy so it can be compared to the original for isDirty check to work
    config: JSON.parse(JSON.stringify(initialFormValues)),
    originalConfig: JSON.parse(JSON.stringify(initialFormValues)),
    isEditingVariableName: {}, // the var name(s) being edited gets stored here
    newVariableName: '',
    newVariableErrorMessage: '',
    newVariableTouched: false,
    showSubmissionBanner: false,
    submissionErrorMessage: '',
    metaAPI: new MetaApi(getAPIClient()),
    focusedSetting: {},

    // Static data
    inputTypes,
    timeDurationOptions,
    platformOptions,
    audienceOptions,
    categories,
  }),
  created() {
    this.importConfig();
  },
  /**
   * Confirm with the user about unsaved edits before navigating to another route within the web app
   * (and in Chrome back/forward arrows)
   */
  beforeRouteLeave(to, from, next) {
    // If the form hasn't been modified, allow navigation
    if (!this.isDirty) {
      next();
      return;
    }

    if (this.isDirty) {
      const answer = window.confirm(
        'Are you sure you want to leave this page? Changes you made may not be saved.'
      );
      if (answer) {
        next(); // Allow navigation
      } else {
        next(false); // Prevent navigation
      }
    }
  },
  mounted() {
    // Confirm with the user about unsaved edits before leaving the page through refresh and tab close
    // (Windows: and in Firefox back/forward arrows)
    // (MAC: refresh, tab close, and back arrow for Firefox/Chrome)
    window.onbeforeunload = (event) => {
      if (this.isDirty) {
        event.preventDefault();
        event.returnValue = true;
        return true;
      }
    };
  },
  beforeUnmount() {
    // Remove the event listener
    window.onbeforeunload = null;
  },
  computed: {
    isDirty() {
      return JSON.stringify(this.originalConfig) !== JSON.stringify(this.config);
    },
    sortedVariableNames() {
      return Object.keys(this.config.variables).sort();
    },
  },
  methods: {
    undoEdits() {
      // Restore the original config that was imported upon page load or last succssfully exported on form submission
      this.config = JSON.parse(JSON.stringify(this.originalConfig));
    },
    // Sets the boilerplate description on the focus of a variable value
    handleFocusVariableValue() {
      this.focusedSetting = {
        label: 'Value',
        description: 'The contents of the variable.',
      };
    },
    // Sets the boilerplate description on the focus of a variable value
    handleFocusVariablePlatform() {
      this.focusedSetting = {
        label: 'Platform',
        description: 'The operating system in which this variable configuration will be used.',
      };
    },
    // Sets the boilerplate description on the focus of a variable value
    handleFocusVariableAudience() {
      this.focusedSetting = {
        label: 'Audience',
        description: 'The audience who this variable configuration will be used for.',
      };
    },
    /**
     * Grabs the information of the setting on focus and stores its state
     * @param id the id of the element that was focused on
     */
    handleFocus(id) {
      // If the id has a period, break it into tokens to access nested attributes
      if (id.includes('.')) {
        const tokens = id.split('.');

        let val = {};

        tokens.forEach((token, i) => {
          if (i === 0) val = this.config[token];
          else val = val[token];
        });

        this.focusedSetting = val;
      } else {
        this.focusedSetting = this.config[id];
      }
    },
    addVariableOnInput() {
      this.newVariableTouched = true;
    },
    editVariableOnInput(variableName) {
      this.isEditingVariableName[variableName].touched = true;
    },
    canEditVariable(variableName) {
      const newName = this.isEditingVariableName[variableName].name;
      const errorMessage = this.validateVariableName(newName);

      // Only show an error message if the field has been touched AND the edited var name is different than the original
      if (this.isEditingVariableName[variableName].touched && variableName !== newName) {
        this.isEditingVariableName[variableName].errorMessage = errorMessage;
      }
      // returns false if there is an errorMessage or if the variable name is empty
      return !errorMessage && newName !== '';
    },
    canAddVariable() {
      // Validate the variable name
      const errorMessage = this.validateVariableName(this.newVariableName);
      // Set the error message, if any
      this.newVariableErrorMessage = errorMessage;

      // Don't show an error message if the field is blank e.g. after a user adds a variable name
      // but still prevent variable addition
      return !errorMessage && this.newVariableName !== ''; // returns false if there is an errorMessage or if the variable name is empty
    },
    validateVariableName(variableName) {
      // Duplicate variable name
      if (variableName in this.config.variables) {
        return 'Duplicate variable name found.';
      }

      // Whitespace only
      if (!variableName.trim()) {
        return 'Must have at least one non-whitespace character.';
      }

      // Curly brace detection
      if (variableName.match(/[{}]/)) {
        return 'Variable name cannot contain any of the following characters: {}';
      }
      return '';
    },
    handleAddVariable() {
      this.config.variables = {
        ...this.config.variables,
        [this.newVariableName.trim()]: {
          values: [
            {
              platform: { type: inputTypes.platform, label: 'Platform', value: '' },
              audience: { type: inputTypes.audience, label: 'Audience', value: '' },
              value: { type: inputTypes.string, label: 'Value', value: '' },
            },
          ],
        },
      };

      this.newVariableName = '';
    },
    handleDeleteVariable(variableName) {
      delete this.config.variables[variableName];
    },
    showVariableNameEditor(variableName) {
      this.isEditingVariableName[variableName] = {
        name: variableName,
        errorMessage: '',
        touched: false,
      };
    },
    /**
     * Save the variable under its new name.
     * This just updates the in-memory config, and doesn't save it to the back-end.
      */
    saveEditVariable(variableName) {
      // Leading & trailing whitespace should be removed.
      variableName = variableName.trim();

      // Copy the current variable data to the new variable.
      const copyData = this.config.variables[variableName];
      this.config.variables[this.isEditingVariableName[variableName].name] = copyData;

      // Delete the current variable name from config.
      this.handleDeleteVariable(variableName);

      delete this.isEditingVariableName[variableName];
    },
    cancelEditVariable(variableName) {
      delete this.isEditingVariableName[variableName];
    },
    /**
     * Adds a blank config for the specified variable
     * @param variableName the variable name to delete a config from
     */
    handleAddVariableConfig(variableName) {
      this.config.variables[variableName].values.push({
        platform: { type: inputTypes.platform, label: 'Platform', value: '' },
        audience: { type: inputTypes.audience, label: 'Audience', value: '' },
        value: { type: inputTypes.string, label: 'Value', value: '' },
      });
    },
    /**
     * Deletes the specified config for the specified variable
     * @param variableName the variable name to delete a config from
     * @param index the index of the config to delete
     */
    handleDeleteVariableConfig(variableName, index) {
      this.config.variables[variableName].values.splice(index, 1);
    },
    canSave() {
      return this.isDirty;
    },
    /**
     * Returns the form values as an object ready to be exported to the backend config
     */
    exportConfig() {
      const configKeys = Object.keys(this.config);
      const configToSave = {};

      configKeys.forEach((key) => {
        if (key === 'mqtt') {
          const { broker, clientID, topic_prefix, username, password } = this.config.mqtt.client;

          configToSave.mqtt = {
            client: {
              broker: broker.value,
              clientID: clientID.value,
              topic_prefix: topic_prefix.value,
              username: username.value,
              password: password.value,
            },
          };
        } else if (key === 'shaman') {
          const { period, maxAge } = this.config.shaman.garbageCollect;
          const { enabled } = this.config.shaman;

          configToSave.shaman = {
            enabled: enabled.value,
            garbageCollect: {
              // empty strings are invalid durations, so set it to 0s if empty
              // this is only an issue when shaman is disabled, otherwise the required attribute prevents empty strings
              period: period.value ?? '0s',
              maxAge: maxAge.value ?? '0s',
            },
          };
        } else if (key === 'variables') {
          configToSave.variables = {};

          // This needs to be dynamic, as variable names and the amount of entries for each are not fixed
          Object.keys(this.config.variables).forEach((variable) => {
            // Initialize the values list for each variable
            configToSave.variables[variable] = { values: [] };
            this.config[key][variable].values.forEach((entry, index) => {
              // Initialize an empty object for each entry of a variable
              configToSave.variables[variable].values.push({});
              Object.keys(entry).forEach((entryKey) => {
                // Grab the content from either platform, value, or audience
                const formValue = this.config.variables[variable].values[index][entryKey].value;
                // No need to save the content if audience is "all", since that is the default
                // Otherwise save the content
                if (entryKey === 'audience' && formValue === 'all') {
                  return;
                }
                configToSave.variables[variable].values[index][entryKey] = formValue;
              });
            });
          });
        } else if (key === '_meta') {
          // _meta is hardcoded so grab it as it is
          configToSave._meta = this.config._meta;
        } else {
          // Set the flat values
          configToSave[key] = this.config[key].value;
        }
      });

      return configToSave;
    },
    /**
     * Exports the form config and overwrites the existing flamenco-manager.yaml
     */
    async saveConfig() {
      const configToSave = this.exportConfig();

      try {
        await this.metaAPI.updateConfigurationFile(configToSave);

        // Update the original config so that isDirty reads false after a successful save
        this.originalConfig = JSON.parse(JSON.stringify(this.config));

        // Discard any existing error message on the banner
        this.submissionErrorMessage = '';

        // Hide the success banner after 3 seconds
        setTimeout(() => {
          this.showSubmissionBanner = false;
        }, 3000);
      } catch (e) {
        console.error(e);
        // Pass on the error message to the banner
        const errorMessage = e.body?.message ?? e.error.message;
        this.submissionErrorMessage = `Failed to save: ${errorMessage}`;
      } finally {
        // Always show the banner on submit whether successful or not
        this.showSubmissionBanner = true;
      }
    },
    /**
     * Imports the config from the backend and populates the form values
     */
    async importConfig() {
      const existingConfig = await this.getYamlConfig();

      const configKeys = Object.keys(existingConfig);
      configKeys.forEach((key) => {
        if (key === 'mqtt') {
          Object.keys(this.config.mqtt.client).forEach(
            (nestedKey) =>
              (this.config.mqtt.client[nestedKey].value = existingConfig.mqtt.client[nestedKey])
          );
        } else if (key === 'shaman') {
          this.config.shaman.enabled.value = existingConfig.shaman.enabled;

          Object.keys(this.config.shaman.garbageCollect).forEach(
            (nestedKey) =>
              (this.config.shaman.garbageCollect[nestedKey].value =
                existingConfig.shaman.garbageCollect[nestedKey])
          );
        } else if (key === 'variables') {
          // This helps with importing the variables to the form
          const blankVariableEntry = {
            platform: { value: '', type: '', label: '' },
            value: { value: '', type: '', label: '' },
            audience: { value: '', type: '', label: '' },
          };

          Object.keys(existingConfig.variables).forEach((variable) => {
            // Initialize the values list for each variable
            this.config.variables[variable] = { values: [] };
            existingConfig.variables[variable].values.forEach((entry, index) => {
              // Initialize an empty object for each entry of a variable
              this.config.variables[variable].values.push({});
              Object.keys(blankVariableEntry).forEach((entryKey) => {
                // Set the content for platform, value, and audience
                this.config.variables[variable].values[index][entryKey] = {
                  value:
                    existingConfig.variables[variable].values[index][entryKey] ??
                    (entryKey === 'audience' ? 'all' : ''), // If the audience value is blank, set it to the default 'all'
                  label: inputTypes[entryKey] ?? 'Value',
                  type: inputTypes[entryKey] ?? inputTypes.string,
                };
              });
            });
          });
        } else if (key === '_meta') {
          // Copy the _meta exactly as is
          this.config._meta = existingConfig._meta;
        } else {
          // Set the flat values
          this.config[key].value = existingConfig[key];
        }
      });

      // make a copy to use for isDirty check
      this.originalConfig = JSON.parse(JSON.stringify(this.config));
    },
    /**
     * Retrieve the config from flamenco-manager.yaml
     */
    async getYamlConfig() {
      const config = await this.metaAPI.getConfigurationFile();
      return config;
    },
    // SocketIO connection event handlers:
    // TODO: reload config if clean; if dirty, show a warning that the form may be out of date
    onSIOReconnected() {},
    onSIODisconnected(reason) {},
  },
};
</script>

<template>
  <main class="yaml-view-container">
    <nav class="nav-container">
      <div v-for="category in categories" :key="category">
        <a :href="'#' + category.id">{{ category.label }}</a>
      </div>
      <button
        type="submit"
        form="config-form"
        class="action-button margin-left-auto"
        :disabled="!canSave()">
        Save
      </button>
      <div
        class="banner"
        :class="{
          success: !this.submissionErrorMessage,
          error: this.submissionErrorMessage,
          hidden: !showSubmissionBanner,
        }">
        <svg
          @click="this.showSubmissionBanner = false"
          xmlns="http://www.w3.org/2000/svg"
          width="1.5em"
          height="1.5em"
          viewBox="0 0 24 24"
          fill="none">
          <path
            fill-rule="evenodd"
            clip-rule="evenodd"
            d="M5.29289 5.29289C5.68342 4.90237 6.31658 4.90237 6.70711 5.29289L12 10.5858L17.2929 5.29289C17.6834 4.90237 18.3166 4.90237 18.7071 5.29289C19.0976 5.68342 19.0976 6.31658 18.7071 6.70711L13.4142 12L18.7071 17.2929C19.0976 17.6834 19.0976 18.3166 18.7071 18.7071C18.3166 19.0976 17.6834 19.0976 17.2929 18.7071L12 13.4142L6.70711 18.7071C6.31658 19.0976 5.68342 19.0976 5.29289 18.7071C4.90237 18.3166 4.90237 17.6834 5.29289 17.2929L10.5858 12L5.29289 6.70711C4.90237 6.31658 4.90237 5.68342 5.29289 5.29289Z"
            fill="#fff" />
        </svg>
        <p>
          {{
            this.submissionErrorMessage
              ? this.submissionErrorMessage
              : 'Settings successfully saved.'
          }}
        </p>
      </div>
    </nav>
    <aside class="side-container">
      <div class="dialog">
        <div class="flex-col gap-col-spacer">
          <div class="flex-col">
            <p class="text-color-hint">
              This editor allows you to configure the settings for the Flamenco Server. These
              changes will directly edit the
              <span class="file-name"> flamenco-manager.yaml </span>
              file. For more information, see
              <a class="link" href="https://flamenco.blender.org/usage/manager-configuration/">
                Manager Configuration</a
              >
            </p>
          </div>
          <div class="flex-col gap-text-spacer">
            <h3>{{ focusedSetting.label }}</h3>
            <p>{{ focusedSetting.description }}</p>
          </div>
          <button
            title="Restore form to match the settings on flamenco-manager.yaml"
            class="action-button margin-top-auto"
            @click="undoEdits()"
            :disabled="!isDirty">
            Undo Edits
          </button>
        </div>
      </div>
    </aside>
    <form id="config-form" class="form-container" @submit.prevent="saveConfig">
      <h1 id="flamenco-manager-setup">Flamenco Manager Setup</h1>
      <template v-for="category in categories" :key="category">
        <h2 :id="category.id">{{ category.label }}</h2>
        <!-- Variables -->
        <template v-if="category.id === 'variables'">
          <div class="form-col">
            <div class="form-row gap-text-spacer">
              <input
                @input="addVariableOnInput"
                @keydown.enter.prevent="canAddVariable() ? handleAddVariable() : null"
                placeholder="variableName"
                type="text"
                :id="newVariableName"
                v-model="newVariableName" />
              <button
                type="button"
                title="Enter a variable"
                @click="handleAddVariable"
                :disabled="!canAddVariable()">
                Add Variable
              </button>
            </div>
            <span
              :class="[
                'error',
                {
                  hidden: !newVariableErrorMessage || !newVariableTouched,
                },
              ]"
              >{{ newVariableErrorMessage }}
            </span>
          </div>
          <section
            class="form-variable-section"
            v-for="variableName in sortedVariableNames"
            :key="variableName">
            {{ void(variable = config.variables[variableName]) }}
            <div
              :class="[
                {
                  'form-variable-header': !isEditingVariableName[variableName],
                  'form-variable-header-edit': isEditingVariableName[variableName],
                },
              ]">
              <template v-if="!isEditingVariableName[variableName]">
                <h3>
                  <pre>{{ variableName }}</pre>
                  <button
                    type="button"
                    class="edit-button"
                    @click="showVariableNameEditor(variableName)">
                    <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                      <g id="SVGRepo_bgCarrier" stroke-width="0" />
                      <g
                        id="SVGRepo_tracerCarrier"
                        stroke-linecap="round"
                        stroke-linejoin="round" />
                      <g id="SVGRepo_iconCarrier">
                        <path
                          d="M12 5H9C7.11438 5 6.17157 5 5.58579 5.58579C5 6.17157 5 7.11438 5 9V15C5 16.8856 5 17.8284 5.58579 18.4142C6.17157 19 7.11438 19 9 19H15C16.8856 19 17.8284 19 18.4142 18.4142C19 17.8284 19 16.8856 19 15V12M9.31899 12.6911L15.2486 6.82803C15.7216 6.36041 16.4744 6.33462 16.9782 6.76876C17.5331 7.24688 17.5723 8.09299 17.064 8.62034L11.2329 14.6702L9 15L9.31899 12.6911Z"
                          stroke-linecap="round"
                          stroke-linejoin="round" />
                      </g>
                    </svg>
                  </button>
                </h3>
              </template>
              <template v-else>
                <div class="form-col">
                  <div class="form-row gap-text-spacer">
                    <input
                      @input="editVariableOnInput(variableName)"
                      @keydown.enter.prevent="
                        canEditVariable(variableName) ? saveEditVariable(variableName) : null
                      "
                      placeholder="variableName"
                      type="text"
                      :id="'edit_'+variableName"
                      v-model="isEditingVariableName[variableName].name" />
                    <div class="form-row gap-text-spacer">
                      <button
                        type="button"
                        :disabled="!canEditVariable(variableName)"
                        @click="saveEditVariable(variableName)">
                        Save
                      </button>
                      <button type="button" @click="cancelEditVariable(variableName)">
                        Cancel
                      </button>
                    </div>
                  </div>
                  <span
                    :class="[
                      'error',
                      {
                        hidden: !isEditingVariableName[variableName].errorMessage,
                      },
                    ]"
                    >{{ isEditingVariableName[variableName].errorMessage }}
                  </span>
                </div>
              </template>
              <button
                type="button"
                class="delete-button"
                @click="handleDeleteVariable(variableName)">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 25 25">
                  <g id="trash">
                    <path
                      class="trash"
                      d="M20.5 4h-3.64l-.69-2.06a1.37 1.37 0 0 0-1.3-.94h-4.74a1.37 1.37 0 0 0-1.3.94L8.14 4H4.5a.5.5 0 0 0 0 1h.34l1 17.59A1.45 1.45 0 0 0 7.2 24h10.6a1.45 1.45 0 0 0 1.41-1.41L20.16 5h.34a.5.5 0 0 0 0-1zM9.77 2.26a.38.38 0 0 1 .36-.26h4.74a.38.38 0 0 1 .36.26L15.81 4H9.19zm8.44 20.27a.45.45 0 0 1-.41.47H7.2a.45.45 0 0 1-.41-.47L5.84 5h13.32z" />
                    <path
                      class="trash"
                      d="M9.5 10a.5.5 0 0 0-.5.5v7a.5.5 0 0 0 1 0v-7a.5.5 0 0 0-.5-.5zM12.5 9a.5.5 0 0 0-.5.5v9a.5.5 0 0 0 1 0v-9a.5.5 0 0 0-.5-.5zM15.5 10a.5.5 0 0 0-.5.5v7a.5.5 0 0 0 1 0v-7a.5.5 0 0 0-.5-.5z" />
                  </g>
                </svg>
              </button>
            </div>
            <div class="form-variable-row" v-for="(entry, index) in variable.values" :key="index">
              <FormInputText
                :id="variableName + '[' + index + ']' + '.value'"
                v-model:value="entry.value.value"
                :label="index === 0 ? entry.value.label : ''"
                @focus="handleFocusVariableValue" />
              <FormInputDropdownSelect
                required
                :label="index === 0 ? entry.platform.label : ''"
                :options="platformOptions"
                v-model="entry.platform.value"
                :id="variableName + index + '.platform'"
                @focus="handleFocusVariablePlatform" />
              <FormInputDropdownSelect
                required
                strict
                :label="index === 0 ? entry.audience.label : ''"
                :options="audienceOptions"
                v-model="entry.audience.value"
                :id="variableName + index + '.audience'"
                @focus="handleFocusVariableAudience" />
              <button
                type="button"
                class="delete-button with-error-message"
                :class="['delete-button', { 'margin-top': index === 0 }]"
                @click="handleDeleteVariableConfig(variableName, index)">
                -
              </button>
            </div>
            <button type="button" class="add-button" @click="handleAddVariableConfig(variableName)">
              +
            </button>
          </section>
        </template>
        <!-- Render all other sections dynamically -->
        <template v-else>
          <section class="form-section">
            <template v-for="key in category.settings" :key="key">
              <!-- Shaman -->
              <template v-if="key === 'shaman'">
                <h3>Shaman Storage</h3>
                <template v-for="(shamanSetting, key) in config.shaman" :key="key">
                  <template v-if="shamanSetting.type === inputTypes.boolean">
                    <FormInputSwitchCheckbox
                      :label="shamanSetting.label"
                      v-model="shamanSetting.value"
                      :description="shamanSetting.description"
                      :moreInfoText="shamanSetting.moreInfoText"
                      :moreInfoLinkUrl="shamanSetting.moreInfoLinkUrl"
                      :moreInfoLinkLabel="shamanSetting.moreInfoLinkLabel" />
                  </template>
                  <!-- Shaman Garbage Collect -->
                  <template v-else-if="key === 'garbageCollect'">
                    <label :class="{ disabled: !this.config.shaman.enabled.value }">
                      Garbage Collection Settings
                    </label>
                    <template
                      v-for="(garbageCollectSetting, garbageCollectKey) in shamanSetting"
                      :key="'garbageCollect' + garbageCollectKey">
                      <template v-if="garbageCollectSetting.type === inputTypes.timeDuration">
                        <FormInputDropdownSelect
                          @focus="handleFocus"
                          strict
                          :required="config.shaman.garbageCollect[garbageCollectKey].required"
                          :label="garbageCollectSetting.label"
                          :disabled="!config.shaman.enabled.value"
                          :options="timeDurationOptions"
                          v-model="garbageCollectSetting.value"
                          :id="'shaman.garbageCollect.' + garbageCollectKey" />
                      </template>
                    </template>
                  </template>
                </template>
              </template>
              <!-- MQTT -->
              <template v-else-if="key === 'mqtt'">
                <template v-for="(mqttSetting, mqttKey) in config.mqtt" :key="mqttKey">
                  <template v-if="mqttSetting.type === inputTypes.boolean">
                    <FormInputSwitchCheckbox
                      :label="mqttSetting.label"
                      v-model="mqttSetting.value"
                      :description="mqttSetting.description"
                      :moreInfoText="mqttSetting.moreInfoText"
                      :moreInfoLinkUrl="mqttSetting.moreInfoLinkUrl"
                      :moreInfoLinkLabel="mqttSetting.moreInfoLinkLabel" />
                  </template>
                  <!-- MQTT Client -->
                  <template
                    v-else-if="mqttKey === 'client'"
                    v-for="(clientSetting, clientKey) in config.mqtt.client"
                    :key="clientKey">
                    <template v-if="clientSetting.type === inputTypes.string">
                      <FormInputText
                        :required="config.mqtt.client[clientKey].required"
                        :disabled="!config.mqtt.enabled.value"
                        :id="'mqtt.client.' + clientKey"
                        v-model:value="clientSetting.value"
                        :label="clientSetting.label"
                        @focus="handleFocus" />
                    </template>
                  </template>
                </template>
              </template>
              <!-- Render all other input types dynamically -->
              <template v-else-if="config[key].type === inputTypes.string">
                <FormInputText
                  @focus="handleFocus"
                  :required="config[key].required"
                  :id="key"
                  v-model:value="config[key].value"
                  :label="config[key].label" />
              </template>
              <template v-else-if="config[key].type === inputTypes.boolean">
                <FormInputSwitchCheckbox
                  :label="config[key].label"
                  v-model="config[key].value"
                  :description="config[key].description" />
              </template>
              <template v-if="config[key].type === inputTypes.number">
                <FormInputNumber
                  @focus="handleFocus"
                  :required="config[key].required"
                  :label="config[key].label"
                  :min="0"
                  v-model:value="config[key].value"
                  :id="key" />
              </template>
              <template v-else-if="config[key].type === inputTypes.timeDuration">
                <FormInputDropdownSelect
                  @focus="handleFocus"
                  :required="config[key].required"
                  :label="config[key].label"
                  :options="timeDurationOptions"
                  v-model="config[key].value"
                  :id="key" />
              </template>
            </template>
          </section>
        </template>
      </template>
    </form>
  </main>

  <footer class="app-footer">
    <notification-bar />
    <update-listener
      ref="updateListener"
      mainSubscription=""
      @sioReconnected="onSIOReconnected"
      @sioDisconnected="onSIODisconnected" />
  </footer>
</template>

<style>
.banner {
  position: absolute;
  top: 0;
  min-width: 200px;
  min-height: var(--nav-height);
  background-color: var(--color-background-column);
  border-radius: var(--border-radius);
  padding: var(--nav-padding);
  max-width: 100%;

  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
}

.banner svg {
  flex-shrink: 0;
  cursor: pointer;
}

.banner p {
  text-align: center;
  padding: 0px var(--container-padding);
}

.banner.success {
  border: 1px solid var(--color-success);
  background-color: #325328;
}
.banner.error {
  border: 1px solid var(--color-status-failed);
  background-color: #551719;
}

.yaml-view-container {
  --nav-height: 35px;
  --nav-padding: 2px 10px;
  --button-height: 35px;
  --delete-button-width: 35px;

  --min-form-area-width: 600px;
  --max-form-area-width: 1fr;
  --min-side-area-width: 300px;
  --max-side-area-width: 425px;
  --max-form-width: 650px;
  --form-padding: 75px;
  --side-padding: 25px;

  --container-margin: 25px;
  --row-item-spacer: 25px;
  --column-item-spacer: 25px;
  --section-spacer: 25px;
  --container-padding: 25px;
  --text-spacer: 8px;

  grid-column-start: col-1;
  grid-column-end: col-3;

  display: grid;
  grid-gap: var(--grid-gap);
  grid-template-areas:
    'header header'
    'side main'
    'footer footer';
  grid-template-columns: minmax(var(--min-side-area-width), var(--max-side-area-width)) minmax(
      var(--min-form-area-width),
      var(--max-form-area-width)
    );
  grid-template-rows: var(--nav-height) 1fr;
}

.hidden {
  display: none;
}

.error {
  color: var(--color-status-failed);
}

.file-name {
  font-style: italic;
}
.link {
  text-decoration: underline;
}
#core-settings,
#storage,
#timeout-failures,
#mqtt,
#variables {
  scroll-margin-top: calc(var(--section-spacer) * 2);
}
#core-settings:target,
#storage:target,
#timeout-failures:target,
#mqtt:target,
#variables:target {
  color: var(--color-accent-text);
}

.error {
  color: var(--color-status-failed);
}

button.edit-button {
  stroke: white;
  background-color: var(--color-background-column);
  outline: none;
  border: none;
  width: 30px;
  padding: 0;
}

button.delete-button {
  border: var(--color-danger) 1px solid;
  color: var(--color-danger);
  background-color: var(--color-background-column);
  width: var(--delete-button-width);
  height: var(--delete-button-width);
  flex-shrink: 0;
}

button.delete-button .trash {
  fill: var(--color-danger);
  width: 25px;
  height: 25px;
}

button.delete-button.margin-top {
  /* This is calculated by subtracting the button height from the form row height,
  aligning it properly with the inputs */
  margin-top: 25px;
}

button.add-button {
  border: var(--color-success) 1px solid;
  color: var(--color-success);
  background-color: var(--color-background-column);
}

button.edit-button:hover {
  stroke: var(--color-accent);
}

button.delete-button:hover,
button.delete-button:hover .trash,
button.add-button:hover {
  fill: var(--color-accent);
  color: var(--color-accent);
  border: 1px solid var(--color-accent);
}

.margin-left-auto {
  margin-left: auto;
}

.margin-top-auto {
  margin-top: auto;
}

button.action-button {
  background-color: var(--color-accent-background);
  color: var(--color-accent-text);
  padding: 5px 64px;
  border-radius: var(--border-radius);
  border: var(--border-width) solid var(--color-accent);
}
button.action-button:hover {
  background-color: var(--color-accent);
}
button.action-button:active {
  color: var(--color-accent);
  background-color: var(--color-accent-background);
}

p {
  line-height: 1.5;
  margin: 0;
  white-space: pre-line;
  color: var(--color-text);
}
.text-color-hint {
  color: var(--color-text-hint);
}

button {
  height: var(--button-height);
}

.nav-container {
  position: sticky;
  top: 0;
  height: var(--nav-height);
  grid-area: header;
  gap: var(--row-item-spacer);
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  background-color: var(--color-background-column);
  padding: var(--nav-padding);
  z-index: 10;
  border-radius: var(--border-radius);
}

.side-container {
  grid-area: side;
  margin: var(--container-margin);
}
.dialog {
  background-color: var(--color-background-column);
  border-radius: var(--border-radius);
  min-height: calc(100vh - var(--nav-height) - var(--header-height) - var(--footer-height) - 100px);
  position: sticky;
  top: 70px;
  padding: var(--side-padding);
  display: flex;
}
.flex-col {
  display: flex;
  flex-direction: column;
}
.gap-text-spacer {
  gap: var(--text-spacer);
}
.gap-col-spacer {
  gap: var(--column-item-spacer);
}

.form-container {
  display: flex;
  flex-direction: column;
  align-items: start;
  grid-area: main;
  margin: var(--container-margin) var(--container-margin) var(--container-margin) 0px;
  max-width: var(--max-form-width);

  background-color: var(--color-background-column);
  border-radius: var(--border-radius);
  padding: calc(var(--form-padding) - var(--section-spacer)) var(--form-padding);
}

h2 {
  margin: var(--section-spacer) 0 var(--section-spacer) 0;
}

h3 {
  margin: var(--section-spacer) 0 0 0;
}

.form-section {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: var(--max-form-width);
  gap: var(--column-item-spacer);
  margin-bottom: 50px;
}

.form-col {
  display: flex;
  align-items: start;
  flex-direction: column;
  gap: var(--text-spacer);
  width: 100%;
}

.form-row {
  display: flex;
  width: 100%;
}

.form-variable-section {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: var(--max-form-width);
  margin-bottom: var(--section-spacer);
}

.form-variable-row {
  display: grid;
  grid-template-columns: 1fr minmax(0, max-content) minmax(0, max-content) var(
      --delete-button-width
    );
  grid-template-areas: 'value platform audience button';
  align-items: start;
  justify-items: center;
  margin-bottom: 15px;
  column-gap: var(--row-item-spacer);
  width: 100%;
}

.form-variable-col {
  display: flex;
  align-items: start;
  flex-direction: column;
  gap: var(--text-spacer);
}

.form-variable-header {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  margin: var(--section-spacer) 0 5px 0;
}

.form-variable-header-edit {
  display: flex;
  flex-direction: row;
  align-items: start;
  justify-content: space-between;
  width: 100%;
  margin: var(--section-spacer) 0 22px 0;
}

.form-variable-header h3 {
  margin: 0;
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 4px;
}

input {
  height: var(--input-height);
}

input:disabled {
  background-color: var(--color-background-column);
}
</style>
