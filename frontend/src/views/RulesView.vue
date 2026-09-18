<template>
  <div ref="rulesPageRef" class="rules-page">
    <div class="rules-tabs">
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'rules' }" @click="switchTab('rules')">归档规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'purify' }" @click="switchTab('purify')">净化规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'link' }" @click="switchTab('link')">链路规则</button>
      <button type="button" class="rules-tabs__item rules-tabs__item--naming" :class="{ 'is-active': activeTab === 'naming' }" @click="switchTab('naming')">命名规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'backup' }" @click="switchTab('backup')">备份规则</button>
      <button type="button" class="rules-tabs__item rules-tabs__item--history" :class="{ 'is-active': activeTab === 'history' }" @click="switchTab('history')">归巢历史</button>
    </div>

    <el-alert v-if="errorMessage" :closable="false" type="error" :title="errorMessage" class="rules-error" />

    <el-card v-show="activeTab === 'rules'" class="page-card rules-card">
      <template #header>
        <div class="rules-card__header">
          <div>
            <div class="rules-card__title">归档规则</div>
          </div>
          <el-button class="add-rule-button" type="primary" round @click="openCreateDialog">+ 添加规则</el-button>
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
          :running="isRuleRunning(rule.id)"
          :cancelling="isRuleCancelling(rule.id)"
          @execute="prepareExecution(rule.id)"
          @cancel="cancelRuleExecution(rule.id)"
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
        <!-- 归巢历史的工具条整体搬进卡片标题行：统计、排序、筛选、搜索、删除都和「归巢历史」
             排在同一行，下面整块高度让给列表。窄屏靠 flex-wrap 折行，不做横向滚动。 -->
        <div class="rules-card__header history-card__header">
          <div class="rules-card__header-main">
            <div class="rules-card__title">归巢历史</div>
            <div class="history-summary">
              <span>累计 {{ historySummary.total }}</span>
              <span>今日 {{ historySummary.today }}</span>
              <span>成功 {{ successCount }}</span>
              <span>跳过 {{ skipCount }}</span>
              <span>失败 {{ failedCount }}</span>
            </div>
          </div>
          <div class="history-header-controls">
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
            <!-- 搜索框回车即搜，不再挂「搜索 / 重置」按钮：清空有自带的小叉，回车就是提交。 -->
            <el-input
              v-model="historyKeywordInput"
              clearable
              class="history-search__input"
              placeholder="搜索规则名 / 摘要，回车"
              @keyup.enter="handleHistorySearch"
            />
            <!-- 原来是「删除成功 / 删除跳过 / 删除失败」三个按钮，收成一个下拉；
                 每一项仍会弹一次确认框（见 clearHistory）。 -->
            <el-dropdown trigger="click" @command="handleHistoryClear">
              <el-button type="danger" plain>
                删除<el-icon class="history-clear__caret"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="success">删除成功记录</el-dropdown-item>
                  <el-dropdown-item command="skip">删除跳过记录</el-dropdown-item>
                  <el-dropdown-item command="failed">删除失败记录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
      </template>

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
          <el-table-column label="折叠任务" width="620">
            <template #default="scope">
              <button type="button" class="history-detail-card" @click.stop="openHistoryDetailDialog(scope.row)">
                <span class="history-detail-card__title">{{ scope.row.title }}</span>
                <span class="history-detail-card__desc">{{ scope.row.description }}</span>
              </button>
            </template>
          </el-table-column>
          <!-- 时间紧跟在「折叠任务」后面，只到月日与时分秒：标题行已经不重复时间，
               列里再带上年份只是噪音。 -->
          <el-table-column label="时间" width="150">
            <template #default="scope">{{ formatMonthDayTime(scope.row.started_at) }}</template>
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
          <!-- 收尾的弹性空列：专门吃掉表格的剩余宽度。少了它，剩余宽度会被摊回上面各列，
               「折叠任务」又会被撑宽，后面的列就跟着往回跑。 -->
          <el-table-column min-width="1" />
        </el-table>

        <el-dialog v-model="historyDetailDialogVisible" class="history-detail-dialog" title="任务详情" width="1080px" top="3vh" destroy-on-close>
          <template v-if="selectedHistoryGroup">
            <div class="detail-dialog-summary">
              <div class="detail-dialog-summary__main">
                <div class="detail-dialog-summary__title">{{ selectedHistoryGroup.title }}</div>
                <!-- 统计项兼作筛选入口：点一下只看这一类明细，再点一下取消。
                     右上角不再挂「成功 / 失败」标签——一条执行里成功、跳过、失败本来就同在一行。 -->
                <div class="detail-dialog-summary__desc">
                  <span class="detail-summary__leading">{{ selectedHistoryGroupLeading }}</span>
                  <span class="detail-summary__sep">·</span>
                  <button
                    v-for="segment in selectedHistoryGroupSegments"
                    :key="segment.key"
                    type="button"
                    class="detail-summary__chip"
                    :class="[`is-${segment.key}`, { 'is-active': detailFilterKey === segment.key }]"
                    @click="toggleDetailFilter(segment.key)"
                  >
                    {{ segment.label }}<span class="detail-summary__count">{{ segment.value }}</span>
                  </button>
                </div>
              </div>
              <div class="detail-dialog-summary__tags">
                <span class="custom-mode-tag" :class="historyModeTagClass(selectedHistoryGroup)">{{ historyModeLabel(selectedHistoryGroup) }}</span>
              </div>
            </div>
            <RunDetailList :manifest="selectedRunDetailManifest" :filter-key="detailFilterKey" />
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
          <el-button class="add-rule-button" type="primary" round @click="openCreatePurifyDialog">+ 添加规则</el-button>
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
          :running="isRuleRunning(rule.id)"
          :cancelling="isRuleCancelling(rule.id)"
          @execute="prepareExecution(rule.id)"
          @cancel="cancelRuleExecution(rule.id)"
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
          <el-button class="add-rule-button" type="primary" round @click="openCreateLinkDialog">+ 添加规则</el-button>
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
          :running="isRuleRunning(rule.id)"
          :cancelling="isRuleCancelling(rule.id)"
          @execute="prepareExecution(rule.id)"
          @cancel="cancelRuleExecution(rule.id)"
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
          :running="isRuleRunning(rule.id)"
          :cancelling="isRuleCancelling(rule.id)"
          @execute="prepareExecution(rule.id)"
          @cancel="cancelRuleExecution(rule.id)"
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
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则名称"><el-input v-model="createNamingForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Cron 表达式"><el-input v-model="createNamingForm.cron_expression" placeholder="留空表示不启用计划执行，例如：0 8 * * *" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="监控路径"><el-input v-model="createNamingForm.source_dir"><template #append><el-button @click="openDirectoryPicker('createNaming', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="命名工坊规则或规则集"><el-select v-model="createNamingForm.rule_set_id" placeholder="请选择规则集" style="width:100%"><el-option v-for="set in availableNamingRuleSets" :key="set.id" :label="`${set.name}（${set.rules.length} 条）`" :value="set.id" /></el-select></el-form-item>
        <el-form-item class="mode-select-field">
          <template #label>
            <span class="mode-select-field__head">
              <span>功能模块</span>
              <button type="button" class="mode-select-field__all" @click="toggleAllMode(createNamingForm.options, namingScopeOptionKeys)">
                {{ createNamingSelection.length === namingScopeOptionKeys.length ? '取消全选' : '全选' }}
              </button>
            </span>
          </template>
          <div class="mode-chip-box">
            <el-tooltip v-for="option in namingScopeOptionKeys" :key="option.key" placement="top" :show-after="600" popper-class="mode-chip-tip">
              <template #content><span class="mode-chip-tip__text">{{ option.description }}</span></template>
              <button type="button" class="mode-chip" :class="{ 'is-on': isModeOptionOn(createNamingForm.options, option.key) }" @click="toggleModeOption(createNamingForm.options, option.key)">{{ option.label }}</button>
            </el-tooltip>
          </div>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="实时监控"><el-switch v-model="createNamingForm.monitor_enabled" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createNamingForm.enabled" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="createNamingDialogVisible = false">取消</el-button><el-button type="primary" :class="{ 'rule-save-button': !!editingNamingRuleID }" :loading="creating" @click="submitCreateNamingRule">{{ editingNamingRuleID ? '保存' : '创建' }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="createDialogVisible" title="新增规则" width="640px">
      <el-form label-position="top">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则名称"><el-input v-model="createForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Cron 表达式"><el-input v-model="createForm.cron_expression" placeholder="留空表示不启用计划执行，例如：0 8 * * *" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="归档模式">
              <el-select v-model="createForm.archive_mode" :class="['mode-select', `mode-select--${createForm.archive_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="打包模式" value="package" />
                <el-option label="收集模式" value="collect" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="监控模式">
              <el-select v-model="createForm.compatibility_mode" :class="['mode-select', `mode-select--${createForm.compatibility_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="本地模式" value="local" />
                <el-option label="兼容模式" value="compatibility" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item class="mode-select-field">
          <template #label>
            <span class="mode-select-field__head">
              <span>{{ getModeTitle(createForm.archive_mode) }}</span>
              <button type="button" class="mode-select-field__all" @click="toggleAllMode(createArchiveOptionSource, createArchiveOptionKeys)">
                {{ createArchiveSelection.length === createArchiveOptionKeys.length ? '取消全选' : '全选' }}
              </button>
            </span>
          </template>
          <div class="mode-chip-box">
            <el-tooltip v-for="option in createArchiveOptionKeys" :key="option.key" placement="top" :show-after="600" popper-class="mode-chip-tip">
              <template #content><span class="mode-chip-tip__text">{{ formatModeOptionDescription(option.description) }}</span></template>
              <button type="button" class="mode-chip" :class="{ 'is-on': isModeOptionOn(createArchiveOptionSource, option.key) }" @click="toggleModeOption(createArchiveOptionSource, option.key)">{{ option.label }}</button>
            </el-tooltip>
          </div>
        </el-form-item>
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
        <el-form-item class="transform-section-input"><el-input v-model="createForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        <template v-if="false">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤清除</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="createForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="实时监控"><el-switch v-model="createForm.monitor_enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createForm.enabled" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editDialogVisible" title="编辑规则" width="640px">
      <el-form label-position="top">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则名称"><el-input v-model="editForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Cron 表达式"><el-input v-model="editForm.cron_expression" placeholder="留空表示不启用计划执行，例如：0 8 * * *" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="归档模式">
              <el-select v-model="editForm.archive_mode" :class="['mode-select', `mode-select--${editForm.archive_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="打包模式" value="package" />
                <el-option label="收集模式" value="collect" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="监控模式">
              <el-select v-model="editForm.compatibility_mode" :class="['mode-select', `mode-select--${editForm.compatibility_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="本地模式" value="local" />
                <el-option label="兼容模式" value="compatibility" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item class="mode-select-field">
          <template #label>
            <span class="mode-select-field__head">
              <span>{{ getModeTitle(editForm.archive_mode) }}</span>
              <button type="button" class="mode-select-field__all" @click="toggleAllMode(editArchiveOptionSource, editArchiveOptionKeys)">
                {{ editArchiveSelection.length === editArchiveOptionKeys.length ? '取消全选' : '全选' }}
              </button>
            </span>
          </template>
          <div class="mode-chip-box">
            <el-tooltip v-for="option in editArchiveOptionKeys" :key="option.key" placement="top" :show-after="600" popper-class="mode-chip-tip">
              <template #content><span class="mode-chip-tip__text">{{ formatModeOptionDescription(option.description) }}</span></template>
              <button type="button" class="mode-chip" :class="{ 'is-on': isModeOptionOn(editArchiveOptionSource, option.key) }" @click="toggleModeOption(editArchiveOptionSource, option.key)">{{ option.label }}</button>
            </el-tooltip>
          </div>
        </el-form-item>
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
        <el-form-item class="transform-section-input"><el-input v-model="editForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        <template v-if="false">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤清除</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="editForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="实时监控"><el-switch v-model="editForm.monitor_enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editForm.enabled" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editDialogVisible = false">取消</el-button><el-button type="primary" class="rule-save-button" :loading="editing" @click="submitUpdateRule">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="createPurifyDialogVisible" title="新增净化规则" width="640px">
      <el-form label-position="top">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则名称"><el-input v-model="createPurifyForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Cron 表达式"><el-input v-model="createPurifyForm.cron_expression" placeholder="留空表示不启用计划执行，例如：0 8 * * *" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="规则模式">
              <el-select v-model="createPurifyForm.archive_mode" :class="['mode-select', `mode-select--${createPurifyForm.archive_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="清理模式" value="cleanup" />
                <el-option label="转换模式" value="transform" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="监控模式">
              <el-select v-model="createPurifyForm.compatibility_mode" :class="['mode-select', `mode-select--${createPurifyForm.compatibility_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="本地模式" value="local" />
                <el-option label="兼容模式" value="compatibility" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item class="mode-select-field">
          <template #label>
            <span class="mode-select-field__head">
              <span>{{ getPurifyModeTitle(createPurifyForm.archive_mode) }}</span>
              <button type="button" class="mode-select-field__all" @click="toggleAllMode(createPurifyOptionSource, createPurifyOptionKeys)">
                {{ createPurifySelection.length === createPurifyOptionKeys.length ? '取消全选' : '全选' }}
              </button>
            </span>
          </template>
          <div class="mode-chip-box">
            <el-tooltip v-for="option in createPurifyOptionKeys" :key="option.key" placement="top" :show-after="600" popper-class="mode-chip-tip">
              <template #content><span class="mode-chip-tip__text">{{ formatModeOptionDescription(option.description) }}</span></template>
              <button type="button" class="mode-chip" :class="{ 'is-on': isModeOptionOn(createPurifyOptionSource, option.key) }" @click="toggleModeOption(createPurifyOptionSource, option.key)">{{ option.label }}</button>
            </el-tooltip>
          </div>
        </el-form-item>
        <el-form-item label="监控目录">
          <div class="source-dir-editor">
            <div v-if="createPurifyForm.source_dirs.length" class="source-dir-editor__list">
              <el-tag v-for="dir in createPurifyForm.source_dirs" :key="dir" class="source-dir-editor__tag" closable @close="removeCreatePurifySourceDir(dir)">{{ dir }}</el-tag>
            </div>
            <p v-else class="source-dir-editor__placeholder">暂未选择监控目录</p>
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
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="实时监控"><el-switch v-model="createPurifyForm.monitor_enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createPurifyForm.enabled" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createPurifyDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreatePurifyRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editPurifyDialogVisible" title="编辑净化规则" width="640px">
      <el-form label-position="top">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则名称"><el-input v-model="editPurifyForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Cron 表达式"><el-input v-model="editPurifyForm.cron_expression" placeholder="留空表示不启用计划执行，例如：0 8 * * *" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="规则模式">
              <el-select v-model="editPurifyForm.archive_mode" :class="['mode-select', `mode-select--${editPurifyForm.archive_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="清理模式" value="cleanup" />
                <el-option label="转换模式" value="transform" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="监控模式">
              <el-select v-model="editPurifyForm.compatibility_mode" :class="['mode-select', `mode-select--${editPurifyForm.compatibility_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="本地模式" value="local" />
                <el-option label="兼容模式" value="compatibility" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item class="mode-select-field">
          <template #label>
            <span class="mode-select-field__head">
              <span>{{ getPurifyModeTitle(editPurifyForm.archive_mode) }}</span>
              <button type="button" class="mode-select-field__all" @click="toggleAllMode(editPurifyOptionSource, editPurifyOptionKeys)">
                {{ editPurifySelection.length === editPurifyOptionKeys.length ? '取消全选' : '全选' }}
              </button>
            </span>
          </template>
          <div class="mode-chip-box">
            <el-tooltip v-for="option in editPurifyOptionKeys" :key="option.key" placement="top" :show-after="600" popper-class="mode-chip-tip">
              <template #content><span class="mode-chip-tip__text">{{ formatModeOptionDescription(option.description) }}</span></template>
              <button type="button" class="mode-chip" :class="{ 'is-on': isModeOptionOn(editPurifyOptionSource, option.key) }" @click="toggleModeOption(editPurifyOptionSource, option.key)">{{ option.label }}</button>
            </el-tooltip>
          </div>
        </el-form-item>
        <el-form-item label="监控目录">
          <div class="source-dir-editor">
            <div v-if="editPurifyForm.source_dirs.length" class="source-dir-editor__list">
              <el-tag v-for="dir in editPurifyForm.source_dirs" :key="dir" class="source-dir-editor__tag" closable @close="removeEditPurifySourceDir(dir)">{{ dir }}</el-tag>
            </div>
            <p v-else class="source-dir-editor__placeholder">暂未选择监控目录</p>
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
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="实时监控"><el-switch v-model="editPurifyForm.monitor_enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editPurifyForm.enabled" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editPurifyDialogVisible = false">取消</el-button><el-button type="primary" class="rule-save-button" :loading="editing" @click="submitUpdatePurifyRule">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="createLinkDialogVisible" title="新增链路规则" width="640px">
      <el-form label-position="top">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则名称"><el-input v-model="createLinkForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Cron 表达式"><el-input v-model="createLinkForm.cron_expression" placeholder="留空表示不启用计划执行，例如：30 4 * * *" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="链路模式">
              <el-select v-model="createLinkForm.link_mode" :class="['mode-select', `mode-select--${createLinkForm.link_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="软链模式" value="soft" />
                <el-option label="硬链模式" value="hard" />
                <el-option label="Strm模式" value="strm" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="源路径"><el-input v-model="createLinkForm.source_dir"><template #append><el-button @click="openDirectoryPicker('createLink', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="createLinkForm.target_dir"><template #append><el-button @click="openDirectoryPicker('createLink', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <template v-if="createLinkForm.link_mode === 'strm'">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">媒体文件</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">Strm 后缀</el-tag></div>
          </button>
          <div class="strm-suffix-editor">
            <div class="strm-suffix-editor__actions">
              <el-tooltip content="视频" placement="top"><el-button class="strm-preset-button strm-preset-button--video" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_suffixes, 'video') }" circle aria-label="视频" @click="fillCreateStrmPreset('video')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="4" y="5" width="16" height="14" rx="2.5" /><path d="m10 9 5 3-5 3Z" /></svg></el-button></el-tooltip>
              <el-tooltip content="音频" placement="top"><el-button class="strm-preset-button strm-preset-button--audio" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_suffixes, 'audio') }" circle aria-label="音频" @click="fillCreateStrmPreset('audio')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M9 18.5a2.5 2.5 0 1 1-1.25-2.17V6.5l9-2v10" /><path d="M16.75 14.5a2.5 2.5 0 1 1-1.25-2.17" /><path d="M7.75 9.25l9-2" /></svg></el-button></el-tooltip>
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
              <el-tooltip content="图片" placement="top"><el-button class="strm-preset-button strm-preset-button--image" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_metadata_suffixes, 'image') }" circle aria-label="图片" @click="fillCreateStrmPreset('image')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="3.5" y="4.5" width="17" height="15" rx="2.5" /><circle cx="9" cy="10" r="1.5" /><path d="m5 18 4.5-4.5 3 3 2.5-2.5 4 4" /></svg></el-button></el-tooltip>
              <el-tooltip content="数据" placement="top"><el-button class="strm-preset-button strm-preset-button--data" :class="{ 'is-active': strmPresetActive(createLinkForm.strm_metadata_suffixes, 'data') }" circle aria-label="数据" @click="fillCreateStrmPreset('data')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M6 4.5h12a1.5 1.5 0 0 1 1.5 1.5v12a1.5 1.5 0 0 1-1.5 1.5H6A1.5 1.5 0 0 1 4.5 18V6A1.5 1.5 0 0 1 6 4.5Z" /><path d="M8.5 8.5h7M8.5 12h7M8.5 15.5h4" /></svg></el-button></el-tooltip>
            </div>
            <div class="strm-suffix-editor__tags">
              <el-tag v-for="suffix in createLinkForm.strm_metadata_suffixes" :key="suffix" closable @close="removeCreateMetadataSuffix(suffix)">{{ suffix }}</el-tag>
              <el-input v-model="createLinkMetadataSuffixInput" class="strm-suffix-editor__input" size="small" placeholder="输入扩展名回车，如 nfo" @keyup.enter="addCreateMetadataSuffix" />
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
          <el-form-item class="transform-section-input"><el-input v-model="createLinkForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <template v-else>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤名单</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="createLinkForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16">
          <el-col :span="linkSwitchSpan(createLinkForm.link_mode === 'strm')"><el-form-item label="实时监控"><el-switch v-model="createLinkForm.monitor_enabled" /></el-form-item></el-col>
          <el-col :span="linkSwitchSpan(createLinkForm.link_mode === 'strm')"><el-form-item label="启用规则"><el-switch v-model="createLinkForm.enabled" /></el-form-item></el-col>
          <el-col v-if="createLinkForm.link_mode === 'strm'" :span="linkSwitchSpan(true)"><el-form-item label="覆盖生成"><el-switch v-model="createLinkForm.strm_overwrite" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="createLinkDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateLinkRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editLinkDialogVisible" title="编辑链路规则" width="640px">
      <el-form label-position="top">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则名称"><el-input v-model="editLinkForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Cron 表达式"><el-input v-model="editLinkForm.cron_expression" placeholder="留空表示不启用计划执行，例如：30 4 * * *" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="链路模式">
              <el-select v-model="editLinkForm.link_mode" :class="['mode-select', `mode-select--${editLinkForm.link_mode}`]" style="width: 100%">
                <template #label="{ label }"><span class="mode-pill">{{ label }}</span></template>
                <el-option label="软链模式" value="soft" />
                <el-option label="硬链模式" value="hard" />
                <el-option label="Strm模式" value="strm" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="源路径"><el-input v-model="editLinkForm.source_dir"><template #append><el-button @click="openDirectoryPicker('editLink', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="editLinkForm.target_dir"><template #append><el-button @click="openDirectoryPicker('editLink', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <template v-if="editLinkForm.link_mode === 'strm'">
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">媒体文件</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">Strm 后缀</el-tag></div>
          </button>
          <div class="strm-suffix-editor">
            <div class="strm-suffix-editor__actions">
              <el-tooltip content="视频" placement="top"><el-button class="strm-preset-button strm-preset-button--video" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_suffixes, 'video') }" circle aria-label="视频" @click="fillEditStrmPreset('video')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="4" y="5" width="16" height="14" rx="2.5" /><path d="m10 9 5 3-5 3Z" /></svg></el-button></el-tooltip>
              <el-tooltip content="音频" placement="top"><el-button class="strm-preset-button strm-preset-button--audio" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_suffixes, 'audio') }" circle aria-label="音频" @click="fillEditStrmPreset('audio')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M9 18.5a2.5 2.5 0 1 1-1.25-2.17V6.5l9-2v10" /><path d="M16.75 14.5a2.5 2.5 0 1 1-1.25-2.17" /><path d="M7.75 9.25l9-2" /></svg></el-button></el-tooltip>
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
              <el-tooltip content="图片" placement="top"><el-button class="strm-preset-button strm-preset-button--image" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_metadata_suffixes, 'image') }" circle aria-label="图片" @click="fillEditStrmPreset('image')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><rect x="3.5" y="4.5" width="17" height="15" rx="2.5" /><circle cx="9" cy="10" r="1.5" /><path d="m5 18 4.5-4.5 3 3 2.5-2.5 4 4" /></svg></el-button></el-tooltip>
              <el-tooltip content="数据" placement="top"><el-button class="strm-preset-button strm-preset-button--data" :class="{ 'is-active': strmPresetActive(editLinkForm.strm_metadata_suffixes, 'data') }" circle aria-label="数据" @click="fillEditStrmPreset('data')"><svg viewBox="0 0 24 24" aria-hidden="true" class="strm-preset-button__icon"><path d="M6 4.5h12a1.5 1.5 0 0 1 1.5 1.5v12a1.5 1.5 0 0 1-1.5 1.5H6A1.5 1.5 0 0 1 4.5 18V6A1.5 1.5 0 0 1 6 4.5Z" /><path d="M8.5 8.5h7M8.5 12h7M8.5 15.5h4" /></svg></el-button></el-tooltip>
            </div>
            <div class="strm-suffix-editor__tags">
              <el-tag v-for="suffix in editLinkForm.strm_metadata_suffixes" :key="suffix" closable @close="removeEditMetadataSuffix(suffix)">{{ suffix }}</el-tag>
              <el-input v-model="editLinkMetadataSuffixInput" class="strm-suffix-editor__input" size="small" placeholder="输入扩展名回车，如 nfo" @keyup.enter="addEditMetadataSuffix" />
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
          <el-form-item class="transform-section-input"><el-input v-model="editLinkForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <template v-else>
          <button type="button" class="mode-config-toggle mode-config-toggle--secondary" disabled>
            <div><div class="mode-config-panel__title">过滤名单</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="warning">规则模板</el-tag></div>
          </button>
          <el-form-item class="transform-section-input"><el-input v-model="editLinkForm.filters_text" type="textarea" :rows="6" :placeholder="archiveRuleMatcherPlaceholder" /></el-form-item>
        </template>
        <el-row :gutter="16">
          <el-col :span="linkSwitchSpan(editLinkForm.link_mode === 'strm')"><el-form-item label="实时监控"><el-switch v-model="editLinkForm.monitor_enabled" /></el-form-item></el-col>
          <el-col :span="linkSwitchSpan(editLinkForm.link_mode === 'strm')"><el-form-item label="启用规则"><el-switch v-model="editLinkForm.enabled" /></el-form-item></el-col>
          <el-col v-if="editLinkForm.link_mode === 'strm'" :span="linkSwitchSpan(true)"><el-form-item label="覆盖生成"><el-switch v-model="editLinkForm.strm_overwrite" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="editLinkDialogVisible = false">取消</el-button><el-button type="primary" class="rule-save-button" :loading="editing" @click="submitUpdateLinkRule">保存</el-button></template>
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
import { ArrowDown, Delete, Edit } from '@element-plus/icons-vue'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import Sortable from 'sortablejs'
import type { SortableEvent } from 'sortablejs'

