import { computed, defineComponent, h, inject, type Component } from 'vue'
import { ElInput, ElInputNumber, ElSelect, formItemContextKey } from 'element-plus'

function outlinedControl(control: Component, kind: string) {
  return defineComponent({
    name: `Outlined${kind}`,
    inheritAttrs: false,
    props: {
      fieldLabel: { type: String, default: '' },
    },
    setup(props, { attrs, slots }) {
      const formItem = inject(formItemContextKey, undefined)
      const label = computed(() => props.fieldLabel || formItem?.label || String(attrs['aria-label'] || attrs.placeholder || ''))

      return () => h('div', {
        class: ['outlined-field', `outlined-field--${kind}`, attrs.class],
        style: attrs.style,
        'data-form-label': Boolean(formItem?.label) || undefined,
      }, [
        h('fieldset', { class: 'outlined-field__border', 'aria-hidden': 'true' }, [
          label.value ? h('legend', { title: label.value }, label.value) : null,
        ]),
        h(control, {
          ...attrs,
          class: undefined,
          style: undefined,
          'aria-label': attrs['aria-label'] || label.value || undefined,
        }, slots),
      ])
    },
  })
}

export const OutlinedInput = outlinedControl(ElInput, 'input')
export const OutlinedSelect = outlinedControl(ElSelect, 'select')
export const OutlinedInputNumber = outlinedControl(ElInputNumber, 'number')
