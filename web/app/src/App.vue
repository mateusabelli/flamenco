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
      </ul>
    </nav>
  </header>
  <header class="farmstatus">
    <farm-status :status="farmStatus.status()" />
  </header>
  <header class="links">
    <api-spinner />
    <span class="header-links-right">
      <router-link :to="{ name: 'settings' }" title="Configure Flamenco">
        <svg class="settings-cog" version="1.0" viewBox="0 0 1280 1280" xmlns="http://www.w3.org/2000/svg">
          <path d="m610 0.6c-42 9.1-110-6-128 44-0.32 23-0.63 47-0.95 70-36 5.1-76 41-107 37-31-40-81-74-121-21-59 48-116 102-153 169-14 42 59 57 50 91-26 24-15 94-55 92-28 3.4-65-10-80 22-21 89-21 184 0 272 18 35 60 17 90 22 27 16 26 72 49 100-17 30-80 60-44 102 51 70 112 137 188 179 42 14 57-59 91-50 24 26 94 15 92 55 3.4 28-10 65 22 80 89 21 184 21 272 0 35-18 17-60 22-90 16-27 72-26 100-49 30 17 60 80 102 45 70-51 137-112 179-188 14-42-59-57-50-91 26-24 15-94 55-92 28-3.4 65 10 80-22 21-89 21-184 0-272-18-35-60-17-90-22-27-16-26-72-49-100 17-30 80-60 44-102-51-70-112-137-188-179-42-14-57 59-91 50-24-26-94-15-92-55-3.4-28 10-65-22-80-54-14-111-16-166-15zm62 378c140 14 250 154 230 294-14 140-154 250-294 230-140-14-250-154-230-294 15-141 153-249 294-230z"/>
        </svg>
      </router-link>
      | <a :href="backendURL('/flamenco-addon.zip')" title="Download the Blender add-on">add-on</a>
      | <a :href="backendURL('/api/v3/swagger-ui/')" title="Explore Flamenco's API">API</a>
      | version: {{ flamencoVersion }}
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
