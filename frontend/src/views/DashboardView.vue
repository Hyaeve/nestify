<template>
  <div>
    <el-row :gutter="16">
      <el-col :span="8">
        <el-card class="page-card"><strong>总规则数</strong><div>0</div></el-card>
      </el-col>
      <el-col :span="8">
        <el-card class="page-card"><strong>今日处理</strong><div>0</div></el-card>
      </el-col>
      <el-col :span="8">
        <el-card class="page-card"><strong>运行中任务</strong><div>0</div></el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top: 16px;">
      <el-col :span="14">
        <el-card class="page-card">
          <h3 class="page-section-title">当前任务进度</h3>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="后端连接状态">
              <el-tag :type="healthTagType">{{ healthStatus }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="服务标识">
              {{ healthService }}
            </el-descriptions-item>
            <el-descriptions-item label="最近检查时间">
              {{ healthTime }}
            </el-descriptions-item>
          </el-descriptions>

          <el-alert
            v-if="healthError"
            style="margin-top: 16px;"
            type="error"
            :closable="false"
            :title="healthError"
          />
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card class="page-card">
          <h3 class="page-section-title">最近执行摘要</h3>
          <el-empty description="骨架阶段：执行摘要待接入后端接口" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { fetchHealth, type HealthPayload } from '../api/system'

const health = ref<HealthPayload | null>(null)
const healthError = ref('')
const loading = ref(false)

const healthStatus = computed(() => {
  if (loading.value) return '检查中'
  if (health.value) return '已连接'
  return '未连接'
})

const healthService = computed(() => health.value?.service ?? '未知')
const healthTime = computed(() => health.value?.time ?? '尚未获取')
const healthTagType = computed(() => (health.value ? 'success' : loading.value ? 'warning' : 'danger'))

async function loadHealth() {
  loading.value = true
  healthError.value = ''

  try {
    const response = await fetchHealth()
    health.value = response.data ?? null
  } catch (error) {
    health.value = null
    healthError.value = error instanceof Error ? error.message : '后端连接失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadHealth()
})
</script>

