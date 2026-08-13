<template>
  <switch-checkbox
    :isChecked="enabled"
    @switch-toggle="toggleWakeLock"
    label="wake lock"
    title="Prevent the screen from going to sleep" />
  <video class="wakelock-video" ref="wakeLockVideo" width="16" height="16" loop>
    <source src="/one-second-video.webm" type="video/webm" />
  </video>
</template>

<script>
import SwitchCheckbox from './SwitchCheckbox.vue';

export default {
  components: {
    SwitchCheckbox,
  },
  data: () => ({
    enabled: false,
  }),
  methods: {
    toggleWakeLock() {
      this.enabled = !this.enabled;
      if (this.enabled) {
        this.$refs.wakeLockVideo.play();
      } else {
        this.$refs.wakeLockVideo.pause();
      }
    },
  },
};
</script>

<style>
.wakelock-video {
  position: absolute;
  top: 0;
  right: 0;
  pointer-events: none;
}
</style>
