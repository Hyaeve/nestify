<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEditing ? '编辑远程挂载' : '添加远程挂载'"
    width="min(600px, calc(100vw - 32px))"
    destroy-on-close
    @update:model-value="handleVisibleChange"
  >
    <el-form label-position="top" class="mount-form" :disabled="loadingSecret || submitting">
      <div class="mount-form__row">
        <el-form-item label="挂载类型" class="mount-form__row-item mount-form__row-item--provider">
          <el-select v-model="form.provider" @change="handleProviderChange">
            <el-option label="WebDAV" value="webdav" />
            <el-option label="OpenList" value="openlist" />
            <el-option label="115 网盘" value="115" />
          </el-select>
        </el-form-item>
        <el-form-item label="挂载名称" class="mount-form__row-item">
          <el-input v-model="form.name" placeholder="例如：移动云盘" maxlength="64" />
        </el-form-item>
      </div>

      <template v-if="is115">
        <el-form-item label="登录方式">
          <el-radio-group v-model="loginMode">
            <el-radio-button value="cookie">CK</el-radio-button>
            <el-radio-button value="qrcode">扫码登录</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <div class="mount-form__row">
          <el-form-item label="CK 对应设备类型" class="mount-form__row-item">
            <el-select v-model="form.device" :loading="loadingDevices">
              <el-option v-for="device in devices" :key="device.value" :label="device.label" :value="device.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="API 请求间隔（毫秒）" class="mount-form__row-item">
            <el-input-number v-model="form.request_interval_ms" :min="0" :max="60000" :step="100" :precision="0" controls-position="right" />
          </el-form-item>
        </div>
        <el-form-item v-if="loginMode === 'cookie'" label="Cookie（CK）">
          <el-input v-model="form.cookie" type="password" show-password autocomplete="new-password" placeholder="UID=…; CID=…; SEID=…; KID=…" :disabled="loadingSecret" />
        </el-form-item>
        <div v-else class="mount-qrcode">
          <img v-if="qrImage" :src="qrImage" width="220" height="220" alt="115 登录二维码" />
          <p role="status" aria-live="polite">{{ qrStatus }}</p>
          <el-button :loading="qrLoading" :disabled="loadingDevices || !devices.length" @click="startQRCode">{{ qrImage ? '重新获取二维码' : '获取二维码' }}</el-button>
        </div>
      </template>

      <template v-else>
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
      </template>

      <el-form-item label="指定路径">
        <el-input v-model="form.base_path" :placeholder="basePathPlaceholder" />
      </el-form-item>

      <el-form-item label="启用挂载">
        <el-switch v-model="form.enabled" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="mount-dialog-footer">
        <el-button :loading="testing" :disabled="loadingSecret || submitting" @click="handleTest">测试</el-button>
        <div class="mount-dialog-footer__spacer" />
        <el-button @click="handleVisibleChange(false)">取消</el-button>
        <el-button type="primary" :loading="submitting" :disabled="loadingSecret || testing" @click="handleSubmit">确认</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'