import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import BackupRulesPanel from '../components/BackupRulesPanel.vue'
import RunDetailList from '../components/RunDetailList.vue'
import RuleCard from '../components/RuleCard.vue'
import CardContextMenu, { type CardContextMenuItem } from '../components/CardContextMenu.vue'
import { cancelRun, fetchActiveRuns, prepareRuleExecution, type RunInstance } from '../api/executions'
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
import {
  backupTriggerLabel,
  buildRunDetailSummarySegments,
  parseRunDetail,
  type RunDetailSummaryKey,
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
}
type DirectoryPickerTarget = 'create.source_dir' | 'create.target_dir' | 'edit.source_dir' | 'edit.target_dir' | 'createPurify.source_dir' | 'editPurify.source_dir' | 'createLink.source_dir' | 'createLink.target_dir' | 'editLink.source_dir' | 'editLink.target_dir' | 'createNaming.source_dir' | null
type TabKey = 'rules' | 'purify' | 'link' | 'naming' | 'backup' | 'history'
type RuleListType = 'archive' | 'cleanup' | 'link' | 'naming'
type StoredNamingRuleSet = { id: number; name: string; rules: Array<Record<string, unknown>> }
type PurifyOptions = Record<CleanupOptionKey | TransformOptionKey, boolean>
type PurifyOptionValues = Record<CleanupOptionValueKey, number>

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

