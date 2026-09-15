<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEditing ? '编辑远程挂载' : '添加远程挂载'"
    width="600px"
    destroy-on-close
    @update:model-value="handleVisibleChange"
  >
    <el-form label-position="top" class="mount-form">
      <div class="mount-form__row">
        <el-form-item label="挂载类型" class="mount-form__row-item mount-form__row-item--provider">
          <el-select v-model="form.provider" @change="handleProviderChange">
            <el-option label="WebDAV" value="webdav" />
            <el-option label="OpenList" value="openlist" />
          </el-select>
        </el-form-item>
        <el-form-item label="挂载名称" class="mount-form__row-item">
          <el-input v-model="form.name" placeholder="例如：移动云盘" maxlength="64" />
        </el-form-item>
      </div>

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

      <el-form-item label="认证方式">
        <el-radio-group v-model="form.auth_type" class="mount-form__auth">
          <el-radio-button value="password">用户名密码</el-radio-button>
          <el-radio-button value="token">令牌</el-radio-button>
        </el-radio-group>
        <div v-if="form.auth_type === 'token'" class="mount-form__hint">
          令牌取自 OpenList 后台「设置 → 令牌」中的永久令牌。
        </div>
      </el-form-item>

      <div v-if="form.auth_type === 'password'" class="mount-form__row">
        <el-form-item label="用户名" class="mount-form__row-item">
          <el-input v-model="form.username" :placeholder="usernamePlaceholder" autocomplete="off" />
        </el-form-item>
        <el-form-item label="密码" class="mount-form__row-item">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="loadingSecret ? '正在加载…' : isEditing ? '已保存的密码' : '登录密码'"
            autocomplete="new-password"
          />
        </el-form-item>
      </div>

      <el-form-item v-else label="令牌">
        <el-input
          v-model="form.token"
          type="password"
          show-password
          :placeholder="loadingSecret ? '正在加载…' : isEditing ? '已保存的令牌' : 'OpenList 永久令牌'"
          autocomplete="new-password"
        />
      </el-form-item>

      <el-form-item label="指定路径">
        <el-input v-model="form.base_path" :placeholder="basePathPlaceholder" />
      </el-form-item>

      <el-form-item label="启用挂载">
        <el-switch v-model="form.enabled" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="mount-dialog-footer">
        <el-button :loading="testing" @click="handleTest">测试</el-button>
        <div class="mount-dialog-footer__spacer" />
        <el-button @click="handleVisibleChange(false)">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确认</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'

import {
  OPENLIST_DEFAULT_PORT,
  createMount,
  fetchMount,
  testMountConnection,
  updateMount,
  type MountAuthType,
  type MountInput,
  type MountProvider,
  type WebdavMount,
} from '../api/mounts'

const props = defineProps<{
  modelValue: boolean
  mount: WebdavMount | null
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'saved'): void
}>()

const submitting = ref(false)
const testing = ref(false)
const isEditing = computed(() => Boolean(props.mount?.id))
const loadingSecret = ref(false)

/** OpenList 的 WebDAV 协议端点固定在 /dav，选定该类型时一并填好。 */
const OPENLIST_WEBDAV_ENDPOINT = '/dav'

const form = reactive<MountInput>({
  name: '',
  provider: 'webdav',
  auth_type: 'password',
  scheme: 'http',
  host: '',
  port: 0,
  username: '',
  password: '',
  token: '',
  base_path: '',
  enabled: true,
})

const isOpenList = computed(() => form.provider === 'openlist')

const usernamePlaceholder = computed(() => (isOpenList.value ? 'OpenList 用户名' : 'WebDAV 用户名'))

const basePathPlaceholder = computed(() =>
  isOpenList.value ? `协议端点路径，留空即用 ${OPENLIST_WEBDAV_ENDPOINT}` : 'WebDAV 端点路径，例如 /dav，留空表示根目录',
)

