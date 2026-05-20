<template>
  <el-card class="page-card">
    <template #header>
      <span>手动打包</span>
    </template>

    <el-alert
      v-if="errorMessage"
      style="margin-bottom: 16px;"
      type="error"
      :closable="false"
      :title="errorMessage"
    />

    <el-form label-position="top">
      <el-form-item label="目录路径">
        <el-input v-model="directoryPath" placeholder="请选择或输入已挂载目录">
          <template #append>
            <el-button @click="directoryPickerVisible = true">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item>
        <el-button @click="directoryPickerVisible = true">浏览目录</el-button>
        <el-button type="primary" :loading="validating" @click="handleValidate">校验目录</el-button>
        <el-button type="success" :loading="preflighting" @click="handlePreflight">执行预检</el-button>
      </el-form-item>

      <el-descriptions v-if="validation" :column="1" border>
        <el-descriptions-item label="路径">{{ validation.path }}</el-descriptions-item>
        <el-descriptions-item label="允许访问">
          <el-tag :type="validation.allowed ? 'success' : 'danger'">{{ validation.allowed ? '是' : '否' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="目录存在">
          <el-tag :type="validation.exists ? 'success' : 'warning'">{{ validation.exists ? '是' : '否' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="可写入">
          <el-tag :type="validation.writable ? 'success' : 'info'">{{ validation.writable ? '是' : '否' }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>

      <el-descriptions v-if="preflightResult" style="margin-top: 16px;" :column="1" border>
        <el-descriptions-item label="输出目录">{{ preflightResult.output_dir }}</el-descriptions-item>
        <el-descriptions-item label="允许执行">
          <el-tag :type="preflightResult.allowed ? 'success' : 'danger'">{{ preflightResult.allowed ? '是' : '否' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="图片数量">{{ preflightResult.image_count }}</el-descriptions-item>
        <el-descriptions-item label="存在嵌套子目录">
          <el-tag :type="preflightResult.has_nested_dirs ? 'warning' : 'success'">{{ preflightResult.has_nested_dirs ? '是' : '否' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="存在非图片文件">
          <el-tag :type="preflightResult.has_non_image_files ? 'warning' : 'success'">{{ preflightResult.has_non_image_files ? '是' : '否' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="拒绝原因">
          <span v-if="preflightResult.rejected_reasons.length === 0">无</span>
          <el-space v-else wrap>
            <el-tag v-for="reason in preflightResult.rejected_reasons" :key="reason" type="danger">{{ reason }}</el-tag>
          </el-space>
        </el-descriptions-item>
      </el-descriptions>

      <el-descriptions v-if="latestRun" style="margin-top: 16px;" :column="1" border>
        <el-descriptions-item label="运行 ID">{{ latestRun.id }}</el-descriptions-item>
        <el-descriptions-item label="触发模式">{{ latestRun.trigger_mode }}</el-descriptions-item>
        <el-descriptions-item label="当前状态">{{ latestRun.status }}</el-descriptions-item>
        <el-descriptions-item label="当前阶段">{{ latestRun.stage }}</el-descriptions-item>
        <el-descriptions-item label="最近日志">
          <el-space direction="vertical" alignment="start">
            <span v-for="log in latestLogs" :key="log.id">[{{ log.level }}] {{ log.message }}</span>
          </el-space>
        </el-descriptions-item>
      </el-descriptions>
    </el-form>
  </el-card>

  <DirectoryPickerDialog
    v-model="directoryPickerVisible"
    title="选择手动打包目录"
    :initial-path="directoryPath"
    @selected="handleDirectorySelected"
  />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

import { fetchRunLogs, prepareManualPreflight, type ManualPreflightResult, type RunInstance, type RunLogEntry } from '../api/executions'
import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import { validateDirectory, type ValidatePathPayload } from '../api/paths'

const directoryPath = ref('')
const directoryPickerVisible = ref(false)
const validating = ref(false)
const preflighting = ref(false)
const errorMessage = ref('')
const validation = ref<ValidatePathPayload | null>(null)
const preflightResult = ref<ManualPreflightResult | null>(null)
const latestRun = ref<RunInstance | null>(null)
const latestLogs = ref<RunLogEntry[]>([])

function handleDirectorySelected(path: string) {
  directoryPath.value = path
  directoryPickerVisible.value = false
}

async function handleValidate() {
  if (!directoryPath.value) {
    errorMessage.value = '请先选择目录'
    return
  }

  validating.value = true
  errorMessage.value = ''

  try {
    const response = await validateDirectory(directoryPath.value)
    validation.value = response.data ?? null
    ElMessage.success('目录校验完成')
  } catch (error) {
    validation.value = null
    errorMessage.value = error instanceof Error ? error.message : '目录校验失败'
  } finally {
    validating.value = false
  }
}

async function handlePreflight() {
  if (!directoryPath.value) {
    errorMessage.value = '请先选择目录'
    return
  }

  preflighting.value = true
  errorMessage.value = ''

  try {
    const response = await prepareManualPreflight(directoryPath.value)
    preflightResult.value = response.data?.preflight ?? null
    latestRun.value = response.data?.run ?? null

    if (latestRun.value) {
      const logsResponse = await fetchRunLogs(latestRun.value.id)
      latestLogs.value = logsResponse.data?.items ?? []
    }

    ElMessage.success('手动打包预检完成')
  } catch (error) {
    preflightResult.value = null
    latestRun.value = null
    latestLogs.value = []
    errorMessage.value = error instanceof Error ? error.message : '手动打包预检失败'
  } finally {
    preflighting.value = false
  }
}
</script>

