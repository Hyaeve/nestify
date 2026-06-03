<template>
  <div class="rules-page">
    <div class="rules-tabs">
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'rules' }" @click="switchTab('rules')">归档规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'purify' }" @click="switchTab('purify')">净化规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'link' }" @click="switchTab('link')">链路规则</button>
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

      <el-table v-if="archiveRules.length" v-loading="archiveLoading" :data="archiveRules" class="rules-table" table-layout="auto">
        <el-table-column prop="name" label="规则名称" min-width="140" />
        <el-table-column label="模式" width="78">
          <template #default="scope">
            <el-tag type="info" effect="plain">{{ scope.row.archive_mode === 'package' ? '打包' : '收集' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="执行模式" width="110">
          <template #default="scope">
            <el-tag :type="scope.row.compatibility_mode === 'compatibility' ? 'warning' : 'success'" effect="plain">
              {{ scope.row.compatibility_mode === 'compatibility' ? '兼容模式' : '本地模式' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source_dir" label="源路径" min-width="320" show-overflow-tooltip />
        <el-table-column prop="target_dir" label="目标路径" min-width="320" show-overflow-tooltip />
        <el-table-column label="监控" min-width="110" align="center">
          <template #default="scope">
            <el-switch
              :model-value="scope.row.monitor_enabled"
              :loading="togglingRuleIds.has(scope.row.id)"
              inline-prompt
              active-text="开"
              inactive-text="关"
              @change="toggleRuleMonitorEnabled(scope.row, $event)"
            />
          </template>
        </el-table-column>
        <el-table-column label="状态" min-width="120" align="center">
          <template #default="scope">
            <el-switch
              :model-value="scope.row.enabled"
              :loading="togglingRuleIds.has(scope.row.id)"
              inline-prompt
              active-text="启用"
              inactive-text="停用"
              @change="toggleRuleEnabled(scope.row, $event)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="140" fixed="right" align="center">
          <template #default="scope">
            <div class="rule-actions">
              <el-tooltip content="编辑" placement="top">
                <el-button link class="rule-action rule-action--primary" @click="openEditDialog(scope.row.id)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="整理" placement="top">
                <el-button link class="rule-action rule-action--success" @click="prepareExecution(scope.row.id)">
                  <el-icon><Operation /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button link class="rule-action rule-action--danger" @click="removeRule(scope.row.id, 'archive')" aria-label="删除">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

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

      <el-table v-loading="historyLoading" :data="historyItems" class="rules-table" table-layout="auto">
        <el-table-column label="规则 / 摘要" min-width="360">
          <template #default="scope">
            <div class="history-rule">
              <div class="history-rule__title">{{ scope.row.rule_name || '未知规则' }}</div>
              <div class="history-rule__desc">{{ scope.row.summary || '—' }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="scope">
            <span class="history-status" :class="`is-${scope.row.status}`">{{ scope.row.status }}</span>
          </template>
        </el-table-column>
        <el-table-column label="统计" width="140">
          <template #default="scope">{{ scope.row.success_count }}/{{ scope.row.skip_count }}/{{ scope.row.failure_count }}</template>
        </el-table-column>
        <el-table-column label="时间" min-width="180">
          <template #default="scope">{{ formatDateTime(scope.row.started_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="scope">
            <el-button link type="danger" @click="removeHistoryItem(scope.row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

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
          <el-button type="primary" round @click="openCreatePurifyDialog">+ 添加净化规则</el-button>
        </div>
      </template>

      <el-table v-if="purifyRules.length" v-loading="purifyLoading" :data="purifyRules" class="rules-table" table-layout="auto">
        <el-table-column prop="name" label="规则名称" min-width="160" />
        <el-table-column label="模式" width="92">
          <template #default="scope">
            <el-tag :type="scope.row.archive_mode === 'transform' ? 'primary' : 'warning'" effect="plain">
              {{ scope.row.archive_mode === 'transform' ? '转换' : '清理' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="执行模式" width="110">
          <template #default="scope">
            <el-tag :type="scope.row.compatibility_mode === 'compatibility' ? 'warning' : 'success'" effect="plain">
              {{ scope.row.compatibility_mode === 'compatibility' ? '兼容模式' : '本地模式' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source_dir" label="监控目录" min-width="340" show-overflow-tooltip />
        <el-table-column label="功能项" min-width="260">
          <template #default="scope">
            <div class="purify-tags">
              <template v-if="scope.row.archive_mode === 'transform'">
                <el-tag v-if="parseOptionJSON(scope.row.options_json, createDefaultTransformOptions()).convert_traditional_to_simplified" size="small" effect="plain">繁简转换</el-tag>
                <el-tag v-if="parseOptionJSON(scope.row.options_json, createDefaultTransformOptions()).convert_matching_text" size="small" effect="plain">匹配转换</el-tag>
              </template>
              <template v-else>
                <el-tag v-if="parseOptionJSON(scope.row.options_json, createDefaultCleanupOptions()).cleanup_empty_dirs" size="small" effect="plain">空文件夹</el-tag>
                <el-tag v-if="parseOptionJSON(scope.row.options_json, createDefaultCleanupOptions()).cleanup_matching_files" size="small" effect="plain">匹配文件</el-tag>
              </template>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="监控" min-width="110" align="center">
          <template #default="scope">
            <el-switch
              :model-value="scope.row.monitor_enabled"
              :loading="togglingRuleIds.has(scope.row.id)"
              inline-prompt
              active-text="开"
              inactive-text="关"
              @change="toggleRuleMonitorEnabled(scope.row, $event)"
            />
          </template>
        </el-table-column>
        <el-table-column label="状态" min-width="120" align="center">
          <template #default="scope">
            <el-switch
              :model-value="scope.row.enabled"
              :loading="togglingRuleIds.has(scope.row.id)"
              inline-prompt
              active-text="启用"
              inactive-text="停用"
              @change="toggleRuleEnabled(scope.row, $event)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="140" fixed="right" align="center">
          <template #default="scope">
            <div class="rule-actions">
              <el-tooltip content="编辑" placement="top">
                <el-button link class="rule-action rule-action--primary" @click="openEditPurifyDialog(scope.row.id)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="净化" placement="top">
                <el-button link class="rule-action rule-action--success" @click="prepareExecution(scope.row.id)">
                  <el-icon><Operation /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button link class="rule-action rule-action--danger" @click="removeRule(scope.row.id, 'cleanup')" aria-label="删除">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

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
          <el-button type="primary" round @click="openCreateLinkDialog">+ 添加链路规则</el-button>
        </div>
      </template>

      <el-table v-if="linkRules.length" v-loading="linkLoading" :data="linkRules" class="rules-table" table-layout="auto">
        <el-table-column prop="name" label="规则名称" min-width="160" />
        <el-table-column label="模式" width="78">
          <template #default="scope">
            <el-tag :type="scope.row.link_mode === 'hard' ? 'danger' : 'primary'" effect="plain">
              {{ scope.row.link_mode === 'hard' ? '硬链模式' : '软链模式' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source_dir" label="监控目录" min-width="300" show-overflow-tooltip />
        <el-table-column prop="target_dir" label="链路目录" min-width="300" show-overflow-tooltip />
        <el-table-column label="监控" min-width="110" align="center">
          <template #default="scope">
            <el-switch
              :model-value="scope.row.monitor_enabled"
              :loading="togglingRuleIds.has(scope.row.id)"
              inline-prompt
              active-text="开"
              inactive-text="关"
              @change="toggleRuleMonitorEnabled(scope.row, $event)"
            />
          </template>
        </el-table-column>
        <el-table-column label="计划" min-width="88" align="center">
          <template #default="scope">
            <el-tag :type="scope.row.run_mode === 'cron' ? 'warning' : 'info'" effect="plain">{{ scope.row.run_mode === 'cron' ? '开' : '关' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Cron" min-width="140">
          <template #default="scope">{{ scope.row.cron_expression || '—' }}</template>
        </el-table-column>
        <el-table-column label="状态" min-width="120" align="center">
          <template #default="scope">
            <el-switch
              :model-value="scope.row.enabled"
              :loading="togglingRuleIds.has(scope.row.id)"
              inline-prompt
              active-text="启用"
              inactive-text="停用"
              @change="toggleRuleEnabled(scope.row, $event)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="140" fixed="right" align="center">
          <template #default="scope">
            <div class="rule-actions">
              <el-tooltip content="编辑" placement="top">
                <el-button link class="rule-action rule-action--primary" @click="openEditLinkDialog(scope.row.id)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="执行" placement="top">
                <el-button link class="rule-action rule-action--success" @click="prepareExecution(scope.row.id)">
                  <el-icon><Operation /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button link class="rule-action rule-action--danger" @click="removeRule(scope.row.id, 'link')" aria-label="删除">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

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

    <el-dialog v-model="createDialogVisible" title="新增规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="归档模式"><el-radio-group v-model="createForm.archive_mode" class="archive-mode-group"><el-radio-button value="package">打包归档</el-radio-button><el-radio-button value="collect">收集归档</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="createForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="createForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="createForm.compatibility_mode">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
          <div class="mode-config-panel__description">本地模式适用于本机目录操作；兼容模式用于挂载网盘等兼容性场景。</div>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="createArchiveOptionsExpanded = !createArchiveOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getModeTitle(createForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">当前模式</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': createArchiveOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="createArchiveOptionsExpanded" class="mode-config-panel">
              <el-row v-if="createForm.archive_mode === 'package'" :gutter="12"><el-col v-for="option in packageModeOptions" :key="option.key" :span="12"><el-tooltip :content="option.description" placement="top" effect="light"><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="createForm.package_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
              <el-row v-else :gutter="12"><el-col v-for="option in collectModeOptions" :key="option.key" :span="12"><el-tooltip :content="option.description" placement="top" effect="light"><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="createForm.collect_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="createForm.schedule_enabled" label="计划表达式"><el-input v-model="createForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="createForm.source_dir"><template #append><el-button @click="openDirectoryPicker('create', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="createForm.target_dir"><template #append><el-button @click="openDirectoryPicker('create', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item v-if="createForm.archive_mode === 'package'" label="匹配归档">
          <el-switch v-model="createForm.package_options.match_archive" inline-prompt active-text="开" inactive-text="关" />
          <div class="mode-config-panel__description">启用后，命中的文件不参与打包，而是直接转移到目标路径对应目录下；匹配时包含文件扩展名。</div>
        </el-form-item>
        <el-form-item v-if="createForm.archive_mode === 'package' && createForm.package_options.match_archive" label="匹配归档规则">
          <el-input v-model="createForm.match_filters_text" type="textarea" :rows="6" placeholder="一行一个关键词或扩展名；例如 mp4、cover。命中后直接转移到目标路径对应文件夹。" />
        </el-form-item>
        <el-form-item label="文件名匹配规则黑名单"><el-input v-model="createForm.filters_text" type="textarea" :rows="10" placeholder="一行一个字符串或正则；命中后跳过归档处理，仅匹配文件名。" /></el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="createForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editDialogVisible" title="编辑规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="editForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="归档模式"><el-radio-group v-model="editForm.archive_mode" class="archive-mode-group"><el-radio-button value="package">打包归档</el-radio-button><el-radio-button value="collect">收集归档</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="editForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="editForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="editForm.compatibility_mode">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
          <div class="mode-config-panel__description">本地模式适用于本机目录操作；兼容模式用于挂载网盘等兼容性场景。</div>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="editArchiveOptionsExpanded = !editArchiveOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getModeTitle(editForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag type="primary">当前模式</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': editArchiveOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="editArchiveOptionsExpanded" class="mode-config-panel">
              <el-row v-if="editForm.archive_mode === 'package'" :gutter="12"><el-col v-for="option in packageModeOptions" :key="option.key" :span="12"><el-tooltip :content="option.description" placement="top" effect="light"><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="editForm.package_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
              <el-row v-else :gutter="12"><el-col v-for="option in collectModeOptions" :key="option.key" :span="12"><el-tooltip :content="option.description" placement="top" effect="light"><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="editForm.collect_options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="editForm.schedule_enabled" label="计划表达式"><el-input v-model="editForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="editForm.source_dir"><template #append><el-button @click="openDirectoryPicker('edit', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="editForm.target_dir"><template #append><el-button @click="openDirectoryPicker('edit', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item v-if="editForm.archive_mode === 'package'" label="匹配归档">
          <el-switch v-model="editForm.package_options.match_archive" inline-prompt active-text="开" inactive-text="关" />
          <div class="mode-config-panel__description">启用后，命中的文件不参与打包，而是直接转移到目标路径对应目录下；匹配时包含文件扩展名。</div>
        </el-form-item>
        <el-form-item v-if="editForm.archive_mode === 'package' && editForm.package_options.match_archive" label="匹配归档规则">
          <el-input v-model="editForm.match_filters_text" type="textarea" :rows="6" placeholder="一行一个关键词或扩展名；例如 mp4、cover。命中后直接转移到目标路径对应文件夹。" />
        </el-form-item>
        <el-form-item label="文件名匹配规则黑名单"><el-input v-model="editForm.filters_text" type="textarea" :rows="10" placeholder="一行一个字符串或正则；命中后跳过归档处理，仅匹配文件名。" /></el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="editForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editDialogVisible = false">取消</el-button><el-button type="primary" :loading="editing" @click="submitUpdateRule">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="createPurifyDialogVisible" title="新增净化规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createPurifyForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则模式"><el-radio-group v-model="createPurifyForm.archive_mode"><el-radio-button value="cleanup">清理模式</el-radio-button><el-radio-button value="transform">转换模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="createPurifyForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="createPurifyForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="createPurifyForm.compatibility_mode">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
          <div class="mode-config-panel__description">净化规则同样支持兼容模式，适合挂载网盘目录；兼容模式下会限制为 1 秒最多读取 3 次。</div>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="createPurifyOptionsExpanded = !createPurifyOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getPurifyModeTitle(createPurifyForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag :type="createPurifyForm.archive_mode === 'transform' ? 'primary' : 'warning'">{{ createPurifyForm.archive_mode === 'transform' ? '转换模式' : '清理模式' }}</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': createPurifyOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="createPurifyOptionsExpanded" class="mode-config-panel">
              <el-row :gutter="12"><el-col v-for="option in getPurifyModeOptions(createPurifyForm.archive_mode)" :key="option.key" :span="12"><el-tooltip :content="option.description" placement="top" effect="light"><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="createPurifyForm.options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="createPurifyForm.schedule_enabled" label="计划表达式"><el-input v-model="createPurifyForm.cron_expression" /></el-form-item>
        <el-form-item label="监控目录"><el-input v-model="createPurifyForm.source_dir"><template #append><el-button @click="openDirectoryPicker('createPurify', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item v-if="createPurifyForm.archive_mode === 'cleanup'" label="文件名匹配规则">
          <el-input v-model="createPurifyForm.filters_text" type="textarea" :rows="10" placeholder="一行一个字符串或正则；仅匹配文件名。" />
        </el-form-item>
        <el-form-item v-if="createPurifyForm.archive_mode === 'transform'" label="匹配转换规则">
          <el-input v-model="createPurifyForm.transform_rules_text" type="textarea" :rows="10" placeholder="一行一条，格式：待转换 => 转换词；支持关键词部分匹配与正则转换。" />
        </el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createPurifyForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="createPurifyForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createPurifyDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreatePurifyRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editPurifyDialogVisible" title="编辑净化规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="editPurifyForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="规则模式"><el-radio-group v-model="editPurifyForm.archive_mode"><el-radio-button value="cleanup">清理模式</el-radio-button><el-radio-button value="transform">转换模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="editPurifyForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="editPurifyForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="editPurifyForm.compatibility_mode">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
          <div class="mode-config-panel__description">净化规则同样支持兼容模式，适合挂载网盘目录；兼容模式下会限制为 1 秒最多读取 3 次。</div>
        </el-form-item>
          <button type="button" class="mode-config-toggle" @click="editPurifyOptionsExpanded = !editPurifyOptionsExpanded">
            <div><div class="mode-config-panel__title">{{ getPurifyModeTitle(editPurifyForm.archive_mode) }}</div></div>
            <div class="mode-config-toggle__meta"><el-tag :type="editPurifyForm.archive_mode === 'transform' ? 'primary' : 'warning'">{{ editPurifyForm.archive_mode === 'transform' ? '转换模式' : '清理模式' }}</el-tag><span class="mode-config-toggle__icon" :class="{ 'is-expanded': editPurifyOptionsExpanded }">⌄</span></div>
          </button>
          <el-collapse-transition>
            <div v-show="editPurifyOptionsExpanded" class="mode-config-panel">
              <el-row :gutter="12"><el-col v-for="option in getPurifyModeOptions(editPurifyForm.archive_mode)" :key="option.key" :span="12"><el-tooltip :content="option.description" placement="top" effect="light"><label class="mode-option-card mode-option-card--compact"><el-checkbox v-model="editPurifyForm.options[option.key]">{{ option.label }}</el-checkbox></label></el-tooltip></el-col></el-row>
            </div>
          </el-collapse-transition>
        <el-form-item v-if="editPurifyForm.schedule_enabled" label="计划表达式"><el-input v-model="editPurifyForm.cron_expression" /></el-form-item>
        <el-form-item label="监控目录"><el-input v-model="editPurifyForm.source_dir"><template #append><el-button @click="openDirectoryPicker('editPurify', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item v-if="editPurifyForm.archive_mode === 'cleanup'" label="文件名匹配规则">
          <el-input v-model="editPurifyForm.filters_text" type="textarea" :rows="10" placeholder="一行一个字符串或正则；仅匹配文件名。" />
        </el-form-item>
        <el-form-item v-if="editPurifyForm.archive_mode === 'transform'" label="匹配转换规则">
          <el-input v-model="editPurifyForm.transform_rules_text" type="textarea" :rows="10" placeholder="一行一条，格式：待转换 => 转换词；支持关键词部分匹配与正则转换。" />
        </el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editPurifyForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="editPurifyForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editPurifyDialogVisible = false">取消</el-button><el-button type="primary" :loading="editing" @click="submitUpdatePurifyRule">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="createLinkDialogVisible" title="新增链路规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createLinkForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="链路模式"><el-radio-group v-model="createLinkForm.link_mode" class="archive-mode-group"><el-radio-button value="soft">软链模式</el-radio-button><el-radio-button value="hard">硬链模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="createLinkForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="createLinkForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item v-if="createLinkForm.schedule_enabled" label="计划表达式"><el-input v-model="createLinkForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="createLinkForm.source_dir"><template #append><el-button @click="openDirectoryPicker('createLink', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="createLinkForm.target_dir"><template #append><el-button @click="openDirectoryPicker('createLink', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="文件名匹配规则黑名单"><el-input v-model="createLinkForm.filters_text" type="textarea" :rows="10" placeholder="一行一个字符串或正则；命中后跳过创建链路，仅匹配文件名。" /></el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createLinkForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="createLinkForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createLinkDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateLinkRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editLinkDialogVisible" title="编辑链路规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="editLinkForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="链路模式"><el-radio-group v-model="editLinkForm.link_mode" class="archive-mode-group"><el-radio-button value="soft">软链模式</el-radio-button><el-radio-button value="hard">硬链模式</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="editLinkForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="editLinkForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item v-if="editLinkForm.schedule_enabled" label="计划表达式"><el-input v-model="editLinkForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="editLinkForm.source_dir"><template #append><el-button @click="openDirectoryPicker('editLink', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="editLinkForm.target_dir"><template #append><el-button @click="openDirectoryPicker('editLink', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="文件名匹配规则黑名单"><el-input v-model="editLinkForm.filters_text" type="textarea" :rows="10" placeholder="一行一个字符串或正则；命中后跳过创建链路，仅匹配文件名。" /></el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editLinkForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="editLinkForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editLinkDialogVisible = false">取消</el-button><el-button type="primary" :loading="editing" @click="submitUpdateLinkRule">保存</el-button></template>
    </el-dialog>

    <DirectoryPickerDialog v-model="directoryPickerVisible" title="选择目录" :initial-path="directoryPickerInitialPath" @selected="applyDirectorySelection" />
  </div>
</template>

<script setup lang="ts">
import { Delete, Edit, Operation } from '@element-plus/icons-vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import { prepareRuleExecution } from '../api/executions'
import { createRule, deleteRule, fetchRule, fetchRules, updateRule, type RuleItem, type UpdateRulePayload } from '../api/rules'
import {
  clearRunHistory,
  deleteRunHistoryItem,
  emptyRunHistory,
  fetchRunHistory,
  type RunHistoryItem,
  type RunHistorySummary,
} from '../api/runHistory'

type ArchiveMode = 'package' | 'collect'
type CompatibilityMode = 'local' | 'compatibility'
type PackageOptionKey = 'flat_archive' | 'include_manifest' | 'verify_after_archive' | 'cleanup_source_after_archive' | 'package_nested_folders' | 'match_archive'
type CollectOptionKey = 'recursive_collect' | 'deduplicate_same_name' | 'keep_latest_only' | 'collect_related_files' | 'cleanup_source_after_archive'
type CleanupOptionKey = 'cleanup_empty_dirs' | 'cleanup_matching_files'
type TransformOptionKey = 'convert_traditional_to_simplified' | 'convert_matching_text'
type PurifyArchiveMode = 'cleanup' | 'transform'
type HistoryStatus = 'success' | 'skip' | 'failed'
type DirectoryPickerTarget = 'create.source_dir' | 'create.target_dir' | 'edit.source_dir' | 'edit.target_dir' | 'createPurify.source_dir' | 'editPurify.source_dir' | 'createLink.source_dir' | 'createLink.target_dir' | 'editLink.source_dir' | 'editLink.target_dir' | null
type TabKey = 'rules' | 'purify' | 'link' | 'history'
type RuleListType = 'archive' | 'cleanup' | 'link'
type PurifyOptions = Record<CleanupOptionKey | TransformOptionKey, boolean>

const defaultCronExpression = '0 8 * * *'
const pageSizeOptions = [25, 50]

const packageModeOptions = [
  { key: 'flat_archive', label: '扁平归档', description: '勾选后直接输出到目标路径；取消勾选时默认额外套一层源文件夹后再生成 CBZ。' },
  { key: 'include_manifest', label: '归档清单', description: '为每次打包写入清单，方便后续核对归档内容。' },
  { key: 'verify_after_archive', label: '归档校验', description: '完成后再次检查结果，减少丢包或缺失问题。' },
  { key: 'cleanup_source_after_archive', label: '清理源文件', description: '确认已归档后再清理原目录中的已处理文件。' },
  { key: 'package_nested_folders', label: '嵌套打包', description: '遇到多层子文件夹时，按原有层级在归档目录内生成对应 CBZ；关闭时默认跳过并记录。' },
  { key: 'match_archive', label: '匹配归档', description: '按规则匹配文件名或扩展名，命中后直接转移到目标路径，不参与打包。' },
] as const

const collectModeOptions = [
  { key: 'recursive_collect', label: '递归收集', description: '自动扫描所有子目录，把符合条件的文件一并收集。' },
  { key: 'deduplicate_same_name', label: '同名文件去重', description: '遇到重复文件时自动跳过重复项，减少覆盖冲突。' },
  { key: 'keep_latest_only', label: '仅保留最新文件', description: '存在多个版本时优先保留最新文件。' },
  { key: 'collect_related_files', label: '收集关联文件', description: '在收集主文件时同步带上相关说明或附属文件。' },
  { key: 'cleanup_source_after_archive', label: '成功后清理源文件夹', description: '收集成功后清理已处理目录中的空文件夹，保持源目录整洁。' },
] as const

const cleanupModeOptions = [
  { key: 'cleanup_empty_dirs', label: '清理空文件夹', description: '递归删除监控目录中的空文件夹。' },
  { key: 'cleanup_matching_files', label: '清理匹配文件', description: '按文件名匹配规则删除命中的文件，支持字符串和正则。' },
] as const

const transformModeOptions = [
  { key: 'convert_traditional_to_simplified', label: '繁简转换', description: '将监控目录下文件和文件夹名称中的中文繁体字转换为简体字，其他字符保持不变。' },
  { key: 'convert_matching_text', label: '匹配转换', description: '按自定义规则重命名文件和文件夹名称，支持关键词部分匹配与正则转换。' },
] as const

function createDefaultPackageOptions(): Record<PackageOptionKey, boolean> {
  return { flat_archive: false, include_manifest: true, verify_after_archive: true, cleanup_source_after_archive: false, package_nested_folders: false, match_archive: false }
}

function createDefaultCollectOptions(): Record<CollectOptionKey, boolean> {
  return { recursive_collect: true, deduplicate_same_name: true, keep_latest_only: false, collect_related_files: true, cleanup_source_after_archive: false }
}

function createDefaultCleanupOptions(): Record<CleanupOptionKey, boolean> {
  return { cleanup_empty_dirs: true, cleanup_matching_files: false }
}

function createDefaultTransformOptions(): Record<TransformOptionKey, boolean> {
  return { convert_traditional_to_simplified: true, convert_matching_text: false }
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

function parseFiltersText(value: string) {
  return value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
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
  return mode === 'package' ? '打包归档功能' : '收集归档功能'
}

function getPurifyModeTitle(mode: PurifyArchiveMode) {
  return mode === 'transform' ? '转换模式功能' : '清理模式功能'
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

function historyStatusText(status: HistoryStatus) {
  if (status === 'success') return '成功'
  if (status === 'skip') return '跳过'
  return '失败'
}

function normalizeRuleType(rule: RuleItem): RuleListType {
  if (rule.rule_type === 'cleanup' || rule.archive_mode === 'cleanup') return 'cleanup'
  if (rule.rule_type === 'link' || rule.archive_mode === 'link') return 'link'
  return 'archive'
}

function buildRuleUpdatePayload(rule: RuleItem, enabled: boolean): UpdateRulePayload {
  const ruleType = normalizeRuleType(rule)
  const scheduleEnabled = rule.run_mode === 'cron' || Boolean(rule.cron_expression)

  return {
    name: rule.name,
    description: rule.description ?? '',
    enabled,
    monitor_enabled: rule.monitor_enabled,
    compatibility_mode: ruleType === 'link' ? 'local' : (rule.compatibility_mode || 'local'),
    archive_mode: rule.archive_mode,
    rule_type: ruleType,
    link_mode: ruleType === 'link' ? (rule.link_mode === 'hard' ? 'hard' : 'soft') : undefined,
    run_mode: resolveRunMode(rule.monitor_enabled, scheduleEnabled),
    source_dir: rule.source_dir,
    target_dir: ruleType === 'cleanup' ? '' : rule.target_dir,
    watch_debounce_ms: rule.watch_debounce_ms,
    cron_expression: scheduleEnabled ? rule.cron_expression : '',
    run_on_start: rule.run_on_start,
    options: ruleType === 'cleanup' ? parseOptionJSON(rule.options_json, createDefaultPurifyOptions()) : {},
    package_options: ruleType === 'archive' && rule.archive_mode === 'package'
      ? parseOptionJSON(rule.package_options_json, createDefaultPackageOptions())
      : {},
    collect_options: ruleType === 'archive' && rule.archive_mode === 'collect'
      ? parseOptionJSON(rule.collect_options_json, createDefaultCollectOptions())
      : {},
    filters: parseFiltersText(parseFiltersJSON(rule.filters_json)),
    match_filters: parseFiltersText(parseFiltersJSON(rule.match_filters_json)),
  }
}

const activeTab = ref<TabKey>('rules')
const loading = ref(false)
const archiveLoading = ref(false)
const purifyLoading = ref(false)
const linkLoading = ref(false)
const historyLoading = ref(false)
const creating = ref(false)
const editing = ref(false)
const errorMessage = ref('')
const togglingRuleIds = ref(new Set<number>())

const createDialogVisible = ref(false)
const editDialogVisible = ref(false)
const createPurifyDialogVisible = ref(false)
const editPurifyDialogVisible = ref(false)
const createLinkDialogVisible = ref(false)
const editLinkDialogVisible = ref(false)
const createArchiveOptionsExpanded = ref(false)
const editArchiveOptionsExpanded = ref(false)
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

const archiveRules = ref<RuleItem[]>([])
const purifyRules = ref<RuleItem[]>([])
const linkRules = ref<RuleItem[]>([])
const historyItems = ref<RunHistoryItem[]>(emptyRunHistory())
const historySummary = ref<RunHistorySummary>(createDefaultHistorySummary())

const successCount = computed(() => historySummary.value.success)
const skipCount = computed(() => historySummary.value.skipped)
const failedCount = computed(() => historySummary.value.failed)

const purifyRulesTotal = ref(0)

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
})

const createPurifyForm = reactive({
  name: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  archive_mode: 'cleanup' as PurifyArchiveMode,
  compatibility_mode: 'local' as CompatibilityMode,
  source_dir: '',
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  options: createDefaultPurifyOptions(),
  filters_text: '',
  transform_rules_text: '',
})

const editPurifyForm = reactive({
  name: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  archive_mode: 'cleanup' as PurifyArchiveMode,
  compatibility_mode: 'local' as CompatibilityMode,
  source_dir: '',
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  options: createDefaultPurifyOptions(),
  filters_text: '',
  transform_rules_text: '',
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
  link_mode: 'soft' as 'soft' | 'hard',
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
  link_mode: 'soft' as 'soft' | 'hard',
  filters_text: '',
})

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
}

function resetCreatePurifyForm() {
  createPurifyForm.name = ''
  createPurifyForm.enabled = true
  createPurifyForm.monitor_enabled = true
  createPurifyForm.schedule_enabled = false
  createPurifyForm.archive_mode = 'cleanup'
  createPurifyForm.compatibility_mode = 'local'
  createPurifyForm.source_dir = ''
  createPurifyForm.cron_expression = ''
  createPurifyForm.watch_debounce_ms = 2000
  createPurifyForm.run_on_start = true
  createPurifyForm.options = createDefaultPurifyOptions()
  createPurifyForm.filters_text = ''
  createPurifyForm.transform_rules_text = ''
}

function resetEditPurifyForm() {
  editPurifyForm.name = ''
  editPurifyForm.enabled = true
  editPurifyForm.monitor_enabled = true
  editPurifyForm.schedule_enabled = false
  editPurifyForm.archive_mode = 'cleanup'
  editPurifyForm.compatibility_mode = 'local'
  editPurifyForm.source_dir = ''
  editPurifyForm.cron_expression = ''
  editPurifyForm.watch_debounce_ms = 2000
  editPurifyForm.run_on_start = true
  editPurifyForm.options = createDefaultPurifyOptions()
  editPurifyForm.filters_text = ''
  editPurifyForm.transform_rules_text = ''
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
    editLinkForm.link_mode = rule.link_mode === 'hard' ? 'hard' : 'soft'
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
      return createPurifyForm.source_dir
    case 'editPurify.source_dir':
      return editPurifyForm.source_dir
    case 'createLink.source_dir':
      return createLinkForm.source_dir
    case 'createLink.target_dir':
      return createLinkForm.target_dir
    case 'editLink.source_dir':
      return editLinkForm.source_dir
    case 'editLink.target_dir':
      return editLinkForm.target_dir
  }
}

function openDirectoryPicker(form: 'create' | 'edit' | 'createPurify' | 'editPurify' | 'createLink' | 'editLink', field: 'source_dir' | 'target_dir') {
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
      createPurifyForm.source_dir = path
      break
    case 'editPurify.source_dir':
      editPurifyForm.source_dir = path
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
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '链路规则加载失败'
  } finally {
    linkLoading.value = false
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

async function switchTab(tab: TabKey) {
  activeTab.value = tab

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
      package_options: { ...createForm.package_options },
      collect_options: { ...createForm.collect_options },
      filters: parseFiltersText(createForm.filters_text),
      match_filters: createForm.archive_mode === 'package' && createForm.package_options.match_archive ? parseFiltersText(createForm.match_filters_text) : [],
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
    editForm.package_options = parseOptionJSON(rule.package_options_json, createDefaultPackageOptions())
    editForm.collect_options = parseOptionJSON(rule.collect_options_json, createDefaultCollectOptions())
    editForm.filters_text = parseFiltersJSON(rule.filters_json)
    editForm.match_filters_text = parseFiltersJSON(rule.match_filters_json)
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
      package_options: { ...editForm.package_options },
      collect_options: { ...editForm.collect_options },
      filters: parseFiltersText(editForm.filters_text),
      match_filters: editForm.archive_mode === 'package' && editForm.package_options.match_archive ? parseFiltersText(editForm.match_filters_text) : [],
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
    editPurifyForm.source_dir = rule.source_dir
    editPurifyForm.cron_expression = rule.cron_expression
    editPurifyForm.watch_debounce_ms = rule.watch_debounce_ms
    editPurifyForm.run_on_start = rule.run_on_start
    editPurifyForm.archive_mode = rule.archive_mode === 'transform' ? 'transform' : 'cleanup'
    editPurifyForm.options = parseOptionJSON(rule.options_json, createDefaultPurifyOptions())
    editPurifyForm.filters_text = parseFiltersJSON(rule.filters_json)
    editPurifyForm.transform_rules_text = parseFiltersJSON(rule.transform_rules_json)
    editPurifyDialogVisible.value = true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '净化规则详情加载失败'
  } finally {
    editing.value = false
  }
}

async function submitCreatePurifyRule() {
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
      source_dir: createPurifyForm.source_dir,
      target_dir: '',
      watch_debounce_ms: createPurifyForm.watch_debounce_ms,
      cron_expression: createPurifyForm.schedule_enabled ? createPurifyForm.cron_expression : '',
      run_on_start: createPurifyForm.run_on_start,
      options: { ...createPurifyForm.options },
      filters: createPurifyForm.archive_mode === 'cleanup' ? parseFiltersText(createPurifyForm.filters_text) : [],
      transform_rules: createPurifyForm.archive_mode === 'transform' ? parseFiltersText(createPurifyForm.transform_rules_text) : [],
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
      source_dir: editPurifyForm.source_dir,
      target_dir: '',
      watch_debounce_ms: editPurifyForm.watch_debounce_ms,
      cron_expression: editPurifyForm.schedule_enabled ? editPurifyForm.cron_expression : '',
      run_on_start: editPurifyForm.run_on_start,
      options: { ...editPurifyForm.options },
      filters: editPurifyForm.archive_mode === 'cleanup' ? parseFiltersText(editPurifyForm.filters_text) : [],
      transform_rules: editPurifyForm.archive_mode === 'transform' ? parseFiltersText(editPurifyForm.transform_rules_text) : [],
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
      options: {},
      package_options: {},
      collect_options: {},
      filters: parseFiltersText(createLinkForm.filters_text),
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

async function submitUpdateLinkRule() {
  if (!editingLinkRuleID.value) {
    errorMessage.value = '缺少链路规则 ID'
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
      options: {},
      package_options: {},
      collect_options: {},
      filters: parseFiltersText(editLinkForm.filters_text),
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

async function prepareExecution(ruleID: number) {
  loading.value = true
  errorMessage.value = ''
  try {
    await prepareRuleExecution(ruleID, 'once')
    ElMessage.success('规则执行已启动')
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

async function refreshRuleList(type: RuleListType) {
  if (type === 'archive') {
    await loadArchiveRules()
    return
  }

  if (type === 'cleanup') {
    await loadPurifyRules()
    return
  }

  await loadLinkRules()
}

async function toggleRuleEnabled(rule: RuleItem, enabled: boolean | string | number) {
  const nextEnabled = Boolean(enabled)
  const ruleType = normalizeRuleType(rule)
  const loadingSet = new Set(togglingRuleIds.value)

  loadingSet.add(rule.id)
  togglingRuleIds.value = loadingSet
  errorMessage.value = ''

  try {
    await updateRule(rule.id, buildRuleUpdatePayload(rule, nextEnabled))
    rule.enabled = nextEnabled
    ElMessage.success(`规则已${nextEnabled ? '启用' : '停用'}`)
    await refreshRuleList(ruleType)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : `规则${nextEnabled ? '启用' : '停用'}失败`)
    await refreshRuleList(ruleType)
  } finally {
    const nextLoadingSet = new Set(togglingRuleIds.value)
    nextLoadingSet.delete(rule.id)
    togglingRuleIds.value = nextLoadingSet
  }
}

async function toggleRuleMonitorEnabled(rule: RuleItem, monitorEnabled: boolean | string | number) {
  const nextMonitorEnabled = Boolean(monitorEnabled)
  const ruleType = normalizeRuleType(rule)
  const loadingSet = new Set(togglingRuleIds.value)

  loadingSet.add(rule.id)
  togglingRuleIds.value = loadingSet
  errorMessage.value = ''

  try {
    await updateRule(rule.id, {
      name: rule.name,
      description: rule.description ?? '',
      enabled: rule.enabled,
      monitor_enabled: nextMonitorEnabled,
      compatibility_mode: ruleType === 'link' ? 'local' : (rule.compatibility_mode || 'local'),
      archive_mode: rule.archive_mode,
      rule_type: ruleType,
      link_mode: ruleType === 'link' ? (rule.link_mode === 'hard' ? 'hard' : 'soft') : undefined,
      run_mode: resolveRunMode(nextMonitorEnabled, rule.run_mode === 'cron' || Boolean(rule.cron_expression)),
      source_dir: rule.source_dir,
      target_dir: ruleType === 'cleanup' ? '' : rule.target_dir,
      watch_debounce_ms: rule.watch_debounce_ms,
      cron_expression: rule.run_mode === 'cron' || Boolean(rule.cron_expression) ? rule.cron_expression : '',
      run_on_start: rule.run_on_start,
      options: ruleType === 'cleanup' ? parseOptionJSON(rule.options_json, createDefaultPurifyOptions()) : {},
      package_options: ruleType === 'archive' && rule.archive_mode === 'package'
        ? parseOptionJSON(rule.package_options_json, createDefaultPackageOptions())
        : {},
      collect_options: ruleType === 'archive' && rule.archive_mode === 'collect'
        ? parseOptionJSON(rule.collect_options_json, createDefaultCollectOptions())
        : {},
      filters: parseFiltersText(parseFiltersJSON(rule.filters_json)),
    })
    rule.monitor_enabled = nextMonitorEnabled
    ElMessage.success(`监控已${nextMonitorEnabled ? '开启' : '关闭'}`)
    await refreshRuleList(ruleType)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : `监控${nextMonitorEnabled ? '开启' : '关闭'}失败`)
    await refreshRuleList(ruleType)
  } finally {
    const nextLoadingSet = new Set(togglingRuleIds.value)
    nextLoadingSet.delete(rule.id)
    togglingRuleIds.value = nextLoadingSet
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
  void loadArchiveRules()
})
</script>

<style scoped>
.rules-page { display: flex; flex-direction: column; gap: 16px; }
.rules-tabs { display: flex; gap: 32px; padding: 0 4px; border-bottom: 1px solid var(--el-border-color-lighter); }
.rules-tabs__item { position: relative; padding: 12px 0; font-size: 15px; background: transparent; border: 0; cursor: pointer; color: var(--el-text-color-regular); transition: color 0.2s ease; }
.rules-tabs__item:hover { color: var(--el-color-primary); }
.rules-tabs__item.is-active { color: var(--el-color-primary); font-weight: 600; }
.rules-tabs__item.is-active::after { content: ''; position: absolute; left: 0; right: 0; bottom: -1px; height: 3px; background: var(--el-color-primary); border-radius: 999px; }
.rules-error { margin-bottom: 4px; }
.rules-card__header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.rules-card__title { font-size: 18px; font-weight: 700; color: var(--el-text-color-primary); }
.history-actions { display: flex; gap: 8px; }
.history-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.history-summary { display: flex; flex-wrap: wrap; gap: 12px; color: var(--el-text-color-secondary); }
.history-search { display: flex; align-items: center; gap: 8px; }
.history-search :deep(.el-input) { width: 260px; }
.history-pagination { display: flex; justify-content: flex-end; margin-top: 16px; }
.history-rule { display: flex; flex-direction: column; gap: 6px; }
.history-rule__title { font-weight: 600; color: var(--el-text-color-primary); }
.history-rule__desc { line-height: 1.6; color: var(--el-text-color-secondary); }
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
  box-shadow: -10px 0 18px rgba(15, 23, 42, 0.05);
}

.rule-actions { display: inline-flex; align-items: center; justify-content: center; gap: 12px; white-space: nowrap; }
.rule-action { flex: 0 0 auto; padding: 4px; font-size: 18px; }
.rule-action--primary { color: var(--el-color-primary); }
.rule-action--success { color: var(--el-color-success); }
.rule-action--danger { color: var(--el-color-danger); }
.rule-action:hover { transform: translateY(-1px); }
.rule-actions :deep(.el-button + .el-button) { margin-left: 0; }

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
.archive-mode-group {
  display: inline-flex;
  width: auto;
  max-width: 100%;
}

.archive-mode-group :deep(.el-radio-button) {
  width: auto;
}

.archive-mode-group :deep(.el-radio-button__inner) {
  min-width: 118px;
  padding-inline: 18px;
}
.mode-config-panel { margin-bottom: 18px; padding: 16px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-fill-color-extra-light); }
.mode-config-panel__header { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 12px; }
.mode-config-panel__title { font-size: 14px; font-weight: 600; color: var(--el-text-color-primary); }
.mode-config-panel__description { margin-top: 4px; font-size: 12px; line-height: 1.5; color: var(--el-text-color-secondary); }
.mode-config-toggle { display: flex; width: 100%; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 12px; padding: 14px 16px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-fill-color-extra-light); cursor: pointer; text-align: left; }
.mode-config-toggle__meta { display: flex; align-items: center; gap: 10px; }
.mode-config-toggle__icon { font-size: 18px; line-height: 1; color: var(--el-text-color-secondary); transition: transform 0.2s ease; }
.mode-config-toggle__icon.is-expanded { transform: rotate(180deg); }
.mode-option-card { display: flex; flex-direction: column; gap: 6px; min-height: 92px; padding: 14px 16px; margin-bottom: 12px; border: 1px solid var(--el-border-color); border-radius: 10px; background: var(--el-bg-color); cursor: pointer; }
.mode-option-card--compact { min-height: auto; padding: 12px 14px; }
.mode-option-card:hover { border-color: var(--el-color-primary-light-5); }
.mode-option-card__description { padding-left: 24px; font-size: 12px; line-height: 1.5; color: var(--el-text-color-secondary); }
.purify-tags { display: flex; flex-wrap: wrap; gap: 8px; }

@media (max-width: 900px) {
  .history-toolbar { flex-direction: column; align-items: stretch; }
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