// 链路规则把后缀分成两块保存：
//   - filters：媒体后缀（生成 .strm）；
//   - metadata_filters：元数据后缀（图片 / 字幕 / nfo，按实体文件同步到目标目录）。
// 后端的元数据后缀是独立字段，所以保存时不再合并成一份；
// splitStrmSuffixes 只用于兼容历史规则（元数据后缀混在 filters 里）。
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

// 链路规则底部的开关行：Strm 模式下「实时监控 / 启用规则 / 覆盖生成」三个开关并排（各 8 栅格），
// 其它模式只有两个开关（实时监控 / 启用规则），保持各占一半。
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

// 折叠任务列表的「时间」列：只到月日与时分秒（9/17 14:05:03）。
// 折叠条目标题里已经不放时间了，列里再带上年份只是噪音。
function formatMonthDayTime(value?: string) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '—'
  }
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${date.getMonth() + 1}/${date.getDate()} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
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
  if (status === 'cancelled') return '已停止'
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
    // 元数据后缀必须原样带回：PUT 是整体覆盖，漏掉它会在开关启用/停用时把清单清空。
    metadata_filters: parseFiltersText(parseFiltersJSON(rule.metadata_filters_json)),
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
// 运行详情（detail_json）：各链路共用，不再只服务备份。
const selectedRunDetail = ref<RunHistoryItem | null>(null)
// 标题下统计项里点中的那一项：null = 不筛选，列出全部文件明细。
const detailFilterKey = ref<RunDetailSummaryKey | null>(null)

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
// 执行明细：本次执行真的动了哪些文件（备份上传/删除，strm 生成/元数据，打包产出…）。
const selectedRunDetailManifest = computed(() => parseRunDetail(selectedRunDetail.value ?? undefined))
// 详情窗口标题下的第一段文字：规则名 + 触发方式。成功 / 跳过 / 失败不再是死文本，
// 而是后面的可点击统计项（见 selectedHistoryGroupSegments）。
// strm 任务再补上「Strm N · 元数据 N」：面板里已去掉动作页签，这两类数目只能在这里给。
const selectedHistoryGroupLeading = computed(() => {
  const group = selectedHistoryGroup.value
  if (!group) {
    return ''
  }
  return `${group.rule_name || '未知规则'} · ${historyTriggerText(group)}`
})
// 统计项：成功 / 跳过 / 失败 + strm 链路额外的 Strm / 元数据。点击即筛选下面的文件明细。
const selectedHistoryGroupSegments = computed(() =>
  buildRunDetailSummarySegments(selectedHistoryGroup.value ?? {}, selectedRunDetailManifest.value),
)
// 点中的统计项：再点一次同一个即取消筛选。
function toggleDetailFilter(key: RunDetailSummaryKey) {
  detailFilterKey.value = detailFilterKey.value === key ? null : key
}

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

