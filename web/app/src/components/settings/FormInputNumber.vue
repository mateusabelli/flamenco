<template>
  <div class="form-col">
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
      @focus="$emit('focus', id)" />
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
  emits: ['update:value, focus'],
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
  watch: {
    value() {
      this.validateInput();
    },
  },
  methods: {
    onInput(event) {
      // Update the v-model value
      this.$emit('update:value', Number(event.target.value));
    },
    validateInput() {
      if (this.value === '' && this.required) {
        this.errorMsg = 'This field is required.';
      } else {
        this.errorMsg = '';
      }

      if (this.value < this.min) {
        this.errorMsg = `The value cannot be below ${this.min}`;
      }
      if (this.value > this.max) {
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
