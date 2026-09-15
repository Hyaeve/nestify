<template>
  <div ref="rulesPageRef" class="rules-page">
    <div class="rules-tabs">
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'rules' }" @click="switchTab('rules')">归档规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'purify' }" @click="switchTab('purify')">净化规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'link' }" @click="switchTab('link')">链路规则</button>
      <button type="button" class="rules-tabs__item rules-tabs__item--naming" :class="{ 'is-active': activeTab === 'naming' }" @click="switchTab('naming')">命名规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'backup' }" @click="switchTab('backup')">备份规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'history' }" @click="switchTab('history')">归巢历史</button>
    </div>

    <el-alert v-if="errorMessage" :closable="false" type="error" :title="errorMessage" class="rules-error" />

    <el-card v-show="activeTab === 'rules'" class="page-card rules-card">
      <template #header>
        <div class="rules-card__header">
          <div>
            <div class="rules-card__title">归档规则</div>
          </div>
          <el-button type="primary" round @click="openCreateDialog">+ 添加规则</el-button>
        </div>
      </template>

      <div v-if="archiveRules.length" ref="archiveGridRef" v-loading="archiveLoading" class="rule-card-grid">
        <RuleCard
          v-for="rule in archiveRules"
          :key="rule.id"
          :rule="rule"
          :mode-label="rule.archive_mode === 'package' ? '打包' : '收集'"
          :mode-class="rule.archive_mode === 'package' ? 'custom-mode-tag--package' : 'custom-mode-tag--collect'"
          :paths="archiveRulePaths(rule)"
          :busy="isRuleStatusUpdating(rule.id)"
          :cron-items="getCronPreviewItems(rule.id)"
          :cron-loading="cronPreviewLoadingRuleId === rule.id"
          :cron-error="cronPreviewErrorRuleId === rule.id ? cronPreviewErrorMessage || '预览失败' : ''"
          @edit="openRuleCardEdit"
          @execute="prepareExecution(rule.id)"
          @remove="removeRule(rule.id, 'archive')"
          @toggle="toggleRuleEnabled(rule)"
          @cron-preview="handleCronPreviewShow(rule.id, rule.cron_expression)"
          @contextmenu="(item, event) => handleRuleContextMenu(item, 'archive', event)"
        />
      </div>

      <el-empty v-else description="暂无归档规则，可添加打包或收集规则" />

      <div v-if="archiveRulesTotal > 0" class="history-pagination">
        <el-pagination
          v-model:current-page="archiveRulesCurrentPage"
          v-model:page-size="archiveRulesPageSize"
          background
          layout="total, sizes, prev, pager, next"
          :page-sizes="pageSizeOptions"
          :total="archiveRulesTotal"
          @current-change="handleArchiveRulesPageChange"
          @size-change="handleArchiveRulesPageSizeChange"
        />
      </div>
    </el-card>

    <el-card v-show="activeTab === 'history'" class="page-card history-card">
      <template #header>
        <div class="rules-card__header">
          <div class="rules-card__title">归巢历史</div>
          <div class="history-actions">
            <el-button type="success" plain @click="clearHistory('success')">删除成功</el-button>
            <el-button type="warning" plain @click="clearHistory('skip')">删除跳过</el-button>
            <el-button type="danger" plain @click="clearHistory('failed')">删除失败</el-button>
          </div>
        </div>
      </template>

        <div class="history-toolbar">
          <div class="history-summary">
            <span>累计 {{ historySummary.total }}</span>
            <span>今日 {{ historySummary.today }}</span>
            <span>成功 {{ successCount }}</span>
            <span>跳过 {{ skipCount }}</span>
            <span>失败 {{ failedCount }}</span>
            <span class="history-summary__controls">
              <el-select v-model="historySortBy" size="small" class="history-summary__control" @change="handleHistorySortChange">
                <el-option label="修改时间" value="modified_at" />
                <el-option label="文件名称" value="name" />
              </el-select>
              <el-tooltip :content="historySortOrder === 'asc' ? '正序' : '倒序'" placement="top" :show-after="300">
                <el-button class="history-sort-order-button" circle :aria-label="historySortOrder === 'asc' ? '正序' : '倒序'" @click="toggleHistorySortOrder">
                  <svg viewBox="0 0 24 24" aria-hidden="true" class="history-sort-order-button__icon">
                    <path d="M7 5.2v13.6" />
                    <path v-if="historySortOrder === 'asc'" d="M3.9 8.35 7 5.2l3.1 3.15" />
                    <path v-else d="m3.9 15.65 3.1 3.15 3.1-3.15" />
                    <path d="M13 7h7" />
                    <path d="M13 12h5.2" />
                    <path d="M13 17h3.4" />
                  </svg>
                </el-button>
              </el-tooltip>
              <el-select v-model="historyStatusFilter" size="small" class="history-summary__control" @change="handleHistoryStatusChange">
                <el-option label="全部状态" value="all" />
                <el-option label="成功" value="success" />
                <el-option label="失败" value="failed" />
                <el-option label="跳过" value="skip" />
              </el-select>
              <el-select v-model="historyRuleTypeFilter" size="small" class="history-summary__control" @change="handleHistoryRuleTypeChange">
                <el-option label="全部规则" value="all" />
                <el-option label="归档规则" value="archive" />
                <el-option label="净化规则" value="cleanup" />
                <el-option label="链路规则" value="link" />
                <el-option label="命名规则" value="naming" />
                <el-option label="备份规则" value="backup" />
              </el-select>
            </span>
          </div>
          <div class="history-search">
            <el-input
              v-model="historyKeywordInput"
              clearable
              placeholder="关键词搜索（规则名 / 摘要）"
              @keyup.enter="handleHistorySearch"
            />
            <el-button type="primary" @click="handleHistorySearch">搜索</el-button>
            <el-button @click="resetHistorySearch">重置</el-button>
          </div>
        </div>

        <el-table v-if="historyViewMode === 'flat'" v-loading="historyLoading" :data="historyItems" class="rules-table" table-layout="auto">
          <el-table-column label="规则 / 摘要" min-width="360">
            <template #default="scope">
              <div class="history-rule">
                <div class="history-rule__title">{{ scope.row.rule_name || '未知规则' }}</div>
                <div class="history-rule__desc">{{ formatRunHistorySummary(scope.row.summary) || '—' }}</div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="模式" width="120">
            <template #default="scope">
              <span class="custom-mode-tag" :class="historyModeTagClass(scope.row)">{{ historyModeLabel(scope.row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="scope">
              <span class="history-status" :class="`is-${scope.row.status}`">{{ historyStatusText(scope.row.status) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="统计" width="140">
            <template #default="scope">{{ scope.row.success_count }}/{{ scope.row.skip_count }}/{{ scope.row.failure_count }}</template>
          </el-table-column>
          <el-table-column label="时间" min-width="180">
            <template #default="scope">{{ formatDateTime(scope.row.started_at) }}</template>
          </el-table-column>
          <el-table-column label="大小" width="120">
            <template #default="scope">{{ formatHistorySize(scope.row.size_bytes) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="130">
            <template #default="scope">
              <el-button link type="primary" @click="openHistoryItemDetail(scope.row)">详情</el-button>
              <el-button link type="danger" @click="removeHistoryItem(scope.row.id)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-table v-else v-loading="historyLoading" :data="historyTreeRows" class="rules-table history-tree-table" row-key="id" table-layout="auto" @row-click="openHistoryDetailDialog">
          <el-table-column label="折叠任务" min-width="420">
            <template #default="scope">
              <button type="button" class="history-detail-card" @click.stop="openHistoryDetailDialog(scope.row)">
                <span class="history-detail-card__title">{{ scope.row.title }}</span>
                <span class="history-detail-card__desc">{{ scope.row.description }}</span>
              </button>
            </template>
          </el-table-column>
          <el-table-column label="模式" width="120">
            <template #default="scope">
              <span class="custom-mode-tag" :class="historyModeTagClass(scope.row)">{{ historyModeLabel(scope.row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="scope">
              <span class="history-status" :class="`is-${scope.row.status}`">{{ historyStatusText(scope.row.status) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="数量" width="120">
            <template #default="scope">{{ scope.row.processed_files }}</template>
          </el-table-column>
          <el-table-column label="统计" width="140">
            <template #default="scope">{{ scope.row.success_count }}/{{ scope.row.skip_count }}/{{ scope.row.failure_count }}</template>
          </el-table-column>
          <el-table-column label="时间" min-width="180">
            <template #default="scope">{{ formatDateTime(scope.row.started_at) }}</template>
          </el-table-column>
          <el-table-column label="大小" width="120">
            <template #default="scope">{{ formatHistorySize(scope.row.size_bytes) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="90">
            <template #default="scope">
              <el-button link type="primary" @click.stop="openHistoryDetailDialog(scope.row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-dialog v-model="historyDetailDialogVisible" class="history-detail-dialog" title="任务详情" width="920px" destroy-on-close>
          <template v-if="selectedHistoryGroup">
            <div class="detail-dialog-summary">
              <div>
                <div class="detail-dialog-summary__title">{{ selectedHistoryGroup.title }}</div>
                <div class="detail-dialog-summary__desc">{{ selectedHistoryGroup.description }}</div>
              </div>
              <div class="detail-dialog-summary__tags">
                <span class="custom-mode-tag" :class="historyModeTagClass(selectedHistoryGroup)">{{ historyModeLabel(selectedHistoryGroup) }}</span>
                <span class="history-status" :class="`is-${selectedHistoryGroup.status}`">{{ historyStatusText(selectedHistoryGroup.status) }}</span>
              </div>
            </div>
            <div v-if="selectedBackupDetailRows.length" class="detail-backup">
              <div class="detail-backup__head">
                <span class="detail-backup__title">备份详情</span>
                <span v-if="!selectedBackupTask" class="detail-backup__hint">对应的备份规则卡片已不存在，仅展示本次执行记录</span>
              </div>
              <div class="detail-backup__grid">
                <div v-for="row in selectedBackupDetailRows" :key="row.label" class="detail-backup__row">
                  <span class="detail-backup__label">{{ row.label }}</span>
                  <div class="detail-backup__values">
                    <span v-for="(value, index) in row.values" :key="`${row.label}-${index}`" class="detail-backup__value" :title="value">{{ value }}</span>
                  </div>
                </div>
              </div>
            </div>
            <BackupFileList :manifest="selectedBackupManifest" />
            <el-table :data="pagedHistoryDetailRows" class="rules-table detail-dialog-table" table-layout="auto" empty-text="暂无明细">
              <el-table-column label="条目" min-width="360">
                <template #default="scope">
                  <div class="history-rule history-rule--child">
                    <div class="history-rule__title">{{ scope.row.title }}</div>
                    <div class="history-rule__desc">{{ scope.row.description }}</div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="scope">
                  <span class="history-status" :class="`is-${scope.row.status}`">{{ historyStatusText(scope.row.status) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="统计" width="140">
                <template #default="scope">{{ scope.row.success_count }}/{{ scope.row.skip_count }}/{{ scope.row.failure_count }}</template>
              </el-table-column>
              <el-table-column label="大小" width="120">
                <template #default="scope">{{ formatHistorySize(scope.row.size_bytes) }}</template>
              </el-table-column>
              <el-table-column label="操作" width="90">
                <template #default="scope">
                  <el-button link type="danger" @click="removeHistoryDetailItem(scope.row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div v-if="selectedHistoryGroupChildren.length > historyDetailPageSize" class="detail-dialog-pagination">
              <el-pagination
                v-model:current-page="historyDetailCurrentPage"
                background
                layout="total, prev, pager, next"
                :page-size="historyDetailPageSize"
                :total="selectedHistoryGroupChildren.length"
              />
            </div>
          </template>
        </el-dialog>

      <div v-if="historyTotal > 0" class="history-pagination">
        <el-pagination
          v-model:current-page="historyCurrentPage"
          v-model:page-size="historyPageSize"
          background
          layout="total, sizes, prev, pager, next"
          :page-sizes="pageSizeOptions"
          :total="historyTotal"
          @current-change="handleHistoryPageChange"
          @size-change="handleHistoryPageSizeChange"
        />
      </div>
    </el-card>

    <el-card v-show="activeTab === 'purify'" class="page-card rules-card">
      <template #header>
        <div class="rules-card__header">
          <div>
            <div class="rules-card__title">净化规则</div>
          </div>
          <el-button type="primary" round @click="openCreatePurifyDialog">+ 添加规则</el-button>
        </div>
      </template>

      <div v-if="purifyRules.length" ref="purifyGridRef" v-loading="purifyLoading" class="rule-card-grid">
        <RuleCard
          v-for="rule in purifyRules"
          :key="rule.id"
          :rule="rule"
          :mode-label="rule.archive_mode === 'transform' ? '转换' : '清理'"
          :mode-class="rule.archive_mode === 'transform' ? 'custom-mode-tag--transform' : 'custom-mode-tag--cleanup'"
          :paths="purifyRulePaths(rule)"
          :busy="isRuleStatusUpdating(rule.id)"
          :cron-items="getCronPreviewItems(rule.id)"
          :cron-loading="cronPreviewLoadingRuleId === rule.id"
          :cron-error="cronPreviewErrorRuleId === rule.id ? cronPreviewErrorMessage || '预览失败' : ''"
          @edit="openRuleCardEdit"
          @execute="prepareExecution(rule.id)"
          @remove="removeRule(rule.id, 'cleanup')"
          @toggle="toggleRuleEnabled(rule)"
          @cron-preview="handleCronPreviewShow(rule.id, rule.cron_expression)"
          @contextmenu="(item, event) => handleRuleContextMenu(item, 'cleanup', event)"
        />
      </div>

      <div v-if="purifyRulesTotal > 0" class="history-pagination">
        <el-pagination
          v-model:current-page="purifyRulesCurrentPage"
          v-model:page-size="purifyRulesPageSize"
          background
          layout="total, sizes, prev, pager, next"
          :page-sizes="pageSizeOptions"
          :total="purifyRulesTotal"
          @current-change="handlePurifyRulesPageChange"
          @size-change="handlePurifyRulesPageSizeChange"
        />
      </div>

      <el-empty v-else description="暂无净化规则，可添加清理模式或转换模式规则" />
    </el-card>

    <el-card v-show="activeTab === 'link'" class="page-card rules-card">
      <template #header>
        <div class="rules-card__header">
          <div>
            <div class="rules-card__title">链路规则</div>
          </div>
          <el-button type="primary" round @click="openCreateLinkDialog">+ 添加规则</el-button>
        </div>
      </template>

      <div v-if="linkRules.length" ref="linkGridRef" v-loading="linkLoading" class="rule-card-grid">
        <RuleCard
          v-for="rule in linkRules"
          :key="rule.id"
          :rule="rule"
          :mode-label="historyModeLabel({ archive_mode: 'link', link_mode: rule.link_mode })"
          :mode-class="historyModeTagClass({ archive_mode: 'link', link_mode: rule.link_mode })"
          :paths="linkRulePaths(rule)"
          :busy="isRuleStatusUpdating(rule.id)"
          :cron-items="getCronPreviewItems(rule.id)"
          :cron-loading="cronPreviewLoadingRuleId === rule.id"
          :cron-error="cronPreviewErrorRuleId === rule.id ? cronPreviewErrorMessage || '预览失败' : ''"
          :strm-sync="rule.link_mode === 'strm'"
          @edit="openRuleCardEdit"
          @execute="prepareExecution(rule.id)"
          @strm-sync="(item, command) => handleStrmSyncCommand(item.id, command)"
          @remove="removeRule(rule.id, 'link')"
          @toggle="toggleRuleEnabled(rule)"
          @cron-preview="handleCronPreviewShow(rule.id, rule.cron_expression)"
          @contextmenu="(item, event) => handleRuleContextMenu(item, 'link', event)"
        />
      </div>

      <div v-if="linkRulesTotal > 0" class="history-pagination">
        <el-pagination
          v-model:current-page="linkRulesCurrentPage"
          v-model:page-size="linkRulesPageSize"
          background
          layout="total, sizes, prev, pager, next"
          :page-sizes="pageSizeOptions"
          :total="linkRulesTotal"
          @current-change="handleLinkRulesPageChange"
          @size-change="handleLinkRulesPageSizeChange"
        />
      </div>

      <el-empty v-else description="暂无链路规则，可添加软链或硬链规则" />
    </el-card>

    <el-card v-show="activeTab === 'naming'" class="page-card rules-card naming-rules-card">
      <template #header><div class="rules-card__header"><div class="rules-card__title">命名规则</div><el-button class="naming-add-button" round @click="openCreateNamingDialog">+ 添加规则</el-button></div></template>
      <div v-if="namingRules.length" ref="namingGridRef" v-loading="namingLoading" class="rule-card-grid">
        <RuleCard
          v-for="rule in namingRules"
          :key="rule.id"
          :rule="rule"
          mode-label="命名"
          mode-class="custom-mode-tag--naming"
          :paths="namingRulePaths(rule)"
          :extra-tags="[namingRuleSetName(rule)]"
          :busy="isRuleStatusUpdating(rule.id)"
          :cron-items="getCronPreviewItems(rule.id)"
          :cron-loading="cronPreviewLoadingRuleId === rule.id"
          :cron-error="cronPreviewErrorRuleId === rule.id ? cronPreviewErrorMessage || '预览失败' : ''"
          @edit="openRuleCardEdit"
          @execute="prepareExecution(rule.id)"
          @remove="removeRule(rule.id, 'naming')"
          @toggle="toggleRuleEnabled(rule)"
          @cron-preview="handleCronPreviewShow(rule.id, rule.cron_expression)"
          @contextmenu="(item, event) => handleRuleContextMenu(item, 'naming', event)"
        />
      </div>
      <el-empty v-if="!namingLoading && namingRules.length === 0" description="暂无命名规则，可选择命名工坊规则或规则集" />
    </el-card>

    <BackupRulesPanel :visible="activeTab === 'backup'" />

    <el-dialog v-model="createNamingDialogVisible" :title="editingNamingRuleID ? '编辑命名规则' : '新增命名规则'" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createNamingForm.name" /></el-form-item>
        <el-form-item label="监控路径"><el-input v-model="createNamingForm.source_dir"><template #append><el-button @click="openDirectoryPicker('createNaming', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="命名工坊规则或规则集"><el-select v-model="createNamingForm.rule_set_id" placeholder="请选择规则集" style="width:100%"><el-option v-for="set in availableNamingRuleSets" :key="set.id" :label="`${set.name}（${set.rules.length} 条）`" :value="set.id" /></el-select></el-form-item>
        <button type="button" class="mode-config-toggle" @click="namingScopeOptionsExpanded = !namingScopeOptionsExpanded">
          <div><div class="mode-config-panel__title">功能模块</div></div>
          <div class="mode-config-toggle__meta"><el-tag type="primary">当前模式</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': namingScopeOptionsExpanded }">⌄</span></div>
        </button>
        <el-collapse-transition>
          <div v-show="namingScopeOptionsExpanded" class="mode-config-panel naming-scope-panel">
            <el-row :gutter="12">
              <el-col :span="12"><label class="mode-option-card mode-option-card--compact naming-scope-card"><el-checkbox v-model="createNamingForm.options.naming_include_dirs">包含文件夹</el-checkbox><span class="mode-option-card__description">将规则集应用于监控路径下的文件夹</span></label></el-col>
              <el-col :span="12"><label class="mode-option-card mode-option-card--compact naming-scope-card"><el-checkbox v-model="createNamingForm.options.naming_include_files">包含文件</el-checkbox><span class="mode-option-card__description">将规则集应用于监控路径下的文件</span></label></el-col>
            </el-row>
          </div>
        </el-collapse-transition>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="新文件触发"><el-switch v-model="createNamingForm.monitor_enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="计划执行"><el-switch v-model="createNamingForm.schedule_enabled" /></el-form-item></el-col></el-row>
        <el-form-item v-if="createNamingForm.schedule_enabled" label="计划表达式"><el-input v-model="createNamingForm.cron_expression" placeholder="例如：0 8 * * *" /></el-form-item>
        <el-form-item label="启用规则"><el-switch v-model="createNamingForm.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="createNamingDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateNamingRule">{{ editingNamingRuleID ? '保存' : '创建' }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="createDialogVisible" title="新增规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="归档模式"><el-radio-group v-model="createForm.archive_mode" :class="['archive-mode-group', `archive-mode-group--${createForm.archive_mode}`]"><el-radio-button value="package">打包模式</el-radio-button><el-radio-button value="collect">收集模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="createForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="createForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="createForm.compatibility_mode" class="uniform-mode-group">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="createArchiveOptionsExpanded = !createArchiveOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getModeTitle(createForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">当前模式</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': createArchiveOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="createArchiveOptionsExpanded" class="mode-config-panel">
              <el-row v-if="createForm.archive_mode === 'package'" :gutter="12"><el-col v-for="option in packageModeOptions" :key="option.key" :span="12"><el-tooltip placement="top" effect="light" :show-after="750" popper-class="mode-option-tooltip"><template #content><div class="mode-option-tooltip__content">{{ formatModeOptionDescription(option.description) }}</div></template><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="createForm.package_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
              <el-row v-else :gutter="12"><el-col v-for="option in collectModeOptions" :key="option.key" :span="12"><el-tooltip placement="top" effect="light" :show-after="750" popper-class="mode-option-tooltip"><template #content><div class="mode-option-tooltip__content">{{ formatModeOptionDescription(option.description) }}</div></template><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="createForm.collect_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="createForm.schedule_enabled" label="计划表达式"><el-input v-model="createForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="createForm.source_dir"><template #append><el-button @click="openDirectoryPicker('create', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="createForm.target_dir"><template #append><el-button @click="openDirectoryPicker('create', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <template v-if="createForm.archive_mode === 'package' && createForm.package_options.match_archive">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配归档</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="createForm.match_filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" />
          </el-form-item>
        </template>
        <template v-if="createForm.archive_mode === 'package' && createForm.package_options.single_file_nesting">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">单件归巢</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="createForm.nest_filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" />
          </el-form-item>
        </template>
        <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
          <div><div class="mode-config-panel__title">过滤清除</div></div>
          <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
        </button>
        <el-form-item class="transform-section-input"><el-input v-model="createForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        <template v-if="false">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤清除</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="createForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="createForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editDialogVisible" title="编辑规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="editForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="归档模式"><el-radio-group v-model="editForm.archive_mode" :class="['archive-mode-group', `archive-mode-group--${editForm.archive_mode}`]"><el-radio-button value="package">打包模式</el-radio-button><el-radio-button value="collect">收集模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="editForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="editForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="editForm.compatibility_mode" class="uniform-mode-group">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="editArchiveOptionsExpanded = !editArchiveOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getModeTitle(editForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">当前模式</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': editArchiveOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="editArchiveOptionsExpanded" class="mode-config-panel">
              <el-row v-if="editForm.archive_mode === 'package'" :gutter="12"><el-col v-for="option in packageModeOptions" :key="option.key" :span="12"><el-tooltip placement="top" effect="light" :show-after="750" popper-class="mode-option-tooltip"><template #content><div class="mode-option-tooltip__content">{{ formatModeOptionDescription(option.description) }}</div></template><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="editForm.package_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
              <el-row v-else :gutter="12"><el-col v-for="option in collectModeOptions" :key="option.key" :span="12"><el-tooltip placement="top" effect="light" :show-after="750" popper-class="mode-option-tooltip"><template #content><div class="mode-option-tooltip__content">{{ formatModeOptionDescription(option.description) }}</div></template><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="editForm.collect_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="editForm.schedule_enabled" label="计划表达式"><el-input v-model="editForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="editForm.source_dir"><template #append><el-button @click="openDirectoryPicker('edit', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="editForm.target_dir"><template #append><el-button @click="openDirectoryPicker('edit', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <template v-if="editForm.archive_mode === 'package' && editForm.package_options.match_archive">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配归档</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="editForm.match_filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" />
          </el-form-item>
        </template>
        <template v-if="editForm.archive_mode === 'package' && editForm.package_options.single_file_nesting">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">单件归巢</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="editForm.nest_filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" />
          </el-form-item>
        </template>
        <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
          <div><div class="mode-config-panel__title">过滤清除</div></div>
          <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
        </button>
        <el-form-item class="transform-section-input"><el-input v-model="editForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        <template v-if="false">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤清除</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="editForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="editForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editDialogVisible = false">取消</el-button><el-button type="primary" :loading="editing" @click="submitUpdateRule">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="createPurifyDialogVisible" title="新增净化规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createPurifyForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则模式"><el-radio-group v-model="createPurifyForm.archive_mode" :class="['archive-mode-group', `archive-mode-group--${createPurifyForm.archive_mode}`]"><el-radio-button value="cleanup">清理模式</el-radio-button><el-radio-button value="transform">转换模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="createPurifyForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="createPurifyForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="createPurifyForm.compatibility_mode" class="uniform-mode-group">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="createPurifyOptionsExpanded = !createPurifyOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getPurifyModeTitle(createPurifyForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag :type="createPurifyForm.archive_mode === 'transform' ? 'primary' : 'warning'">{{ createPurifyForm.archive_mode === 'transform' ? '转换模式' : '清理模式' }}</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': createPurifyOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="createPurifyOptionsExpanded" class="mode-config-panel">
              <el-row :gutter="12"><el-col v-for="option in getPurifyModeOptions(createPurifyForm.archive_mode)" :key="option.key" :span="12"><el-tooltip placement="top" effect="light" :show-after="750" popper-class="mode-option-tooltip"><template #content><div class="mode-option-tooltip__content">{{ formatModeOptionDescription(option.description) }}</div></template><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="createPurifyForm.options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="createPurifyForm.schedule_enabled" label="计划表达式"><el-input v-model="createPurifyForm.cron_expression" /></el-form-item>
        <el-form-item label="监控目录">
          <div class="source-dir-editor">
            <div v-if="createPurifyForm.source_dirs.length" class="source-dir-editor__list">
              <el-tag v-for="dir in createPurifyForm.source_dirs" :key="dir" class="source-dir-editor__tag" closable @close="removeCreatePurifySourceDir(dir)">{{ dir }}</el-tag>
            </div>
            <el-empty v-else class="source-dir-editor__empty" description="暂未选择监控目录" :image-size="56" />
            <el-button type="primary" plain @click="openDirectoryPicker('createPurify', 'source_dir')">选择目录</el-button>
          </div>
        </el-form-item>
        <template v-if="createPurifyForm.archive_mode === 'cleanup' && createPurifyForm.options.cleanup_matching_files">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配清理</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="createPurifyForm.filters_text" type="textarea" :rows="10" :placeholder="cleanupRuleMatcherPlaceholder" />
          </el-form-item>
        </template>
        <template v-if="createPurifyForm.archive_mode === 'cleanup' && createPurifyForm.options.cleanup_empty_dirs">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">白名单匹配</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">匹配规则</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="createPurifyForm.whitelist_text" type="textarea" :rows="6" placeholder="一行一个文件夹全称；命中的这一层文件夹不会因空目录清理而删除，子文件夹若不在白名单内仍会继续清理。" />
          </el-form-item>
        </template>
        <template v-if="createPurifyForm.archive_mode === 'cleanup' && createPurifyForm.options.cleanup_expired_files">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过期清除</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">保留设置</el-tag></div>
          </button>
          <el-form-item label="保留天数" class="cleanup-retention-input">
            <el-input-number v-model="createPurifyForm.option_values.cleanup_retention_days" :min="1" :max="36500" controls-position="right" />
          </el-form-item>
        </template>
        <template v-if="createPurifyForm.archive_mode === 'transform' && createPurifyForm.options.convert_matching_text">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配转换</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">转换规则</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="createPurifyForm.transform_rules_text" type="textarea" :rows="10" placeholder="一行一条。默认：待转换 => 转换词（匹配文件名）；文件夹名：/待转换/ => /转换词/；支持关键词部分匹配，也可用正则实现整段替换。" />
          </el-form-item>
        </template>
        <template v-if="createPurifyForm.archive_mode === 'transform' && createPurifyForm.options.filter_matching_text">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配过滤</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">过滤规则</el-tag></div>
          </button>
          <el-form-item class="transform-section-input transform-section-input--filters">
            <el-input v-model="createPurifyForm.transform_filters_text" type="textarea" :rows="6" placeholder="支持关键词匹配和正则匹配&#10;文件字段过滤：匹配词&#10;文件夹字段过滤：&lt;-匹配词-&gt;" />
          </el-form-item>
        </template>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createPurifyForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="createPurifyForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createPurifyDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreatePurifyRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editPurifyDialogVisible" title="编辑净化规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="editPurifyForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则模式"><el-radio-group v-model="editPurifyForm.archive_mode" :class="['archive-mode-group', `archive-mode-group--${editPurifyForm.archive_mode}`]"><el-radio-button value="cleanup">清理模式</el-radio-button><el-radio-button value="transform">转换模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="editPurifyForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="editPurifyForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="editPurifyForm.compatibility_mode" class="uniform-mode-group">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="editPurifyOptionsExpanded = !editPurifyOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getPurifyModeTitle(editPurifyForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag :type="editPurifyForm.archive_mode === 'transform' ? 'primary' : 'warning'">{{ editPurifyForm.archive_mode === 'transform' ? '转换模式' : '清理模式' }}</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': editPurifyOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="editPurifyOptionsExpanded" class="mode-config-panel">
              <el-row :gutter="12"><el-col v-for="option in getPurifyModeOptions(editPurifyForm.archive_mode)" :key="option.key" :span="12"><el-tooltip placement="top" effect="light" :show-after="750" popper-class="mode-option-tooltip"><template #content><div class="mode-option-tooltip__content">{{ formatModeOptionDescription(option.description) }}</div></template><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="editPurifyForm.options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="editPurifyForm.schedule_enabled" label="计划表达式"><el-input v-model="editPurifyForm.cron_expression" /></el-form-item>
        <el-form-item label="监控目录">
          <div class="source-dir-editor">
            <div v-if="editPurifyForm.source_dirs.length" class="source-dir-editor__list">
              <el-tag v-for="dir in editPurifyForm.source_dirs" :key="dir" class="source-dir-editor__tag" closable @close="removeEditPurifySourceDir(dir)">{{ dir }}</el-tag>
            </div>
            <el-empty v-else class="source-dir-editor__empty" description="暂未选择监控目录" :image-size="56" />
            <el-button type="primary" plain @click="openDirectoryPicker('editPurify', 'source_dir')">选择目录</el-button>
          </div>
        </el-form-item>
        <template v-if="editPurifyForm.archive_mode === 'cleanup' && editPurifyForm.options.cleanup_matching_files">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配清理</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="editPurifyForm.filters_text" type="textarea" :rows="10" :placeholder="cleanupRuleMatcherPlaceholder" />
          </el-form-item>
        </template>
        <template v-if="editPurifyForm.archive_mode === 'cleanup' && editPurifyForm.options.cleanup_empty_dirs">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">白名单匹配</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">匹配规则</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="editPurifyForm.whitelist_text" type="textarea" :rows="6" placeholder="一行一个文件夹全称；命中的这一层文件夹不会因空目录清理而删除，子文件夹若不在白名单内仍会继续清理。" />
          </el-form-item>
        </template>
        <template v-if="editPurifyForm.archive_mode === 'cleanup' && editPurifyForm.options.cleanup_expired_files">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过期清除</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">保留设置</el-tag></div>
          </button>
          <el-form-item label="保留天数" class="cleanup-retention-input">
            <el-input-number v-model="editPurifyForm.option_values.cleanup_retention_days" :min="1" :max="36500" controls-position="right" />
          </el-form-item>
        </template>
        <template v-if="editPurifyForm.archive_mode === 'transform' && editPurifyForm.options.convert_matching_text">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配转换</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">转换规则</el-tag></div>
          </button>
          <el-form-item class="transform-section-input">
            <el-input v-model="editPurifyForm.transform_rules_text" type="textarea" :rows="10" placeholder="一行一条。默认：待转换 => 转换词（匹配文件名）；文件夹名：/待转换/ => /转换词/；支持关键词部分匹配，也可用正则实现整段替换。" />
          </el-form-item>
        </template>
        <template v-if="editPurifyForm.archive_mode === 'transform' && editPurifyForm.options.filter_matching_text">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">匹配过滤</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">过滤规则</el-tag></div>
          </button>
          <el-form-item class="transform-section-input transform-section-input--filters">
            <el-input v-model="editPurifyForm.transform_filters_text" type="textarea" :rows="6" placeholder="支持关键词匹配和正则匹配&#10;文件字段过滤：匹配词&#10;文件夹字段过滤：&lt;-匹配词-&gt;" />
          </el-form-item>
        </template>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editPurifyForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="editPurifyForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editPurifyDialogVisible = false">取消</el-button><el-button type="primary" :loading="editing" @click="submitUpdatePurifyRule">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="createLinkDialogVisible" title="新增链路规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createLinkForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="链路模式"><el-radio-group v-model="createLinkForm.link_mode" :class="['archive-mode-group', `archive-mode-group--${createLinkForm.link_mode}`]"><el-radio-button value="soft">软链模式</el-radio-button><el-radio-button value="hard">硬链模式</el-radio-button><el-radio-button value="strm">Strm模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="createLinkForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="createLinkForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item v-if="createLinkForm.schedule_enabled" label="计划表达式"><el-input v-model="createLinkForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="createLinkForm.source_dir"><template #append><el-button @click="openDirectoryPicker('createLink', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="createLinkForm.target_dir"><template #append><el-button @click="openDirectoryPicker('createLink', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <template v-if="createLinkForm.link_mode === 'strm'">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">媒体文件</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">Strm 后缀</el-tag></div>
          </button>
          <div class="strm-suffix-editor">
            <div class="strm-suffix-editor__actions">
              <el-tooltip content="视频" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_suffixes, 'video') }" circle aria-label="视频" @click="fillCreateStrmPreset('video')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="4" y="5" width="16" height="14" rx="2.5" /><path d="m10 9 5 3-5 3Z" /></svg></el-button></el-tooltip>
              <el-tooltip content="音频" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_suffixes, 'audio') }" circle aria-label="音频" @click="fillCreateStrmPreset('audio')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M9 18.5a2.5 2.5 0 1 1-1.25-2.17V6.5l9-2v10" /><path d="M16.75 14.5a2.5 2.5 0 1 1-1.25-2.17" /><path d="M7.75 9.25l9-2" /></svg></el-button></el-tooltip>
            </div>
            <div class="strm-suffix-editor__tags">
              <el-tag v-for="suffix in createLinkForm.strm_suffixes" :key="suffix" closable @close="removeCreateStrmSuffix(suffix)">{{ suffix }}</el-tag>
              <el-input v-model="createLinkStrmSuffixInput" class="strm-suffix-editor__input" size="small" placeholder="输入扩展名回车，如 mp4" @keyup.enter="addCreateStrmSuffix" />
            </div>
          </div>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">元数据文件</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="info">图片 / 数据</el-tag></div>
          </button>
          <div class="strm-suffix-editor">
            <div class="strm-suffix-editor__actions">
              <el-tooltip content="图片" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_metadata_suffixes, 'image') }" circle aria-label="图片" @click="fillCreateStrmPreset('image')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="3.5" y="4.5" width="17" height="15" rx="2.5" /><circle cx="9" cy="10" r="1.5" /><path d="m5 18 4.5-4.5 3 3 2.5-2.5 4 4" /></svg></el-button></el-tooltip>
              <el-tooltip content="数据" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_metadata_suffixes, 'data') }" circle aria-label="数据" @click="fillCreateStrmPreset('data')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M6 4.5h12a1.5 1.5 0 0 1 1.5 1.5v12a1.5 1.5 0 0 1-1.5 1.5H6A1.5 1.5 0 0 1 4.5 18V6A1.5 1.5 0 0 1 6 4.5Z" /><path d="M8.5 8.5h7M8.5 12h7M8.5 15.5h4" /></svg></el-button></el-tooltip>
            </div>
            <div class="strm-suffix-editor__tags">
              <el-tag v-for="suffix in createLinkForm.strm_metadata_suffixes" :key="suffix" closable @close="removeCreateMetadataSuffix(suffix)">{{ suffix }}</el-tag>
              <el-input v-model="createLinkMetadataSuffixInput" class="strm-suffix-editor__input" size="small" placeholder="输入扩展名回车，如 mp4" @keyup.enter="addCreateMetadataSuffix" />
            </div>
          </div>
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="API 请求间隔">
                <el-input-number v-model="createLinkForm.strm_api_interval_seconds" class="strm-option-number" :min="0.1" :max="60" :step="0.1" :precision="1" controls-position="right" />
                <span class="strm-option-unit">秒</span>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="最小视频">
                <el-input-number v-model="createLinkForm.strm_min_video_mb" class="strm-option-number" :min="0" :max="1048576" :step="1" controls-position="right" />
                <span class="strm-option-unit">MB</span>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="下载线程数">
                <el-input-number v-model="createLinkForm.strm_download_threads" class="strm-option-number" :min="1" :max="16" :step="1" controls-position="right" />
              </el-form-item>
            </el-col>
          </el-row>
          <div class="strm-options-hint">API 请求间隔：每个线程对网盘接口的最小请求间隔；最小视频：小于该值的视频不生成 Strm（0 表示不限制）；下载线程数：并发请求数。</div>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤名单</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="createLinkForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <template v-else>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤名单</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="createLinkForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16">
          <el-col :span="linkSwitchSpan(createLinkForm.link_mode === 'strm')"><el-form-item label="启用规则"><el-switch v-model="createLinkForm.enabled" /></el-form-item></el-col>
          <el-col :span="linkSwitchSpan(createLinkForm.link_mode === 'strm')"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="createLinkForm.run_on_start" /></el-form-item></el-col>
          <el-col v-if="createLinkForm.link_mode === 'strm'" :span="linkSwitchSpan(true)"><el-form-item label="覆盖生成"><el-switch v-model="createLinkForm.strm_overwrite" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="createLinkDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateLinkRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editLinkDialogVisible" title="编辑链路规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="editLinkForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="链路模式"><el-radio-group v-model="editLinkForm.link_mode" :class="['archive-mode-group', `archive-mode-group--${editLinkForm.link_mode}`]"><el-radio-button value="soft">软链模式</el-radio-button><el-radio-button value="hard">硬链模式</el-radio-button><el-radio-button value="strm">Strm模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="editLinkForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="editLinkForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item v-if="editLinkForm.schedule_enabled" label="计划表达式"><el-input v-model="editLinkForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="editLinkForm.source_dir"><template #append><el-button @click="openDirectoryPicker('editLink', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="editLinkForm.target_dir"><template #append><el-button @click="openDirectoryPicker('editLink', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <template v-if="editLinkForm.link_mode === 'strm'">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">媒体文件</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">Strm 后缀</el-tag></div>
          </button>
          <div class="strm-suffix-editor">
            <div class="strm-suffix-editor__actions">
              <el-tooltip content="视频" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_suffixes, 'video') }" circle aria-label="视频" @click="fillEditStrmPreset('video')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="4" y="5" width="16" height="14" rx="2.5" /><path d="m10 9 5 3-5 3Z" /></svg></el-button></el-tooltip>
              <el-tooltip content="音频" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_suffixes, 'audio') }" circle aria-label="音频" @click="fillEditStrmPreset('audio')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M9 18.5a2.5 2.5 0 1 1-1.25-2.17V6.5l9-2v10" /><path d="M16.75 14.5a2.5 2.5 0 1 1-1.25-2.17" /><path d="M7.75 9.25l9-2" /></svg></el-button></el-tooltip>
            </div>
            <div class="strm-suffix-editor__tags">
              <el-tag v-for="suffix in editLinkForm.strm_suffixes" :key="suffix" closable @close="removeEditStrmSuffix(suffix)">{{ suffix }}</el-tag>
              <el-input v-model="editLinkStrmSuffixInput" class="strm-suffix-editor__input" size="small" placeholder="输入扩展名回车，如 mp4" @keyup.enter="addEditStrmSuffix" />
            </div>
          </div>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">元数据文件</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="info">图片 / 数据</el-tag></div>
          </button>
          <div class="strm-suffix-editor">
            <div class="strm-suffix-editor__actions">
              <el-tooltip content="图片" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_metadata_suffixes, 'image') }" circle aria-label="图片" @click="fillEditStrmPreset('image')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="3.5" y="4.5" width="17" height="15" rx="2.5" /><circle cx="9" cy="10" r="1.5" /><path d="m5 18 4.5-4.5 3 3 2.5-2.5 4 4" /></svg></el-button></el-tooltip>
              <el-tooltip content="数据" placement="top"><el-button class="strm-preset-button" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_metadata_suffixes, 'data') }" circle aria-label="数据" @click="fillEditStrmPreset('data')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M6 4.5h12a1.5 1.5 0 0 1 1.5 1.5v12a1.5 1.5 0 0 1-1.5 1.5H6A1.5 1.5 0 0 1 4.5 18V6A1.5 1.5 0 0 1 6 4.5Z" /><path d="M8.5 8.5h7M8.5 12h7M8.5 15.5h4" /></svg></el-button></el-tooltip>
            </div>
            <div class="strm-suffix-editor__tags">
              <el-tag v-for="suffix in editLinkForm.strm_metadata_suffixes" :key="suffix" closable @close="removeEditMetadataSuffix(suffix)">{{ suffix }}</el-tag>
              <el-input v-model="editLinkMetadataSuffixInput" class="strm-suffix-editor__input" size="small" placeholder="输入扩展名回车，如 mp4" @keyup.enter="addEditMetadataSuffix" />
            </div>
          </div>
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="API 请求间隔">
                <el-input-number v-model="editLinkForm.strm_api_interval_seconds" class="strm-option-number" :min="0.1" :max="60" :step="0.1" :precision="1" controls-position="right" />
                <span class="strm-option-unit">秒</span>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="最小视频">
                <el-input-number v-model="editLinkForm.strm_min_video_mb" class="strm-option-number" :min="0" :max="1048576" :step="1" controls-position="right" />
                <span class="strm-option-unit">MB</span>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="下载线程数">
                <el-input-number v-model="editLinkForm.strm_download_threads" class="strm-option-number" :min="1" :max="16" :step="1" controls-position="right" />
              </el-form-item>
            </el-col>
          </el-row>
          <div class="strm-options-hint">API 请求间隔：每个线程对网盘接口的最小请求间隔；最小视频：小于该值的视频不生成 Strm（0 表示不限制）；下载线程数：并发请求数。</div>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤名单</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="editLinkForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <template v-else>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤名单</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="editLinkForm.filters_text" type="textarea" :rows="10" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16">
          <el-col :span="linkSwitchSpan(editLinkForm.link_mode === 'strm')"><el-form-item label="启用规则"><el-switch v-model="editLinkForm.enabled" /></el-form-item></el-col>
          <el-col :span="linkSwitchSpan(editLinkForm.link_mode === 'strm')"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="editLinkForm.run_on_start" /></el-form-item></el-col>
          <el-col v-if="editLinkForm.link_mode === 'strm'" :span="linkSwitchSpan(true)"><el-form-item label="覆盖生成"><el-switch v-model="editLinkForm.strm_overwrite" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="editLinkDialogVisible = false">取消</el-button><el-button type="primary" :loading="editing" @click="submitUpdateLinkRule">保存</el-button></template>
    </el-dialog>

    <DirectoryPickerDialog v-model="directoryPickerVisible" title="选择目录" :initial-path="directoryPickerInitialPath" @selected="applyDirectorySelection" />

    <!-- 右键规则卡片：跟随指针的悬浮菜单（取代原来的居中确认框）。 -->
    <CardContextMenu
      :visible="cardContextMenu.visible"
      :x="cardContextMenu.x"
      :y="cardContextMenu.y"
      :items="cardContextMenuItems"
      @close="closeRuleContextMenu"
      @select="handleRuleContextMenuSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { Delete, Edit } from '@element-plus/icons-vue'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import Sortable from 'sortablejs'
