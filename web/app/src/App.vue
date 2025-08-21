<template>
  <header class="mainmenu">
    <router-link :to="{ name: 'index' }" class="navbar-brand">{{ flamencoName }}</router-link>
    <nav>
      <ul>
        <li>
          <router-link :to="{ name: 'jobs' }">Jobs</router-link>
        </li>
        <li>
          <router-link :to="{ name: 'workers' }">Workers</router-link>
        </li>
        <li>
          <router-link :to="{ name: 'tags' }">Tags</router-link>
        </li>
        <li>
          <router-link :to="{ name: 'last-rendered' }">Last Rendered</router-link>
        </li>
        <li>
          <router-link :to="{ name: 'settings' }">Settings</router-link>
        </li>
      </ul>
    </nav>
  </header>
  <header class="farmstatus">
    <farm-status :status="farmStatus.status()" />
  </header>
  <header class="links">
    <api-spinner />
    <span class="app-version">
      <a :href="backendURL('/flamenco-addon.zip')">add-on</a>
      | <a :href="backendURL('/api/v3/swagger-ui/')">API</a> | version: {{ flamencoVersion }}
    </span>
  </header>
  <router-view></router-view>
</template>

<script>
import * as API from '@/manager-api';
import { getAPIClient } from '@/api-client';
import { backendURL } from '@/urls';
import { useSocketStatus } from '@/stores/socket-status';
import { useFarmStatus } from '@/stores/farmstatus';

import ApiSpinner from '@/components/ApiSpinner.vue';
import FarmStatus from '@/components/FarmStatus.vue';

const DEFAULT_FLAMENCO_NAME = 'Flamenco';
const DEFAULT_FLAMENCO_VERSION = 'unknown';

export default {
  name: 'App',
  components: {
    ApiSpinner,
    FarmStatus,
  },
  data: () => ({
    flamencoName: DEFAULT_FLAMENCO_NAME,
    flamencoVersion: DEFAULT_FLAMENCO_VERSION,
    backendURL: backendURL,
    farmStatus: useFarmStatus(),
  }),
  mounted() {
    window.app = this;
    this.fetchManagerInfo();
    this.fetchFarmStatus();

    const sockStatus = useSocketStatus();
    this.$watch(
      () => sockStatus.isConnected,
      (isConnected) => {
        if (!isConnected) return;
        if (!sockStatus.wasEverDisconnected) return;
        this.socketIOReconnect();
      }
    );
  },
  methods: {
    fetchManagerInfo() {
      const metaAPI = new API.MetaApi(getAPIClient());
      metaAPI.getVersion().then((version) => {
        this.flamencoName = version.name;
        this.flamencoVersion = version.version;
        document.title = version.name;
      });
    },

    fetchFarmStatus() {
      const metaAPI = new API.MetaApi(getAPIClient());
      metaAPI.getFarmStatus().then((statusReport) => {
        const apiStatusReport = API.FarmStatusReport.constructFromObject(statusReport);
        this.farmStatus.lastStatusReport = apiStatusReport;
      });
    },

    socketIOReconnect() {
      const metaAPI = new API.MetaApi(getAPIClient());
      metaAPI.getVersion().then((version) => {
        if (version.name === this.flamencoName && version.version == this.flamencoVersion) return;
        console.log(`Updated from ${this.flamencoVersion} to ${version.version}`);
        location.reload();
      });
    },
  },
};
</script>

<style>
@import 'assets/base.css';
@import 'assets/tabulator.css';
</style>
