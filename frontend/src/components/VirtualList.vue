<template>
  <!-- 虚拟滚动容器：长列表只渲染可视区那几十行，滚动条高度由偏移表算出来的总高撑起。
       自带滚动条（全局 6px 细滚动条），高度由使用者给（flex 撑满 / 定高 / calc 都行）。 -->
  <div ref="viewportRef" class="virtual-list" @scroll.passive="handleScroll">
    <div class="virtual-list__sizer" :style="{ height: `${totalHeight}px` }">
      <div
        v-for="row in windowRows"
        :key="row.key"
        class="virtual-list__row"
        :style="{ height: `${row.height}px`, transform: `translateY(${row.top}px)` }"
      >
        <slot :item="row.item" :index="row.index"></slot>
      </div>
    </div>

    <!-- 尾部（「加载更多 / 加载中」这类挂在列表下方的整块）走普通文档流，
         位置正好接在撑高的 sizer 后面，不参与虚拟定位。 -->
    <div v-if="$slots.footer" class="virtual-list__footer">
      <slot name="footer"></slot>
    </div>
  </div>
</template>

<script setup lang="ts" generic="T">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    // 全部条目（**未分页**）：虚拟滚动的前提就是整份数据在手里，只把渲染量压下去。
    items: readonly T[]
    // 行高：定值，或按条目算（明细里「两行条目」比「单行条目」高）。
    // 注意它必须与真实渲染高度一致，否则行会重叠或露缝 —— 行是绝对定位 + 固定高的。
    itemSize: number | ((item: T, index: number) => number)
    // 行 key：默认用下标。传条目自身的稳定 id 更好（插入/删除时少重建 DOM）。
    itemKey?: (item: T, index: number) => string | number
    // 可视区上下各多渲染几行，滚动时不会看到空白。
    overscan?: number
    // 距末尾还剩几行就触发 reach-end（配合「滚到底自动续加载」）。
    reachEndOffset?: number
    // 还有没有下一页；为 false 时不再触发 reach-end。
    hasMore?: boolean
    // 正在取下一页：期间不重复触发 reach-end。
    loadingMore?: boolean
    // 换数据源（换目录 / 换筛选 / 换搜索词 / 换排序）时把这个值改一下，
    // 列表就会自动滚回顶部 —— 父组件因此不需要拿组件实例来调方法。
    resetKey?: unknown
  }>(),
  {
    itemKey: undefined,
    overscan: 4,
    reachEndOffset: 6,
    hasMore: false,
    loadingMore: false,
    resetKey: undefined,
  },
)

const emit = defineEmits<{ (event: 'reach-end'): void }>()

const viewportRef = ref<HTMLElement | null>(null)
// 滚动位置与可视高度是「算出可视区间」的两个输入。
const scrollTop = ref(0)
const viewportHeight = ref(0)

// 前缀和偏移表：offsets[i] = 第 i 行顶部位置，offsets[n] = 总高。
// 只随「条目 / 行高口径」变化重算，滚动时不动它（滚动只做二分查找）。
const metrics = computed(() => {
  const items = props.items
  const offsets = new Float64Array(items.length + 1)
  const size = props.itemSize
  const sizeOf = typeof size === 'function' ? size : () => size
  for (let index = 0; index < items.length; index += 1) {
    offsets[index + 1] = offsets[index] + sizeOf(items[index], index)
  }
  return { offsets, total: offsets[items.length] }
})

const totalHeight = computed(() => Math.ceil(metrics.value.total))

interface VirtualRow<R> {
  key: string | number
  index: number
  item: R
  height: number
  top: number
}

// 二分找「包含 y 的那一行」：offsets 递增，最后一个 offsets[i] <= y 的 i。
function findRowIndex(offsets: Float64Array, y: number) {
  const lastIndex = offsets.length - 2
  if (lastIndex < 0 || y <= 0) {
    return 0
  }
  let low = 0
  let high = lastIndex
  while (low < high) {
    const mid = (low + high + 1) >> 1
    if (offsets[mid] <= y) {
      low = mid
    } else {
      high = mid - 1
    }
  }
  return low
}

// 当前该渲染的区间：可视区首尾各让出 overscan 行。
const visibleRange = computed(() => {
  const count = props.items.length
  if (count === 0) {
    return { start: 0, end: -1 }
  }
  const { offsets } = metrics.value
  const start = Math.max(0, findRowIndex(offsets, scrollTop.value) - props.overscan)
  const bottom = scrollTop.value + Math.max(viewportHeight.value, 1) - 1
  const end = Math.min(count - 1, findRowIndex(offsets, bottom) + props.overscan)
  return { start, end }
})

