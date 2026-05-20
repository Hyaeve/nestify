<template>
  <el-card class="page-card">
    <template #header>
      <span>系统设置</span>
    </template>

    <el-tabs>
      <el-tab-pane label="基础设置">
        <el-form label-position="top">
          <el-form-item label="时区">
            <el-input value="Asia/Shanghai" />
          </el-form-item>
          <el-form-item label="日志保留天数">
            <el-input value="30" />
          </el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="管理员账号">
        <el-empty description="骨架阶段：待接入账号修改接口" />
      </el-tab-pane>
    </el-tabs>
  </el-card>
</template>

<script setup lang="ts">
</script>
