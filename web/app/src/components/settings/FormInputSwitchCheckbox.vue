<template>
  <div class="form-row">
    <label class="form-switch-row">
      <span>{{ label }}</span>
      <span class="switch">
        <input v-model="model" :value="value" :name="name" type="checkbox" />
        <span class="slider round"></span>
      </span>
    </label>
    <p>
      {{ description }}
      <template v-if="moreInfoText">
        {{ moreInfoText }}
        <a v-if="moreInfoLinkLabel && moreInfoLinkUrl" class="link" :href="moreInfoLinkUrl"
          >{{ moreInfoLinkLabel }}
        </a>
        <span>{{ `.` }}</span>
      </template>
    </p>
  </div>
</template>

<script>
export default {
  name: 'FormInputSwitchCheckbox',
  props: {
    label: {
      type: String,
      required: true,
    },
    modelValue: {
      type: [Array, Boolean],
      required: true,
    },
    name: {
      type: String,
      required: false,
    },
    value: {
      type: [Boolean, Object],
      required: false,
    },
    description: {
      type: String,
      required: false,
    },
    moreInfoText: {
      type: String,
      required: false,
    },
    moreInfoLinkUrl: {
      type: String,
      required: false,
    },
    moreInfoLinkLabel: {
      type: String,
      required: false,
    },
  },
  emits: ['update:modelValue'],
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
};
</script>

<style scoped>
.form-switch-row {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

/* The switch - the box around the slider */
.switch {
  position: relative;
  display: inline-block;
  width: 52px;
  height: 30px;
}

/* Hide default HTML checkbox */
.switch input[type='checkbox'] {
  opacity: 0;
  width: 0;
  height: 0;
}

/* The slider */
.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--color-text-muted);
  border: var(--border-width) solid var(--border-color);
  -webkit-transition: 0.4s;
  transition: 0.4s;
}

.slider:before {
  position: absolute;
  content: '';
  height: 20px;
  width: 20px;
  left: 4px;
  bottom: 3px;
  background-color: white;
  -webkit-transition: 0.4s;
  transition: 0.4s;
}

input:checked + .slider {
  background-color: var(--color-accent);
}

input:focus + .slider {
  box-shadow: 0 0 1px var(--color-accent);
  border: var(--border-width) solid white;
}

input:checked + .slider:before {
  -webkit-transform: translateX(20px);
  -ms-transform: translateX(20px);
  transform: translateX(20px);
}

/* Rounded sliders */
.slider.round {
  border-radius: 34px;
}

.slider.round:before {
  border-radius: 50%;
}
</style>