const windowRows = computed<VirtualRow<T>[]>(() => {
  const { start, end } = visibleRange.value
  if (end < start) {
    return []
  }
  const { offsets } = metrics.value
  const size = props.itemSize
  const sizeOf = typeof size === 'function' ? size : () => size
  const keyOf = props.itemKey
  const rows: VirtualRow<T>[] = []
  for (let index = start; index <= end; index += 1) {
    const item = props.items[index]
    rows.push({
      key: keyOf ? keyOf(item, index) : index,
      index,
      item,
      height: sizeOf(item, index),
      top: offsets[index],
    })
  }
  return rows
})

// 滚动只需重算可视区间，rAF 节流把高频 scroll 事件压到每帧一次。
let frame = 0

function syncViewport() {
  const element = viewportRef.value
  if (!element) {
    return
  }
  viewportHeight.value = element.clientHeight
  scrollTop.value = element.scrollTop
}

function handleScroll() {
  if (frame) {
    return
  }
  frame = requestAnimationFrame(() => {
    frame = 0
    syncViewport()
  })
}

let observer: ResizeObserver | null = null

onMounted(() => {
  syncViewport()
  if (typeof ResizeObserver !== 'undefined' && viewportRef.value) {
    observer = new ResizeObserver(() => {
      viewportHeight.value = viewportRef.value?.clientHeight ?? 0
    })
    observer.observe(viewportRef.value)
  }
})

onBeforeUnmount(() => {
  if (frame) {
    cancelAnimationFrame(frame)
    frame = 0
  }
  observer?.disconnect()
  observer = null
})

// 条目变少（换筛选、换目录）时浏览器会把 scrollTop 夹回合法范围，这里跟着同步；
// 否则偏移表短了、scrollTop 还停在旧值上，会算出一段空区间。
watch(
  () => metrics.value.total,
  (total) => {
    const element = viewportRef.value
    if (!element) {
      return
    }
    if (element.scrollTop > total) {
      element.scrollTop = Math.max(0, total)
    }
    scrollTop.value = element.scrollTop
  },
)

// 末尾进入视野就请求下一页。按「条目数」去重：只有条目真的长了才允许再触发一次，
// 避免同一页被反复请求。
let lastRequestedCount = -1

watch(
  () => [visibleRange.value.end, props.items.length] as const,
  () => {
    if (!props.hasMore || props.loadingMore) {
      return
    }
    const count = props.items.length
    if (count === 0 || visibleRange.value.end < count - 1 - props.reachEndOffset) {
      return
    }
    if (count === lastRequestedCount) {
      return
    }
    lastRequestedCount = count
    emit('reach-end')
  },
  { flush: 'post' },
)

function scrollToTop() {
  const element = viewportRef.value
  if (element) {
    element.scrollTop = 0
  }
  scrollTop.value = 0
}

// 数据源换了（resetKey 变）就回到顶部。同一批数据只刷新条目（比如重新拉同一个目录）
// 也一样要回顶，所以父组件在「重新取数」时也会把这个键推进一格。
watch(
  () => props.resetKey,
  () => {
    scrollToTop()
  },
)

function scrollToIndex(index: number) {
  const element = viewportRef.value
  if (!element || props.items.length === 0) {
    return
  }
  const clamped = Math.min(Math.max(index, 0), props.items.length - 1)
  element.scrollTop = metrics.value.offsets[clamped]
  scrollTop.value = element.scrollTop
}

defineExpose({ scrollToTop, scrollToIndex })
</script>

<style scoped>
/* 滚动容器：overflow-y 必须挂在这里（不能挂 scoped 里够不着的父级），
   否则撑高的 sizer 会把外层页面一起顶高，就白虚拟了。 */
.virtual-list {
  position: relative;
  height: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
}

/* 只负责撑出滚动条的总高度；行全部绝对定位在它里面。 */
.virtual-list__sizer {
  position: relative;
  width: 100%;
}

/* 行高由 JS 按偏移表写死（内联 style），行内内容超出请自行省略，别指望换行撑高。 */
.virtual-list__row {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  overflow: hidden;
}

.virtual-list__footer {
  position: relative;
}
</style>