import type { SortableEvent } from 'sortablejs'

import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import BackupRulesPanel from '../components/BackupRulesPanel.vue'
import BackupFileList from '../components/BackupFileList.vue'
import RuleCard from '../components/RuleCard.vue'
import CardContextMenu, { type CardContextMenuItem } from '../components/CardContextMenu.vue'
import { fetchRun, prepareRuleExecution } from '../api/executions'
import { createRule, deleteRule, fetchCronPreview, fetchRule, fetchRules, reorderRules, updateRule, type RuleItem, type UpdateRulePayload } from '../api/rules'
import {
  clearRunHistory,
  deleteRunHistoryItem,
  emptyRunHistory,
  fetchRunHistory,
  fetchRunHistoryDetail,
  type RunHistoryItem,
  type RunHistorySummary,
} from '../api/runHistory'
import { fetchSettings } from '../api/system'
import { fetchBackups, type BackupTask } from '../api/backups'
import {
  backupTriggerLabel,
  buildBackupDetailRows,
  parseBackupManifest,
  resolveBackupDeletedCount,
  type BackupDetailRow,
} from '../utils/backupDetail'
import { formatRunHistorySummary } from '../utils/runHistorySummary'

type ArchiveMode = 'package' | 'collect'
type CompatibilityMode = 'local' | 'compatibility'
type LinkMode = 'soft' | 'hard' | 'strm'
type StrmSyncMode = 'incremental' | 'full'
type PackageOptionKey = 'flat_archive' | 'include_manifest' | 'verify_after_archive' | 'cleanup_source_after_archive' | 'package_nested_folders' | 'match_archive' | 'match_archive_parent_rename' | 'single_file_nesting' | 'hierarchical_archive'
type CollectOptionKey = 'recursive_collect' | 'cleanup_source_after_archive'
type CleanupOptionKey = 'cleanup_empty_dirs' | 'cleanup_matching_files' | 'cleanup_expired_files'
type CleanupOptionValueKey = 'cleanup_retention_days'
type TransformOptionKey = 'convert_traditional_to_simplified' | 'convert_matching_text' | 'filter_matching_text' | 'merge_same_name_dirs'
type PurifyArchiveMode = 'cleanup' | 'transform'
type HistoryStatus = 'success' | 'skip' | 'failed'
type HistoryViewMode = 'flat' | 'tree'
type HistoryTreeRow = RunHistoryItem & {
  id: string
  title: string
  description: string
  is_group: boolean
  source?: RunHistoryItem
  children?: HistoryTreeRow[]
}
type DirectoryPickerTarget = 'create.source_dir' | 'create.target_dir' | 'edit.source_dir' | 'edit.target_dir' | 'createPurify.source_dir' | 'editPurify.source_dir' | 'createLink.source_dir' | 'createLink.target_dir' | 'editLink.source_dir' | 'editLink.target_dir' | 'createNaming.source_dir' | null
type TabKey = 'rules' | 'purify' | 'link' | 'naming' | 'backup' | 'history'
type RuleListType = 'archive' | 'cleanup' | 'link' | 'naming'
type StoredNamingRuleSet = { id: number; name: string; rules: Array<Record<string, unknown>> }
type PurifyOptions = Record<CleanupOptionKey | TransformOptionKey, boolean>
type PurifyOptionValues = Record<CleanupOptionValueKey, number>

