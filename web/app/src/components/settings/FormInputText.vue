<template>
  <div
    :class="{
      hidden: hidden,
      'form-col': !hidden,
    }">
    <label v-if="label" :for="id">{{ label }}</label>
    <input
      :placeholder="placeholder"
      :required="required"
      type="text"
      :disabled="disabled"
      :id="id"
      :value="value"
      @input="onInput"
      @change="onChange" />
    <span :class="{ hidden: !errorMsg, error: errorMsg }">{{ errorMsg }}</span>
  </div>
</template>

<script>
export default {
  name: 'FormInputText',
  props: {
    label: {
      type: String,
      required: true,
    },
    value: {
      type: String,
      required: true,
    },
    id: {
      type: String,
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
    placeholder: {
      type: String,
      required: false,
    },
    hidden: {
      type: Boolean,
      required: false,
    },
  },
  emits: ['update:value'],
  data() {
    return {
      errorMsg: '',
    };
  },
  watch: {},
  methods: {
    onInput(event) {
      // Update the v-model value
      this.$emit('update:value', event.target.value);
    },
    onChange(event) {
      // Supports .lazy
      // Can add validation here
      if (event.target.value === '' && this.required) {
        this.errorMsg = 'This field is required.';
      } else {
        this.errorMsg = '';
      }
    },
  },
};
</script>
