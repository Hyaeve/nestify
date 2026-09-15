<template>
  <div class="card-action-bar" @click.stop>
    <button
      type="button"
      class="card-action-bar__toggle"
      :class="enabled ? 'is-on' : 'is-off'"
      :disabled="busy"
      @click.stop="emit('toggle')"
    >
      <!-- 已启用 = 圆角实心播放三角（正在运行）；已禁用 = 双圆角竖条（已暂停）。 -->
      <!-- 三段 Q 命令把三角三个角磨圆，避免小尺寸下尖角扎眼；尺寸按 24 网格撑满，与其它按钮图标等大。 -->
      <svg class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
        <path v-if="enabled" d="M9.62 5.7 17.38 10.7Q19.4 12 17.38 13.3L9.62 18.3Q7.6 19.6 7.6 17.2V6.8Q7.6 4.4 9.62 5.7Z" fill="currentColor" />
        <g v-else>
          <rect x="7.3" y="4.4" width="3.5" height="15.2" rx="1.75" fill="currentColor" />
          <rect x="13.2" y="4.4" width="3.5" height="15.2" rx="1.75" fill="currentColor" />
        </g>
      </svg>
      <span>{{ enabled ? '已启用' : '已禁用' }}</span>
    </button>

    <div class="card-action-bar__actions">
      <el-dropdown v-if="strmSync" trigger="click" @command="handleStrmCommand">
        <button type="button" class="card-action-bar__btn card-action-bar__btn--run" @click.stop>
          <svg class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
            <rect x="4.2" y="4.2" width="15.6" height="15.6" rx="4.4" />
            <path d="M9.62 8.47 14.39 11.33Q15.5 12 14.39 12.67L9.62 15.53Q8.5 16.2 8.5 14.9V9.1Q8.5 7.8 9.62 8.47Z" fill="currentColor" />
          </svg>
          <span>{{ executeLabel }}</span>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="incremental">增量同步</el-dropdown-item>
            <el-dropdown-item command="full">全量同步</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <button v-else type="button" class="card-action-bar__btn card-action-bar__btn--run" @click.stop="emit('execute')">
        <!-- 「执行」= 圆角方框 + 实心圆角三角（标准「运行」按钮语义，与启用态的裸三角、备份卡片的扫描框都能分开）。 -->
        <svg v-if="executeIcon === 'rescan'" class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M4 9.5V7A3 3 0 0 1 7 4h2.5" />
          <path d="M14.5 4H17a3 3 0 0 1 3 3v2.5" />
          <path d="M20 14.5V17a3 3 0 0 1-3 3h-2.5" />
          <path d="M9.5 20H7a3 3 0 0 1-3-3v-2.5" />
          <path d="M5.4 12h13.2" />
        </svg>
        <svg v-else class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="4.2" y="4.2" width="15.6" height="15.6" rx="4.4" />
          <path d="M9.62 8.47 14.39 11.33Q15.5 12 14.39 12.67L9.62 15.53Q8.5 16.2 8.5 14.9V9.1Q8.5 7.8 9.62 8.47Z" fill="currentColor" />
        </svg>
        <span>{{ executeLabel }}</span>
      </button>

      <button type="button" class="card-action-bar__btn card-action-bar__btn--edit" @click.stop="emit('edit')">
        <!-- 铅笔/垃圾桶改为自绘：Element Plus 的 Edit / Delete 是实心填充图形，棱角硬，
             与操作条里其它 1.7 描边 + 圆头圆角的线条图标不是一路，所以自己画一版更圆润的。 -->
        <svg class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M5.1 19.7 6.2 15.3 15.8 5.7a2.5 2.5 0 0 1 3.5 3.5L9.7 18.8a1.3 1.3 0 0 1-.9.4z" />
          <path d="m14.3 7.2 3.5 3.5" />
        </svg>
        <span>编辑</span>
      </button>

      <button type="button" class="card-action-bar__btn card-action-bar__btn--remove" @click.stop="emit('remove')">
        <svg class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M4.9 7.1h14.2" />
          <path d="M9.7 7.1V5.5a1.3 1.3 0 0 1 1.3-1.3h2a1.3 1.3 0 0 1 1.3 1.3v1.6" />
          <path d="M7 7.1l.75 11.5a2.1 2.1 0 0 0 2.1 1.97h4.3a2.1 2.1 0 0 0 2.1-1.97L17 7.1" />
          <path d="M10.5 10.9v5.4" />
          <path d="M13.5 10.9v5.4" />
        </svg>
        <span>移除</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    enabled: boolean
    /** 状态切换请求进行中：禁用按钮避免重复提交。 */
    busy?: boolean
    /** Strm 模式的链路规则：执行按钮改为「增量同步 / 全量同步」下拉。 */
    strmSync?: boolean
    /** 执行按钮文案：规则卡片用「执行」，备份卡片用「重新扫描」。 */
    executeLabel?: string
    /** 执行按钮图标：'run' = 圆角方框 + 圆角三角（立即执行）；'rescan' = 四角扫描框（重新扫描）。 */
    executeIcon?: 'run' | 'rescan'
  }>(),
  {
    busy: false,
    strmSync: false,
    executeLabel: '执行',
    executeIcon: 'run',
  },
)