watch(
  () => props.modelValue,
  (visible) => {
    if (!visible) {
      return
    }
    const mount = props.mount
    form.name = mount?.name ?? ''
    form.provider = mount?.provider ?? 'webdav'
    form.auth_type = mount?.auth_type ?? 'password'
    form.scheme = mount?.scheme ?? 'http'
    form.host = mount?.host ?? ''
    // 新建时按类型给默认端口（OpenList 5244），编辑时用已保存的值。
    form.port = mount?.port ?? (form.provider === 'openlist' ? OPENLIST_DEFAULT_PORT : 0)
    form.username = mount?.username ?? ''
    form.password = mount?.password ?? ''
    form.token = mount?.token ?? ''
    form.base_path = mount?.base_path ?? (form.provider === 'openlist' ? OPENLIST_WEBDAV_ENDPOINT : '')
    form.enabled = mount?.enabled ?? true

    // 编辑已有挂载时，密码/令牌可能尚未随列表接口返回，需要单独拉取详情回填。
    if (mount?.id) {
      void loadMountSecrets(mount.id)
    }
  },
  { immediate: true },
)

function handleProviderChange(next: MountProvider | string) {
  // 选定 OpenList 自动填入 5244 与 /dav；用户手动改过的值不覆盖。
  if (next === 'openlist') {
    if (!form.port) {
      form.port = OPENLIST_DEFAULT_PORT
    }
    if (!form.base_path.trim()) {
      form.base_path = OPENLIST_WEBDAV_ENDPOINT
    }
    return
  }
  // 从 OpenList 切回 WebDAV 时，把明显来自 OpenList 的默认值清掉。
  if (form.port === OPENLIST_DEFAULT_PORT) {
    form.port = 0
  }
  if (form.base_path.trim() === OPENLIST_WEBDAV_ENDPOINT) {
    form.base_path = ''
  }
}

async function loadMountSecrets(id: number) {
  loadingSecret.value = true
  try {
    const response = await fetchMount(id)
    if (response.data?.password) {
      form.password = response.data.password
    }
    if (response.data?.token) {
      form.token = response.data.token
    }
    // 历史挂载没有类型字段时，以服务端回退后的值为准。
    if (response.data?.provider) {
      form.provider = response.data.provider
    }
    if (response.data?.auth_type) {
      form.auth_type = response.data.auth_type
    }
  } catch {
    // 拉取失败时保持空值，用户可自行重新填写。
  } finally {
    loadingSecret.value = false
  }
}

function handleVisibleChange(value: boolean) {
  emit('update:modelValue', value)
}

function buildPayload(): MountInput {
  return {
    name: form.name.trim(),
    provider: form.provider,
    auth_type: form.auth_type,
    scheme: form.scheme,
    host: form.host.trim(),
    port: Number(form.port) || 0,
    username: form.username.trim(),
    password: form.password,
    token: form.token.trim(),
    base_path: form.base_path.trim(),
    enabled: form.enabled,
  }
}

function validateForm(): string | null {
  if (!form.name.trim()) {
    return '请填写挂载名称'
  }
  if (!form.host.trim()) {
    return '请填写域名或 IP'
  }
  if (form.auth_type === 'token' && !form.token.trim()) {
    return '请填写令牌'
  }
  return null
}

async function handleTest() {
  const message = validateForm()
  if (message) {
    ElMessage.error(message)
    return
  }

  testing.value = true
  try {
    const response = await testMountConnection(buildPayload())
    ElMessage.success(`连接成功，发现 ${response.data?.count ?? 0} 个目录/文件`)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '连接测试失败')
  } finally {
    testing.value = false
  }
}

async function handleSubmit() {
  const message = validateForm()
  if (message) {
    ElMessage.error(message)
    return
  }

  submitting.value = true
  try {
    const payload: MountInput = buildPayload()

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

.mount-form__row-item--provider {
  flex: 0 0 148px;
}

.mount-form__row-item--scheme {
  flex: 0 0 108px;
}

.mount-form__row-item--port {
  flex: 0 0 132px;
}

/* 认证方式做成整行的分段选择器，与下方字段的「二选一」关系一眼可见。 */
.mount-form__auth {
  display: flex;
  width: 100%;
}

.mount-form__auth :deep(.el-radio-button) {
  flex: 1 1 0;
  min-width: 0;
}

.mount-form__auth :deep(.el-radio-button__inner) {
  width: 100%;
  border-radius: 0;
}

.mount-form__hint {
  flex: 0 0 100%;
  width: 100%;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.5;
  color: #8a9a94;
}

.mount-dialog-footer {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mount-dialog-footer__spacer {
  flex: 1 1 auto;
}
</style>
