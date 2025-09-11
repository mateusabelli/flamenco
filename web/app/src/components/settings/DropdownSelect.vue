<style scoped>
select {
  height: var(--input-height);
}
</style>

<script>
export default {
  data() {
    return {
      errorMsg: '',
    };
  },
  props: {
    modelValue: {
      type: String,
      required: true,
    },
    id: {
      type: String,
      required: true,
    },
    // options is a k,v map where
    // k is the value to be saved in modelValue and
    // v is the label to be rendered to the user
    options: {
      type: Object,
      required: true,
    },
    disabled: {
      type: Boolean,
      required: false,
    },
    required: {
      type: Boolean,
      required: false,
    },
    // Input validation to ensure the value matches one of the options
    strict: {
      type: Boolean,
      required: false,
    },
  },
  emits: ['update:modelValue, focus'],
  methods: {
    onChange(event) {
      // Update the value from the parent component
      this.$emit('update:modelValue', event.target.value);
    },
  },
};
</script>

<template>
  <select
    :required="required"
    :id="id"
    :value="modelValue"
    @change="onChange"
    @focus="$emit('focus', id)"
    :disabled="disabled">
    <!-- The default to show and select if modelValue is a non-option and either an empty string, null, or undefined -->
    <option
      :value="''"
      :selected="
        !(modelValue in options) &&
        (modelValue === '' || modelValue === null || modelValue === undefined)
      ">
      {{ 'Select an option' }}
    </option>
    <!-- Show the non-option value if it is not an empty string, null, or undefined; disable it if strict is enabled -->
    <option
      v-if="
        !(modelValue in options) &&
        modelValue !== '' &&
        modelValue !== null &&
        modelValue !== undefined
      "
      :disabled="!(modelValue in options) && strict"
      :value="modelValue"
      :selected="!(modelValue in options) && !strict">
      {{ modelValue }}
    </option>
    <template :key="o" v-for="o in Object.keys(options)">
      <option :value="o">{{ options[o] }}</option>
    </template>
  </select>
</template>