// —— 「功能模块」候选框 ——
// 历史形态是「折叠面板 + 一排复选框」（mode-config-toggle / mode-option-card），中途一度
// 改成多选下拉，现在定稿为「候选框 + 胶囊」：每个功能一个胶囊，点一下启用（高亮）、
// 再点一下停用（灰），悬停 0.6s 才出小字说明（见 .mode-chip-tip）。
// 库里仍然把选择存成 options 对象的布尔位（提交结构不动）；selection 计算属性只用于
// 「全选 / 取消全选」按钮的文案判断。
const namingScopeOptions = [
  { key: 'naming_include_dirs', label: '包含文件夹', description: '将规则集应用于监控路径下的文件夹' },
  { key: 'naming_include_files', label: '包含文件', description: '将规则集应用于监控路径下的文件' },
] as const

type ModeOptionItem = { key: string; label: string; description: string }
type ModeOptionsRecord = Record<string, boolean | undefined>

function pickModeKeys(options: ModeOptionsRecord, keys: readonly ModeOptionItem[]): string[] {
  return keys.filter((option) => options[option.key]).map((option) => option.key)
}

function setModeKeys(options: ModeOptionsRecord, keys: readonly ModeOptionItem[], selected: string[]): void {
  const chosen = new Set(selected)
  for (const option of keys) options[option.key] = chosen.has(option.key)
}

