<template>
  <div class="form-row">
    <label :for="id">{{ label }}</label>
    <input
      :required="required"
      type="number"
      :disabled="disabled"
      :id="id"
      :value="value"
      :min="min"
      :max="max"
      @input="onInput"
      @change="onChange" />
    <span :class="{ hidden: !errorMsg, error: errorMsg }">{{ errorMsg }}</span>
  </div>
</template>

<script>
export default {
  name: 'FormInputNumber',
  props: {
    label: {
      type: String,
      required: true,
    },
    value: {
      type: Number,
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
    min: {
      type: Number,
      required: false,
    },
    max: {
      type: Number,
      required: false,
    },
  },
  emits: ['update:value'],
  data() {
    return {
      errorMsg: '',
    };
  },
  computed: {
    name() {
      return this.label.toLowerCase();
    },
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
        this.errorMsg = 'Field required.';
      } else {
        this.errorMsg = '';
      }

      if (event.target.value < this.min) {
        this.errorMsg = `The value cannot be below ${this.min}`;
      }
      if (event.target.value > this.max) {
        this.errorMsg = `The value cannot be above ${this.max}`;
      }
    },
  },
};
</script>

<style scoped>
input[type='number'] {
  max-width: 75px;
}
</style>
