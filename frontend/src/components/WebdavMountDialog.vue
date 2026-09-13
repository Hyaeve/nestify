<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEditing ? '编辑 WebDAV 挂载' : '添加 WebDAV 挂载'"
    width="560px"
    destroy-on-close
    @update:model-value="handleVisibleChange"
  >
    <el-form label-position="top" class="mount-form">
      <el-form-item label="挂载名称">
        <el-input v-model="form.name" placeholder="例如：移动云盘" maxlength="64" />
      </el-form-item>

      <div class="mount-form__row">
        <el-form-item label="协议" class="mount-form__row-item mount-form__row-item--scheme">
          <el-select v-model="form.scheme">
            <el-option label="http" value="http" />
            <el-option label="https" value="https" />
          </el-select>
        </el-form-item>
        <el-form-item label="域名或 IP" class="mount-form__row-item">
          <el-input v-model="form.host" placeholder="例如：10.0.0.31" />
        </el-form-item>
        <el-form-item label="端口" class="mount-form__row-item mount-form__row-item--port">
          <el-input-number v-model="form.port" :min="0" :max="65535" controls-position="right" />
        </el-form-item>
      </div>

      <div class="mount-form__row">
        <el-form-item label="用户名" class="mount-form__row-item">
          <el-input v-model="form.username" placeholder="WebDAV 用户名" autocomplete="off" />
        </el-form-item>
        <el-form-item label="密码" class="mount-form__row-item">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="isEditing && mount?.has_password ? '留空表示不修改' : 'WebDAV 密码'"
            autocomplete="new-password"
          />
        </el-form-item>
      </div>

      <el-form-item label="指定路径">
        <el-input v-model="form.base_path" placeholder="可选，例如 /d，留空表示根目录" />
      </el-form-item>

      <el-form-item label="启用挂载">
        <el-switch v-model="form.enabled" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="handleVisibleChange(false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">确认</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'

import { createMount, updateMount, type MountInput, type WebdavMount } from '../api/mounts'

const props = defineProps<{
  modelValue: boolean
  mount: WebdavMount | null
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'saved'): void
}>()

const submitting = ref(false)
const isEditing = computed(() => Boolean(props.mount?.id))

const form = reactive<MountInput>({
  name: '',
  scheme: 'http',
  host: '',
  port: 5244,
  username: '',
  password: '',
  base_path: '',
  enabled: true,
})

watch(
  () => props.modelValue,
  (visible) => {
    if (!visible) {
      return
    }
    const mount = props.mount
    form.name = mount?.name ?? ''
    form.scheme = mount?.scheme ?? 'http'
    form.host = mount?.host ?? ''
    form.port = mount?.port ?? 5244
    form.username = mount?.username ?? ''
    form.password = ''
    form.base_path = mount?.base_path ?? ''
    form.enabled = mount?.enabled ?? true
  },
  { immediate: true },
)

function handleVisibleChange(value: boolean) {
  emit('update:modelValue', value)
}

async function handleSubmit() {
  if (!form.name.trim()) {
    ElMessage.error('请填写挂载名称')
    return
  }
  if (!form.host.trim()) {
    ElMessage.error('请填写域名或 IP')
    return
  }

  submitting.value = true
  try {
    const payload: MountInput = {
      name: form.name.trim(),
      scheme: form.scheme,
      host: form.host.trim(),
      port: Number(form.port) || 0,
      username: form.username.trim(),
      password: form.password,
      base_path: form.base_path.trim(),
      enabled: form.enabled,
    }

    if (isEditing.value && props.mount) {
      await updateMount(props.mount.id, payload)
      ElMessage.success('挂载已更新')
    } else {
      await createMount(payload)
      ElMessage.success('挂载已添加')
    }

    emit('saved')
    handleVisibleChange(false)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存挂载失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.mount-form__row {
  display: flex;
  gap: 12px;
}

.mount-form__row-item {
  flex: 1 1 0;
  min-width: 0;
}

.mount-form__row-item--scheme {
  flex: 0 0 108px;
}

.mount-form__row-item--port {
  flex: 0 0 132px;
}
</style>