function modeSelection(resolveOptions: () => ModeOptionsRecord, resolveKeys: () => readonly ModeOptionItem[]) {
  return computed<string[]>({
    get: () => pickModeKeys(resolveOptions(), resolveKeys()),
    set: (selected) => setModeKeys(resolveOptions(), resolveKeys(), selected),
  })
}

// 「全选」按钮：已全中则整体取消，否则补齐。直接写回 options 对象（和 v-model 同一份数据），
// 不绕 selection 计算属性传参 —— 模板里 ref 会自动解包，传进去拿不到 .value。
function toggleAllMode(options: ModeOptionsRecord, keys: readonly ModeOptionItem[]) {
  const allSelected = keys.every((option) => options[option.key])
  for (const option of keys) options[option.key] = !allSelected
}

// 单个「功能模块」胶囊的开关。key 走索引签名，所以调用方传 options 对象本身
// （模板里计算属性会自动解包），别把数组型的 selection 传进来。
function toggleModeOption(options: ModeOptionsRecord, key: string) {
  options[key] = !options[key]
}

function isModeOptionOn(options: ModeOptionsRecord, key: string) {
  return Boolean(options[key])
}

const namingScopeOptionKeys: readonly ModeOptionItem[] = namingScopeOptions
const createNamingSelection = modeSelection(
  () => createNamingForm.options as ModeOptionsRecord,
  () => namingScopeOptionKeys,
)

const createArchiveOptionKeys = computed<readonly ModeOptionItem[]>(() => (createForm.archive_mode === 'package' ? packageModeOptions : collectModeOptions))
const createArchiveOptionSource = computed<ModeOptionsRecord>(
  () => (createForm.archive_mode === 'package' ? createForm.package_options : createForm.collect_options) as ModeOptionsRecord,
)
const createArchiveSelection = modeSelection(() => createArchiveOptionSource.value, () => createArchiveOptionKeys.value)

const editArchiveOptionKeys = computed<readonly ModeOptionItem[]>(() => (editForm.archive_mode === 'package' ? packageModeOptions : collectModeOptions))
const editArchiveOptionSource = computed<ModeOptionsRecord>(
  () => (editForm.archive_mode === 'package' ? editForm.package_options : editForm.collect_options) as ModeOptionsRecord,
)
const editArchiveSelection = modeSelection(() => editArchiveOptionSource.value, () => editArchiveOptionKeys.value)

const createPurifyOptionKeys = computed<readonly ModeOptionItem[]>(() => getPurifyModeOptions(createPurifyForm.archive_mode))
const createPurifyOptionSource = computed<ModeOptionsRecord>(() => createPurifyForm.options as ModeOptionsRecord)
const createPurifySelection = modeSelection(() => createPurifyOptionSource.value, () => createPurifyOptionKeys.value)

const editPurifyOptionKeys = computed<readonly ModeOptionItem[]>(() => getPurifyModeOptions(editPurifyForm.archive_mode))
const editPurifyOptionSource = computed<ModeOptionsRecord>(() => editPurifyForm.options as ModeOptionsRecord)
const editPurifySelection = modeSelection(() => editPurifyOptionSource.value, () => editPurifyOptionKeys.value)
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

// 「Cron 表达式」不再配开关：填了就是计划执行，留空就是不启用。
// schedule_enabled 保留下来当**派生标志**（由 cron 文本推导），这样提交 payload
// （cron_expression / run_mode）、resolveRunMode 以及既有的读取逻辑都不用改。
// 注意方向只能是 cron → 标志，反向回写 cron 会和这里互相触发。
function syncScheduleFromCron(form: { cron_expression: string; schedule_enabled: boolean }) {
  form.schedule_enabled = form.cron_expression.trim() !== ''
}

watch(() => createForm.cron_expression, () => syncScheduleFromCron(createForm))
watch(() => editForm.cron_expression, () => syncScheduleFromCron(editForm))
watch(() => createPurifyForm.cron_expression, () => syncScheduleFromCron(createPurifyForm))
watch(() => editPurifyForm.cron_expression, () => syncScheduleFromCron(editPurifyForm))
watch(() => createLinkForm.cron_expression, () => syncScheduleFromCron(createLinkForm))
watch(() => editLinkForm.cron_expression, () => syncScheduleFromCron(editLinkForm))
watch(() => createNamingForm.cron_expression, () => syncScheduleFromCron(createNamingForm))

// Strm 模式默认不勾选「实时监控」。
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
    // 元数据后缀是后端独立字段：优先按它还原；
    // 历史规则（升级前保存、元数据后缀混在 filters 里）回落到按预设拆分。
    const mediaSuffixes = normalizeStrmSuffixes(parseFiltersArray(rule.filters_json))
    const metadataSuffixes = normalizeStrmSuffixes(parseFiltersArray(rule.metadata_filters_json))
    if (metadataSuffixes.length > 0) {
      editLinkForm.strm_suffixes = mediaSuffixes
      editLinkForm.strm_metadata_suffixes = metadataSuffixes
    } else {
      const strmSuffixGroups = splitStrmSuffixes(mediaSuffixes)
      editLinkForm.strm_suffixes = strmSuffixGroups.media
      editLinkForm.strm_metadata_suffixes = strmSuffixGroups.metadata
    }
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

function resolveHistoryGroupStatus(items: RunHistoryItem[]) {
  if (items.some((item) => item.status === 'failed')) return 'failed'
  if (items.some((item) => item.status === 'skip')) return 'skip'
  if (items.some((item) => item.status === 'cancelled')) return 'cancelled'
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
      // 标题 = 「是什么任务 + 这条规则的自定义名」（如「备份任务 · 剧集追更备份」）：
      // 时间不放在这里，它在右侧的独立列里，两边重复没意义。
      title: `${historyModeLabel(first)}任务 · ${first.rule_name || '未知规则'}`,
      // 副行给「触发方式 + 操作量 + 明细条数」，折叠起来也看得出这次干了多少活。
      description: `${historyTriggerPhrase(first)} · 操作 ${processed} 个文件或文件夹 · 共 ${groupItems.length} 条明细`,
      is_group: true,
    }
  })
}

// 详情窗口 = 任务级摘要（标题 + 可点击的统计项 + 模式标签）+ 文件级执行明细。
// 备份任务的规则卡片（源 / 目标 / 扫描配置 / 删除策略）不在这里重复一遍：
// 那一整块就是备份卡片本身，照搬过来只会把明细挤下去。
function openHistoryDetailDialog(row: HistoryTreeRow) {
  if (!row.is_group) return
  selectedHistoryGroup.value = row
  selectedRunDetail.value = null
  // 换一条记录就回到「不筛选」，免得把上一条的筛选态带过来。
  detailFilterKey.value = null
  historyDetailDialogVisible.value = true
  void loadSelectedRunDetail(row)
}

// 执行明细单独拉取：列表接口为避免响应过大不带 detail_json。
// 各链路（备份 / strm / 打包…）共用同一个明细载荷，有就展示、没有就自然隐藏面板。
async function loadSelectedRunDetail(row: HistoryTreeRow) {
  selectedRunDetail.value = null
  const historyID = row.source?.id
  if (!historyID) {
    return
  }
  try {
    const payload = await fetchRunHistoryDetail(historyID)
    selectedRunDetail.value = payload.data?.item ?? null
  } catch {
    selectedRunDetail.value = null
  }
}