const defaultCronExpression = '0 8 * * *'
const pageSizeOptions = [25, 50]
const ruleMatcherPlaceholder = '一行一个规则：无前缀默认全称匹配文件名，如 漫画；* 前缀匹配文件关键词，如 *漫画；. 前缀匹配扩展名，如 .mp4；/ 前缀匹配文件夹全称，如 /合集；/* 前缀匹配文件夹关键词，如 /*合集；/内容/ 全局全称匹配文件或文件夹，如 /mp4/；/*内容/ 全局关键词匹配文件或文件夹，如 /*mp4/。'
const archiveRuleMatcherPlaceholder = '文件匹配：待匹配\n文件关键词匹配：*待匹配\n扩展名匹配：.待匹配\n文件夹匹配：/待匹配\n文件夹关键词匹配：/*待匹配\n全局匹配：/待匹配/\n全局关键词匹配：/*待匹配/'
const cleanupRuleMatcherPlaceholder = '文件匹配：待匹配\n文件关键词匹配：*待匹配\n扩展名匹配：.待匹配\n文件夹匹配：/待匹配\n文件夹关键词匹配：/*待匹配\n全局匹配：/待匹配/\n全局关键词匹配：/*待匹配/'

const packageModeOptions = [
  { key: 'match_archive', label: '匹配归档', description: '按四元规则匹配文件名、扩展名、文件夹名或全局规则，命中后直接转移到目标路径，不参与打包。' },
  { key: 'match_archive_parent_rename', label: '父级重名', description: '可独立开启。\n对打包生成的 CBZ 按父目录名重命名；若同时开启匹配归档，则命中项也会按父目录名重命名。\n扁平归档开启时直接输出到目标路径；同一父级多个命中项时，按数字顺序追加 -part1、-part2、-part3、-part4……' },
  { key: 'flat_archive', label: '扁平归档', description: '勾选后直接输出到目标路径；取消勾选时默认额外套一层源文件夹后再生成 CBZ。' },
  { key: 'single_file_nesting', label: '单件归巢', description: '对监控目录下裸露文件按规则命中后，以文件名创建同名目录再转移进去。' },
  { key: 'hierarchical_archive', label: '层级归档', description: '勾选后，打包生成的 CBZ 会按原监控目录下的相对目录结构放置到目标路径中；关闭时保持当前打包输出规则。' },
  { key: 'cleanup_source_after_archive', label: '清理源件', description: '确认已归档后再清理原目录中的已处理源件。' },
] as const

const collectModeOptions = [
  { key: 'recursive_collect', label: '递归收集', description: '默认递归扫描监控目录下的全部子目录，并将父目录中的所有文件统一收集到目标目录对应的同名文件夹下。' },
  { key: 'cleanup_source_after_archive', label: '清理源件', description: '默认勾选；收集完成后删除已成功归档的源件，关闭后保留原始文件。' },
] as const

const cleanupModeOptions = [
  { key: 'cleanup_empty_dirs', label: '清理空夹', description: '递归删除监控目录中的空文件夹。' },
  { key: 'cleanup_matching_files', label: '匹配清理', description: '按四元规则删除命中的文件或文件夹，支持字符串和正则。' },
  { key: 'cleanup_expired_files', label: '过期清除', description: '按保留天数删除监控目录下超过期限的文件。' },
] as const

const transformModeOptions = [
  { key: 'convert_traditional_to_simplified', label: '繁简转换', description: '将监控目录下文件和文件夹名称中的中文繁体字转换为简体字，其他字符保持不变。' },
  { key: 'convert_matching_text', label: '匹配转换', description: '默认按文件名执行“待转换 => 转换词”；若两侧都写成 /待转换/ => /转换词/ 则按文件夹名执行。支持关键词部分匹配，也可用正则实现整段替换。' },
  { key: 'filter_matching_text', label: '匹配过滤', description: '按自定义规则过滤文件夹名称中的指定片段，支持普通文本与正则匹配，命中后会在重命名结果中移除匹配内容。' },
  { key: 'merge_same_name_dirs', label: '同名合并', description: '默认不勾选；关闭时文件夹转换后若出现同名冲突，则自动追加 -re、-re1、-re2……；开启后会将转换后同名的文件夹自动合并为一个目录。' },
] as const

function normalizeHistoryViewMode(value?: string): HistoryViewMode {
  return value === 'tree' ? 'tree' : 'flat'
}

function createDefaultPackageOptions(): Record<PackageOptionKey, boolean> {
  return { flat_archive: false, include_manifest: true, verify_after_archive: true, cleanup_source_after_archive: false, package_nested_folders: true, match_archive: false, match_archive_parent_rename: false, single_file_nesting: false, hierarchical_archive: false }
}

function normalizeFixedPackageOptions(options: Record<PackageOptionKey, boolean>): Record<PackageOptionKey, boolean> {
  return { ...options, include_manifest: true, verify_after_archive: true, package_nested_folders: true }
}

function createDefaultCollectOptions(): Record<CollectOptionKey, boolean> {
	return { recursive_collect: true, cleanup_source_after_archive: true }
}

function createDefaultCleanupOptions(): Record<CleanupOptionKey, boolean> {
  return { cleanup_empty_dirs: true, cleanup_matching_files: false, cleanup_expired_files: false }
}

function createDefaultCleanupOptionValues(): PurifyOptionValues {
	return { cleanup_retention_days: 5 }
}

function createDefaultTransformOptions(): Record<TransformOptionKey, boolean> {
	return { convert_traditional_to_simplified: true, convert_matching_text: false, filter_matching_text: false, merge_same_name_dirs: false }
}

function createDefaultPurifyOptions(): PurifyOptions {
  return { ...createDefaultCleanupOptions(), ...createDefaultTransformOptions() }
}

function createDefaultHistorySummary(): RunHistorySummary {
  return {
    total: 0,
    today: 0,
    success: 0,
    failed: 0,
    skipped: 0,
  }
}

function parseOptionJSON<T extends Record<string, boolean>>(raw: string | undefined, defaults: T): T {
  if (!raw) return { ...defaults }
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const normalized = { ...defaults }
    for (const key of Object.keys(defaults)) {
      if (typeof parsed[key] === 'boolean') normalized[key as keyof T] = parsed[key] as T[keyof T]
    }
    return normalized
  } catch {
    return { ...defaults }
  }
}

function parseNumberOptionJSON<T extends Record<string, number>>(raw: string | undefined, defaults: T): T {
	if (!raw) return { ...defaults }
	try {
		const parsed = JSON.parse(raw) as Record<string, unknown>
		const normalized = { ...defaults }
		for (const key of Object.keys(defaults)) {
			if (typeof parsed[key] === 'number' && Number.isFinite(parsed[key])) normalized[key as keyof T] = parsed[key] as T[keyof T]
		}
		return normalized
	} catch {
		return { ...defaults }
	}
}

function parseFiltersText(value: string) {
  return value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
}

// Strm 后缀统一「无点 + 小写」写法（mp4 而不是 .mp4），后端 normalizeStrmExtensions 会自动补点并忽略大小写。
const strmVideoSuffixPreset = ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'webm', 'm4v', 'ts', 'm2ts', 'rmvb', 'rm', '3gp', 'iso']
const strmAudioSuffixPreset = ['mp3', 'flac', 'wav', 'aac', 'm4a', 'ogg', 'wma', 'ape', 'alac', 'opus']
const strmImageSuffixPreset = ['jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp', 'tiff', 'avif']
const strmDataSuffixPreset = ['ass', 'srt', 'ssa', 'sub', 'nfo', 'vtt', 'idx']

function normalizeLinkMode(value?: string): LinkMode {
  if (value === 'hard' || value === 'strm') return value
  return 'soft'
}

// 后缀统一成「无点 + 小写」：输入 mp4 / .mp4 / MP4 都归一为 mp4，界面里不显示那个点。
function normalizeStrmSuffix(value: string) {
  const trimmed = value
    .trim()
    .replace(/^['"]+|['"]+$/g, '')
    .replace(/^\*+/, '')
    .replace(/^\.+/, '')
    .trim()
  if (!trimmed) return ''
  return trimmed.toLowerCase()
}

function normalizeStrmSuffixes(values: string[]) {
  const seen = new Set<string>()
  const suffixes: string[] = []
  for (const value of values) {
    const suffix = normalizeStrmSuffix(value)
    if (!suffix || seen.has(suffix)) continue
    seen.add(suffix)
    suffixes.push(suffix)
  }
  return suffixes
}

// Strm 规则实际保存的是「媒体 + 元数据」合并后的后缀列表；
// 界面上两块分开编辑，保存时合并、载入时按预设类别拆回。
function mergeStrmSuffixes(...groups: string[][]) {
  return normalizeStrmSuffixes(groups.flat())
}

function splitStrmSuffixes(values: string[]) {
  const metadataPreset = new Set(
    normalizeStrmSuffixes([...strmPresetByType('image'), ...strmPresetByType('data')]),
  )
  const media: string[] = []
  const metadata: string[] = []
  for (const suffix of values) {
    if (metadataPreset.has(suffix)) {
      metadata.push(suffix)
    } else {
      media.push(suffix)
    }
  }
  return { media, metadata }
}

function parseFiltersArray(raw?: string) {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.map((item) => String(item).trim()).filter(Boolean)
  } catch {
    return []
  }
}

const strmOptionDefaults = { strm_full_sync: false, strm_overwrite: false }

function parseStrmSyncMode(raw?: string): StrmSyncMode {
  return parseOptionJSON(raw, strmOptionDefaults).strm_full_sync ? 'full' : 'incremental'
}

// 覆盖生成：目标目录里已存在的 Strm 会被重新写入，而不是跳过。
function parseStrmOverwrite(raw?: string): boolean {
  return parseOptionJSON(raw, strmOptionDefaults).strm_overwrite
}

function buildStrmOptions(syncMode: StrmSyncMode, overwrite: boolean) {
  return { strm_full_sync: syncMode === 'full', strm_overwrite: overwrite }
}

// 链路规则底部的开关行：Strm 模式下「启用规则 / 立即运行一次（启动后）/ 覆盖生成」三个开关并排（各 8 栅格），
// 其它模式只有两个开关，保持各占一半。
function linkSwitchSpan(showOverwrite: boolean) {
  return showOverwrite ? 8 : 12
}

// Strm 规则的数值参数（存 rules.option_values_json）。
// API 请求间隔后端存毫秒，界面上按「秒 / 步长 0.1」编辑。
type StrmOptionValues = {
  strm_api_interval_ms: number
  strm_min_video_mb: number
  strm_download_threads: number
}

const strmOptionValueDefaults: StrmOptionValues = {
  strm_api_interval_ms: 2000,
  strm_min_video_mb: 0,
  strm_download_threads: 3,
}

function parseStrmOptionValues(raw?: string): StrmOptionValues {
  return parseNumberOptionJSON(raw, strmOptionValueDefaults)
}

function parseStrmAPIIntervalSeconds(raw?: string) {
  return parseStrmOptionValues(raw).strm_api_interval_ms / 1000
}

function parseStrmMinVideoMB(raw?: string) {
  return parseStrmOptionValues(raw).strm_min_video_mb
}

function parseStrmDownloadThreads(raw?: string) {
  return parseStrmOptionValues(raw).strm_download_threads
}

function createDefaultStrmAPIIntervalSeconds() {
  return strmOptionValueDefaults.strm_api_interval_ms / 1000
}

function createDefaultStrmMinVideoMB() {
  return strmOptionValueDefaults.strm_min_video_mb
}

function createDefaultStrmDownloadThreads() {
  return strmOptionValueDefaults.strm_download_threads
}

function buildStrmOptionValues(form: { strm_api_interval_seconds: number; strm_min_video_mb: number; strm_download_threads: number }): StrmOptionValues {
  const seconds = Number.isFinite(form.strm_api_interval_seconds) ? form.strm_api_interval_seconds : createDefaultStrmAPIIntervalSeconds()
  const megabytes = Number.isFinite(form.strm_min_video_mb) ? form.strm_min_video_mb : 0
  const threads = Number.isFinite(form.strm_download_threads) ? form.strm_download_threads : 3
  return {
    // 与后端 strmMinAPIIntervalMS 保持一致，最小 0.1 秒。
    strm_api_interval_ms: Math.max(100, Math.round(seconds * 1000)),
    strm_min_video_mb: Math.max(0, Math.round(megabytes)),
    strm_download_threads: Math.min(16, Math.max(1, Math.round(threads))),
  }
}

// 原样读出库里已存的数值参数（不补默认值）：
// 用于「只切换启用状态」这类 PUT，避免凭空写入本不存在的参数。
function parseStoredOptionValues(raw?: string): Record<string, number> {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const result: Record<string, number> = {}
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof value === 'number' && Number.isFinite(value)) result[key] = value
    }
    return result
  } catch {
    return {}
  }
}

