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
      <!-- 执行中：按钮常驻显示状态，再点一次即停止（图标换成实心方块的「停止」语义）。 -->
      <button
        v-if="running"
        type="button"
        class="card-action-bar__btn card-action-bar__btn--run is-running"
        :disabled="cancelling"
        title="点击停止这次执行"
        @click.stop="emit('cancel')"
      >
        <svg class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="6" y="6" width="12" height="12" rx="2.6" fill="currentColor" />
        </svg>
        <span>{{ cancelling ? '停止中' : '执行中' }}</span>
      </button>

      <el-dropdown v-else-if="strmSync" trigger="click" @command="handleStrmCommand">
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
        <!-- 编辑沿用 Element Plus 的 Edit（造型比自绘铅笔更受认可），圆润度交给样式里的同色描边处理。 -->
        <el-icon class="card-action-bar__icon"><Edit /></el-icon>
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
import { Edit } from '@element-plus/icons-vue'

withDefaults(
  defineProps<{
    enabled: boolean
    /** 状态切换请求进行中：禁用按钮避免重复提交。 */
    busy?: boolean
    /** Strm 模式的链路规则：执行按钮改为「增量同步 / 全量同步」下拉。 */
    strmSync?: boolean
    /** 执行按钮文案：规则卡片用「执行」，备份卡片用「重新扫描」。 */
    executeLabel?: string
    /** 执行按钮图标：'run' = 圆角方框 + 圆角三角（立即执行）；'rescan' = 四角扫描框（扫描）。 */
    executeIcon?: 'run' | 'rescan'
    /** 该规则 / 任务当前是否正在执行：执行按钮常驻显示「执行中」，再点一次即停止。 */
    running?: boolean
    /** 停止请求已发出、后端还在收尾：按钮短暂禁用，避免重复点。 */
    cancelling?: boolean
  }>(),
  {
    busy: false,
    strmSync: false,
    executeLabel: '执行',
    executeIcon: 'run',
    running: false,
    cancelling: false,
  },
)

const emit = defineEmits<{
  (e: 'toggle'): void
  (e: 'execute'): void
  (e: 'strm-sync', command: string): void
  (e: 'cancel'): void
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

/* 「已启用」= 浅绿底 + 深绿字（绿=运行中，和「已禁用」的砖红天然对立，一眼能分）。
   字色 #15803d 与备份卡片「实时监控」标签同一个绿，同卡片里两处绿不打架；
   不用更浅的绿，否则在浅底上对比太弱、看着发淡。悬浮态再深一档。 */
.card-action-bar__toggle.is-on {
  color: #15803d;
  border-color: rgba(34, 197, 94, 0.42);
  background: rgba(34, 197, 94, 0.14);
}

/* 悬浮只比静置态加深一档：原来直接铺满强调色实心块，颜色过重、和卡片的浅色基调冲突。 */
.card-action-bar__toggle.is-on:hover {
  color: #0f6a33;
  border-color: rgba(34, 197, 94, 0.72);
  background: rgba(34, 197, 94, 0.26);
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
  color: #055a8f;
  border-color: rgba(32, 159, 238, 0.5);
  background: rgba(32, 159, 238, 0.12);
}

/* 执行中：浅蓝底 + 深蓝字 + 一圈呼吸灯，状态常驻在按钮上；
   图标这时是实心方块（停止），再点一次即发出停止请求。 */
.card-action-bar__btn--run.is-running {
  color: #0f5c8f;
  border-color: rgba(11, 157, 248, 0.5);
  background: rgba(11, 157, 248, 0.16);
  animation: card-action-bar-pulse 1.7s ease-in-out infinite;
}

.card-action-bar__btn--run.is-running:hover {
  color: #0a4a73;
  border-color: rgba(11, 157, 248, 0.74);
  background: rgba(11, 157, 248, 0.26);
}

/* 停止请求已发出、后端还在收尾：禁用重复点击，同时停掉呼吸灯免得看着像还在正常跑。 */
.card-action-bar__btn--run.is-running:disabled {
  opacity: 0.72;
  cursor: progress;
  animation: none;
}

@keyframes card-action-bar-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(11, 157, 248, 0.3);
  }
  50% {
    box-shadow: 0 0 0 5px rgba(11, 157, 248, 0);
  }
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

/* 编辑按钮：保留 Element Plus `Edit` 的造型（用户点名要这一版），只把偏硬朗的转折磨圆一点。
   它的网格是 1024，描边宽度必须按 1024 计 —— 而描边是**整圈外扩**，会连带把笔画加粗，所以只能给极小值：
     · EP Edit 方框的环厚 64 单位 → 19px 下 1.19px；   24 网格图标 stroke 1.7 → 19px 下 1.35px；
     · stroke-width 12 → 19px 下仅 +0.22px（1.41px），与其它图标基本等重；
     · 曾用 48（+0.89px → 2.08px，比其他图标粗 54%），肉眼一看就不协调，别再放大。 */
.card-action-bar__btn--edit .card-action-bar__icon :deep(svg) {
  stroke: currentColor;
  stroke-width: 12;
  stroke-linejoin: round;
  stroke-linecap: round;
}
</style>
