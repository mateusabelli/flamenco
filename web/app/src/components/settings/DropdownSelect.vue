<style scoped>
select {
  height: var(--input-height);
}
</style>

<script>
export default {
  props: ['modelValue', 'id', 'disabled', 'options'],
  emits: ['update:modelValue'],
};
</script>

<template>
  <select
    class="time"
    :id="id"
    :value="modelValue"
    @change="$emit('update:modelValue', $event.target.value)"
    :disabled="disabled">
    <!-- This option only shows if its current value does not match any of the preset options. -->
    <option v-if="!(modelValue in options)" :value="modelValue" selected>
      {{ modelValue }}
    </option>
    <template :key="o" v-for="o in Object.keys(options)">
      <option :value="o">{{ options[o] }}</option>
    </template>
  </select>
</template>
