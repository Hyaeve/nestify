/**
 * 只在文本真的溢出（被 CSS 省略号截断）时才给元素挂原生 title tooltip。
 *
 * 背景：规则卡片、备份卡片的路径行原本无条件写 :title，于是未设置的行、
 * 以及完整显示得下的行，鼠标移上去也会弹出一个空/多余的文本框。
 *
 * 用法：`@mouseenter="syncOverflowTitle($event, value)"`（不要再用 :title 绑定，
 * 否则 Vue 重渲染时会把这里移除的属性又写回去）。
 */
export function syncOverflowTitle(event: MouseEvent, value?: string | null) {
  const element = event.currentTarget as HTMLElement | null
  if (!element) return

  const text = (value ?? '').trim()
  if (!text || element.scrollWidth <= element.clientWidth) {
    element.removeAttribute('title')
    return
  }
  element.setAttribute('title', text)
}
