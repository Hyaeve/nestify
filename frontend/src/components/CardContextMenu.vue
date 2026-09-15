<template>
  <Teleport to="body">
    <div
      v-if="visible"
      ref="menuRef"
      class="card-context-menu"
      role="menu"
      :style="{ left: `${left}px`, top: `${top}px` }"
      @contextmenu.prevent
    >
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        role="menuitem"
        class="card-context-menu__item"
        :class="{ 'is-danger': item.danger }"
        @click="selectItem(item.key)"
      >
        <svg v-if="item.icon === 'copy'" class="card-context-menu__icon" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="8" y="7" width="10" height="12" rx="2" />
          <path d="M6 15H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v1" />
        </svg>
        <span class="card-context-menu__label">{{ item.label }}</span>
      </button>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

export interface CardContextMenuItem {
  key: string
  label: string
  /** 目前仅支持「复制」自绘线条图标；留空则只显示文字。 */
  icon?: 'copy'
  danger?: boolean
}

const props = withDefaults(
  defineProps<{
    visible: boolean
    /** 指针位置（clientX / clientY）：菜单以指针为左上角展开。 */
    x: number
    y: number
    items: CardContextMenuItem[]
  }>(),
  {
    x: 0,
    y: 0,
    items: () => [],
  },
)

const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'close'): void
}>()

/** 距离视口边缘的最小留白：贴边时把菜单整体推回来，避免被裁掉。 */
const VIEWPORT_EDGE = 8

const menuRef = ref<HTMLElement | null>(null)
const left = ref(props.x)
const top = ref(props.y)

/**
 * 先按指针原样摆放，量到真实尺寸后再往回钳制：
 * 右键贴近右/下边缘时菜单会向左/上翻转，不会溢出视口。
 */
async function placeMenu() {
  left.value = props.x
  top.value = props.y

  await nextTick()

  const element = menuRef.value
  if (!element) return

  const rect = element.getBoundingClientRect()
  const maxLeft = window.innerWidth - rect.width - VIEWPORT_EDGE
  const maxTop = window.innerHeight - rect.height - VIEWPORT_EDGE
  left.value = Math.max(VIEWPORT_EDGE, Math.min(props.x, maxLeft))
  top.value = Math.max(VIEWPORT_EDGE, Math.min(props.y, maxTop))
}

function closeMenu() {
  emit('close')
}

function selectItem(key: string) {
  emit('close')
  emit('select', key)
}

/** 外部（含滚动容器）任意按下即收起；菜单内部的按下要放过，否则点不中条目。 */
function handleDocumentPointerDown(event: MouseEvent) {
  const element = menuRef.value
  if (element && event.target instanceof Node && element.contains(event.target)) return
  closeMenu()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closeMenu()
}

function bindGlobalListeners() {
  document.addEventListener('mousedown', handleDocumentPointerDown, true)
  document.addEventListener('keydown', handleKeydown)
  document.addEventListener('scroll', closeMenu, true)
  window.addEventListener('resize', closeMenu)
  window.addEventListener('blur', closeMenu)
}

function unbindGlobalListeners() {
  document.removeEventListener('mousedown', handleDocumentPointerDown, true)
  document.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('scroll', closeMenu, true)
  window.removeEventListener('resize', closeMenu)
  window.removeEventListener('blur', closeMenu)
}

watch(
  () => (props.visible ? `${props.x}:${props.y}` : ''),
  () => {
    if (props.visible) void placeMenu()
  },
)

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      bindGlobalListeners()
      void placeMenu()
      return
    }
    unbindGlobalListeners()
  },
)

onBeforeUnmount(unbindGlobalListeners)
</script>

<style scoped lang="scss">
/* 规则卡片 / 备份卡片共用的右键悬浮菜单：跟随指针展开，主题色跟随 Element Plus 变量。 */
.card-context-menu {
  position: fixed;
  z-index: 3000;
  min-width: 156px;
  padding: 6px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.16);
}

.card-context-menu__item {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 8px 12px;
  border: 0;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  text-align: left;
  color: var(--el-text-color-primary);
  background: transparent;
  cursor: pointer;
  transition:
    color 0.16s ease,
    background-color 0.16s ease;
}

.card-context-menu__item:hover {
  color: #2e5f6b;
  background: rgba(107, 159, 176, 0.12);
}

.card-context-menu__item.is-danger {
  color: #a1443a;
}

.card-context-menu__item.is-danger:hover {
  color: #a1443a;
  background: rgba(176, 90, 80, 0.12);
}

.card-context-menu__icon {
  flex: 0 0 auto;
  width: 15px;
  height: 15px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.card-context-menu__label {
  min-width: 0;
  white-space: nowrap;
}
</style>
