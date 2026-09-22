import { computed, defineComponent, h, inject, type Component, type InjectionKey } from 'vue'
import { ElInput, ElInputNumber, ElSelect, formItemContextKey } from 'element-plus'

/**
 * 标题（标签）与控件框的相对位置：
 * - `inset`（默认）：标题做成 fieldset 的 legend，嵌在 12px 圆角描边的顶边上 —— 规则 / 设置等
 *   密集表单用，省一行高度；
 * - `outside`：标题独占一行、放在控件上方，圆角描边只包住控件本身 —— 文件管理 / 运行日志 /
 *   命名工坊用（用户 2026-09-22 明确要求「标题跟输入框分离，不要内嵌到框里」）。
 *
 * 页面用 `provide(outlinedFieldLabelPlacementKey, 'outside')` 声明；弹窗虽然 teleport 到 body，
 * 但组件树仍在页面内，所以会一并生效。
 */
export type OutlinedFieldLabelPlacement = 'inset' | 'outside'
export const outlinedFieldLabelPlacementKey: InjectionKey<OutlinedFieldLabelPlacement> =
  Symbol('outlinedFieldLabelPlacement')

function outlinedControl(control: Component, kind: string) {
  return defineComponent({
    name: `Outlined${kind}`,
    inheritAttrs: false,
    props: {
      fieldLabel: { type: String, default: '' },
    },
    setup(props, { attrs, slots }) {
      const formItem = inject(formItemContextKey, undefined)
      const placement = inject(outlinedFieldLabelPlacementKey, 'inset')
      const label = computed(() => props.fieldLabel || formItem?.label || String(attrs['aria-label'] || attrs.placeholder || ''))
      // 没标题的字段（只有占位符那种）不进「标题在框外」分支，否则会凭空多出一整行。
      const labelOutside = computed(() => placement === 'outside' && Boolean(label.value))

      return () => h('div', {
        class: [
          'outlined-field',
          `outlined-field--${kind}`,
          labelOutside.value ? 'outlined-field--label-outside' : undefined,
          attrs.class,
        ],
        style: attrs.style,
        'data-form-label': Boolean(formItem?.label) || undefined,
      }, [
        labelOutside.value ? h('span', { class: 'outlined-field__label', title: label.value }, label.value) : null,
        h('fieldset', { class: 'outlined-field__border', 'aria-hidden': 'true' }, [
          !labelOutside.value && label.value ? h('legend', { title: label.value }, label.value) : null,
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
