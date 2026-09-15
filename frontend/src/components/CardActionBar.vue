<template>
  <div class="card-action-bar" @click.stop>
    <button
      type="button"
      class="card-action-bar__toggle"
      :class="enabled ? 'is-on' : 'is-off'"
      :disabled="busy"
      @click.stop="emit('toggle')"
    >
      <el-icon class="card-action-bar__icon">
        <CircleCheckFilled v-if="enabled" />
        <CircleCloseFilled v-else />
      </el-icon>
      <span>{{ enabled ? '已启用' : '已禁用' }}</span>
    </button>

    <div class="card-action-bar__actions">
      <el-dropdown v-if="strmSync" trigger="click" @command="handleStrmCommand">
        <button type="button" class="card-action-bar__btn card-action-bar__btn--run" @click.stop>
          <svg class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M5 4.5h11.5v4" />
            <path d="M8 7.5h11v12H8z" />
            <path d="M11 13.5h5.5" />
            <path d="m14.5 10.5 3 3-3 3" />
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
        <svg class="card-action-bar__icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M5 4.5h11.5v4" />
          <path d="M8 7.5h11v12H8z" />
          <path d="M11 13.5h5.5" />
          <path d="m14.5 10.5 3 3-3 3" />
        </svg>
        <span>{{ executeLabel }}</span>
      </button>

      <button type="button" class="card-action-bar__btn card-action-bar__btn--edit" @click.stop="emit('edit')">
        <el-icon class="card-action-bar__icon"><Edit /></el-icon>
        <span>编辑</span>
      </button>

      <button type="button" class="card-action-bar__btn card-action-bar__btn--remove" @click.stop="emit('remove')">
        <el-icon class="card-action-bar__icon"><Delete /></el-icon>
        <span>移除</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { CircleCheckFilled, CircleCloseFilled, Delete, Edit } from '@element-plus/icons-vue'

withDefaults(
  defineProps<{
    enabled: boolean
    /** 状态切换请求进行中：禁用按钮避免重复提交。 */
    busy?: boolean
    /** Strm 模式的链路规则：执行按钮改为「增量同步 / 全量同步」下拉。 */
    strmSync?: boolean
    /** 执行按钮文案：规则卡片用「执行」，备份卡片用「重新扫描」。 */
    executeLabel?: string
  }>(),
  {
    busy: false,
    strmSync: false,
    executeLabel: '执行',
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
  gap: 10px;
  min-width: 0;
}

.card-action-bar__toggle,
.card-action-bar__btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 12px;
  border: 1px solid var(--border-color);
  border-radius: 999px;
  font-size: 12px;
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

.card-action-bar__toggle.is-on:hover {
  color: #fff;
  border-color: #5b7a6e;
  background: #5b7a6e;
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
  gap: 6px;
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

.card-action-bar__icon {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
  font-size: 14px;
}

.card-action-bar__btn svg.card-action-bar__icon {
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

@media (max-width: 640px) {
  .card-action-bar {
    flex-wrap: wrap;
  }
}
</style>