// 平铺视图里把单条记录包装成任务分组，同样可以打开详情。
// 标题 / 副行与折叠模式保持同一套措辞，免得同一个任务在两个视图里长得不一样。
function openHistoryItemDetail(item: RunHistoryItem) {
  openHistoryDetailDialog({
    ...item,
    id: `single-${item.id}`,
    title: `${historyModeLabel(item)}任务 · ${item.rule_name || '未知规则'}`,
    description: `${historyTriggerPhrase(item)} · 操作 ${Math.max(0, Number(item.processed_files || 0))} 个文件或文件夹`,
    is_group: true,
    source: item,
  })
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

// 折叠条目副行的触发措辞：统一带上「触发」二字
//（实时监控触发 / 计划扫描触发 / 手动执行触发），各链路一视同仁。
function historyTriggerPhrase(item?: { trigger_mode?: string }) {
  if (item?.trigger_mode === 'watch') {
    return '实时监控触发'
  }
  if (item?.trigger_mode === 'cron') {
    return '计划扫描触发'
  }
  return '手动执行触发'
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
      filters: createLinkForm.link_mode === 'strm' ? mergeStrmSuffixes(createLinkForm.strm_suffixes) : parseFiltersText(createLinkForm.filters_text),
      metadata_filters: createLinkForm.link_mode === 'strm' ? mergeStrmSuffixes(createLinkForm.strm_metadata_suffixes) : [],
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
      filters: editLinkForm.link_mode === 'strm' ? mergeStrmSuffixes(editLinkForm.strm_suffixes) : parseFiltersText(editLinkForm.filters_text),
      metadata_filters: editLinkForm.link_mode === 'strm' ? mergeStrmSuffixes(editLinkForm.strm_metadata_suffixes) : [],
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

// —— 执行状态常驻 + 二次点击停止 ——
// 以前点「执行」是「发请求 → 一直等到跑完 → 自动跳到归巢历史」，卡片上看不出在跑，
// 也没法中途叫停。现在改成：提交后立刻返回，卡片底部按钮常驻显示「执行中」，
// 再点一次就发停止请求。状态来自 /api/v1/runs/active 轮询
// （只回未结束的执行，比拉全量 runs 轻）。
const activeRuns = ref<RunInstance[]>([])
const cancellingRuleIds = ref<Set<number>>(new Set())

const activeRunPollRunningMs = 2000
const activeRunPollIdleMs = 8000
let activeRunPollTimer: number | undefined

function activeRunForRule(ruleID: number) {
  return activeRuns.value.find((run) => run.rule_id === ruleID)
}

function isRuleRunning(ruleID: number) {
  return activeRunForRule(ruleID) !== undefined
}

function isRuleCancelling(ruleID: number) {
  return cancellingRuleIds.value.has(ruleID)
}

async function refreshActiveRuns() {
  try {
    const response = await fetchActiveRuns()
    activeRuns.value = response.data?.items ?? []
  } catch {
    // 轮询失败不打断页面：保留上一次的状态，下个周期再试。
  }
}

function scheduleActiveRunPoll() {
  // 有任务在跑就 2s 一次（按钮上的状态要跟得上），空闲时降到 8s
  // —— 监控 / Cron 触发的执行也要能自动点亮「执行中」，所以不能干脆不轮询。
  const delay = activeRuns.value.length ? activeRunPollRunningMs : activeRunPollIdleMs
  activeRunPollTimer = window.setTimeout(async () => {
    await refreshActiveRuns()
    scheduleActiveRunPoll()
  }, delay)
}

async function prepareExecution(ruleID: number, options: Record<string, boolean> = {}) {
  loading.value = true
  errorMessage.value = ''
  try {
    await prepareRuleExecution(ruleID, 'once', options)
    // 不阻塞、不跳页：把状态刷出来，按钮自己会变成「执行中」。
    await refreshActiveRuns()
    ElMessage.success('规则执行已启动')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则执行失败'
  } finally {
    loading.value = false
  }
}

async function cancelRuleExecution(ruleID: number) {
  const run = activeRunForRule(ruleID)
  if (!run) return

  const nextCancelling = new Set(cancellingRuleIds.value)
  nextCancelling.add(ruleID)
  cancellingRuleIds.value = nextCancelling

  try {
    await cancelRun(run.id)
    ElMessage.success('已请求停止执行')
    await refreshActiveRuns()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '停止执行失败')
  } finally {
    const settled = new Set(cancellingRuleIds.value)
    settled.delete(ruleID)
    cancellingRuleIds.value = settled
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

// 删除下拉（成功 / 跳过 / 失败）：由 el-dropdown 的 command 传进来，
// 三个入口共用 clearHistory 里那套二次确认。
function handleHistoryClear(command: string | number | object) {
  if (command === 'success' || command === 'skip' || command === 'failed') {
    void clearHistory(command)
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

  void refreshActiveRuns().then(() => scheduleActiveRunPoll())
})

onBeforeUnmount(() => {
  window.removeEventListener('wheel', handleRulesTableWheel)
  if (activeRunPollTimer !== undefined) {
    window.clearTimeout(activeRunPollTimer)
    activeRunPollTimer = undefined
  }
})
</script>

<style scoped>
.rules-page { display: flex; flex-direction: column; gap: 16px; }
/* 顶部栏目是纯导航：禁掉双击选字（双击页签只切换，不留下文字选中高亮）。 */
.rules-tabs { display: flex; gap: 32px; padding: 0 4px; border-bottom: 1px solid var(--el-border-color-lighter); user-select: none; -webkit-user-select: none; }
.rules-tabs__item { position: relative; padding: 12px 0; font-size: 15px; background: transparent; border: 0; cursor: pointer; color: var(--el-text-color-regular); transition: color 0.2s ease; }
/* 页签选中态用比主色更深一档的青蓝（同色相 H203，明度 53%→46%）：
   #209fee 做 15px 文字在白底上偏飘，加深后才「有颜色」。 */
.rules-tabs__item:hover { color: #098ee1; }
.rules-tabs__item.is-active { color: #098ee1; font-weight: 600; }
.rules-tabs__item.is-active::after { content: ''; position: absolute; left: 0; right: 0; bottom: -1px; height: 3px; background: #098ee1; border-radius: 999px; }
.rules-tabs__item--naming.is-active { color: #0f9f87; }
.rules-tabs__item--naming.is-active::after { background: #0f9f87; }
/* 归巢历史单独走暖橙，与规则类页签的青蓝区分开。
   沿革：轮 69 #d99a00 → #e8a90c（琥珀金），轮 71 暖橙 #f07f16，轮 72 定稿 #f2a65a。
   橙色在 L51 的**感知亮度明显低于同明度的蓝**，看着就比旁边页签"沉"——所以「提亮 + 降饱和」
   双管齐下：明度 51% → 65%、饱和 88% → 85%，色相仍锁在 30° 暖橙区间
   —— 直观（一眼还是橙）、不辣眼（压了饱和）、清透（提了明度）。 */
.rules-tabs__item--history:hover { color: #f2a65a; }
.rules-tabs__item--history.is-active { color: #f2a65a; }
.rules-tabs__item--history.is-active::after { background: #f2a65a; }
.rules-error { margin-bottom: 4px; }
.rules-card__header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.rules-card__title { font-size: 18px; font-weight: 700; color: var(--el-text-color-primary); }
/* 归巢历史的标题行要装下「统计 + 排序 + 筛选 + 搜索 + 删除」，比其它页签满得多，
   所以这一页额外允许折行；其它页签的 .rules-card__header 不动。 */
.history-card__header { flex-wrap: wrap; row-gap: 10px; }
.rules-card__header-main { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 14px; min-width: 0; }
/* 右侧控件组：排序 / 状态 / 规则 / 搜索 / 删除。margin-left:auto 保证折行后也贴右。 */
.history-header-controls { display: flex; align-items: center; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-left: auto; }
/* 统计文字跟着标题走，比原来占一整行那版小一档，免得抢标题。 */
.history-summary { display: flex; flex-wrap: wrap; gap: 10px; font-size: 12px; color: var(--el-text-color-secondary); }
.history-summary__control { width: 118px; }
/* 删除下拉的箭头：紧贴文字，别把按钮撑宽。 */
.history-clear__caret { margin-left: 2px; font-size: 12px; }
.history-sort-order-button { width: 32px; height: 32px; min-height: 32px; color: var(--el-text-color-primary); background: rgba(255, 255, 255, 0.82); border-color: var(--el-border-color); transition: color 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease; }
.history-sort-order-button:not(.is-disabled):hover { color: #0975b8; border-color: rgba(32, 159, 238, 0.42); background: rgba(32, 159, 238, 0.14); box-shadow: 0 12px 24px rgba(15, 23, 42, 0.1); transform: translateY(-1px); }
.history-sort-order-button__icon { display: block; width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
/* 标题行里的搜索框：比独占一行那版窄一档，给删除下拉留位置。 */
.history-search__input { width: 230px; }
.history-pagination { display: flex; justify-content: flex-end; margin-top: 16px; }
.history-rule { display: flex; flex-direction: column; gap: 6px; }
.history-rule__title { font-weight: 600; color: var(--el-text-color-primary); }
.history-rule__desc { line-height: 1.6; color: var(--el-text-color-secondary); }
.history-tree-table :deep(.el-table__row) { cursor: pointer; }
/* 折叠任务列：文字整体往右挪一档（表头与内容一起挪，保持对齐）。
   为了让后面几列往左靠，任务列宽度由 min-width 改成了定宽，见模板注释。
   注意 `.history-tree-table` 本身就是 el-table 根元素，选择器里**不能再写 `.el-table`**
   （那要求 el-table 是它的后代，永远匹配不上）。 */
.history-tree-table :deep(th:first-child .cell),
.history-tree-table :deep(td:first-child .cell) { padding-left: 24px; }
/* 折叠任务列的任务条目（标题 + 副行）用鸿蒙字体，见 styles/index.scss 顶部的 @font-face。
   只作用于这张折叠表（平铺表的行是另一套标记），不动页面其它文字。 */
.history-detail-card { display: flex; flex-direction: column; gap: 6px; width: 100%; padding: 0; text-align: left; background: transparent; border: 0; cursor: pointer; font-family: var(--font-harmony); }
.history-detail-card__title { font-size: 16px; font-weight: 700; line-height: 1.55; color: var(--el-text-color-primary); }
.history-detail-card__desc { font-size: 13px; line-height: 1.6; color: var(--el-text-color-secondary); }
/* 详情弹窗高度固定，摘要卡定为不伸缩的顶块：内边距与外边距都收紧，
   让「源路径 / 目标路径」表头尽量贴近上面的标题与统计小字。 */
.detail-dialog-summary { display: flex; align-items: flex-start; justify-content: space-between; flex: 0 0 auto; gap: 18px; margin-bottom: 8px; padding: 8px 14px; border: 1px solid var(--el-border-color-lighter); border-radius: 14px; background: var(--el-fill-color-extra-light); }
.detail-dialog-summary__main { min-width: 0; flex: 1 1 auto; }
.detail-dialog-summary__title { font-size: 15px; font-weight: 800; color: var(--el-text-color-primary); }
/* 前缀文字与可点击的统计项排在一行，靠 gap 分隔，窄了自动换行。 */
.detail-dialog-summary__desc { display: flex; align-items: center; flex-wrap: wrap; gap: 4px; margin-top: 3px; font-size: 13px; font-weight: 600; line-height: 1.5; color: var(--el-text-color-secondary); }
.detail-dialog-summary__tags { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.history-status { display: inline-flex; align-items: center; justify-content: center; min-width: 68px; padding: 6px 10px; border-radius: 10px; border: 2px solid currentColor; font-weight: 700; transform: rotate(-8deg); }
.history-status.is-success { color: #22c55e; }
.history-status.is-skip { color: #f59e0b; }
.history-status.is-failed { color: #ef4444; }
/* 手动停止：中性灰蓝，既不亮成「成功」也不红成「失败」。 */
.history-status.is-cancelled { color: #64748b; }
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
/* 「+ 添加规则」按钮的配色统一收在全局 styles/index.scss（.el-button.add-rule-button）：
   冰蓝磨砂玻璃胶囊，归档 / 净化 / 链路 / 备份四处共用。这里不再单独覆盖文字色。 */
/* 命名规则那条添加按钮保持自己的专色绿（命名语义色不动），但工艺跟四处蓝按钮完全一致：
   半透明浅底 + 磨砂模糊 + 顶部白色高光 + 同色细描边，字色改对应的深绿（浅底上白字会糊）。 */
.naming-add-button {
  color: #0c6d5c;
  border-color: rgba(15, 159, 135, 0.48);
  background: rgba(15, 159, 135, 0.14);
  background-image: none;
  -webkit-backdrop-filter: blur(10px) saturate(160%);
  backdrop-filter: blur(10px) saturate(160%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72), 0 6px 16px rgba(15, 159, 135, 0.1);
}
.naming-add-button:hover {
  color: #0a5748;
  border-color: rgba(15, 159, 135, 0.76);
  background: rgba(15, 159, 135, 0.26);
  background-image: none;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.85), 0 8px 20px rgba(15, 159, 135, 0.18);
}
.naming-add-button:active {
  color: #084c40;
  border-color: rgba(15, 159, 135, 0.86);
  background: rgba(15, 159, 135, 0.34);
  background-image: none;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.55), 0 3px 10px rgba(15, 159, 135, 0.14);
}
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
/* —— 模式下拉选择器 ——
   归档模式 / 规则模式 / 链路模式 / 监控模式 原来是 el-radio-group 单选按钮组，
   现在统一改成 el-select。模式各自带颜色（沿用原单选按钮的色相），并且选定后不再用
   纯文字显示，而是走 #label 插槽渲染成「胶囊圆角框」（.mode-pill）—— 几何与配色
   对齐运行日志页的 .logs-mode-tag，扫一眼就知道当前是什么模式。 */
.mode-select {
  --mode-accent: var(--el-text-color-primary);
  --mode-tint: rgba(148, 163, 184, 0.12);
}

.mode-select :deep(.el-select__selected-item),
.mode-select :deep(.el-select__placeholder) {
  color: var(--mode-accent);
  font-weight: 600;
}

/* EP 把 #label 插槽的内容放在绝对定位的 .el-select__placeholder 里，胶囊自己
   决定高度即可，不会影响 wrapper 的排版。 */
.mode-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 62px;
  padding: 4px 12px;
  color: var(--mode-accent);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
  background: var(--mode-tint);
  border: 1px solid currentColor;
  border-radius: 8px;
}

.mode-select--package { --mode-accent: #d58a2f; --mode-tint: rgba(213, 138, 47, 0.08); }
.mode-select--collect { --mode-accent: #8a74d6; --mode-tint: rgba(138, 116, 214, 0.1); }
.mode-select--cleanup { --mode-accent: #5f9f45; --mode-tint: rgba(95, 159, 69, 0.12); }
.mode-select--transform { --mode-accent: #64b9d8; --mode-tint: rgba(100, 185, 216, 0.12); }
.mode-select--soft { --mode-accent: #c47c98; --mode-tint: rgba(196, 124, 152, 0.12); }
.mode-select--hard { --mode-accent: #2f3136; --mode-tint: rgba(47, 49, 54, 0.08); }
.mode-select--strm { --mode-accent: #2f8f9d; --mode-tint: rgba(47, 143, 157, 0.12); }
.mode-select--local { --mode-accent: #3f7fd8; --mode-tint: rgba(63, 127, 216, 0.12); }
.mode-select--compatibility { --mode-accent: #b5793a; --mode-tint: rgba(181, 121, 58, 0.12); }

/* 「功能模块」候选框：每个功能一个胶囊，点一下启用（浅底深字高亮），再点一下停用
   （灰底灰字），悬停 0.6s 才出小字说明（.mode-chip-tip，样式写在 styles/index.scss，
   因为 EP tooltip 会被 Teleport 到 body）。 */
.mode-chip-box {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  width: 100%;
  padding: 12px;
  background: var(--el-fill-color-extra-light);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
}

.mode-chip {
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  background: rgba(148, 163, 184, 0.14);
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 999px;
  cursor: pointer;
  transition: color 0.16s ease, background-color 0.16s ease, border-color 0.16s ease;
}

.mode-chip:hover {
  color: var(--el-text-color-primary);
  background: rgba(148, 163, 184, 0.22);
}

/* 勾选态：晴空蓝「浅底 + 深字」。原先的 #0975b8（L37）读起来发闷、饱和度也压得低，
   换成 #0f5c8f 配 18% 晴空蓝底 —— 亮度够、色相正，勾没勾一眼能分。 */
.mode-chip.is-on {
  color: #0f5c8f;
  background: rgba(11, 157, 248, 0.18);
  border-color: rgba(11, 157, 248, 0.52);
}

.mode-chip.is-on:hover {
  color: #0a4a73;
  background: rgba(11, 157, 248, 0.28);
  border-color: rgba(11, 157, 248, 0.74);
}

/* 「功能模块」多选下拉：标签行右侧挂一个「全选 / 取消全选」按钮。 */
.mode-select-field :deep(.el-form-item__label) {
  width: 100%;
}

.mode-select-field__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}

.mode-select-field__all {
  padding: 2px 10px;
  font-size: 12px;
  font-weight: 600;
  color: #0975b8;
  background: rgba(32, 159, 238, 0.1);
  border: 0;
  border-radius: 999px;
  cursor: pointer;
  transition: color 0.16s ease, background-color 0.16s ease;
}

.mode-select-field__all:hover {
  color: #055a8f;
  background: rgba(32, 159, 238, 0.2);
}

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
  box-shadow: 0 14px 30px rgba(32, 159, 238, 0.22);
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
.purify-tags { display: flex; flex-wrap: wrap; gap: 8px; }
.source-dir-editor { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; width: 100%; padding: 12px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-bg-color); }
.source-dir-editor__list { display: flex; flex-wrap: wrap; gap: 8px; width: 100%; min-height: 32px; }
/* 已添加的监控目录：EP 默认 tag 走主色浅档（appletv 主题下是 #73b5de），
   压在半透明玻璃底上几乎看不见。这里显式给「浅蓝底 + 深蓝字 + 可见描边」。 */
.source-dir-editor__tag {
  max-width: 100%;
  color: #0f5c8f;
  background: rgba(11, 157, 248, 0.16);
  border: 1px solid rgba(11, 157, 248, 0.42);
}
.source-dir-editor__tag :deep(.el-tag__content) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.source-dir-editor__placeholder { display: flex; align-items: center; width: 100%; min-height: 32px; margin: 0; padding: 0 2px; font-size: 13px; color: var(--el-text-color-secondary); }
.strm-suffix-editor { display: flex; flex-direction: column; gap: 10px; margin: -4px 0 16px; padding: 12px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-bg-color); }
.strm-suffix-editor__actions { display: flex; flex-wrap: wrap; gap: 14px; }
.strm-preset-button { display: inline-flex; align-items: center; justify-content: center; width: 42px; height: 42px; padding: 0; color: var(--preset-accent, #2f8f9d); background: var(--el-bg-color); }
/* 悬浮：跟着本按钮自己的色走，别回落到 EP 主色（否则每个按钮一悬停就变色）。 */
.strm-preset-button:hover { --el-button-hover-bg-color: var(--preset-accent-tint, var(--el-fill-color-light)); --el-button-hover-text-color: var(--preset-accent, #2f8f9d); --el-button-hover-border-color: var(--preset-accent, #2f8f9d); }
.strm-preset-button__icon { width: 21px; height: 21px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.strm-suffix-editor__tags { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; min-height: 32px; }
.strm-suffix-editor__input { width: 150px; }
/* 点亮态（高亮）：清透半透明浅底 + 本色的深字图标 + 本色描边 + 柔光晕，不做实心沉色块。 */
.strm-preset-button.is-active { color: var(--preset-accent-ink, #0f5c8f); background: var(--preset-accent-tint, rgba(47, 143, 157, 0.18)); border-color: var(--preset-accent-ring, rgba(47, 143, 157, 0.5)); box-shadow: 0 4px 12px var(--preset-accent-shadow, rgba(47, 143, 157, 0.2)); }
.strm-preset-button.is-active:hover { background: var(--preset-accent-tint-strong, rgba(47, 143, 157, 0.3)); border-color: var(--preset-accent-ring-strong, rgba(47, 143, 157, 0.72)); }
.strm-preset-button.is-active .strm-preset-button__icon { stroke-width: 2; }
/* 快捷添加扩展名的图标按钮：四类各一色，故意跳出蓝色家族（视频紫 / 音频红 / 图片黄绿 / 数据蓝）。
   每色五支变量：accent 常态描边 / ink 点亮后的深字 / tint 点亮后的半透明浅底 / ring 点亮描边 / shadow 柔光晕。 */
.strm-preset-button--video { --preset-accent: #9d7bf5; --preset-accent-ink: #7c4ddb; --preset-accent-tint: rgba(157, 123, 245, 0.18); --preset-accent-tint-strong: rgba(157, 123, 245, 0.3); --preset-accent-ring: rgba(157, 123, 245, 0.5); --preset-accent-ring-strong: rgba(157, 123, 245, 0.72); --preset-accent-shadow: rgba(157, 123, 245, 0.2); }
.strm-preset-button--audio { --preset-accent: #f2665a; --preset-accent-ink: #d43f30; --preset-accent-tint: rgba(242, 102, 90, 0.17); --preset-accent-tint-strong: rgba(242, 102, 90, 0.29); --preset-accent-ring: rgba(242, 102, 90, 0.5); --preset-accent-ring-strong: rgba(242, 102, 90, 0.72); --preset-accent-shadow: rgba(242, 102, 90, 0.19); }
.strm-preset-button--image { --preset-accent: #a4cd44; --preset-accent-ink: #6f9210; --preset-accent-tint: rgba(164, 205, 68, 0.22); --preset-accent-tint-strong: rgba(164, 205, 68, 0.34); --preset-accent-ring: rgba(164, 205, 68, 0.55); --preset-accent-ring-strong: rgba(164, 205, 68, 0.76); --preset-accent-shadow: rgba(164, 205, 68, 0.2); }
.strm-preset-button--data { --preset-accent: #4aa6ea; --preset-accent-ink: #1577bd; --preset-accent-tint: rgba(74, 166, 234, 0.17); --preset-accent-tint-strong: rgba(74, 166, 234, 0.29); --preset-accent-ring: rgba(74, 166, 234, 0.5); --preset-accent-ring-strong: rgba(74, 166, 234, 0.72); --preset-accent-shadow: rgba(74, 166, 234, 0.19); }
/* 后缀标签跟着所在窗口走：媒体文件一种蓝、元数据文件另一种蓝，一眼分得开。 */
.strm-suffix-editor:has(.strm-preset-button--video) :deep(.el-tag) { color: #0f5c8f; background: rgba(11, 157, 248, 0.16); border-color: rgba(11, 157, 248, 0.42); }
.strm-suffix-editor:has(.strm-preset-button--image) :deep(.el-tag) { color: #3f45b8; background: rgba(106, 111, 240, 0.15); border-color: rgba(106, 111, 240, 0.44); }
/* 规则弹窗「保存」按钮：清透天青蓝（与设置页「保存设置」同一支），比默认主色蓝再亮一档，别往深蓝走。 */
.rule-save-button { --el-button-bg-color: #12a9e8; --el-button-border-color: #12a9e8; --el-button-text-color: #ffffff; --el-button-hover-bg-color: #2eb8f0; --el-button-hover-border-color: #2eb8f0; --el-button-hover-text-color: #ffffff; --el-button-active-bg-color: #0d8fc6; --el-button-active-border-color: #0d8fc6; --el-button-active-text-color: #ffffff; --el-button-disabled-bg-color: rgba(18, 169, 232, 0.5); --el-button-disabled-border-color: rgba(18, 169, 232, 0.5); --el-button-disabled-text-color: #ffffff; }
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
  /* 标题行里的控件在窄屏铺满一行：搜索框跟着吃满剩余宽度。 */
  .history-header-controls { justify-content: flex-start; }
  .history-summary__control { width: calc((100% - 48px) / 3); min-width: 0; }
  .history-search__input { flex: 1 1 180px; width: auto; }
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