const emit = defineEmits<{
  (e: 'toggle'): void
  (e: 'execute'): void
  (e: 'strm-sync', command: string): void
  (e: 'edit'): void
  (e: 'remove'): void
}>()

function handleStrmCommand(command: string) {
  emit('strm-sync', command)
}
</script>

<style scoped lang="scss">
/* 规则卡片统一的底部操作条：左侧启用/禁用，右侧执行、编辑、移除，均为图标 + 文字的圆角按钮。 */
.card-action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  /* 窄卡片放不下时才会换行；卡片网格的最小列宽（430/450px）已保证正常情况是一行。 */
  flex-wrap: wrap;
  gap: 9px 10px;
  min-width: 0;
}

.card-action-bar__toggle,
.card-action-bar__btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding: 0 13px;
  border: 1px solid var(--border-color);
  /* 圆角矩形而非胶囊：36px 高配 10px 圆角，既有圆润感又保留矩形轮廓。 */
  border-radius: 10px;
  font-size: 14px;
  font-weight: 700;
  white-space: nowrap;
  color: var(--text-primary);
  background: var(--bg-elevated);
  cursor: pointer;
  transition:
    color 0.16s ease,
    border-color 0.16s ease,
    background-color 0.16s ease,
    box-shadow 0.16s ease;
}

.card-action-bar__toggle:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* 「已启用」用更浅的低饱和青灰，避免深绿过于抢眼（禁用态保持不变）。 */
.card-action-bar__toggle.is-on {
  color: #7fb2a2;
  border-color: rgba(127, 178, 162, 0.42);
  background: rgba(127, 178, 162, 0.13);
}

/* 悬浮只比静置态加深一档：原来直接铺满 #5b7a6e 实心块，颜色过重、和卡片的浅色基调冲突。 */
.card-action-bar__toggle.is-on:hover {
  color: #35604f;
  border-color: rgba(127, 178, 162, 0.72);
  background: rgba(127, 178, 162, 0.26);
}

.card-action-bar__toggle.is-off {
  color: #8a5a52;
  border-color: rgba(176, 118, 106, 0.4);
  background: rgba(176, 118, 106, 0.12);
}

.card-action-bar__toggle.is-off:hover {
  color: #fff;
  border-color: #b0766a;
  background: #b0766a;
}

.card-action-bar__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  /* 按钮放大后窄卡片可能放不下：允许换行到第二行，别溢出卡片。 */
  flex-wrap: wrap;
  gap: 7px;
  min-width: 0;
}

.card-action-bar__btn--run:hover {
  color: #2f6b57;
  border-color: rgba(91, 122, 110, 0.5);
  background: rgba(91, 122, 110, 0.12);
}

.card-action-bar__btn--edit:hover {
  color: #365f86;
  border-color: rgba(95, 127, 168, 0.5);
  background: rgba(95, 127, 168, 0.12);
}

.card-action-bar__btn--remove:hover {
  color: #a1443a;
  border-color: rgba(176, 90, 80, 0.5);
  background: rgba(176, 90, 80, 0.12);
}

/* 所有按钮图标统一 19px（含启用/禁用那两枚自绘图形），彼此墨量才一致。 */
.card-action-bar__icon {
  width: 19px;
  height: 19px;
  flex: 0 0 auto;
  font-size: 19px;
}

.card-action-bar__btn svg.card-action-bar__icon {
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

/* 自带实心填充的图形（执行闪电）不参与描边，否则笔画会被再描一圈、比启用态三角粗一档。 */
.card-action-bar__btn svg.card-action-bar__icon path[fill='currentColor'] {
  stroke: none;
}
</style>
