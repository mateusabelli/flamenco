<template>
  <div class="form-col">
    <label v-if="label" :for="id">{{ label }}</label>
    <DropdownSelect
      :required="required"
      :strict="strict"
      :disabled="disabled"
      :options="options"
      v-model="model"
      @focus="$emit('focus', id)"
      @change="onChange"
      :id="id" />
    <span :class="{ hidden: !errorMsg, error: errorMsg }">{{ errorMsg }}</span>
  </div>
</template>

<script>
import DropdownSelect from '@/components/settings/DropdownSelect.vue';
export default {
  name: 'FormInputDropdownSelect',
  components: {
    DropdownSelect,
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
    label: {
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
  data() {
    return {
      errorMsg: '',
    };
  },
  computed: {
    model: {
      get() {
        return this.modelValue;
      },
      set(value) {
        this.$emit('update:modelValue', value);
      },
    },
  },
  watch: {
    modelValue() {
      // If the value gets populated after component creation, check for strictness again
      this.enforceStrict();
    },
  },
  created() {
    // Check for strictness upon component creation
    this.enforceStrict();
  },
  methods: {
    enforceStrict() {
      // If strict is enabled and the current selection is not in the provided options, print an error message.
      if (
        this.strict &&
        !(this.modelValue in this.options) &&
        this.modelValue !== '' &&
        this.modelValue !== null &&
        this.modelValue !== undefined
      ) {
        this.errorMsg = 'Invalid option.';
      }
    },
    onChange(event) {
      // If required is enabled, and the value is empty, print the error message
      if (event.target.value === '' && this.required) {
        this.errorMsg = 'Selection required.';
      } else {
        this.errorMsg = '';
      }

      // Update the value from the parent component
      this.$emit('update:modelValue', event.target.value);
    },
  },
};
</script>