function parseFiltersJSON(raw?: string) {
  if (!raw) return ''
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return ''
    return parsed.map((item) => String(item).trim()).filter(Boolean).join('\n')
  } catch {
    return ''
  }
}

function getModeTitle(mode: ArchiveMode) {
  return mode === 'package' ? '功能模块' : '功能模块'
}

function getPurifyModeTitle(mode: PurifyArchiveMode) {
  return mode === 'transform' ? '功能模块' : '功能模块'
}

function getPurifyModeOptions(mode: PurifyArchiveMode) {
  return mode === 'transform' ? transformModeOptions : cleanupModeOptions
}

function resolveRunMode(monitorEnabled: boolean, scheduleEnabled: boolean): 'watch' | 'cron' | 'once' {
  if (scheduleEnabled) return 'cron'
  if (monitorEnabled) return 'watch'
  return 'once'
}

function formatDateTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function formatHistorySize(sizeBytes?: number) {
  const size = Number(sizeBytes || 0)
  if (!Number.isFinite(size) || size <= 0) {
    return '—'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }

  return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(2)} ${units[index]}`
}

const RULES_WHEEL_SCROLLBAR_BAND = 10
const rulesPageRef = ref<HTMLElement | null>(null)

function getActiveRulesTableScroll(): HTMLElement | null {
  const root = rulesPageRef.value
  if (!root) return null

  const containers = Array.from(root.querySelectorAll<HTMLElement>('.rules-table-scroll'))
  return containers.find((item) => item.offsetParent !== null && item.clientWidth > 0) ?? null
}

function handleRulesTableWheel(event: WheelEvent) {
  const target = event.target as HTMLElement | null
  if (!target) return
  if (target.closest('.el-overlay, .el-popper')) return

  const container = getActiveRulesTableScroll()
  if (!container) return

  const rect = container.getBoundingClientRect()
  const withinColumns = event.clientX >= rect.left && event.clientX <= rect.right
  const isTableHeader = Boolean(target.closest('.el-table__header-wrapper'))
  const isBelowScrollbarTop = event.clientY >= rect.bottom - RULES_WHEEL_SCROLLBAR_BAND
  if (!isTableHeader && !(withinColumns && isBelowScrollbarTop)) return

  const maxScrollLeft = Math.max(0, container.scrollWidth - container.clientWidth)
  if (maxScrollLeft <= 0) return

  const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY
  if (!delta) return

  const previous = container.scrollLeft
  const next = Math.max(0, Math.min(maxScrollLeft, previous + delta))
  if (next !== previous) {
    container.scrollLeft = next
  }

  event.preventDefault()
}

function getCronPreviewItems(ruleId: number) {
  return cronPreviewCache[ruleId] || []
}

async function handleCronPreviewShow(ruleId: number, expression: string) {
  const trimmed = expression.trim()
  if (!trimmed || cronPreviewCache[ruleId]?.length) return

  cronPreviewLoadingRuleId.value = ruleId
  cronPreviewErrorRuleId.value = null
  cronPreviewErrorMessage.value = ''

  try {
    const response = await fetchCronPreview(trimmed)
    cronPreviewCache[ruleId] = response.data?.next_runs || []
  } catch (error) {
    cronPreviewErrorRuleId.value = ruleId
    cronPreviewErrorMessage.value = error instanceof Error ? error.message : 'Cron 预览失败'
  } finally {
    if (cronPreviewLoadingRuleId.value === ruleId) {
      cronPreviewLoadingRuleId.value = null
    }
  }
}

function historyStatusText(status: string) {
  if (status === 'success') return '成功'
  if (status === 'skip') return '跳过'
  if (status === 'failed') return '失败'
  return status || '未知'
}

function normalizeRuleType(rule: RuleItem): RuleListType {
	if (rule.rule_type === 'naming' || rule.archive_mode === 'naming') return 'naming'
  if (rule.rule_type === 'cleanup' || rule.archive_mode === 'cleanup') return 'cleanup'
  if (rule.rule_type === 'link' || rule.archive_mode === 'link') return 'link'
  return 'archive'
}

function normalizeSourceDirs(sourceDir?: string, sourceDirs: string[] = []) {
  const seen = new Set<string>()
  const items: string[] = []
  const append = (value?: string) => {
    const trimmed = (value || '').trim()
    if (!trimmed || seen.has(trimmed)) return
    seen.add(trimmed)
    items.push(trimmed)
  }

  sourceDirs.forEach(append)
  append(sourceDir)
  return items
}

function getRuleSourceDirs(rule: RuleItem) {
  return normalizeSourceDirs(rule.source_dir, rule.source_dirs)
}

function getRuleSourceDirsText(rule: RuleItem) {
  return getRuleSourceDirs(rule).join('  |  ')
}

// —— 规则卡片上展示的路径信息 ——
function archiveRulePaths(rule: RuleItem) {
  return [
    { label: '源路径', value: rule.source_dir },
    { label: '目标路径', value: rule.target_dir },
  ]
}

function purifyRulePaths(rule: RuleItem) {
  return [{ label: '监控目录', value: getRuleSourceDirsText(rule) }]
}

function linkRulePaths(rule: RuleItem) {
  return [
    { label: '源路径', value: rule.source_dir },
    { label: '目标路径', value: rule.target_dir },
  ]
}

function namingRulePaths(rule: RuleItem) {
  return [{ label: '监控路径', value: rule.source_dir }]
}

function addPurifySourceDir(target: 'create' | 'edit', path: string) {
  const trimmed = path.trim()
  if (!trimmed) return
  const form = target === 'create' ? createPurifyForm : editPurifyForm
  form.source_dirs = normalizeSourceDirs(trimmed, form.source_dirs)
  form.source_dir = form.source_dirs[0] || ''
}

function removeCreatePurifySourceDir(path: string) {
  createPurifyForm.source_dirs = createPurifyForm.source_dirs.filter((item) => item !== path)
  createPurifyForm.source_dir = createPurifyForm.source_dirs[0] || ''
}

function removeEditPurifySourceDir(path: string) {
  editPurifyForm.source_dirs = editPurifyForm.source_dirs.filter((item) => item !== path)
  editPurifyForm.source_dir = editPurifyForm.source_dirs[0] || ''
}

function buildRuleUpdatePayload(rule: RuleItem, overrides: Partial<UpdateRulePayload> = {}): UpdateRulePayload {
  const ruleType = normalizeRuleType(rule)
  const scheduleEnabled = rule.run_mode === 'cron' || Boolean(rule.cron_expression)
  const purifyOptions = ruleType === 'cleanup' ? parseOptionJSON(rule.options_json, createDefaultPurifyOptions()) : null
  const linkMode = ruleType === 'link' ? normalizeLinkMode(rule.link_mode) : undefined
  const transformRules = parseFiltersText(parseFiltersJSON(rule.transform_rules_json))
  const transformFilters = parseFiltersText(parseFiltersJSON(rule.transform_filters_json))

  return {
    name: rule.name,
    description: rule.description ?? '',
    enabled: rule.enabled,
    monitor_enabled: rule.monitor_enabled,
    compatibility_mode: ruleType === 'link' || ruleType === 'naming' ? 'local' : (rule.compatibility_mode || 'local'),
    archive_mode: rule.archive_mode,
    rule_type: ruleType,
    link_mode: linkMode,
    run_mode: resolveRunMode(rule.monitor_enabled, scheduleEnabled),
    source_dir: getRuleSourceDirs(rule)[0] || rule.source_dir,
    source_dirs: getRuleSourceDirs(rule),
    target_dir: ruleType === 'cleanup' || ruleType === 'naming' ? '' : rule.target_dir,
    watch_debounce_ms: rule.watch_debounce_ms,
    cron_expression: scheduleEnabled ? rule.cron_expression : '',
    run_on_start: rule.run_on_start,
    options: purifyOptions ?? (linkMode === 'strm' ? buildStrmOptions(parseStrmSyncMode(rule.options_json), parseStrmOverwrite(rule.options_json)) : {}),
    // 原样带回库里已有的数值参数：PUT 是整体覆盖，缺这个字段会把清理保留天数等一起清空。
    option_values: parseStoredOptionValues(rule.option_values_json),
    package_options: ruleType === 'archive' && rule.archive_mode === 'package'
      ? normalizeFixedPackageOptions(parseOptionJSON(rule.package_options_json, createDefaultPackageOptions()))
      : {},
    collect_options: ruleType === 'archive' && rule.archive_mode === 'collect'
      ? parseOptionJSON(rule.collect_options_json, createDefaultCollectOptions())
      : {},
    filters: parseFiltersText(parseFiltersJSON(rule.filters_json)),
    whitelist: parseFiltersText(parseFiltersJSON(rule.whitelist_json)),
    match_filters: parseFiltersText(parseFiltersJSON(rule.match_filters_json)),
    nest_filters: parseFiltersText(parseFiltersJSON(rule.nest_filters_json)),
    transform_rules: ruleType === 'naming' ? transformRules : (ruleType === 'cleanup' && rule.archive_mode === 'transform' && purifyOptions?.convert_matching_text ? transformRules : []),
    transform_filters: ruleType === 'cleanup' && rule.archive_mode === 'transform' && purifyOptions?.filter_matching_text ? transformFilters : [],
    ...overrides,
  }
}

const activeTab = ref<TabKey>('rules')
// 必须在 setup 顶层获取路由实例：inject 仅在组件 setup 作用域内可用，
// 若在事件回调/生命周期回调里调用 useRoute()/useRouter() 会拿到 undefined 并抛错，
// 从而中断后续的数据加载与 URL 同步。
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const archiveLoading = ref(false)
const purifyLoading = ref(false)
const linkLoading = ref(false)
const namingLoading = ref(false)
const historyLoading = ref(false)
const creating = ref(false)
const editing = ref(false)
const errorMessage = ref('')
const updatingRuleStatusIds = ref(new Set<number>())
const archiveGridRef = ref<HTMLElement | null>(null)
const purifyGridRef = ref<HTMLElement | null>(null)
const linkGridRef = ref<HTMLElement | null>(null)
const namingGridRef = ref<HTMLElement | null>(null)
const reorderingRuleType = ref<RuleListType | null>(null)
const cronPreviewLoadingRuleId = ref<number | null>(null)
const cronPreviewErrorRuleId = ref<number | null>(null)
const cronPreviewErrorMessage = ref('')
const cronPreviewCache = reactive<Record<number, string[]>>({})

const createDialogVisible = ref(false)
const editDialogVisible = ref(false)
const createPurifyDialogVisible = ref(false)
const editPurifyDialogVisible = ref(false)
const createLinkDialogVisible = ref(false)
const createNamingDialogVisible = ref(false)
const editLinkDialogVisible = ref(false)
const createArchiveOptionsExpanded = ref(false)
const editArchiveOptionsExpanded = ref(false)
const namingScopeOptionsExpanded = ref(false)
const createPurifyOptionsExpanded = ref(false)
const editPurifyOptionsExpanded = ref(false)

const editingRuleID = ref<number | null>(null)
const editingPurifyRuleID = ref<number | null>(null)
const editingLinkRuleID = ref<number | null>(null)

const directoryPickerVisible = ref(false)
const directoryPickerInitialPath = ref('')
const directoryPickerTarget = ref<DirectoryPickerTarget>(null)

const archiveRulesCurrentPage = ref(1)
const archiveRulesPageSize = ref(25)
const archiveRulesTotal = ref(0)
const purifyRulesCurrentPage = ref(1)
const purifyRulesPageSize = ref(25)
const linkRulesCurrentPage = ref(1)
const linkRulesPageSize = ref(25)
const linkRulesTotal = ref(0)
const historyPageSize = ref(25)
const historyCurrentPage = ref(1)
const historyTotal = ref(0)
const historyKeywordInput = ref('')
const historyKeyword = ref('')
const historySortBy = ref<'name' | 'modified_at'>('modified_at')
const historySortOrder = ref<'asc' | 'desc'>('desc')
const historyStatusFilter = ref<'all' | 'success' | 'failed' | 'skip'>('all')
const historyRuleTypeFilter = ref<'all' | 'archive' | 'cleanup' | 'link' | 'naming' | 'backup'>('all')
const historyViewMode = ref<HistoryViewMode>('flat')
const historyDetailDialogVisible = ref(false)
const selectedHistoryGroup = ref<HistoryTreeRow | null>(null)
const selectedBackupTask = ref<BackupTask | null>(null)
const selectedBackupDetail = ref<RunHistoryItem | null>(null)
const historyDetailPageSize = 25
const historyDetailCurrentPage = ref(1)

const archiveRules = ref<RuleItem[]>([])
const purifyRules = ref<RuleItem[]>([])
const linkRules = ref<RuleItem[]>([])
const namingRules = ref<RuleItem[]>([])
const availableNamingRuleSets = ref<StoredNamingRuleSet[]>([])
const historyItems = ref<RunHistoryItem[]>(emptyRunHistory())
const historySummary = ref<RunHistorySummary>(createDefaultHistorySummary())

const successCount = computed(() => historySummary.value.success)
const skipCount = computed(() => historySummary.value.skipped)
const failedCount = computed(() => historySummary.value.failed)
const historyTreeRows = computed(() => buildHistoryTreeRows(historyItems.value))
const selectedHistoryGroupChildren = computed(() => selectedHistoryGroup.value?.children ?? [])
const pagedHistoryDetailRows = computed(() => {
  const start = (historyDetailCurrentPage.value - 1) * historyDetailPageSize
  return selectedHistoryGroupChildren.value.slice(start, start + historyDetailPageSize)
})
// 备份任务的详情面板：规则卡片信息 + 本次执行的来源去向、触发方式与删除情况。
const selectedBackupDetailRows = computed<BackupDetailRow[]>(() => {
  const group = selectedHistoryGroup.value
  if (!group || group.archive_mode !== 'backup') {
    return []
  }
  return buildBackupDetailRows(selectedBackupTask.value, {
    triggerMode: group.trigger_mode,
    deletedCount: resolveBackupDeletedCount(group),
    successCount: group.success_count,
    skipCount: group.skip_count,
    failureCount: group.failure_count,
  })
})
// 备份文件明细：本次执行上传/跳过/失败/删除的具体文件清单（来自 detail_json）。
const selectedBackupManifest = computed(() => parseBackupManifest(selectedBackupDetail.value ?? undefined))

const purifyRulesTotal = ref(0)

let archiveSortable: Sortable | null = null
let purifySortable: Sortable | null = null
let linkSortable: Sortable | null = null
let namingSortable: Sortable | null = null

const createForm = reactive({
  name: '',
  description: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  compatibility_mode: 'local' as CompatibilityMode,
  archive_mode: 'package' as ArchiveMode,
  source_dir: '',
  target_dir: '',
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  package_options: createDefaultPackageOptions(),
  collect_options: createDefaultCollectOptions(),
  filters_text: '',
  match_filters_text: '',
  nest_filters_text: '',
})

const editForm = reactive({
  name: '',
  description: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  compatibility_mode: 'local' as CompatibilityMode,
  archive_mode: 'package' as ArchiveMode,
  source_dir: '',
  target_dir: '',
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  package_options: createDefaultPackageOptions(),
  collect_options: createDefaultCollectOptions(),
  filters_text: '',
  match_filters_text: '',
  nest_filters_text: '',
})

const createPurifyForm = reactive({
  name: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  archive_mode: 'cleanup' as PurifyArchiveMode,
  compatibility_mode: 'local' as CompatibilityMode,
  source_dir: '',
  source_dirs: [] as string[],
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  options: createDefaultPurifyOptions() as PurifyOptions,
  option_values: createDefaultCleanupOptionValues() as PurifyOptionValues,
  filters_text: '',
  whitelist_text: '',
  transform_rules_text: '',
  transform_filters_text: '',
})

const editPurifyForm = reactive({
  name: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  archive_mode: 'cleanup' as PurifyArchiveMode,
  compatibility_mode: 'local' as CompatibilityMode,
  source_dir: '',
  source_dirs: [] as string[],
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  options: createDefaultPurifyOptions() as PurifyOptions,
  option_values: createDefaultCleanupOptionValues() as PurifyOptionValues,
  filters_text: '',
  whitelist_text: '',
  transform_rules_text: '',
  transform_filters_text: '',
})

const createLinkForm = reactive({
  name: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: true,
  source_dir: '',
  target_dir: '',
  cron_expression: '30 4 * * *',
  watch_debounce_ms: 2000,
  run_on_start: false,
  link_mode: 'soft' as LinkMode,
  strm_sync_mode: 'incremental' as StrmSyncMode,
  strm_overwrite: false,
  strm_api_interval_seconds: createDefaultStrmAPIIntervalSeconds(),
  strm_min_video_mb: createDefaultStrmMinVideoMB(),
  strm_download_threads: createDefaultStrmDownloadThreads(),
  strm_suffixes: [] as string[],
  strm_metadata_suffixes: [] as string[],
  filters_text: '',
})

const editLinkForm = reactive({
  name: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: true,
  source_dir: '',
  target_dir: '',
  cron_expression: '30 4 * * *',
  watch_debounce_ms: 2000,
  run_on_start: false,
  link_mode: 'soft' as LinkMode,
  strm_sync_mode: 'incremental' as StrmSyncMode,
  strm_overwrite: false,
  strm_api_interval_seconds: createDefaultStrmAPIIntervalSeconds(),
  strm_min_video_mb: createDefaultStrmMinVideoMB(),
  strm_download_threads: createDefaultStrmDownloadThreads(),
  strm_suffixes: [] as string[],
  strm_metadata_suffixes: [] as string[],
  filters_text: '',
})

const createNamingForm = reactive({ name: '', enabled: true, monitor_enabled: true, schedule_enabled: false, source_dir: '', cron_expression: '', rule_set_id: null as number | null, options: { naming_include_files: false, naming_include_dirs: false } })
// 非空表示当前命名规则弹窗处于「编辑」状态。
const editingNamingRuleID = ref<number | null>(null)
const editingNamingRule = ref<RuleItem | null>(null)

const createLinkStrmSuffixInput = ref('')
const editLinkStrmSuffixInput = ref('')
const createLinkMetadataSuffixInput = ref('')
const editLinkMetadataSuffixInput = ref('')

function resetCreateForm() {
  createForm.name = ''
  createForm.description = ''
  createForm.enabled = true
  createForm.monitor_enabled = true
  createForm.schedule_enabled = false
  createForm.compatibility_mode = 'local'
  createForm.archive_mode = 'package'
  createForm.source_dir = ''
  createForm.target_dir = ''
  createForm.cron_expression = ''
  createForm.watch_debounce_ms = 2000
  createForm.run_on_start = true
  createForm.package_options = createDefaultPackageOptions()
  createForm.collect_options = createDefaultCollectOptions()
  createForm.filters_text = ''
  createForm.match_filters_text = ''
  createForm.nest_filters_text = ''
}

function resetEditForm() {
  editForm.name = ''
  editForm.description = ''
  editForm.enabled = true
  editForm.monitor_enabled = true
  editForm.schedule_enabled = false
  editForm.compatibility_mode = 'local'
  editForm.archive_mode = 'package'
  editForm.source_dir = ''
  editForm.target_dir = ''
  editForm.cron_expression = ''
  editForm.watch_debounce_ms = 2000
  editForm.run_on_start = true
  editForm.package_options = createDefaultPackageOptions()
  editForm.collect_options = createDefaultCollectOptions()
  editForm.filters_text = ''
  editForm.match_filters_text = ''
  editForm.nest_filters_text = ''
}

function resetCreatePurifyForm() {
  createPurifyForm.name = ''
  createPurifyForm.enabled = true
  createPurifyForm.monitor_enabled = true
  createPurifyForm.schedule_enabled = false
  createPurifyForm.archive_mode = 'cleanup'
  createPurifyForm.compatibility_mode = 'local'
  createPurifyForm.source_dir = ''
  createPurifyForm.source_dirs = []
  createPurifyForm.cron_expression = ''
  createPurifyForm.watch_debounce_ms = 2000
  createPurifyForm.run_on_start = true
  createPurifyForm.options = createDefaultPurifyOptions()
  createPurifyForm.filters_text = ''
  createPurifyForm.whitelist_text = ''
  createPurifyForm.transform_rules_text = ''
  createPurifyForm.transform_filters_text = ''
}

function resetEditPurifyForm() {
  editPurifyForm.name = ''
  editPurifyForm.enabled = true
  editPurifyForm.monitor_enabled = true
  editPurifyForm.schedule_enabled = false
  editPurifyForm.archive_mode = 'cleanup'
  editPurifyForm.compatibility_mode = 'local'
  editPurifyForm.source_dir = ''
  editPurifyForm.source_dirs = []
  editPurifyForm.cron_expression = ''
  editPurifyForm.watch_debounce_ms = 2000
  editPurifyForm.run_on_start = true
  editPurifyForm.options = createDefaultPurifyOptions()
  editPurifyForm.filters_text = ''
  editPurifyForm.whitelist_text = ''
  editPurifyForm.transform_rules_text = ''
  editPurifyForm.transform_filters_text = ''
}

function resetCreateLinkForm() {
  createLinkForm.name = ''
  createLinkForm.enabled = true
  createLinkForm.monitor_enabled = true
  createLinkForm.schedule_enabled = true
  createLinkForm.source_dir = ''
  createLinkForm.target_dir = ''
  createLinkForm.cron_expression = '30 4 * * *'
  createLinkForm.watch_debounce_ms = 2000
  createLinkForm.run_on_start = false
  createLinkForm.link_mode = 'soft'
  createLinkForm.strm_sync_mode = 'incremental'
  createLinkForm.strm_overwrite = false
  createLinkForm.strm_api_interval_seconds = createDefaultStrmAPIIntervalSeconds()
  createLinkForm.strm_min_video_mb = createDefaultStrmMinVideoMB()
  createLinkForm.strm_download_threads = createDefaultStrmDownloadThreads()
  createLinkForm.strm_suffixes = []
  createLinkForm.strm_metadata_suffixes = []
  createLinkForm.filters_text = ''
  createLinkStrmSuffixInput.value = ''
  createLinkMetadataSuffixInput.value = ''
}

function resetEditLinkForm() {
  editLinkForm.name = ''
  editLinkForm.enabled = true
  editLinkForm.monitor_enabled = true
  editLinkForm.schedule_enabled = true
  editLinkForm.source_dir = ''
  editLinkForm.target_dir = ''
  editLinkForm.cron_expression = '30 4 * * *'
  editLinkForm.watch_debounce_ms = 2000
  editLinkForm.run_on_start = false
  editLinkForm.link_mode = 'soft'
  editLinkForm.strm_sync_mode = 'incremental'
  editLinkForm.strm_overwrite = false
  editLinkForm.strm_api_interval_seconds = createDefaultStrmAPIIntervalSeconds()
  editLinkForm.strm_min_video_mb = createDefaultStrmMinVideoMB()
  editLinkForm.strm_download_threads = createDefaultStrmDownloadThreads()
  editLinkForm.strm_suffixes = []
  editLinkForm.strm_metadata_suffixes = []
  editLinkForm.filters_text = ''
  editLinkStrmSuffixInput.value = ''
  editLinkMetadataSuffixInput.value = ''
}

function addStrmSuffix(target: string[], value: string) {
  const suffix = normalizeStrmSuffix(value)
  if (!suffix || target.includes(suffix)) return false
  target.push(suffix)
  return true
}

function addCreateStrmSuffix() {
  if (addStrmSuffix(createLinkForm.strm_suffixes, createLinkStrmSuffixInput.value)) {
    createLinkStrmSuffixInput.value = ''
  }
}

function addEditStrmSuffix() {
  if (addStrmSuffix(editLinkForm.strm_suffixes, editLinkStrmSuffixInput.value)) {
    editLinkStrmSuffixInput.value = ''
  }
}

function addCreateMetadataSuffix() {
  if (addStrmSuffix(createLinkForm.strm_metadata_suffixes, createLinkMetadataSuffixInput.value)) {
    createLinkMetadataSuffixInput.value = ''
  }
}

function addEditMetadataSuffix() {
  if (addStrmSuffix(editLinkForm.strm_metadata_suffixes, editLinkMetadataSuffixInput.value)) {
    editLinkMetadataSuffixInput.value = ''
  }
}

function removeCreateStrmSuffix(suffix: string) {
  createLinkForm.strm_suffixes = createLinkForm.strm_suffixes.filter((item) => item !== suffix)
}

function removeEditStrmSuffix(suffix: string) {
  editLinkForm.strm_suffixes = editLinkForm.strm_suffixes.filter((item) => item !== suffix)
}

function removeCreateMetadataSuffix(suffix: string) {
  createLinkForm.strm_metadata_suffixes = createLinkForm.strm_metadata_suffixes.filter((item) => item !== suffix)
}

function removeEditMetadataSuffix(suffix: string) {
  editLinkForm.strm_metadata_suffixes = editLinkForm.strm_metadata_suffixes.filter((item) => item !== suffix)
}

// 图片 / 数据属于元数据类后缀，单独放进「元数据文件」窗口，避免与媒体文件后缀混在一起。
function isMetadataStrmPreset(type: 'video' | 'audio' | 'image' | 'data') {
  return type === 'image' || type === 'data'
}

function fillCreateStrmPreset(type: 'video' | 'audio' | 'image' | 'data') {
  const preset = strmPresetByType(type)
  if (isMetadataStrmPreset(type)) {
    createLinkForm.strm_metadata_suffixes = toggleStrmPreset(createLinkForm.strm_metadata_suffixes, preset)
    return
  }
  createLinkForm.strm_suffixes = toggleStrmPreset(createLinkForm.strm_suffixes, preset)
}

function fillEditStrmPreset(type: 'video' | 'audio' | 'image' | 'data') {
  const preset = strmPresetByType(type)
  if (isMetadataStrmPreset(type)) {
    editLinkForm.strm_metadata_suffixes = toggleStrmPreset(editLinkForm.strm_metadata_suffixes, preset)
    return
  }
  editLinkForm.strm_suffixes = toggleStrmPreset(editLinkForm.strm_suffixes, preset)
}

function strmPresetByType(type: 'video' | 'audio' | 'image' | 'data'): string[] {
  switch (type) {
    case 'audio':
      return strmAudioSuffixPreset
    case 'image':
      return strmImageSuffixPreset
    case 'data':
      return strmDataSuffixPreset
    default:
      return strmVideoSuffixPreset
  }
}

// 再次点击同一类别：若该类别的后缀已全部存在则全部移除，否则补齐缺失项。
function toggleStrmPreset(current: string[], preset: string[]) {
  const normalizedPreset = normalizeStrmSuffixes(preset)
  const currentSet = new Set(current)
  const allPresent = normalizedPreset.every((suffix) => currentSet.has(suffix))
  if (allPresent) {
    const presetSet = new Set(normalizedPreset)
    return current.filter((suffix) => !presetSet.has(suffix))
  }
  return normalizeStrmSuffixes([...current, ...normalizedPreset])
}

function strmPresetActive(current: string[], type: 'video' | 'audio' | 'image' | 'data'): boolean {
  const normalizedPreset = normalizeStrmSuffixes(strmPresetByType(type))
  if (normalizedPreset.length === 0) return false
  const currentSet = new Set(current)
  return normalizedPreset.every((suffix) => currentSet.has(suffix))
}

function buildDuplicateRuleName(name: string) {
  const trimmed = name.trim()
  return trimmed ? `${trimmed} - 副本` : '规则副本'
}

async function duplicateRule(rule: RuleItem, type: RuleListType) {
  errorMessage.value = ''
  try {
    // 副本一律以「停用」创建：复制出来的规则往往还要改路径/参数，
    // 直接继承启用态会和原规则抢同一批文件（monitor/cron 双双触发）。
    const payload = buildRuleUpdatePayload(rule, {
      name: buildDuplicateRuleName(rule.name),
      enabled: false,
    })
    await createRule(payload)
    ElMessage.success('规则复制成功（副本默认停用）')
    await refreshRuleList(type)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '复制规则失败'
  }
}

// —— 右键规则卡片：跟随指针的悬浮菜单 ——
// 归档 / 净化 / 链路 / 命名四个 tab 的卡片共用一份菜单状态。
const cardContextMenuItems: CardContextMenuItem[] = [{ key: 'duplicate', label: '复制规则', icon: 'copy' }]

const cardContextMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  rule: null as RuleItem | null,
  type: 'archive' as RuleListType,
})

function handleRuleContextMenu(row: RuleItem, type: RuleListType, event: MouseEvent) {
  event.preventDefault()
  cardContextMenu.rule = row
  cardContextMenu.type = type
  cardContextMenu.x = event.clientX
  cardContextMenu.y = event.clientY
  cardContextMenu.visible = true
}

function closeRuleContextMenu() {
  cardContextMenu.visible = false
}

function handleRuleContextMenuSelect(key: string) {
  const rule = cardContextMenu.rule
  if (key !== 'duplicate' || !rule) return
  void duplicateRule(rule, cardContextMenu.type)
}

function applyDefaultCronOnEnable(enabled: boolean, cronExpression: string) {
  return enabled && !cronExpression.trim() ? defaultCronExpression : cronExpression
}

watch(() => createForm.schedule_enabled, (enabled) => {
  createForm.cron_expression = applyDefaultCronOnEnable(enabled, createForm.cron_expression)
})

watch(() => editForm.schedule_enabled, (enabled) => {
  editForm.cron_expression = applyDefaultCronOnEnable(enabled, editForm.cron_expression)
})

watch(() => createPurifyForm.schedule_enabled, (enabled) => {
  createPurifyForm.cron_expression = applyDefaultCronOnEnable(enabled, createPurifyForm.cron_expression)
})

watch(() => editPurifyForm.schedule_enabled, (enabled) => {
  editPurifyForm.cron_expression = applyDefaultCronOnEnable(enabled, editPurifyForm.cron_expression)
})

watch(() => createLinkForm.schedule_enabled, (enabled) => {
  createLinkForm.cron_expression = enabled && !createLinkForm.cron_expression.trim() ? '30 4 * * *' : createLinkForm.cron_expression
})

watch(() => editLinkForm.schedule_enabled, (enabled) => {
  editLinkForm.cron_expression = enabled && !editLinkForm.cron_expression.trim() ? '30 4 * * *' : editLinkForm.cron_expression
})

// Strm 模式默认不勾选「新文件触发」。
watch(() => createLinkForm.link_mode, (mode, previous) => {
  if (mode === 'strm' && previous !== 'strm') {
    createLinkForm.monitor_enabled = false
  }
})

watch(() => editLinkForm.link_mode, (mode, previous) => {
  if (mode === 'strm' && previous !== 'strm') {
    editLinkForm.monitor_enabled = false
  }
})

function openCreateDialog() {
  resetCreateForm()
  createDialogVisible.value = true
}

function openCreatePurifyDialog() {
  resetCreatePurifyForm()
  createPurifyDialogVisible.value = true
}

function openCreateLinkDialog() {
  resetCreateLinkForm()
  createLinkDialogVisible.value = true
}

async function openEditLinkDialog(id: number) {
  try {
    const response = await fetchRule(id)
    const rule = response.data
    if (!rule) {
      throw new Error('规则不存在')
    }
    editingLinkRuleID.value = rule.id
    editLinkForm.name = rule.name
    editLinkForm.enabled = rule.enabled
    editLinkForm.monitor_enabled = rule.monitor_enabled
    editLinkForm.schedule_enabled = rule.run_mode === 'cron' || Boolean(rule.cron_expression)
    editLinkForm.source_dir = rule.source_dir
    editLinkForm.target_dir = rule.target_dir
    editLinkForm.cron_expression = rule.cron_expression || '30 4 * * *'
    editLinkForm.watch_debounce_ms = rule.watch_debounce_ms
    editLinkForm.run_on_start = rule.run_on_start
    editLinkForm.link_mode = normalizeLinkMode(rule.link_mode)
    editLinkForm.strm_sync_mode = parseStrmSyncMode(rule.options_json)
    editLinkForm.strm_overwrite = parseStrmOverwrite(rule.options_json)
    editLinkForm.strm_api_interval_seconds = parseStrmAPIIntervalSeconds(rule.option_values_json)
    editLinkForm.strm_min_video_mb = parseStrmMinVideoMB(rule.option_values_json)
    editLinkForm.strm_download_threads = parseStrmDownloadThreads(rule.option_values_json)
    const strmSuffixGroups = splitStrmSuffixes(normalizeStrmSuffixes(parseFiltersArray(rule.filters_json)))
    editLinkForm.strm_suffixes = strmSuffixGroups.media
    editLinkForm.strm_metadata_suffixes = strmSuffixGroups.metadata
    editLinkForm.filters_text = editLinkForm.link_mode === 'strm' ? parseFiltersJSON(rule.whitelist_json) : parseFiltersJSON(rule.filters_json)
    editLinkStrmSuffixInput.value = ''
    editLinkDialogVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '链路规则加载失败')
  }
}

function resolveInitialDirectory(form: Exclude<DirectoryPickerTarget, null>) {
  switch (form) {
    case 'create.source_dir':
      return createForm.source_dir
    case 'create.target_dir':
      return createForm.target_dir
    case 'edit.source_dir':
      return editForm.source_dir
    case 'edit.target_dir':
      return editForm.target_dir
    case 'createPurify.source_dir':
      return createPurifyForm.source_dirs[createPurifyForm.source_dirs.length - 1] || createPurifyForm.source_dir
    case 'editPurify.source_dir':
      return editPurifyForm.source_dirs[editPurifyForm.source_dirs.length - 1] || editPurifyForm.source_dir
    case 'createLink.source_dir':
      return createLinkForm.source_dir
    case 'createLink.target_dir':
      return createLinkForm.target_dir
    case 'editLink.source_dir':
      return editLinkForm.source_dir
    case 'editLink.target_dir':
      return editLinkForm.target_dir
    case 'createNaming.source_dir':
      return createNamingForm.source_dir
    default:
      return ''
  }
}

function openDirectoryPicker(form: 'create' | 'edit' | 'createPurify' | 'editPurify' | 'createLink' | 'editLink' | 'createNaming', field: 'source_dir' | 'target_dir') {
  const target = `${form}.${field}` as DirectoryPickerTarget
  directoryPickerTarget.value = target
  directoryPickerInitialPath.value = target ? resolveInitialDirectory(target as Exclude<DirectoryPickerTarget, null>) : ''
  directoryPickerVisible.value = true
}

function applyDirectorySelection(path: string) {
  switch (directoryPickerTarget.value) {
    case 'create.source_dir':
      createForm.source_dir = path
      break
    case 'create.target_dir':
      createForm.target_dir = path
      break
    case 'edit.source_dir':
      editForm.source_dir = path
      break
    case 'edit.target_dir':
      editForm.target_dir = path
      break
    case 'createPurify.source_dir':
      addPurifySourceDir('create', path)
      break
    case 'editPurify.source_dir':
      addPurifySourceDir('edit', path)
      break
    case 'createLink.source_dir':
      createLinkForm.source_dir = path
      break
    case 'createLink.target_dir':
      createLinkForm.target_dir = path
      break
    case 'editLink.source_dir':
      editLinkForm.source_dir = path
      break
    case 'editLink.target_dir':
      editLinkForm.target_dir = path
      break
    case 'createNaming.source_dir':
      createNamingForm.source_dir = path
      break
  }

  directoryPickerVisible.value = false
}

async function loadArchiveRules() {
  archiveLoading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchRules({
      page: archiveRulesCurrentPage.value,
      page_size: archiveRulesPageSize.value,
      rule_type: 'archive',
    })
    archiveRules.value = response.data?.items ?? []
    archiveRulesTotal.value = response.data?.total ?? 0
    await nextTick()
    setupRuleSortable('archive')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '归档规则加载失败'
  } finally {
    archiveLoading.value = false
  }
}

async function loadPurifyRules() {
  purifyLoading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchRules({
      page: purifyRulesCurrentPage.value,
      page_size: purifyRulesPageSize.value,
      rule_type: 'cleanup',
    })
    purifyRules.value = response.data?.items ?? []
    purifyRulesTotal.value = response.data?.total ?? 0
    await nextTick()
    setupRuleSortable('cleanup')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '净化规则加载失败'
  } finally {
    purifyLoading.value = false
  }
}

async function loadLinkRules() {
  linkLoading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchRules({
      page: linkRulesCurrentPage.value,
      page_size: linkRulesPageSize.value,
      rule_type: 'link',
    })
    linkRules.value = response.data?.items ?? []
    linkRulesTotal.value = response.data?.total ?? 0
    await nextTick()
    setupRuleSortable('link')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '链路规则加载失败'
  } finally {
    linkLoading.value = false
  }
}

function loadAvailableNamingRuleSets() {
  try {
    const parsed = JSON.parse(localStorage.getItem('nestify.namingRuleSets') || '[]') as StoredNamingRuleSet[]
    const singleRules = JSON.parse(localStorage.getItem('nestify.namingRules') || '[]') as Array<Record<string, unknown> & { id?: number; label?: string }>
    const singleOptions = Array.isArray(singleRules) ? singleRules.map((rule, index) => ({ id: -(Number(rule.id) || index + 1), name: `单规则：${String(rule.label || '命名规则')}`, rules: [rule] })) : []
    availableNamingRuleSets.value = [...singleOptions, ...(Array.isArray(parsed) ? parsed : [])]
  } catch {
    availableNamingRuleSets.value = []
  }
}

function openCreateNamingDialog() {
  loadAvailableNamingRuleSets()
  editingNamingRuleID.value = null
  editingNamingRule.value = null
  Object.assign(createNamingForm, { name: '', enabled: true, monitor_enabled: true, schedule_enabled: false, source_dir: '', cron_expression: '', rule_set_id: availableNamingRuleSets.value[0]?.id ?? null, options: { naming_include_files: false, naming_include_dirs: false } })
  createNamingDialogVisible.value = true
}

// 通过卡片单击进入命名规则编辑：把已存配置回填进同一个弹窗，提交时走更新接口。
async function openEditNamingDialog(id: number) {
  loadAvailableNamingRuleSets()
  errorMessage.value = ''
  try {
    const response = await fetchRule(id)
    const rule = response.data
    if (!rule) {
      throw new Error('规则不存在')
    }
    const options = parseOptionJSON(rule.options_json, { naming_include_files: false, naming_include_dirs: false })
    const matchedSet = availableNamingRuleSets.value.find((set) => set.name === rule.description)
    editingNamingRuleID.value = id
    editingNamingRule.value = rule
    Object.assign(createNamingForm, {
      name: rule.name,
      enabled: rule.enabled,
      monitor_enabled: rule.monitor_enabled,
      schedule_enabled: rule.run_mode === 'cron' || Boolean(rule.cron_expression),
      source_dir: rule.source_dir,
      cron_expression: rule.cron_expression || '',
      rule_set_id: matchedSet?.id ?? null,
      options: {
        naming_include_files: Boolean(options.naming_include_files),
        naming_include_dirs: Boolean(options.naming_include_dirs),
      },
    })
    createNamingDialogVisible.value = true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '命名规则加载失败'
  }
}

function namingRuleSetName(rule: RuleItem) {
  return rule.description || `${parseFiltersJSON(rule.transform_rules_json).length} 条命名规则`
}

async function loadNamingRules() {
  namingLoading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchRules({ page: 1, page_size: 50, rule_type: 'naming' })
    namingRules.value = response.data?.items ?? []
    await nextTick()
    setupRuleSortable('naming')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '命名规则加载失败'
  } finally {
    namingLoading.value = false
  }
}

function getRuleItemsByType(ruleType: RuleListType) {
  if (ruleType === 'archive') return archiveRules.value
  if (ruleType === 'cleanup') return purifyRules.value
  if (ruleType === 'link') return linkRules.value
  return namingRules.value
}

function setRuleItemsByType(ruleType: RuleListType, items: RuleItem[]) {
  if (ruleType === 'archive') {
    archiveRules.value = items
    return
  }
  if (ruleType === 'cleanup') {
    purifyRules.value = items
    return
  }
  if (ruleType === 'link') linkRules.value = items
  else namingRules.value = items
}

// 记录最近一次拖拽结束时间：Sortable 拖完后浏览器仍会补一个 click，
// 用它把这次「点击」吞掉，避免拖拽结束后顺手弹出编辑窗口。
let lastRuleDragAt = 0

function openRuleCardEdit(rule: RuleItem) {
  if (Date.now() - lastRuleDragAt < 260) return
  const ruleType = normalizeRuleType(rule)
  if (ruleType === 'cleanup') {
    void openEditPurifyDialog(rule.id)
    return
  }
  if (ruleType === 'link') {
    void openEditLinkDialog(rule.id)
    return
  }
  if (ruleType === 'naming') {
    void openEditNamingDialog(rule.id)
    return
  }
  void openEditDialog(rule.id)
}

function getGridElementByType(ruleType: RuleListType) {
  if (ruleType === 'archive') return archiveGridRef.value
  if (ruleType === 'cleanup') return purifyGridRef.value
  if (ruleType === 'link') return linkGridRef.value
  return namingGridRef.value
}

function getSortableByType(ruleType: RuleListType) {
  if (ruleType === 'archive') return archiveSortable
  if (ruleType === 'cleanup') return purifySortable
  if (ruleType === 'link') return linkSortable
  return namingSortable
}

function setSortableByType(ruleType: RuleListType, instance: Sortable | null) {
  if (ruleType === 'archive') {
    archiveSortable = instance
    return
  }
  if (ruleType === 'cleanup') {
    purifySortable = instance
    return
  }
  if (ruleType === 'link') {
    linkSortable = instance
    return
  }
  namingSortable = instance
}

function setupRuleSortable(ruleType: RuleListType) {
  const current = getSortableByType(ruleType)
  if (current) {
    current.destroy()
    setSortableByType(ruleType, null)
  }

  const grid = getGridElementByType(ruleType)
  if (!grid) return

  const sortable = Sortable.create(grid, {
    animation: 180,
    // 拖拽手柄固定在卡片左上角：卡片其余区域留给「单击打开编辑」，两者互不干扰。
    handle: '.rule-card__grip',
    ghostClass: 'rule-sortable-ghost',
    chosenClass: 'rule-sortable-chosen',
    dragClass: 'rule-sortable-drag',
    onStart: () => {
      lastRuleDragAt = Date.now()
    },
    onEnd: (event: SortableEvent) => {
      lastRuleDragAt = Date.now()
      void handleRuleReorder(ruleType, event.oldIndex ?? -1, event.newIndex ?? -1)
    },
  })
  setSortableByType(ruleType, sortable)
}

async function handleRuleReorder(ruleType: RuleListType, oldIndex: number, newIndex: number) {
  if (oldIndex < 0 || newIndex < 0 || oldIndex === newIndex || reorderingRuleType.value) return

  const items = [...getRuleItemsByType(ruleType)]
  const [moved] = items.splice(oldIndex, 1)
  if (!moved) return
  items.splice(newIndex, 0, moved)

  const previous = getRuleItemsByType(ruleType)
  setRuleItemsByType(ruleType, items)
  reorderingRuleType.value = ruleType

  try {
    await reorderRules(items.map((item, index) => ({ id: item.id, sort_order: index + 1 })))
    ElMessage.success('规则顺序已更新')
    if (ruleType === 'archive') await loadArchiveRules()
    else if (ruleType === 'cleanup') await loadPurifyRules()
    else if (ruleType === 'link') await loadLinkRules()
    else await loadNamingRules()
  } catch (error) {
    setRuleItemsByType(ruleType, previous)
    errorMessage.value = error instanceof Error ? error.message : '规则排序更新失败'
    await nextTick()
    setupRuleSortable(ruleType)
  } finally {
    reorderingRuleType.value = null
  }
}

async function loadHistoryViewModeSetting() {
  try {
    const response = await fetchSettings()
    historyViewMode.value = normalizeHistoryViewMode(response.data?.history_view_mode)
  } catch {
    historyViewMode.value = 'flat'
  }
}

async function loadHistory() {
  historyLoading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchRunHistory({
      page: historyCurrentPage.value,
      page_size: historyPageSize.value,
      keyword: historyKeyword.value || undefined,
      status: historyStatusFilter.value === 'all' ? undefined : historyStatusFilter.value,
      rule_type: historyRuleTypeFilter.value === 'all' ? undefined : historyRuleTypeFilter.value,
      sort_by: historySortBy.value,
      sort_order: historySortOrder.value,
      view_mode: historyViewMode.value,
    })
    historyItems.value = response.data?.items ?? []
    historyTotal.value = response.data?.total ?? 0
    historySummary.value = response.data?.summary ?? createDefaultHistorySummary()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '历史记录加载失败'
  } finally {
    historyLoading.value = false
  }
}

const TabKeys: TabKey[] = ['rules', 'purify', 'link', 'naming', 'backup', 'history']

function syncTabToUrl(tab: TabKey) {
  if (route.query.tab !== tab) {
    void router.replace({ query: { ...route.query, tab } })
  }
}

async function switchTab(tab: TabKey) {
  activeTab.value = tab
  // URL 同步失败不应阻断数据加载
  try {
    syncTabToUrl(tab)
  } catch {
    /* ignore */
  }

  if (tab === 'rules') {
    await loadArchiveRules()
    return
  }

  if (tab === 'purify') {
    await loadPurifyRules()
    return
  }

  if (tab === 'link') {
    await loadLinkRules()
    return
  }
  if (tab === 'naming') {
    await loadNamingRules()
    return
  }
  if (tab === 'backup') {
    return
  }

  await loadHistoryViewModeSetting()
  await loadHistory()
}

function handleLinkRulesPageChange(page: number) {
  linkRulesCurrentPage.value = page
  void loadLinkRules()
}

function handleLinkRulesPageSizeChange(size: number) {
  linkRulesPageSize.value = size
  linkRulesCurrentPage.value = 1
  void loadLinkRules()
}

function handleArchiveRulesPageChange(page: number) {
  archiveRulesCurrentPage.value = page
  void loadArchiveRules()
}

function handleArchiveRulesPageSizeChange(pageSize: number) {
  archiveRulesPageSize.value = pageSize
  archiveRulesCurrentPage.value = 1
  void loadArchiveRules()
}

function handlePurifyRulesPageChange(page: number) {
  purifyRulesCurrentPage.value = page
  void loadPurifyRules()
}

function handlePurifyRulesPageSizeChange(pageSize: number) {
  purifyRulesPageSize.value = pageSize
  purifyRulesCurrentPage.value = 1
  void loadPurifyRules()
}

function handleHistoryPageChange(page: number) {
  historyCurrentPage.value = page
  void loadHistory()
}

function handleHistoryPageSizeChange(pageSize: number) {
  historyPageSize.value = pageSize
  historyCurrentPage.value = 1
  void loadHistory()
}

function handleHistorySearch() {
  historyKeyword.value = historyKeywordInput.value.trim()
  historyCurrentPage.value = 1
  void loadHistory()
}

function resetHistorySearch() {
  historyKeywordInput.value = ''
  historyKeyword.value = ''
  historyCurrentPage.value = 1
  void loadHistory()
}

function handleHistorySortChange() {
	historyCurrentPage.value = 1
	void loadHistory()
}

function toggleHistorySortOrder() {
  historySortOrder.value = historySortOrder.value === 'asc' ? 'desc' : 'asc'
  handleHistorySortChange()
}

function handleHistoryStatusChange() {
	historyCurrentPage.value = 1
	void loadHistory()
}

function handleHistoryRuleTypeChange() {
	historyCurrentPage.value = 1
	void loadHistory()
}

function buildHistoryGroupKey(item: RunHistoryItem) {
  return [item.rule_id ?? 'manual', item.rule_name || '', item.trigger_mode || '', item.archive_mode || '', item.link_mode || '', item.started_at || ''].join('|')
}

function buildHistoryChildRow(item: RunHistoryItem): HistoryTreeRow {
  return {
    ...item,
    id: `item-${item.id}`,
    title: formatRunHistorySummary(item.summary) || item.summary || '未记录具体条目',
    description: `${item.rule_name || '未知规则'} · ${historyTriggerText(item)} · ${formatDateTime(item.started_at)}`,
    is_group: false,
    source: item,
  }
}

function resolveHistoryGroupStatus(items: RunHistoryItem[]) {
  if (items.some((item) => item.status === 'failed')) return 'failed'
  if (items.some((item) => item.status === 'skip')) return 'skip'
  return 'success'
}

function buildHistoryTreeRows(items: RunHistoryItem[]): HistoryTreeRow[] {
  const groups = new Map<string, RunHistoryItem[]>()
  for (const item of items) {
    const key = buildHistoryGroupKey(item)
    const current = groups.get(key)
    if (current) {
      current.push(item)
    } else {
      groups.set(key, [item])
    }
  }

  return Array.from(groups.entries()).map(([key, groupItems]) => {
    const first = groupItems[0]
    const processed = groupItems.reduce((total, item) => total + Math.max(0, Number(item.processed_files || 0)), 0) || groupItems.length
    const success = groupItems.reduce((total, item) => total + Math.max(0, Number(item.success_count || 0)), 0)
    const skipped = groupItems.reduce((total, item) => total + Math.max(0, Number(item.skip_count || 0)), 0)
    const failed = groupItems.reduce((total, item) => total + Math.max(0, Number(item.failure_count || 0)), 0)
    const sizeBytes = groupItems.reduce((total, item) => total + Math.max(0, Number(item.size_bytes || 0)), 0)
    const updatedAt = groupItems.reduce((latest, item) => {
      const value = item.updated_at || item.finished_at || item.started_at
      return !latest || new Date(value).getTime() > new Date(latest).getTime() ? value : latest
    }, first.updated_at || first.finished_at || first.started_at)

    return {
      ...first,
      id: `group-${key}`,
      source: first,
      status: resolveHistoryGroupStatus(groupItems),
      processed_files: processed,
      success_count: success,
      skip_count: skipped,
      failure_count: failed,
      size_bytes: sizeBytes,
      updated_at: updatedAt,
      title: `${historyModeLabel(first)}任务 · ${formatDateTime(first.started_at)}`,
      description: `${first.rule_name || '未知规则'} · ${historyTriggerText(first)} · 操作 ${processed} 个文件或文件夹 · 共 ${groupItems.length} 条明细`,
      is_group: true,
      children: groupItems.map(buildHistoryChildRow),
    }
  })
}

// 备份任务的详情需要现场拉取对应的备份规则卡片信息（源/目标/删除策略等）。
async function loadSelectedBackupTask(row: HistoryTreeRow) {
  selectedBackupTask.value = null
  if (row.archive_mode !== 'backup' || row.rule_id == null) {
    return
  }
  try {
    const payload = await fetchBackups()
    const items = payload.data?.items ?? []
    selectedBackupTask.value = items.find((task) => task.id === row.rule_id) ?? null
  } catch {
    selectedBackupTask.value = null
  }
}

function openHistoryDetailDialog(row: HistoryTreeRow) {
  if (!row.is_group) return
  selectedHistoryGroup.value = row
  selectedBackupTask.value = null
  selectedBackupDetail.value = null
  historyDetailCurrentPage.value = 1
  historyDetailDialogVisible.value = true
  if (row.archive_mode === 'backup') {
    void loadSelectedBackupTask(row)
    void loadSelectedBackupDetail(row)
  }
}

// 备份文件明细单独拉取：列表接口为避免响应过大不带 detail_json。
async function loadSelectedBackupDetail(row: HistoryTreeRow) {
  selectedBackupDetail.value = null
  const historyID = row.source?.id
  if (row.archive_mode !== 'backup' || !historyID) {
    return
  }
  try {
    const payload = await fetchRunHistoryDetail(historyID)
    selectedBackupDetail.value = payload.data?.item ?? null
  } catch {
    selectedBackupDetail.value = null
  }
}

// 平铺视图里把单条记录包装成「只有一个明细」的任务分组，同样可以打开详情。
function openHistoryItemDetail(item: RunHistoryItem) {
  openHistoryDetailDialog({
    ...item,
    id: `single-${item.id}`,
    title: `${historyModeLabel(item)}任务 · ${formatDateTime(item.started_at)}`,
    description: `${item.rule_name || '未知规则'} · ${historyTriggerText(item)}`,
    is_group: true,
    source: item,
    children: [buildHistoryChildRow(item)],
  })
}

async function removeHistoryDetailItem(row: HistoryTreeRow) {
  const sourceID = row.source?.id
  if (!sourceID) return
  await removeHistoryItem(sourceID)
  const remainingRows = selectedHistoryGroupChildren.value.filter((item) => item.source?.id !== sourceID)
  if (remainingRows.length === 0) {
    historyDetailDialogVisible.value = false
    selectedHistoryGroup.value = null
    return
  }
  const maxPage = Math.max(1, Math.ceil(remainingRows.length / historyDetailPageSize))
  if (historyDetailCurrentPage.value > maxPage) {
    historyDetailCurrentPage.value = maxPage
  }
}

function triggerModeText(mode?: string) {
  if (mode === 'cron') return '定时'
  if (mode === 'watch') return '监听'
  return '手动'
}

// 备份任务的触发方式用「实时监控 / 计划扫描 / 手动」表述，与其它规则区分。
function historyTriggerText(item?: { archive_mode?: string; trigger_mode?: string }) {
  if (item?.archive_mode === 'backup') {
    return backupTriggerLabel(item.trigger_mode)
  }
  return triggerModeText(item?.trigger_mode)
}

function historyModeLabel(item?: { archive_mode?: string; link_mode?: string }) {
	switch (item?.archive_mode) {
	case 'package':
		return '打包'
	case 'collect':
		return '收集'
	case 'cleanup':
		return '清理'
	case 'transform':
		return '转换'
	case 'link':
		if (item?.link_mode === 'strm') return 'Strm'
		return item?.link_mode === 'hard' ? '硬链' : '软链'
	case 'naming':
		return '命名'
	case 'backup':
		return '备份'
	default:
		return '—'
	}
}

function historyModeTagClass(item?: { archive_mode?: string; link_mode?: string }) {
	switch (item?.archive_mode) {
	case 'package':
		return 'custom-mode-tag--package'
	case 'collect':
		return 'custom-mode-tag--collect'
	case 'cleanup':
		return 'custom-mode-tag--cleanup'
	case 'transform':
		return 'custom-mode-tag--transform'
	case 'link':
		if (item?.link_mode === 'strm') return 'custom-mode-tag--strm'
		return item?.link_mode === 'hard' ? 'custom-mode-tag--hardlink' : 'custom-mode-tag--softlink'
	case 'naming':
		return 'custom-mode-tag--naming'
	case 'backup':
		return 'custom-mode-tag--backup'
	default:
		return ''
	}
}

function formatModeOptionDescription(description: string) {
	return description
}

async function submitCreateRule() {
  creating.value = true
  errorMessage.value = ''
  try {
    await createRule({
      name: createForm.name,
      description: createForm.description,
      enabled: createForm.enabled,
      monitor_enabled: createForm.monitor_enabled,
      compatibility_mode: createForm.compatibility_mode,
      archive_mode: createForm.archive_mode,
      run_mode: resolveRunMode(createForm.monitor_enabled, createForm.schedule_enabled),
      source_dir: createForm.source_dir,
      target_dir: createForm.target_dir,
      watch_debounce_ms: createForm.watch_debounce_ms,
      cron_expression: createForm.schedule_enabled ? createForm.cron_expression : '',
      run_on_start: createForm.run_on_start,
      package_options: normalizeFixedPackageOptions({ ...createForm.package_options }),
      collect_options: { ...createForm.collect_options },
      filters: parseFiltersText(createForm.filters_text),
      match_filters: createForm.archive_mode === 'package' && createForm.package_options.match_archive ? parseFiltersText(createForm.match_filters_text) : [],
      nest_filters: createForm.archive_mode === 'package' && createForm.package_options.single_file_nesting ? parseFiltersText(createForm.nest_filters_text) : [],
    })
    ElMessage.success('规则创建成功')
    createDialogVisible.value = false
    resetCreateForm()
    await loadArchiveRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则创建失败'
  } finally {
    creating.value = false
  }
}

async function openEditDialog(id: number) {
  editing.value = true
  errorMessage.value = ''
  try {
    const rule = (await fetchRule(id)).data
    if (!rule) throw new Error('规则详情不存在')

    editingRuleID.value = rule.id
    editForm.name = rule.name
    editForm.description = rule.description
    editForm.enabled = rule.enabled
    editForm.monitor_enabled = rule.monitor_enabled
    editForm.schedule_enabled = rule.run_mode === 'cron'
    editForm.compatibility_mode = rule.compatibility_mode || 'local'
    editForm.archive_mode = rule.archive_mode === 'collect' ? 'collect' : 'package'
    editForm.source_dir = rule.source_dir
    editForm.target_dir = rule.target_dir
    editForm.cron_expression = rule.cron_expression
    editForm.watch_debounce_ms = rule.watch_debounce_ms
    editForm.run_on_start = rule.run_on_start
    editForm.package_options = normalizeFixedPackageOptions(parseOptionJSON(rule.package_options_json, createDefaultPackageOptions()))
    editForm.collect_options = parseOptionJSON(rule.collect_options_json, createDefaultCollectOptions())
    editForm.filters_text = parseFiltersJSON(rule.filters_json)
    editForm.match_filters_text = parseFiltersJSON(rule.match_filters_json)
    editForm.nest_filters_text = parseFiltersJSON(rule.nest_filters_json)
    editDialogVisible.value = true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则详情加载失败'
  } finally {
    editing.value = false
  }
}

async function submitUpdateRule() {
  if (!editingRuleID.value) {
    errorMessage.value = '缺少规则 ID'
    return
  }

  editing.value = true
  errorMessage.value = ''
  try {
    await updateRule(editingRuleID.value, {
      name: editForm.name,
      description: editForm.description,
      enabled: editForm.enabled,
      monitor_enabled: editForm.monitor_enabled,
      compatibility_mode: editForm.compatibility_mode,
      archive_mode: editForm.archive_mode,
      run_mode: resolveRunMode(editForm.monitor_enabled, editForm.schedule_enabled),
      source_dir: editForm.source_dir,
      target_dir: editForm.target_dir,
      watch_debounce_ms: editForm.watch_debounce_ms,
      cron_expression: editForm.schedule_enabled ? editForm.cron_expression : '',
      run_on_start: editForm.run_on_start,
      package_options: normalizeFixedPackageOptions({ ...editForm.package_options }),
      collect_options: { ...editForm.collect_options },
      filters: parseFiltersText(editForm.filters_text),
      match_filters: editForm.archive_mode === 'package' && editForm.package_options.match_archive ? parseFiltersText(editForm.match_filters_text) : [],
      nest_filters: editForm.archive_mode === 'package' && editForm.package_options.single_file_nesting ? parseFiltersText(editForm.nest_filters_text) : [],
    })
    ElMessage.success('规则更新成功')
    editDialogVisible.value = false
    editingRuleID.value = null
    resetEditForm()
    await loadArchiveRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则更新失败'
  } finally {
    editing.value = false
  }
}

async function openEditPurifyDialog(id: number) {
  editing.value = true
  errorMessage.value = ''
  try {
    const rule = (await fetchRule(id)).data
    if (!rule) throw new Error('规则详情不存在')
    if (rule.archive_mode !== 'cleanup' && rule.archive_mode !== 'transform') throw new Error('当前规则不是净化规则')

    editingPurifyRuleID.value = rule.id
    editPurifyForm.name = rule.name
    editPurifyForm.enabled = rule.enabled
    editPurifyForm.monitor_enabled = rule.monitor_enabled
    editPurifyForm.schedule_enabled = rule.run_mode === 'cron'
    editPurifyForm.compatibility_mode = rule.compatibility_mode || 'local'
    editPurifyForm.source_dirs = getRuleSourceDirs(rule)
    editPurifyForm.source_dir = editPurifyForm.source_dirs[0] || rule.source_dir
    editPurifyForm.cron_expression = rule.cron_expression
    editPurifyForm.watch_debounce_ms = rule.watch_debounce_ms
    editPurifyForm.run_on_start = rule.run_on_start
    editPurifyForm.archive_mode = rule.archive_mode === 'transform' ? 'transform' : 'cleanup'
    editPurifyForm.options = parseOptionJSON(rule.options_json, createDefaultPurifyOptions())
    editPurifyForm.filters_text = parseFiltersJSON(rule.filters_json)
    editPurifyForm.whitelist_text = parseFiltersJSON(rule.whitelist_json)
    editPurifyForm.transform_rules_text = parseFiltersJSON(rule.transform_rules_json)
    editPurifyForm.transform_filters_text = parseFiltersJSON(rule.transform_filters_json)
    editPurifyDialogVisible.value = true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '净化规则详情加载失败'
  } finally {
    editing.value = false
  }
}

async function submitCreatePurifyRule() {
  const sourceDirs = normalizeSourceDirs(createPurifyForm.source_dir, createPurifyForm.source_dirs)
  if (sourceDirs.length === 0) {
    errorMessage.value = '请至少选择一个监控目录'
    return
  }

  creating.value = true
  errorMessage.value = ''
  try {
    await createRule({
      name: createPurifyForm.name,
      description: '',
      enabled: createPurifyForm.enabled,
      monitor_enabled: createPurifyForm.monitor_enabled,
      compatibility_mode: createPurifyForm.compatibility_mode,
      archive_mode: createPurifyForm.archive_mode,
      run_mode: resolveRunMode(createPurifyForm.monitor_enabled, createPurifyForm.schedule_enabled),
      source_dir: sourceDirs[0],
      source_dirs: sourceDirs,
      target_dir: '',
      watch_debounce_ms: createPurifyForm.watch_debounce_ms,
      cron_expression: createPurifyForm.schedule_enabled ? createPurifyForm.cron_expression : '',
      run_on_start: createPurifyForm.run_on_start,
      options: { ...createPurifyForm.options },
      filters: createPurifyForm.archive_mode === 'cleanup' ? parseFiltersText(createPurifyForm.filters_text) : [],
      whitelist: createPurifyForm.archive_mode === 'cleanup' && createPurifyForm.options.cleanup_empty_dirs ? parseFiltersText(createPurifyForm.whitelist_text) : [],
      transform_rules: createPurifyForm.archive_mode === 'transform' && createPurifyForm.options.convert_matching_text ? parseFiltersText(createPurifyForm.transform_rules_text) : [],
      transform_filters: createPurifyForm.archive_mode === 'transform' && createPurifyForm.options.filter_matching_text ? parseFiltersText(createPurifyForm.transform_filters_text) : [],
    })
    ElMessage.success('净化规则创建成功')
    createPurifyDialogVisible.value = false
    resetCreatePurifyForm()
    await loadPurifyRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '净化规则创建失败'
  } finally {
    creating.value = false
  }
}

async function submitUpdatePurifyRule() {
  if (!editingPurifyRuleID.value) {
    errorMessage.value = '缺少净化规则 ID'
    return
  }

  const sourceDirs = normalizeSourceDirs(editPurifyForm.source_dir, editPurifyForm.source_dirs)
  if (sourceDirs.length === 0) {
    errorMessage.value = '请至少选择一个监控目录'
    return
  }

  editing.value = true
  errorMessage.value = ''
  try {
    await updateRule(editingPurifyRuleID.value, {
      name: editPurifyForm.name,
      description: '',
      enabled: editPurifyForm.enabled,
      monitor_enabled: editPurifyForm.monitor_enabled,
      compatibility_mode: editPurifyForm.compatibility_mode,
      archive_mode: editPurifyForm.archive_mode,
      run_mode: resolveRunMode(editPurifyForm.monitor_enabled, editPurifyForm.schedule_enabled),
      source_dir: sourceDirs[0],
      source_dirs: sourceDirs,
      target_dir: '',
      watch_debounce_ms: editPurifyForm.watch_debounce_ms,
      cron_expression: editPurifyForm.schedule_enabled ? editPurifyForm.cron_expression : '',
      run_on_start: editPurifyForm.run_on_start,
      options: { ...editPurifyForm.options },
      filters: editPurifyForm.archive_mode === 'cleanup' ? parseFiltersText(editPurifyForm.filters_text) : [],
      whitelist: editPurifyForm.archive_mode === 'cleanup' && editPurifyForm.options.cleanup_empty_dirs ? parseFiltersText(editPurifyForm.whitelist_text) : [],
      transform_rules: editPurifyForm.archive_mode === 'transform' && editPurifyForm.options.convert_matching_text ? parseFiltersText(editPurifyForm.transform_rules_text) : [],
      transform_filters: editPurifyForm.archive_mode === 'transform' && editPurifyForm.options.filter_matching_text ? parseFiltersText(editPurifyForm.transform_filters_text) : [],
    })
    ElMessage.success('净化规则更新成功')
    editPurifyDialogVisible.value = false
    editingPurifyRuleID.value = null
    resetEditPurifyForm()
    await loadPurifyRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '净化规则更新失败'
  } finally {
    editing.value = false
  }
}

async function submitCreateLinkRule() {
  if (createLinkForm.link_mode === 'strm' && createLinkForm.strm_suffixes.length + createLinkForm.strm_metadata_suffixes.length === 0) {
    errorMessage.value = 'Strm 模式至少需要添加一个命中文件后缀'
    return
  }

  creating.value = true
  errorMessage.value = ''
  try {
    await createRule({
      name: createLinkForm.name,
      description: '',
      enabled: createLinkForm.enabled,
      monitor_enabled: createLinkForm.monitor_enabled,
      compatibility_mode: 'local',
      archive_mode: 'link',
      rule_type: 'link',
      link_mode: createLinkForm.link_mode,
      run_mode: resolveRunMode(createLinkForm.monitor_enabled, createLinkForm.schedule_enabled),
      source_dir: createLinkForm.source_dir,
      target_dir: createLinkForm.target_dir,
      watch_debounce_ms: createLinkForm.watch_debounce_ms,
      cron_expression: createLinkForm.schedule_enabled ? createLinkForm.cron_expression : '',
      run_on_start: createLinkForm.run_on_start,
      options: createLinkForm.link_mode === 'strm' ? buildStrmOptions(createLinkForm.strm_sync_mode, createLinkForm.strm_overwrite) : {},
      option_values: createLinkForm.link_mode === 'strm' ? buildStrmOptionValues(createLinkForm) : {},
      package_options: {},
      collect_options: {},
      filters: createLinkForm.link_mode === 'strm' ? mergeStrmSuffixes(createLinkForm.strm_suffixes, createLinkForm.strm_metadata_suffixes) : parseFiltersText(createLinkForm.filters_text),
      whitelist: createLinkForm.link_mode === 'strm' ? parseFiltersText(createLinkForm.filters_text) : [],
    })
    ElMessage.success('链路规则创建成功')
    createLinkDialogVisible.value = false
    resetCreateLinkForm()
    await loadLinkRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '链路规则创建失败'
  } finally {
    creating.value = false
  }
}

async function submitCreateNamingRule() {
  const selectedSet = availableNamingRuleSets.value.find((item) => item.id === createNamingForm.rule_set_id)
  if (!createNamingForm.name.trim() || !createNamingForm.source_dir.trim() || !selectedSet) {
    errorMessage.value = '请填写规则名称、监控路径并选择命名工坊规则集'
    return
  }
  creating.value = true
  errorMessage.value = ''
  try {
    const transformRules = selectedSet.rules.map((rule) => JSON.stringify(rule))
    const editingRule = editingNamingRuleID.value ? editingNamingRule.value : null

    if (editingRule) {
      await updateRule(editingRule.id, buildRuleUpdatePayload(editingRule, {
        name: createNamingForm.name,
        description: selectedSet.name,
        enabled: createNamingForm.enabled,
        monitor_enabled: createNamingForm.monitor_enabled,
        run_mode: resolveRunMode(createNamingForm.monitor_enabled, createNamingForm.schedule_enabled),
        source_dir: createNamingForm.source_dir,
        source_dirs: [createNamingForm.source_dir],
        target_dir: '',
        cron_expression: createNamingForm.schedule_enabled ? createNamingForm.cron_expression : '',
        options: { ...createNamingForm.options },
        transform_rules: transformRules,
      }))
      createNamingDialogVisible.value = false
      ElMessage.success('命名规则已更新')
    } else {
      await createRule({
        name: createNamingForm.name,
        description: selectedSet.name,
        enabled: createNamingForm.enabled,
        monitor_enabled: createNamingForm.monitor_enabled,
        compatibility_mode: 'local',
        archive_mode: 'naming',
        rule_type: 'naming',
        run_mode: resolveRunMode(createNamingForm.monitor_enabled, createNamingForm.schedule_enabled),
        source_dir: createNamingForm.source_dir,
        target_dir: '',
        watch_debounce_ms: 2000,
        cron_expression: createNamingForm.schedule_enabled ? createNamingForm.cron_expression : '',
        run_on_start: false,
        options: { ...createNamingForm.options },
        transform_rules: transformRules,
      })
      createNamingDialogVisible.value = false
      ElMessage.success('命名规则创建成功')
    }
    editingNamingRuleID.value = null
    editingNamingRule.value = null
    await loadNamingRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '命名规则保存失败'
  } finally {
    creating.value = false
  }
}

async function submitUpdateLinkRule() {
  if (!editingLinkRuleID.value) {
    errorMessage.value = '缺少链路规则 ID'
    return
  }

  if (editLinkForm.link_mode === 'strm' && editLinkForm.strm_suffixes.length + editLinkForm.strm_metadata_suffixes.length === 0) {
    errorMessage.value = 'Strm 模式至少需要添加一个命中文件后缀'
    return
  }

  editing.value = true
  errorMessage.value = ''
  try {
    await updateRule(editingLinkRuleID.value, {
      name: editLinkForm.name,
      description: '',
      enabled: editLinkForm.enabled,
      monitor_enabled: editLinkForm.monitor_enabled,
      compatibility_mode: 'local',
      archive_mode: 'link',
      rule_type: 'link',
      link_mode: editLinkForm.link_mode,
      run_mode: resolveRunMode(editLinkForm.monitor_enabled, editLinkForm.schedule_enabled),
      source_dir: editLinkForm.source_dir,
      target_dir: editLinkForm.target_dir,
      watch_debounce_ms: editLinkForm.watch_debounce_ms,
      cron_expression: editLinkForm.schedule_enabled ? editLinkForm.cron_expression : '',
      run_on_start: editLinkForm.run_on_start,
      options: editLinkForm.link_mode === 'strm' ? buildStrmOptions(editLinkForm.strm_sync_mode, editLinkForm.strm_overwrite) : {},
      option_values: editLinkForm.link_mode === 'strm' ? buildStrmOptionValues(editLinkForm) : {},
      package_options: {},
      collect_options: {},
      filters: editLinkForm.link_mode === 'strm' ? mergeStrmSuffixes(editLinkForm.strm_suffixes, editLinkForm.strm_metadata_suffixes) : parseFiltersText(editLinkForm.filters_text),
      whitelist: editLinkForm.link_mode === 'strm' ? parseFiltersText(editLinkForm.filters_text) : [],
    })
    ElMessage.success('链路规则更新成功')
    editLinkDialogVisible.value = false
    editingLinkRuleID.value = null
    resetEditLinkForm()
    await loadLinkRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '链路规则更新失败'
  } finally {
    editing.value = false
  }
}

const ruleExecutionHistoryWaitMs = 450
const ruleExecutionHistoryMaxAttempts = 12

function waitForRuleExecutionHistory() {
  return new Promise<void>((resolve) => {
    window.setTimeout(resolve, ruleExecutionHistoryWaitMs)
  })
}

async function waitUntilRuleExecutionSettled(runID?: string) {
  if (!runID) {
    await waitForRuleExecutionHistory()
    return
  }

  for (let attempt = 0; attempt < ruleExecutionHistoryMaxAttempts; attempt += 1) {
    const response = await fetchRun(runID)
    const status = response.data?.status
    if (status && status !== 'pending' && status !== 'running') {
      return
    }
    await waitForRuleExecutionHistory()
  }
}

async function prepareExecution(ruleID: number, options: Record<string, boolean> = {}) {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await prepareRuleExecution(ruleID, 'once', options)
    ElMessage.success('规则执行已启动')
    await waitUntilRuleExecutionSettled(response.data?.run.id)
    await loadArchiveRules()
    await loadPurifyRules()
    await loadLinkRules()
    historyCurrentPage.value = 1
    await switchTab('history')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则执行失败'
  } finally {
    loading.value = false
  }
}

async function handleStrmSyncCommand(ruleID: number, command: string) {
  await prepareExecution(ruleID, { strm_full_sync: command === 'full' })
}

async function refreshRuleList(type: RuleListType) {
  if (type === 'archive') {
    await loadArchiveRules()
    return
  }

  if (type === 'cleanup') {
    await loadPurifyRules()
    return
  }

  if (type === 'link') await loadLinkRules()
  else await loadNamingRules()
}

function isRuleStatusUpdating(ruleId: number) {
  return updatingRuleStatusIds.value.has(ruleId)
}

async function toggleRuleEnabled(rule: RuleItem) {
  const ruleType = normalizeRuleType(rule)
  const nextEnabled = !rule.enabled
  const loadingSet = new Set(updatingRuleStatusIds.value)
  let overrides: Partial<UpdateRulePayload> = { enabled: nextEnabled }

  if (ruleType !== 'archive') {
    const options = parseOptionJSON(rule.options_json, createDefaultPurifyOptions())
    if (!nextEnabled && rule.archive_mode === 'transform' && options.filter_matching_text) {
      overrides = {
        ...overrides,
        transform_filters: parseFiltersText(parseFiltersJSON(rule.transform_filters_json)),
        transform_rules: options.convert_matching_text ? parseFiltersText(parseFiltersJSON(rule.transform_rules_json)) : [],
      }
    }
  }

  loadingSet.add(rule.id)
  updatingRuleStatusIds.value = loadingSet
  errorMessage.value = ''

  try {
    await updateRule(rule.id, buildRuleUpdatePayload(rule, overrides))
    ElMessage.success(nextEnabled ? '规则已启用' : '规则已停用')
    await refreshRuleList(ruleType)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '状态更新失败')
    await refreshRuleList(ruleType)
  } finally {
    const nextLoadingSet = new Set(updatingRuleStatusIds.value)
    nextLoadingSet.delete(rule.id)
    updatingRuleStatusIds.value = nextLoadingSet
  }
}

async function removeRule(id: number, type: RuleListType) {
  try {
    await ElMessageBox.confirm('确认删除该规则？', '删除规则', { type: 'warning' })
    await deleteRule(id)
    ElMessage.success('规则已删除')
    if (type === 'archive') {
      await loadArchiveRules()
    } else if (type === 'link') {
      await loadLinkRules()
    } else if (type === 'naming') {
      await loadNamingRules()
    } else {
      await loadPurifyRules()
    }
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    errorMessage.value = error instanceof Error ? error.message : '删除规则失败'
  }
}

async function clearHistory(status: HistoryStatus) {
  try {
    await ElMessageBox.confirm(`确认删除全部${historyStatusText(status)}记录吗？`, '删除历史', { type: 'warning' })
    await clearRunHistory(status)
    historyCurrentPage.value = 1
    await loadHistory()
    ElMessage.success(`已删除${historyStatusText(status)}记录`)
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    errorMessage.value = error instanceof Error ? error.message : '删除历史记录失败'
  }
}

async function removeHistoryItem(id: string) {
  try {
    await deleteRunHistoryItem(id)
    await loadHistory()
    ElMessage.success('历史记录已删除')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '删除历史记录失败'
  }
}

onMounted(() => {
  window.addEventListener('wheel', handleRulesTableWheel, { passive: false })

  const initialTab = route.query.tab
  if (typeof initialTab === 'string' && (TabKeys as string[]).includes(initialTab)) {
    void switchTab(initialTab as TabKey)
  } else {
    // 默认归档规则也写入后缀，保证每个细分栏目都有专属浏览器后缀
    void switchTab('rules')
  }
  loadAvailableNamingRuleSets()
})

onBeforeUnmount(() => {
  window.removeEventListener('wheel', handleRulesTableWheel)
})
</script>

<style scoped>
.rules-page { display: flex; flex-direction: column; gap: 16px; }
.rules-tabs { display: flex; gap: 32px; padding: 0 4px; border-bottom: 1px solid var(--el-border-color-lighter); }
.rules-tabs__item { position: relative; padding: 12px 0; font-size: 15px; background: transparent; border: 0; cursor: pointer; color: var(--el-text-color-regular); transition: color 0.2s ease; }
.rules-tabs__item:hover { color: var(--el-color-primary); }
.rules-tabs__item.is-active { color: var(--el-color-primary); font-weight: 600; }
.rules-tabs__item.is-active::after { content: ''; position: absolute; left: 0; right: 0; bottom: -1px; height: 3px; background: var(--el-color-primary); border-radius: 999px; }
.rules-tabs__item--naming.is-active { color: #0f9f87; }
.rules-tabs__item--naming.is-active::after { background: #0f9f87; }
.rules-error { margin-bottom: 4px; }
.rules-card__header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.rules-card__title { font-size: 18px; font-weight: 700; color: var(--el-text-color-primary); }
.history-actions { display: flex; gap: 8px; }
.history-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.history-summary { display: flex; flex-wrap: wrap; gap: 12px; color: var(--el-text-color-secondary); }
.history-summary__controls { display: inline-flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.history-summary__control { width: 120px; }
.history-sort-order-button { width: 32px; height: 32px; min-height: 32px; color: var(--el-text-color-primary); background: rgba(255, 255, 255, 0.82); border-color: var(--el-border-color); transition: color 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease; }
.history-sort-order-button:not(.is-disabled):hover { color: #2f6fd6; border-color: #b7d8ff; background: #edf7ff; box-shadow: 0 12px 24px rgba(15, 23, 42, 0.1); transform: translateY(-1px); }
.history-sort-order-button__icon { display: block; width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.history-search { display: flex; align-items: center; gap: 8px; }
.history-search :deep(.el-input) { width: 260px; }
.history-pagination { display: flex; justify-content: flex-end; margin-top: 16px; }
.history-rule { display: flex; flex-direction: column; gap: 6px; }
.history-rule--child { padding-left: 6px; border-left: 3px solid var(--el-border-color-lighter); }
.history-rule__title { font-weight: 600; color: var(--el-text-color-primary); }
.history-rule__desc { line-height: 1.6; color: var(--el-text-color-secondary); }
.history-tree-table :deep(.el-table__row) { cursor: pointer; }
.history-detail-card { display: flex; flex-direction: column; gap: 6px; width: 100%; padding: 0; text-align: left; background: transparent; border: 0; cursor: pointer; }
.history-detail-card__title { font-weight: 700; color: var(--el-text-color-primary); }
.history-detail-card__desc { line-height: 1.6; color: var(--el-text-color-secondary); }
.detail-dialog-summary { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; margin-bottom: 16px; padding: 16px; border: 1px solid var(--el-border-color-lighter); border-radius: 18px; background: var(--el-fill-color-extra-light); }
.detail-dialog-summary__title { font-size: 16px; font-weight: 800; color: var(--el-text-color-primary); }
.detail-dialog-summary__desc { margin-top: 6px; line-height: 1.6; color: var(--el-text-color-secondary); }
.detail-dialog-summary__tags { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.detail-dialog-table { margin-top: 8px; }
.detail-dialog-pagination { display: flex; justify-content: flex-end; margin-top: 16px; }
/* 备份任务详情：规则卡片信息 + 来源去向 + 触发方式 + 删除情况 */
.detail-backup { margin-bottom: 16px; padding: 14px 16px; border: 1px solid var(--el-border-color-lighter); border-radius: 16px; background: var(--el-bg-color); }
.detail-backup__head { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin-bottom: 10px; }
.detail-backup__title { font-size: 14px; font-weight: 800; color: #5b7a6e; }
.detail-backup__hint { font-size: 12px; color: var(--el-text-color-secondary); }
.detail-backup__grid { display: flex; flex-direction: column; gap: 8px; }
.detail-backup__row { display: flex; align-items: flex-start; gap: 12px; }
.detail-backup__label { flex: 0 0 88px; font-size: 13px; font-weight: 600; color: var(--el-text-color-secondary); }
.detail-backup__values { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.detail-backup__value { font-size: 13px; line-height: 1.6; color: var(--el-text-color-primary); word-break: break-all; }
.history-status { display: inline-flex; align-items: center; justify-content: center; min-width: 68px; padding: 6px 10px; border-radius: 10px; border: 2px solid currentColor; font-weight: 700; transform: rotate(-8deg); }
.history-status.is-success { color: #22c55e; }
.history-status.is-skip { color: #f59e0b; }
.history-status.is-failed { color: #ef4444; }
.rules-page :deep(.rules-table) {
  width: 100%;
}

.rules-page :deep(.rules-table .el-table__inner-wrapper) {
  width: 100%;
}

.rules-page :deep(.rules-table .cell) {
  min-width: 0;
}

.rules-page :deep(.rules-table .el-table__cell:not(.gutter) .cell) {
  overflow: hidden;
  text-overflow: ellipsis;
}

.rules-page :deep(.rules-table .el-table__cell) {
  padding: 14px 0;
}

.rules-page :deep(.rules-table .el-table__cell.is-center .cell) {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.rules-page :deep(.rules-table .el-table__fixed-right) {
  z-index: 20;
  background: var(--el-bg-color);
  box-shadow: -12px 0 20px rgba(15, 23, 42, 0.12);
}

.rules-page :deep(.rules-table .el-table-fixed-column--right) {
  z-index: 21 !important;
  background: var(--el-bg-color) !important;
}

.rules-page :deep(.rules-table th.el-table-fixed-column--right) {
  background: var(--el-fill-color-light) !important;
}

.rules-page :deep(.rules-table .el-table__fixed-right::before),
.rules-page :deep(.rules-table .el-table__fixed-right-patch) {
  z-index: 22;
  background: var(--el-bg-color);
}

.rule-actions { display: inline-flex; align-items: center; justify-content: center; gap: 12px; white-space: nowrap; }
.rule-actions--nowrap { width: 100%; flex-wrap: nowrap; gap: 18px; }

/* 规则卡片网格：自适应列宽，整卡可拖拽排序（按钮区已被 Sortable 的 filter 排除）。
   最小列宽 430px 是按底部操作条一行放得下算出来的（启用 + 执行 + 编辑 + 移除 ≈ 400px 内容宽），
   改小会让「已启用」被挤到上一行。 */
.rule-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(430px, 1fr));
  gap: 14px;
  align-items: stretch;
}

.rule-card-grid > .rule-card {
  min-width: 0;
}

@media (max-width: 720px) {
  .rule-card-grid {
    grid-template-columns: 1fr;
  }
}

.rules-table-scroll {
  width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.38) transparent;
}
.rules-table-scroll::-webkit-scrollbar { height: 6px; }
.rules-table-scroll::-webkit-scrollbar-track { background: transparent; }
.rules-table-scroll::-webkit-scrollbar-thumb { background: rgba(148, 163, 184, 0.34); border-radius: 999px; }
.rules-table-scroll::-webkit-scrollbar-thumb:hover { background: rgba(100, 116, 139, 0.48); }
.rules-table--wide, .naming-rules-table { min-width: 1380px; }
.naming-add-button { color: #fff; border-color: #0f9f87; background: #0f9f87; }
.naming-add-button:hover { color: #fff; border-color: #0c8a75; background: #0c8a75; }
.rule-action--naming { color: #0f9f87; }
.rule-action__execute-icon--naming { stroke-width: 1.75; }
.rule-action { flex: 0 0 auto; padding: 4px; font-size: 18px; }
.rule-action--primary { color: var(--el-color-primary); }
.rule-action--success { color: var(--el-color-success); }
.rule-action--danger { color: var(--el-color-danger); }
.rule-action:hover { transform: translateY(-1px); }
.rule-action__execute-icon { display: block; width: 20px; height: 20px; fill: none; stroke: currentColor; stroke-width: 1.9; stroke-linecap: round; stroke-linejoin: round; }
.rule-actions :deep(.el-button + .el-button) { margin-left: 0; }
.custom-mode-tag { display: inline-flex; align-items: center; justify-content: center; min-width: 62px; padding: 4px 12px; border-radius: 8px; border: 1px solid currentColor; font-size: 14px; line-height: 1.2; background: var(--bg-elevated); }
.custom-mode-tag--package { color: #d58a2f; background: rgba(213, 138, 47, 0.08); }
.custom-mode-tag--collect { color: #8a74d6; background: rgba(138, 116, 214, 0.1); }
.custom-mode-tag--cleanup { color: #5f9f45; background: rgba(95, 159, 69, 0.12); }
.custom-mode-tag--transform { color: #64b9d8; background: rgba(100, 185, 216, 0.12); }
.custom-mode-tag--hardlink { color: #2f3136; background: rgba(47, 49, 54, 0.08); }
.custom-mode-tag--softlink { color: #c47c98; background: rgba(196, 124, 152, 0.12); }
.custom-mode-tag--strm { color: #2f8f9d; background: rgba(47, 143, 157, 0.12); }
.custom-mode-tag--naming { color: #0f8f79; background: rgba(15, 159, 135, 0.12); border-color: rgba(15, 159, 135, 0.28); }
/* 备份：莫奈低饱和雾霾蓝，与运行日志页的模式色保持一致 */
.custom-mode-tag--backup { color: #5f7fa8; background: rgba(95, 127, 168, 0.12); border-color: rgba(95, 127, 168, 0.28); }
.rules-page :deep(.rules-table .el-switch) {
  flex-shrink: 0;
}

.rules-page :deep(.rules-table .el-switch__label) {
  overflow: visible;
  text-overflow: clip;
}

.rules-page :deep(.rules-table .el-switch__label.is-active),
.rules-page :deep(.rules-table .el-switch__label.is-inactive) {
  min-width: auto;
}
.archive-mode-group,
.uniform-mode-group {
  display: inline-flex;
  width: 100%;
  max-width: 416px;
}

.archive-mode-group :deep(.el-radio-button),
.uniform-mode-group :deep(.el-radio-button) {
  flex: 1 1 0;
}

.archive-mode-group :deep(.el-radio-button__inner),
.uniform-mode-group :deep(.el-radio-button__inner) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
  width: 100%;
  padding-inline: 18px;
}

.archive-mode-group--package :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #d58a2f; background: rgba(213, 138, 47, 0.08); border-color: #d58a2f; box-shadow: -1px 0 0 0 #d58a2f; }
.archive-mode-group--collect :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #8a74d6; background: rgba(138, 116, 214, 0.1); border-color: #8a74d6; box-shadow: -1px 0 0 0 #8a74d6; }
.archive-mode-group--cleanup :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #5f9f45; background: rgba(95, 159, 69, 0.12); border-color: #5f9f45; box-shadow: -1px 0 0 0 #5f9f45; }
.archive-mode-group--transform :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #64b9d8; background: rgba(100, 185, 216, 0.12); border-color: #64b9d8; box-shadow: -1px 0 0 0 #64b9d8; }
.archive-mode-group--soft :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #c47c98; background: rgba(196, 124, 152, 0.12); border-color: #c47c98; box-shadow: -1px 0 0 0 #c47c98; }
.archive-mode-group--hard :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #2f3136; background: rgba(47, 49, 54, 0.08); border-color: #2f3136; box-shadow: -1px 0 0 0 #2f3136; }
.archive-mode-group--strm :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #2f8f9d; background: rgba(47, 143, 157, 0.12); border-color: #2f8f9d; box-shadow: -1px 0 0 0 #2f8f9d; }

.rules-page :deep(.rules-table--sortable .el-table__row) {
  transition: background-color 0.2s ease, box-shadow 0.2s ease;
}

.rules-page :deep(.rules-table--sortable .el-table__row:hover) {
  background: var(--el-fill-color-light);
}

.rule-sortable-ghost {
  opacity: 0.45;
}

/* 卡片式规则列表的拖拽反馈（表格版用的是 td 背景色）。 */
.rule-card-grid .rule-sortable-chosen {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 14px 30px rgba(91, 122, 110, 0.22);
}

.rule-card-grid .rule-sortable-drag {
  transform: rotate(1.2deg);
  box-shadow: 0 18px 36px rgba(15, 23, 42, 0.18);
}

.rules-page :deep(.rule-sortable-chosen > td) {
  background: var(--el-color-primary-light-9);
}

.rules-page :deep(.rule-sortable-drag > td) {
  background: var(--el-color-primary-light-8);
}

.rules-page :deep(.el-input-group__append) {
  padding: 0;
}

.rules-page :deep(.el-input-group__append .el-button) {
  min-width: 108px;
  height: 100%;
  margin: 0;
  border: 0;
  border-radius: 0;
}

.mode-config-panel { margin-bottom: 18px; padding: 16px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-fill-color-extra-light); }
.mode-config-panel__header { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 12px; }
.mode-config-panel__title { font-size: 14px; font-weight: 600; color: var(--el-text-color-primary); }
.mode-config-panel__description { margin-top: 4px; font-size: 12px; line-height: 1.5; color: var(--el-text-color-secondary); }
.transform-section-input { margin-top: -8px; margin-bottom: 12px; }
.transform-section-input--filters { margin-top: -6px; }
.cleanup-retention-input { margin-top: -6px; }
.cleanup-retention-input :deep(.el-input-number) { width: 100%; max-width: 320px; }
.mode-config-toggle { display: flex; width: 100%; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 12px; padding: 14px 16px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-fill-color-extra-light); cursor: pointer; text-align: left; }
.mode-config-toggle__meta { display: flex; align-items: center; gap: 10px; }
.mode-config-toggle__icon { font-size: 18px; line-height: 1; color: var(--el-text-color-secondary); transition: transform 0.2s ease; }
.mode-config-toggle__icon.is-expanded { transform: rotate(180deg); }
.naming-scope-panel { margin-bottom: 18px; }
.mode-option-card { display: flex; flex-direction: column; gap: 6px; min-height: 92px; padding: 14px 16px; margin-bottom: 12px; border: 1px solid var(--el-border-color); border-radius: 10px; background: var(--el-bg-color); cursor: pointer; }
.mode-option-card--compact { min-height: auto; padding: 12px 14px; }
.naming-scope-card { min-height: 92px; margin-bottom: 0; }
.naming-scope-card .mode-option-card__description { padding-left: 24px; }
.mode-option-card:hover { border-color: var(--el-color-primary-light-5); }
.mode-option-card.is-disabled { cursor: not-allowed; opacity: 0.68; }
.mode-option-card.is-disabled:hover { border-color: var(--el-border-color); }
.mode-option-card__description { padding-left: 24px; font-size: 12px; line-height: 1.5; color: var(--el-text-color-secondary); }
.mode-option-tooltip__content { max-width: 18em; line-height: 1.6; white-space: pre-wrap; word-break: break-all; }
:global(.mode-option-tooltip) { max-width: none; }
.purify-tags { display: flex; flex-wrap: wrap; gap: 8px; }
.source-dir-editor { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; width: 100%; padding: 12px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-bg-color); }
.source-dir-editor__list { display: flex; flex-wrap: wrap; gap: 8px; width: 100%; min-height: 32px; }
.source-dir-editor__tag { max-width: 100%; }
.source-dir-editor__tag :deep(.el-tag__content) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.source-dir-editor__empty { width: 100%; padding: 8px 0; }
.strm-suffix-editor { display: flex; flex-direction: column; gap: 10px; margin: -4px 0 16px; padding: 12px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-bg-color); }
.strm-suffix-editor__actions { display: flex; flex-wrap: wrap; gap: 14px; }
.strm-preset-button { display: inline-flex; align-items: center; justify-content: center; width: 42px; height: 42px; padding: 0; color: #2f8f9d; background: var(--el-bg-color); }
.strm-preset-button__icon { width: 21px; height: 21px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.strm-suffix-editor__tags { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; min-height: 32px; }
.strm-suffix-editor__input { width: 150px; }
.strm-preset-button.is-active { color: #fff; background: #2f8f9d; border-color: #2f8f9d; box-shadow: 0 6px 14px rgba(47, 143, 157, 0.22); }
/* Strm 数值参数行（API 请求间隔 / 最小视频 / 下载线程数）：窄输入框 + 紧随其后的单位。 */
.strm-option-number { width: 118px; }
.strm-option-unit { margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary); }
.strm-options-hint { margin: -4px 0 16px; font-size: 12px; line-height: 1.6; color: var(--el-text-color-secondary); }

/* —— 横栏（标题条）与其下方内容合并为同一个窗口 —— */
/* 横栏后面紧跟输入区 / 折叠面板时：去掉下方圆角与下边框，交给下方内容自己那条上边框当分隔线。 */
.mode-config-toggle:has(+ .transform-section-input),
.mode-config-toggle:has(+ .cleanup-retention-input),
.mode-config-toggle:has(+ .strm-suffix-editor),
.mode-config-toggle:has(+ .mode-config-panel:not([style*='display: none'])) {
  margin-bottom: 0;
  border-bottom-color: transparent;
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
}

/* 下方内容：去掉上方圆角并顶到横栏上，与外框拼成一个完整的窗口。 */
.transform-section-input,
.cleanup-retention-input,
.strm-suffix-editor {
  margin-top: 0;
}

.transform-section-input :deep(.el-textarea__inner),
.strm-suffix-editor,
.mode-config-panel {
  border-top-left-radius: 0;
  border-top-right-radius: 0;
}

/* 保留天数这类带表单标签的输入区：本身补上下半段外框，让标签也落在同一个窗口内。 */
.cleanup-retention-input {
  padding: 12px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 0 0 12px 12px;
  background: var(--el-bg-color);
}

@media (max-width: 900px) {
  .history-toolbar { flex-direction: column; align-items: stretch; }
  .history-summary__controls { width: 100%; }
  .history-summary__control { width: calc((100% - 48px) / 3); min-width: 0; }
  .history-search { flex-wrap: wrap; }
  .history-search :deep(.el-input) { width: 100%; }
}

@media (max-width: 1440px) {
  .rules-page :deep(.rules-table .el-switch__label) {
    display: none;
  }

  .rules-page :deep(.rules-table .el-table__cell) {
    padding: 12px 0;
  }
}
</style>
