<template>
  <el-card class="page-card">
    <template #header>
      <span>任务日志</span>
    </template>

    <el-form inline>
      <el-form-item>
        <el-input placeholder="按规则名搜索" />
      </el-form-item>
      <el-form-item>
        <el-select placeholder="运行模式" style="width: 140px;" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary">筛选</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="[]" border>
      <el-table-column label="执行时间" min-width="180" />
      <el-table-column label="规则" min-width="160" />
      <el-table-column label="模式" width="120" />
      <el-table-column label="结果" width="120" />
      <el-table-column label="成功/跳过/失败" min-width="180" />
      <el-table-column label="错误摘要" min-width="220" />
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
</script>