import {
  OPENLIST_DEFAULT_PORT,
  fetch115Devices,
  start115QRCode,
  poll115QRCode,
  createMount,
  fetchMount,
  testMountConnection,
  updateMount,
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
const is115 = computed(() => form.provider === '115')
const loginMode = ref<'cookie' | 'qrcode'>('cookie')
const devices = ref<{ value: string; label: string }[]>([])
const loadingDevices = ref(false)
const qrImage = ref('')
const qrStatus = ref('')
const qrLoading = ref(false)
let qrGeneration = 0
let dialogGeneration = 0
let qrCookieDevice = ''
let qrTimer: ReturnType<typeof setTimeout> | undefined

function stopQRCode() {
  qrGeneration++
  if (qrTimer) clearTimeout(qrTimer)
  qrTimer = undefined
  qrLoading.value = false
  qrImage.value = ''
  qrStatus.value = ''
}

async function loadDevices() {
  if (loadingDevices.value || devices.value.length) return
  loadingDevices.value = true
  const generation = dialogGeneration
  try {
    const response = await fetch115Devices()
    devices.value = response.data ?? []
  } catch (error) {
    if (generation === dialogGeneration && props.modelValue) ElMessage.error(error instanceof Error ? error.message : '无法加载 115 设备类型')
  } finally {
    loadingDevices.value = false
  }
}

async function startQRCode() {
  stopQRCode()
  const generation = qrGeneration
  qrLoading.value = true
  form.cookie = ''
  try {
    const response = await start115QRCode(form.device, form.request_interval_ms)
    if (generation !== qrGeneration) return
    if (!response.success || !response.data) throw new Error(response.message || '获取二维码失败')
    qrImage.value = response.data.image
    qrStatus.value = '等待扫码'
    const { session_id, expires_at } = response.data
    const poll = async () => {
      if (generation !== qrGeneration) return
      if (Date.now() >= Date.parse(expires_at)) {
        qrStatus.value = '二维码已过期，请重新获取'
        return
      }
      try {
        const result = await poll115QRCode(session_id)
        if (generation !== qrGeneration) return
        if (!result.success || !result.data) throw new Error(result.message || '检查扫码状态失败')
        const { status, cookie, device } = result.data
        if (status === 2 && cookie && device === form.device) {
          form.cookie = cookie
          qrCookieDevice = device
          qrStatus.value = '登录成功'
          return
        }
        if (status < 0) {
          qrStatus.value = status === -2 ? '已取消登录，请重新获取' : '二维码已过期，请重新获取'
          return
        }
        qrStatus.value = status === 1 ? '已扫码，请在手机上确认' : '等待扫码'
        qrTimer = setTimeout(poll, Math.max(2000, form.request_interval_ms))
      } catch (error) {
        if (generation === qrGeneration) qrStatus.value = error instanceof Error ? error.message : '检查扫码状态失败'
      }
    }
    qrTimer = setTimeout(poll, 2000)
  } catch (error) {
    if (generation === qrGeneration) qrStatus.value = error instanceof Error ? error.message : '获取二维码失败'
  } finally {
    if (generation === qrGeneration) qrLoading.value = false
  }
}

onBeforeUnmount(() => { dialogGeneration++; stopQRCode() })

/** OpenList 的 WebDAV 协议端点固定在 /dav，选定该类型时一并填好。 */
const OPENLIST_WEBDAV_ENDPOINT = '/dav'

const form = reactive<MountInput>({
  cookie: '',
  device: 'web',
  request_interval_ms: 1000,
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
watch(() => [form.device, form.provider, loginMode.value], stopQRCode)
watch(() => form.device, (device) => {
  if (qrCookieDevice && qrCookieDevice !== device) {
    form.cookie = ''
    qrCookieDevice = ''
  }
})

const usernamePlaceholder = computed(() => (isOpenList.value ? 'OpenList 用户名' : 'WebDAV 用户名'))

const basePathPlaceholder = computed(() =>
  is115.value ? '网盘内路径，例如 /影视，留空表示根目录' : isOpenList.value ? `协议端点路径，留空即用 ${OPENLIST_WEBDAV_ENDPOINT}` : 'WebDAV 端点路径，例如 /dav，留空表示根目录',
)

watch(
  () => props.modelValue,
  (visible) => {
    dialogGeneration++
    stopQRCode()
    if (!visible) {
      form.cookie = ''
      form.password = ''
      form.token = ''
      return
    }
    const mount = props.mount
    loginMode.value = 'cookie'
    qrCookieDevice = ''
    form.cookie = mount?.cookie ?? ''
    form.device = mount?.device || 'web'
    form.request_interval_ms = mount?.request_interval_ms ?? 1000
    loadingSecret.value = false
    form.name = mount?.name ?? ''
    form.provider = mount?.provider ?? 'webdav'
    if (form.provider === '115') void loadDevices()
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
  stopQRCode()
  form.cookie = ''
  if (next === '115') {
    void loadDevices()
    form.base_path = ''
    return
  }
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
  const generation = dialogGeneration
  loadingSecret.value = true
  try {
    const response = await fetchMount(id)
    if (generation !== dialogGeneration || !props.modelValue || props.mount?.id !== id) return
    form.cookie = response.data?.cookie ?? ''
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
    if (generation === dialogGeneration) loadingSecret.value = false
  }
}

function handleVisibleChange(value: boolean) {
  if (!value) stopQRCode()
  emit('update:modelValue', value)
}

function buildPayload(): MountInput {
  return {
    cookie: is115.value ? form.cookie.trim() : '',
    device: form.device,
    request_interval_ms: Number(form.request_interval_ms) || 0,
    name: form.name.trim(),
    provider: form.provider,
    auth_type: form.auth_type,
    scheme: form.scheme,
    host: form.host.trim(),
    port: Number(form.port) || 0,
    username: form.username.trim(),
    password: is115.value ? '' : form.password,
    token: is115.value ? '' : form.token.trim(),
    base_path: form.base_path.trim(),
    enabled: form.enabled,
  }
}

function validateForm(): string | null {
  if (!form.name.trim()) {
    return '请填写挂载名称'
  }
  if (is115.value) {
    if (!form.cookie.trim()) return '请填写 CK 或完成扫码登录'
    if (!devices.value.some(device => device.value === form.device)) return '请选择设备类型'
    return null
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
    if (!response.success) throw new Error(response.message || '连接测试失败')
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

.mount-qrcode {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-bottom: 18px;
}

.mount-qrcode img {
  max-width: 100%;
  object-fit: contain;
}

.mount-qrcode p {
  margin: 0;
  color: var(--el-text-color-regular);
}

@media (max-width: 540px) {
  .mount-form__row {
    flex-wrap: wrap;
  }
  .mount-form__row-item {
    flex: 1 1 100%;
  }
}
</style>
