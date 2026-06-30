<template>
  <div class="naming-workshop-view">
    <el-card class="workshop-card" shadow="never">
      <div class="workshop-shell">
        <section class="workshop-pane workshop-pane--source">
          <header class="pane-header">
            <div class="pane-title-row">
              <div class="pane-title">待命名</div>
              <div class="pane-count">{{ sortedSourceItems.length }} 项</div>
            </div>
            <div class="pane-header__tools">
              <div class="source-toolbar">
                <el-tooltip content="添加文件" placement="top" :show-after="500">
                  <el-button class="icon-action icon-action--file" circle aria-label="添加文件" @click="openFilePicker">
                    <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                      <path d="M7.2 3.8h6.65L18.8 8.7v9.65a2.05 2.05 0 0 1-2.05 2.05H7.2a2.05 2.05 0 0 1-2.05-2.05V5.85A2.05 2.05 0 0 1 7.2 3.8Z" />
                      <path d="M13.65 3.95v4.9h4.9" />
                      <path d="M12 11.25v5.5" />
                      <path d="M9.25 14h5.5" />
                    </svg>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="添加文件夹" placement="top" :show-after="500">
                  <el-button class="icon-action icon-action--folder" circle aria-label="添加文件夹" @click="openFolderPicker">
                    <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                      <path d="M3.9 7.1a2 2 0 0 1 2-2h4.05l1.65 2h6.5a2 2 0 0 1 2 2v1.05" />
                      <path d="M4.25 10.35h15.5a1.75 1.75 0 0 1 1.72 2.1l-.9 4.48a2.55 2.55 0 0 1-2.5 2.05H5.95a2.55 2.55 0 0 1-2.5-2.08l-.88-4.52a1.75 1.75 0 0 1 1.68-2.03Z" />
                      <path d="M12 12.7v4.2" />
                      <path d="M9.9 14.8h4.2" />
                    </svg>
                  </el-button>
                </el-tooltip>
                <span class="toolbar-divider"></span>
                <el-tooltip content="排序字段" placement="top" :show-after="500">
                  <el-select v-model="sortBy" class="sort-select" size="small" aria-label="排序字段">
                    <el-option label="名称" value="name" />
                    <el-option label="文件类型" value="type" />
                    <el-option label="修改时间" value="modifiedAt" />
                  </el-select>
                </el-tooltip>
                <el-tooltip :content="sortOrder === 'asc' ? '正序' : '倒序'" placement="top" :show-after="500">
                  <el-button class="icon-action icon-action--sort" circle :aria-label="sortOrder === 'asc' ? '正序' : '倒序'" @click="toggleSortOrder">
                    <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                      <path d="M7 5.2v13.6" />
                      <path v-if="sortOrder === 'asc'" d="M3.9 8.35 7 5.2l3.1 3.15" />
                      <path v-else d="m3.9 15.65 3.1 3.15 3.1-3.15" />
                      <path d="M13 7h7" />
                      <path d="M13 12h5.2" />
                      <path d="M13 17h3.4" />
                    </svg>
                  </el-button>
                </el-tooltip>
              </div>
            </div>
          </header>

          <div class="item-list">
            <article v-for="item in sortedSourceItems" :key="item.path" class="file-item">
              <span :class="['file-item__icon', `file-item__icon--${item.kind}`]">
                <svg v-if="item.kind === 'folder'" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M3.8 7.4a2 2 0 0 1 2-2h4.2l1.6 2h6.6a2 2 0 0 1 2 2v7.2a2 2 0 0 1-2 2H5.8a2 2 0 0 1-2-2V7.4Z" />
                </svg>
                <svg v-else viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M7.2 3.9h6.5l5.1 5.05v9.2a2 2 0 0 1-2 2H7.2a2 2 0 0 1-2-2V5.9a2 2 0 0 1 2-2Z" />
                  <path d="M13.55 4.05v5.05h5.05" />
                </svg>
              </span>
              <div class="file-item__meta">
                <div class="file-item__name">{{ item.name }}</div>
                <div class="file-item__path">{{ item.path }}</div>
              </div>
              <div class="file-item__extra">
                <span>{{ item.type }}</span>
                <span>{{ item.modifiedAt }}</span>
              </div>
              <el-tooltip content="移除" placement="top" :show-after="300">
                <el-button class="line-delete-button file-item__delete" text aria-label="移除待命名条目" @click="removeSourceItem(item.path)">
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M5.2 6.8h13.6" />
                    <path d="M9.2 6.8V5.2h5.6v1.6" />
                    <path d="m8.2 10 .55 8.2h6.5L15.8 10" />
                    <path d="M10.8 11.8v4.6" />
                    <path d="M13.2 11.8v4.6" />
                  </svg>
                </el-button>
              </el-tooltip>
            </article>
          </div>
        </section>

        <aside class="control-rail" aria-label="命名控制栏">
          <div class="rail-actions rail-actions--top">
            <el-tooltip content="筛选器" placement="right" :show-after="500">
              <el-button class="rail-button rail-button--filter" circle aria-label="筛选器" @click="filterDialogVisible = true">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                  <path d="M4.2 5.4h15.6" />
                  <path d="M6.8 10.2h10.4" />
                  <path d="M9.7 15h4.6" />
                  <path d="M11.1 15v3.4l1.8.9V15" />
                </svg>
              </el-button>
            </el-tooltip>
            <el-tooltip content="规则集" placement="right" :show-after="500">
              <el-button class="rail-button rail-button--set" circle aria-label="规则集" @click="openRuleSetDialog">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                  <path d="M5.2 5.2h13.6v4.1H5.2V5.2Z" />
                  <path d="M5.2 12.1h13.6v6.7H5.2v-6.7Z" />
                  <path d="M8 7.25h4.8" />
                  <path d="M8 14.7h5.7" />
                  <path d="M15.9 15.7h2.4" />
                </svg>
              </el-button>
            </el-tooltip>
            <el-tooltip content="添加规则" placement="right" :show-after="500">
              <el-button class="rail-button rail-button--rule" circle aria-label="添加规则" @click="ruleDialogVisible = true">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                  <path d="M6.2 4.8h8.9l2.7 2.7v11.7H6.2V4.8Z" />
                  <path d="M14.9 4.95v2.75h2.75" />
                  <path d="M8.9 10.5h6.2" />
                  <path d="M8.9 14.2h3.1" />
                  <path d="M15.8 13.05v5" />
                  <path d="M13.3 15.55h5" />
                </svg>
              </el-button>
            </el-tooltip>
          </div>

          <div class="rail-arrow" aria-hidden="true">
            <svg viewBox="0 0 24 24">
              <path d="M4 12h14.2" />
              <path d="m13.4 6.6 5.4 5.4-5.4 5.4" />
            </svg>
          </div>

          <div class="rail-actions rail-actions--bottom">
            <el-tooltip content="预览" placement="right" :show-after="500">
              <el-button class="rail-button rail-button--preview" circle aria-label="预览" @click="handlePreview">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                  <path d="M3.8 12s2.85-5.45 8.2-5.45S20.2 12 20.2 12s-2.85 5.45-8.2 5.45S3.8 12 3.8 12Z" />
                  <path d="M12 14.65a2.65 2.65 0 1 0 0-5.3 2.65 2.65 0 0 0 0 5.3Z" />
                </svg>
              </el-button>
            </el-tooltip>
            <el-tooltip content="重命名" placement="right" :show-after="500">
              <el-button class="rail-button rail-button--rename" circle aria-label="重命名" @click="confirmRename">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                  <path d="M4.3 17.9h5.4l9.2-9.2a2.1 2.1 0 0 0-3-3l-9.2 9.2-2.4 3Z" />
                  <path d="m14.45 7.15 2.4 2.4" />
                  <path d="M13.4 18.4h6.3" />
                </svg>
              </el-button>
            </el-tooltip>
          </div>
        </aside>

        <section class="workshop-pane workshop-pane--result">
          <header class="pane-header pane-header--result">
            <div class="pane-title-row">
              <div class="pane-title">命名结果</div>
            </div>
            <el-tooltip content="撤销" placement="top" :show-after="500">
              <el-button class="icon-action icon-action--undo" circle aria-label="撤销" @click="confirmUndo">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                  <path d="M8.2 8.05H5.1V4.95" />
                  <path d="M5.45 8.05A7.95 7.95 0 1 1 4.8 16" />
                  <path d="M5.1 8.05h7.1" />
                </svg>
              </el-button>
            </el-tooltip>
          </header>

          <div v-if="resultItems.length" class="result-list">
            <article v-for="item in resultItems" :key="item.path" class="result-item">
              <div class="result-item__before">{{ item.name }}</div>
              <svg viewBox="0 0 24 24" aria-hidden="true" class="result-item__arrow">
                <path d="M4.5 12h14" />
                <path d="m14 7.5 4.5 4.5-4.5 4.5" />
              </svg>
              <div class="result-item__after">{{ item.nextName }}</div>
              <span :class="['result-item__tag', `result-item__tag--${resultMode}`]">{{ resultModeLabel }}</span>
            </article>
          </div>
          <el-empty v-else description="点击中间的预览按钮后显示命名结果" />
        </section>
      </div>
    </el-card>

    <DirectoryPickerDialog v-model="folderPickerVisible" title="添加挂载目录文件夹" :initial-path="mountedPickerInitialPath" @selected="handleMountedFolderSelected" />

    <el-dialog v-model="filePickerVisible" title="添加挂载目录文件" width="860px" destroy-on-close>
      <div class="mounted-file-picker">
        <div class="mounted-file-picker__toolbar">
          <div class="mounted-file-picker__path">
            <span>当前目录</span>
            <el-tag type="info">{{ mountedCurrentPath || '未选择' }}</el-tag>
          </div>
          <div class="mounted-file-picker__actions">
            <el-button size="small" :disabled="!mountedParentPath || mountedFilePickerLoading" @click="openMountedPath(mountedParentPath)">上级目录</el-button>
            <el-button size="small" :loading="mountedFilePickerLoading" :disabled="!mountedCurrentPath" @click="reloadMountedPath">刷新</el-button>
          </div>
        </div>

        <el-alert v-if="mountedFilePickerError" type="error" :closable="false" :title="mountedFilePickerError" />

        <el-table
          v-loading="mountedFilePickerLoading"
          :data="mountedEntries"
          height="420"
          class="mounted-file-picker__table"
          @selection-change="handleMountedFileSelectionChange"
          @row-dblclick="handleMountedEntryDoubleClick"
        >
          <el-table-column type="selection" width="48" :selectable="isMountedFileSelectable" />
          <el-table-column label="名称" min-width="360">
            <template #default="scope">
              <div :class="['mounted-entry-name', { 'is-folder': scope.row.is_dir }]">
                <span :class="['mounted-entry-name__icon', scope.row.is_dir ? 'is-folder' : 'is-file']">
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path v-if="scope.row.is_dir" d="M3.8 7.4a2 2 0 0 1 2-2h4.2l1.6 2h6.6a2 2 0 0 1 2 2v7.2a2 2 0 0 1-2 2H5.8a2 2 0 0 1-2-2V7.4Z" />
                    <path v-else d="M7.2 3.9h6.5l5.1 5.05v9.2a2 2 0 0 1-2 2H7.2a2 2 0 0 1-2-2V5.9a2 2 0 0 1 2-2Z" />
                    <path v-if="!scope.row.is_dir" d="M13.55 4.05v5.05h5.05" />
                  </svg>
                </span>
                <span>{{ scope.row.name }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="120">
            <template #default="scope">{{ scope.row.is_dir ? '文件夹' : resolveFileType(scope.row.name, false) }}</template>
          </el-table-column>
          <el-table-column label="修改时间" width="190">
            <template #default="scope">{{ formatMountedTimestamp(scope.row.modified_at) }}</template>
          </el-table-column>
        </el-table>
      </div>
      <template #footer>
        <el-button @click="filePickerVisible = false">取消</el-button>
        <el-button type="primary" :disabled="mountedSelectedFiles.length === 0" @click="appendSelectedMountedFiles">添加所选文件</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="filterDialogVisible" title="筛选器" width="560px" destroy-on-close>
      <div class="folder-options">
        <div class="folder-options__title">添加文件夹的默认属性：</div>
        <label v-for="option in folderAttributeOptions" :key="option.key" class="folder-option">
          <span :class="['folder-option__icon', `folder-option__icon--${option.tone}`]">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path v-if="option.icon === 'file-plus'" d="M7 3.9h6.6L18 8.3v11.8H7V3.9Z" />
              <path v-if="option.icon === 'file-plus'" d="M13.45 4.05v4.4h4.4" />
              <path v-if="option.icon === 'file-plus'" d="M10.1 14.2h4.8" />
              <path v-if="option.icon === 'file-plus'" d="M12.5 11.8v4.8" />
              <path v-if="option.icon === 'folder-plus'" d="M3.8 7.2a2 2 0 0 1 2-2h4.15l1.65 2h6.6a2 2 0 0 1 2 2v8.6H3.8V7.2Z" />
              <path v-if="option.icon === 'folder-plus'" d="M10.2 14.2h4.8" />
              <path v-if="option.icon === 'folder-plus'" d="M12.6 11.8v4.8" />
              <path v-if="option.icon === 'folder-search'" d="M3.8 7.2a2 2 0 0 1 2-2h4.15l1.65 2h6.6a2 2 0 0 1 2 2v5.2" />
              <path v-if="option.icon === 'folder-search'" d="M3.8 9.3h16.4v8.5H3.8V9.3Z" />
              <path v-if="option.icon === 'folder-search'" d="M14.6 14.4a2.2 2.2 0 1 0 0 4.4 2.2 2.2 0 0 0 0-4.4Z" />
              <path v-if="option.icon === 'folder-search'" d="m16.25 18.25 2.1 2.1" />
              <path v-if="option.icon === 'hidden-file'" d="M7 3.9h6.6L18 8.3v11.8H7V3.9Z" />
              <path v-if="option.icon === 'hidden-file'" d="M13.45 4.05v4.4h4.4" />
              <path v-if="option.icon === 'hidden-file'" d="M9.5 13.9s1.05-2.1 3-2.1 3 2.1 3 2.1-1.05 2.1-3 2.1-3-2.1-3-2.1Z" />
              <path v-if="option.icon === 'hidden-file'" d="M12.5 14.65a.75.75 0 1 0 0-1.5.75.75 0 0 0 0 1.5Z" />
              <path v-if="option.icon === 'system-file'" d="M7 3.9h6.6L18 8.3v11.8H7V3.9Z" />
              <path v-if="option.icon === 'system-file'" d="M13.45 4.05v4.4h4.4" />
              <path v-if="option.icon === 'system-file'" d="M10 15.6c.55.55 1.45.9 2.5.9 1.35 0 2.35-.62 2.35-1.55 0-.85-.65-1.2-2.25-1.55-1.55-.34-2.2-.7-2.2-1.52 0-.9.92-1.48 2.18-1.48.92 0 1.68.27 2.2.72" />
              <path v-if="option.icon === 'folder-block'" d="M3.8 7.2a2 2 0 0 1 2-2h4.15l1.65 2h6.6a2 2 0 0 1 2 2v8.6H3.8V7.2Z" />
              <path v-if="option.icon === 'folder-block'" d="m8.9 13 6.2 6.2" />
              <path v-if="option.icon === 'folder-block'" d="M15.1 13 8.9 19.2" />
            </svg>
          </span>
          <el-checkbox v-model="filterDraft[option.key]" :disabled="option.disabled">{{ option.label }}</el-checkbox>
        </label>
      </div>
      <template #footer>
        <el-button @click="filterDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveFilterDraft">保存占位</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ruleDialogVisible" title="添加规则" width="860px" destroy-on-close class="naming-rule-dialog">
      <div class="rule-dialog-shell">
        <aside class="rule-dialog-nav" aria-label="规则类型">
          <button
            v-for="item in ruleCategories"
            :key="item.key"
            type="button"
            :class="['rule-nav-item', { 'is-active': activeRuleCategory === item.key }]"
            @click="activeRuleCategory = item.key"
          >
            {{ item.label }}
          </button>
        </aside>

        <section class="rule-config-panel">
          <header class="rule-config-panel__header">
            <div class="rule-config-panel__title">配置：{{ activeRuleCategoryLabel }}</div>
          </header>

          <div v-if="activeRuleCategory === 'insert'" class="insert-config">
            <el-form label-position="top">
              <div class="insert-config__box">
                <div class="insert-config__box-title">插入位置：</div>
                <div class="insert-position-line">
                  <span>从文件名{{ insertRuleDraft.fromRight ? '右侧' : '左侧' }}第</span>
                  <el-input-number v-model="insertRuleDraft.index" :min="1" :max="999" size="small" controls-position="right" />
                  <span>个计数点位置开始插入</span>
                </div>
                <div class="insert-config__checks">
                  <el-checkbox v-model="insertRuleDraft.fromRight">从右往左</el-checkbox>
                </div>
                <div class="insert-config__hint">默认不勾选时按从左到右计数，勾选后按从右到左计数确定插入点。</div>
              </div>

              <el-form-item label="自定义插入字段">
                <el-input v-model="insertRuleDraft.content" size="small" class="insert-text-input" placeholder="请输入要插入的字段" />
              </el-form-item>

              <el-form-item label="">
                <el-checkbox v-model="insertRuleDraft.ignoreExtension">忽略扩展名</el-checkbox>
              </el-form-item>
            </el-form>
          </div>

          <div v-else-if="activeRuleCategory === 'delete'" class="delete-config">
            <el-form label-position="top">
              <div class="delete-config__range">
                <div class="delete-config__box">
                  <div class="delete-config__box-title">始于：</div>
                  <el-radio-group v-model="deleteRuleDraft.startMode" class="delete-radio-group">
                    <el-radio value="position">
                      <span class="inline-radio-control">
                        位置：
                        <el-input-number v-model="deleteRuleDraft.startIndex" :min="1" :max="999" size="small" controls-position="right" />
                      </span>
                    </el-radio>
                    <el-radio value="separator">
                      <span class="inline-radio-control">
                        分隔符：
                        <el-input v-model="deleteRuleDraft.startSeparator" size="small" class="inline-text-input inline-text-input--short" />
                      </span>
                    </el-radio>
                  </el-radio-group>
                </div>

                <div class="delete-config__box">
                  <div class="delete-config__box-title">直到：</div>
                  <el-radio-group v-model="deleteRuleDraft.endMode" class="delete-radio-group">
                    <el-radio value="count">
                      <span class="inline-radio-control">
                        计数：
                        <el-input-number v-model="deleteRuleDraft.count" :min="1" :max="999" size="small" controls-position="right" />
                      </span>
                    </el-radio>
                    <el-radio value="separator">
                      <span class="inline-radio-control">
                        分隔符：
                        <el-input v-model="deleteRuleDraft.endSeparator" size="small" class="inline-text-input inline-text-input--short" />
                      </span>
                    </el-radio>
                    <el-radio value="tail">直到末尾</el-radio>
                  </el-radio-group>
                </div>
              </div>

              <div class="delete-config__checks">
                <el-checkbox v-model="deleteRuleDraft.deleteCurrentName">删除当前名称</el-checkbox>
                <el-checkbox v-model="deleteRuleDraft.fromRight">从右到左</el-checkbox>
                <el-checkbox v-model="deleteRuleDraft.ignoreExtension">忽略扩展名</el-checkbox>
                <el-checkbox v-model="deleteRuleDraft.preventRemoveSeparator">禁止移除分隔符</el-checkbox>
              </div>

              <el-form-item label="自定义删除字符（一行一个）">
                <el-input v-model="deleteRuleDraft.customCharacters" type="textarea" :rows="5" placeholder="例如：&#10;[&#10;]&#10;-" />
              </el-form-item>
            </el-form>
          </div>

          <div v-else class="rule-placeholder-panel">
            <svg viewBox="0 0 24 24" aria-hidden="true" class="rule-placeholder-panel__icon">
              <path d="M5.2 5.2h13.6v13.6H5.2V5.2Z" />
              <path d="M8.2 9.2h7.6" />
              <path d="M8.2 12h5.4" />
              <path d="M8.2 14.8h6.8" />
            </svg>
            <div>
              <div class="rule-placeholder-panel__title">{{ activeRuleCategoryLabel }}规则配置待接入</div>
              <div class="rule-placeholder-panel__desc">当前仅保留栏目入口，后续可在此处扩展具体配置项。</div>
            </div>
          </div>

          <section class="added-rules-panel">
            <header class="added-rules-panel__header">
              <div class="added-rules-panel__title-row">
                <div class="added-rules-panel__title">已添加规则</div>
                <span class="added-rules-panel__count">{{ addedRules.length }} 条</span>
              </div>
              <el-button class="create-rule-set-button" :disabled="addedRules.length === 0" @click="createRuleSetFromAddedRules">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="line-icon">
                  <path d="M5.2 6.1h8.2" />
                  <path d="M5.2 10.4h6.3" />
                  <path d="M5.2 14.7h5.1" />
                  <path d="M15.6 5.4v6.2" />
                  <path d="M12.5 8.5h6.2" />
                  <path d="M13.2 15.2h5.6" />
                  <path d="M13.2 18.5h3.8" />
                </svg>
                创建规则集
              </el-button>
            </header>

            <el-empty v-if="addedRules.length === 0" description="暂未添加规则" :image-size="72" />
            <div v-else class="added-rule-list">
              <article v-for="(rule, index) in addedRules" :key="rule.id" class="added-rule-item">
                <span class="added-rule-item__order">{{ index + 1 }}</span>
                <div class="added-rule-item__meta">
                  <div class="added-rule-item__title">{{ rule.label }}</div>
                  <div class="added-rule-item__desc">{{ rule.description }}</div>
                </div>
                <el-tooltip content="删除规则" placement="top" :show-after="300">
                  <el-button class="line-delete-button added-rule-item__delete" text aria-label="删除已添加规则" @click="removeAddedRule(rule.id)">
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <path d="M5.2 6.8h13.6" />
                      <path d="M9.2 6.8V5.2h5.6v1.6" />
                      <path d="m8.2 10 .55 8.2h6.5L15.8 10" />
                      <path d="M10.8 11.8v4.6" />
                      <path d="M13.2 11.8v4.6" />
                    </svg>
                  </el-button>
                </el-tooltip>
              </article>
            </div>
          </section>
        </section>
      </div>
      <template #footer>
        <el-button @click="ruleDialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="saveRuleDraft">添加规则</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ruleSetDialogVisible" title="规则集" width="820px" destroy-on-close>
      <div class="rule-set-dialog">
        <el-empty v-if="namingRuleSets.length === 0" description="暂未创建规则集，请先在添加规则窗口中创建" :image-size="88" />

        <div v-else class="rule-set-manager">
          <aside class="rule-set-manager__list" aria-label="已创建规则集">
            <button
              v-for="set in namingRuleSets"
              :key="set.id"
              type="button"
              :class="['rule-set-card', { 'is-active': selectedRuleSetID === set.id }]"
              @click="selectRuleSet(set.id)"
            >
              <span class="rule-set-card__name">{{ set.name }}</span>
              <span class="rule-set-card__count">{{ set.rules.length }} 条规则</span>
            </button>
          </aside>

          <section v-if="selectedRuleSet" class="rule-set-manager__detail">
            <div class="rule-set-editor__header">
              <el-input v-model="selectedRuleSet.name" placeholder="规则集名称" />
              <el-button type="danger" plain @click="deleteSelectedRuleSet">删除规则集</el-button>
            </div>
            <div class="rule-set-dialog__hint">规则从上到下依次执行，可通过上移/下移调整当前规则集内的栏目顺序。</div>
            <div class="rule-set-list">
              <article v-for="(item, index) in selectedRuleSet.rules" :key="item.id" class="rule-set-item">
                <span class="rule-set-item__drag" aria-hidden="true">
                  <svg viewBox="0 0 24 24">
                    <path d="M8 7h.01" />
                    <path d="M8 12h.01" />
                    <path d="M8 17h.01" />
                    <path d="M14 7h.01" />
                    <path d="M14 12h.01" />
                    <path d="M14 17h.01" />
                  </svg>
                </span>
                <span class="rule-set-item__order">{{ index + 1 }}</span>
                <div class="rule-set-item__meta">
                  <div class="rule-set-item__title">{{ item.label }}</div>
                  <div class="rule-set-item__desc">{{ item.description }}</div>
                </div>
                <div class="rule-set-item__actions">
                  <el-button text :disabled="index === 0" @click="moveRuleSetItem(index, -1)">上移</el-button>
                  <el-button text :disabled="index === selectedRuleSet.rules.length - 1" @click="moveRuleSetItem(index, 1)">下移</el-button>
                </div>
              </article>
            </div>
          </section>
        </div>
      </div>
      <template #footer>
        <el-button @click="ruleSetDialogVisible = false">关闭</el-button>
        <el-button type="primary" :disabled="!selectedRuleSet" @click="applySelectedRuleSet">应用选中规则集</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import { browseDirectories, fetchBrowseRoots, type DirectoryEntry } from '../api/paths'

type SourceItemKind = 'file' | 'folder'
type SortBy = 'name' | 'type' | 'modifiedAt'
type SortOrder = 'asc' | 'desc'
type ResultMode = 'preview' | 'renamed'
type RuleCategoryKey = 'insert' | 'delete' | 'replace' | 'rewrite' | 'extension' | 'remove' | 'case' | 'sequence' | 'randomize' | 'pad' | 'purge' | 'transliterate' | 'dateDedup' | 'regex'

interface SourceItem {
  name: string
  nextName: string
  path: string
  type: string
  modifiedAt: string
  kind: SourceItemKind
}

interface AddedNamingRule {
  id: number
  category: RuleCategoryKey
  label: string
  description: string
}

interface NamingRuleSet {
  id: number
  name: string
  rules: AddedNamingRule[]
}

interface MountedEntry extends DirectoryEntry {}

const sortBy = ref<SortBy>('name')
const sortOrder = ref<SortOrder>('asc')
const filterDialogVisible = ref(false)
const ruleDialogVisible = ref(false)
const ruleSetDialogVisible = ref(false)
const resultItems = ref<SourceItem[]>([])
const resultMode = ref<ResultMode>('preview')
const activeRuleCategory = ref<RuleCategoryKey>('insert')

const sourceItems = ref<SourceItem[]>([])
const filePickerVisible = ref(false)
const folderPickerVisible = ref(false)
const mountedCurrentPath = ref('')
const mountedParentPath = ref('')
const mountedEntries = ref<MountedEntry[]>([])
const mountedSelectedFiles = ref<MountedEntry[]>([])
const mountedFilePickerLoading = ref(false)
const mountedFilePickerError = ref('')

const filterDraft = reactive({
  includeFiles: true,
  includeFolderNames: false,
  includeSubdirectories: true,
  includeHiddenFiles: false,
  includeSystemFiles: false,
  ignoreRootWhenIncludeFolderNames: false,
})

const folderAttributeOptions: Array<{
  key: keyof typeof filterDraft
  label: string
  icon: 'file-plus' | 'folder-plus' | 'folder-search' | 'hidden-file' | 'system-file' | 'folder-block'
  tone: 'blue' | 'yellow' | 'cyan' | 'gray' | 'slate' | 'orange'
  disabled?: boolean
}> = [
  { key: 'includeFiles', label: '包含目录下所有文件', icon: 'file-plus', tone: 'blue' },
  { key: 'includeFolderNames', label: '包含文件夹名', icon: 'folder-plus', tone: 'yellow' },
  { key: 'includeSubdirectories', label: '包含子目录', icon: 'folder-search', tone: 'cyan' },
  { key: 'includeHiddenFiles', label: '包含隐藏文件', icon: 'hidden-file', tone: 'gray' },
  { key: 'includeSystemFiles', label: '包含系统文件', icon: 'system-file', tone: 'slate' },
  {
    key: 'ignoreRootWhenIncludeFolderNames',
    label: '包含文件夹名时忽略根目录',
    icon: 'folder-block',
    tone: 'orange',
    disabled: false,
  },
]

const ruleCategories: Array<{ key: RuleCategoryKey; label: string }> = [
  { key: 'insert', label: '插入' },
  { key: 'delete', label: '删除' },
  { key: 'replace', label: '替换' },
  { key: 'rewrite', label: '重写' },
  { key: 'extension', label: '扩展名' },
  { key: 'remove', label: '去除' },
  { key: 'case', label: '大小写' },
  { key: 'sequence', label: '序列化' },
  { key: 'randomize', label: '随机化' },
  { key: 'pad', label: '填充' },
  { key: 'purge', label: '去除' },
  { key: 'transliterate', label: '音译' },
  { key: 'dateDedup', label: '日期格式重转' },
  { key: 'regex', label: '正则' },
]

const ruleDraft = reactive({
  name: '',
})

const insertRuleDraft = reactive({
  content: '',
  index: 1,
  fromRight: false,
  ignoreExtension: true,
})

const deleteRuleDraft = reactive({
  startMode: 'position',
  startIndex: 1,
  startSeparator: '',
  endMode: 'count',
  count: 1,
  endSeparator: '',
  deleteCurrentName: false,
  fromRight: false,
  ignoreExtension: true,
  preventRemoveSeparator: false,
  customCharacters: '',
})

const namingRuleSets = ref<NamingRuleSet[]>([])
const selectedRuleSetID = ref<number | null>(null)
const nextRuleSetID = ref(1)

const addedRules = ref<AddedNamingRule[]>([])
const nextAddedRuleID = ref(1)

const sortedSourceItems = computed(() => {
  const items = [...sourceItems.value]
  const direction = sortOrder.value === 'asc' ? 1 : -1

  return items.sort((left, right) => String(left[sortBy.value]).localeCompare(String(right[sortBy.value]), 'zh-CN') * direction)
})

const resultModeLabel = computed(() => (resultMode.value === 'preview' ? '预览' : '已命名'))
const activeRuleCategoryLabel = computed(() => ruleCategories.find((item) => item.key === activeRuleCategory.value)?.label ?? '插入')
const mountedPickerInitialPath = computed(() => mountedCurrentPath.value || '')
const selectedRuleSet = computed(() => namingRuleSets.value.find((item) => item.id === selectedRuleSetID.value) ?? null)

function toggleSortOrder() {
  sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
}

async function openFilePicker() {
  filePickerVisible.value = true
  if (!mountedCurrentPath.value) {
    await openDefaultMountedPath()
    return
  }

  await openMountedPath(mountedCurrentPath.value)
}

function openFolderPicker() {
  folderPickerVisible.value = true
}

async function openDefaultMountedPath() {
  mountedFilePickerLoading.value = true
  mountedFilePickerError.value = ''

  try {
    const response = await fetchBrowseRoots()
    const rootPath = response.data?.items?.[0]?.path ?? ''
    if (!rootPath) {
      mountedEntries.value = []
      mountedFilePickerError.value = '未配置可选择的挂载目录'
      return
    }

    await openMountedPath(rootPath)
  } catch (error) {
    mountedFilePickerError.value = error instanceof Error ? error.message : '挂载目录加载失败'
  } finally {
    mountedFilePickerLoading.value = false
  }
}

async function openMountedPath(path: string) {
  const targetPath = path.trim()
  if (!targetPath) return

  mountedFilePickerLoading.value = true
  mountedFilePickerError.value = ''
  mountedSelectedFiles.value = []

  try {
    const response = await browseDirectories(targetPath)
    mountedCurrentPath.value = response.data?.current_path ?? targetPath
    mountedParentPath.value = response.data?.parent_path ?? ''
    mountedEntries.value = response.data?.entries ?? []
  } catch (error) {
    mountedFilePickerError.value = error instanceof Error ? error.message : '目录浏览失败'
  } finally {
    mountedFilePickerLoading.value = false
  }
}

async function reloadMountedPath() {
  if (!mountedCurrentPath.value) return
  await openMountedPath(mountedCurrentPath.value)
}

function handleMountedFileSelectionChange(selection: MountedEntry[]) {
  mountedSelectedFiles.value = selection.filter((item) => !item.is_dir)
}

function isMountedFileSelectable(row: MountedEntry) {
  return !row.is_dir
}

async function handleMountedEntryDoubleClick(row: MountedEntry) {
  if (!row.is_dir) return
  await openMountedPath(row.path)
}

function appendSelectedMountedFiles() {
  appendSourceItems(mountedSelectedFiles.value.map((entry) => createSourceItemFromMountedEntry(entry)))
  filePickerVisible.value = false
}

function handleMountedFolderSelected(path: string) {
  const name = getPathName(path)
  appendSourceItems([
    {
      name,
      nextName: name,
      path,
      type: '文件夹',
      modifiedAt: '—',
      kind: 'folder',
    },
  ])
  mountedCurrentPath.value = path
}

function appendSourceItems(nextItems: SourceItem[]) {
  if (!nextItems.length) {
    return
  }

  const existingPaths = new Set(sourceItems.value.map((item) => item.path))
  const uniqueItems = nextItems.filter((item) => !existingPaths.has(item.path))
  sourceItems.value = [...sourceItems.value, ...uniqueItems]
  resultItems.value = []
  ElMessage.success(`已添加 ${uniqueItems.length} 个待命名条目`)
}

function createSourceItemFromMountedEntry(entry: MountedEntry): SourceItem {
  return {
    name: entry.name,
    nextName: entry.name,
    path: entry.path,
    type: resolveFileType(entry.name, false),
    modifiedAt: formatMountedTimestamp(entry.modified_at),
    kind: entry.is_dir ? 'folder' : 'file',
  }
}

function resolveFileType(name: string, isFolderImport: boolean) {
  if (isFolderImport && !name.includes('.')) {
    return '文件夹'
  }

  const extension = name.split('.').pop()
  return extension && extension !== name ? extension.toUpperCase() : '文件'
}

function formatMountedTimestamp(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function getPathName(path: string) {
  const normalizedPath = path.replace(/\\/g, '/').replace(/\/+$/, '')
  return normalizedPath.split('/').pop() || path
}

function handlePreview() {
  resultMode.value = 'preview'
  resultItems.value = sortedSourceItems.value.map((item) => ({ ...item }))
  ElMessage.success('已生成命名预览')
}

async function confirmRename() {
  try {
    await ElMessageBox.confirm('确认执行本次重命名？当前仅完成页面流程占位，实际文件操作将在后续接入。', '确认重命名', {
      type: 'warning',
      confirmButtonText: '确认重命名',
      cancelButtonText: '取消',
    })

    resultMode.value = 'renamed'
    resultItems.value = sortedSourceItems.value.map((item) => ({ ...item }))
    ElMessage.success('已确认重命名占位流程，后续会记录为“命名”任务')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error instanceof Error ? error.message : '重命名确认失败')
    }
  }
}

async function confirmUndo() {
  try {
    await ElMessageBox.confirm('确认撤销最近一次命名？当前为页面占位流程，实际恢复文件将在后续接入。', '确认撤销', {
      type: 'warning',
      confirmButtonText: '确认撤销',
      cancelButtonText: '取消',
    })

    resultMode.value = 'preview'
    resultItems.value = []
    ElMessage.success('已撤销命名占位结果')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error instanceof Error ? error.message : '撤销确认失败')
    }
  }
}

function saveFilterDraft() {
  filterDialogVisible.value = false
  ElMessage.success('文件夹默认属性已保存')
}

function removeSourceItem(path: string) {
  sourceItems.value = sourceItems.value.filter((item) => item.path !== path)
  resultItems.value = resultItems.value.filter((item) => item.path !== path)
  ElMessage.success('已移除待命名条目')
}

function saveRuleDraft() {
  addedRules.value.push({
    id: nextAddedRuleID.value,
    category: activeRuleCategory.value,
    label: activeRuleCategoryLabel.value,
    description: buildActiveRuleDescription(),
  })
  nextAddedRuleID.value += 1
  ElMessage.success(`已添加第 ${addedRules.value.length} 条命名规则`)
}

function removeAddedRule(id: number) {
  addedRules.value = addedRules.value.filter((rule) => rule.id !== id)
  ElMessage.success('已删除添加的命名规则')
}

function buildActiveRuleDescription() {
  if (activeRuleCategory.value === 'insert') {
    return `从文件名${insertRuleDraft.fromRight ? '右侧' : '左侧'}第 ${insertRuleDraft.index} 个计数点位置插入“${insertRuleDraft.content || '未填写'}”，${insertRuleDraft.fromRight ? '从右到左' : '从左到右'}，${insertRuleDraft.ignoreExtension ? '忽略扩展名' : '包含扩展名'}`
  }

  if (activeRuleCategory.value === 'delete') {
    const startText = deleteRuleDraft.startMode === 'position' ? `从位置 ${deleteRuleDraft.startIndex} 开始` : `从分隔符“${deleteRuleDraft.startSeparator || '未填写'}”开始`
    const endText = deleteRuleDraft.endMode === 'count'
      ? `删除 ${deleteRuleDraft.count} 个计数单位`
      : deleteRuleDraft.endMode === 'separator'
        ? `直到分隔符“${deleteRuleDraft.endSeparator || '未填写'}”`
        : '直到末尾'
    const customText = deleteRuleDraft.customCharacters.trim() ? `，自定义删除字符 ${deleteRuleDraft.customCharacters.split('\n').filter(Boolean).length} 个` : ''
    return `${startText}，${endText}，${deleteRuleDraft.fromRight ? '从右到左' : '从左到右'}，${deleteRuleDraft.ignoreExtension ? '忽略扩展名' : '包含扩展名'}${customText}`
  }

  return `${activeRuleCategoryLabel.value}规则栏目已添加，具体配置待后续接入`
}

function cloneAddedRule(rule: AddedNamingRule): AddedNamingRule {
  return {
    id: rule.id,
    category: rule.category,
    label: rule.label,
    description: rule.description,
  }
}

function openRuleSetDialog() {
  if (namingRuleSets.value.length > 0 && !selectedRuleSetID.value) {
    selectedRuleSetID.value = namingRuleSets.value[0].id
  }
  ruleSetDialogVisible.value = true
}

function selectRuleSet(id: number) {
  selectedRuleSetID.value = id
}

async function createRuleSetFromAddedRules() {
  if (addedRules.value.length === 0) {
    ElMessage.warning('请先添加规则后再创建规则集')
    return
  }

  try {
    const { value } = await ElMessageBox.prompt('将按照当前已添加规则的先后顺序创建规则集，请输入规则集名称。', '创建规则集', {
      confirmButtonText: '确认创建',
      cancelButtonText: '取消',
      inputPlaceholder: '例如：漫画文件名整理',
      inputPattern: /\S+/,
      inputErrorMessage: '规则集名称不能为空',
      type: 'info',
    })

    const nextSet: NamingRuleSet = {
      id: nextRuleSetID.value,
      name: String(value).trim(),
      rules: addedRules.value.map(cloneAddedRule),
    }
    namingRuleSets.value = [...namingRuleSets.value, nextSet]
    selectedRuleSetID.value = nextSet.id
    nextRuleSetID.value += 1
    ElMessage.success(`规则集“${nextSet.name}”已创建`)
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error instanceof Error ? error.message : '创建规则集失败')
    }
  }
}

async function deleteSelectedRuleSet() {
  const currentSet = selectedRuleSet.value
  if (!currentSet) return

  try {
    await ElMessageBox.confirm(`确认删除规则集“${currentSet.name}”？`, '删除规则集', {
      type: 'warning',
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
    })

    namingRuleSets.value = namingRuleSets.value.filter((item) => item.id !== currentSet.id)
    selectedRuleSetID.value = namingRuleSets.value[0]?.id ?? null
    ElMessage.success('规则集已删除')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error instanceof Error ? error.message : '删除规则集失败')
    }
  }
}

function applySelectedRuleSet() {
  const currentSet = selectedRuleSet.value
  if (!currentSet) {
    ElMessage.warning('请先选择要应用的规则集')
    return
  }

  addedRules.value = currentSet.rules.map((rule) => ({
    ...cloneAddedRule(rule),
    id: nextAddedRuleID.value++,
  }))
  ruleSetDialogVisible.value = false
  ruleDialogVisible.value = true
  ElMessage.success(`已应用规则集“${currentSet.name}”`)
}

function moveRuleSetItem(index: number, direction: -1 | 1) {
  const currentSet = selectedRuleSet.value
  if (!currentSet) return

  const targetIndex = index + direction
  if (targetIndex < 0 || targetIndex >= currentSet.rules.length) {
    return
  }

  const items = [...currentSet.rules]
  const [item] = items.splice(index, 1)
  items.splice(targetIndex, 0, item)
  currentSet.rules = items
}
</script>

<style scoped lang="scss">
.naming-workshop-view {
  position: relative;
}

.workshop-card {
  border-radius: 22px;
  border-color: var(--border-color);
  background: var(--bg-elevated);
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.08);
}

.workshop-shell {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 72px minmax(0, 1fr);
  gap: 18px;
  min-height: 680px;
}

.workshop-pane,
.control-rail {
  border: 1px solid var(--border-color);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(248, 250, 252, 0.72));
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.06);
}

.workshop-pane {
  min-width: 0;
  padding: 18px;
  border-radius: 20px;
}

.pane-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  min-height: 32px;
  margin-bottom: 16px;
}

.pane-header__tools {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.pane-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.pane-title {
  color: var(--text-primary);
  font-size: 17px;
  font-weight: 800;
}

.pane-count {
  flex-shrink: 0;
  padding: 5px 10px;
  border-radius: 999px;
  color: #2f6fd6;
  background: rgba(64, 158, 255, 0.1);
  font-size: 12px;
  font-weight: 700;
}

.source-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: nowrap;
  margin-top: 0;
}

.mounted-file-picker {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.mounted-file-picker__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
}

.mounted-file-picker__path,
.mounted-file-picker__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.mounted-file-picker__path span {
  color: var(--text-secondary);
  font-size: 13px;
}

.mounted-file-picker__table {
  border: 1px solid var(--el-border-color-light);
  border-radius: 14px;
  overflow: hidden;
}

.mounted-entry-name {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.mounted-entry-name.is-folder {
  cursor: pointer;
}

.mounted-entry-name__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 10px;
  flex-shrink: 0;
}

.mounted-entry-name__icon svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.75;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.mounted-entry-name__icon.is-folder {
  color: #b7791f;
  background: rgba(245, 210, 123, 0.18);
}

.mounted-entry-name__icon.is-file {
  color: #2f6fd6;
  background: rgba(64, 158, 255, 0.1);
}

.icon-action,
.rail-button {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.82);
  border-color: var(--border-color);
  transition: color 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.icon-action:not(.is-disabled):hover,
.rail-button:not(.is-disabled):hover {
  transform: translateY(-1px);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.1);
}

.icon-action--file:not(.is-disabled):hover,
.icon-action--folder:not(.is-disabled):hover,
.icon-action--sort:not(.is-disabled):hover {
  color: #2f6fd6;
  border-color: #b7d8ff;
  background: #edf7ff;
}

.icon-action--undo:not(.is-disabled):hover {
  color: #e67e22;
  border-color: #ffd2a6;
  background: #fff7ed;
}

.line-icon {
  display: block;
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.toolbar-divider {
  width: 1px;
  height: 28px;
  background: var(--el-border-color-lighter);
}

.sort-select {
  width: 112px;
}

.item-list,
.result-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.file-item,
.result-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(226, 232, 240, 0.88);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.file-item:hover,
.result-item:hover {
  border-color: rgba(64, 158, 255, 0.32);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.07);
  transform: translateY(-1px);
}

.file-item__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 38px;
  width: 38px;
  height: 38px;
  border-radius: 14px;
}

.file-item__icon svg {
  width: 22px;
  height: 22px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.75;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.file-item__icon--file {
  color: #2f6fd6;
  background: rgba(64, 158, 255, 0.1);
}

.file-item__icon--folder {
  color: #b7791f;
  background: rgba(245, 210, 123, 0.18);
}

.file-item__meta {
  min-width: 0;
  flex: 1;
}

.file-item__name,
.result-item__before,
.result-item__after {
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 800;
}

.file-item__path {
  margin-top: 4px;
  color: var(--text-secondary);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-item__extra {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  color: var(--text-secondary);
  font-size: 11px;
  white-space: nowrap;
}

.line-delete-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  min-height: 30px;
  padding: 0;
  color: #94a3b8;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  transition: color 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, transform 0.2s ease;
}

.line-delete-button svg {
  width: 17px;
  height: 17px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.line-delete-button:not(.is-disabled):hover,
.line-delete-button:not(.is-disabled):focus-visible {
  color: #ef4444;
  border-color: rgba(239, 68, 68, 0.22);
  background: rgba(254, 242, 242, 0.9);
  transform: translateY(-1px);
}

.file-item__delete {
  flex-shrink: 0;
}

.control-rail {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 18px;
  padding: 18px 0;
  border-radius: 999px;
}

.rail-actions {
  display: flex;
  width: 100%;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.rail-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.rail-button {
  width: 54px;
  height: 54px;
  min-height: 54px;
  padding: 0;
  border-color: transparent;
  background: transparent;
  box-shadow: none;
}

.rail-button .line-icon {
  width: 24px;
  height: 24px;
  stroke-width: 2;
}

.rail-button:not(.is-disabled):hover,
.rail-button:not(.is-disabled):focus-visible,
.rail-button.is-active {
  border-color: rgba(148, 163, 184, 0.24);
  background: rgba(255, 255, 255, 0.72);
}

.rail-button--filter:not(.is-disabled):hover {
  color: #1f9d8b;
  border-color: #9be7d7;
  background: #effcf8;
}

.rail-button--set:not(.is-disabled):hover {
  color: #2f6fd6;
  border-color: #b7d8ff;
  background: #edf7ff;
}

.rail-button--rule:not(.is-disabled):hover {
  color: #7c52c8;
  border-color: #d9c2ff;
  background: #f5efff;
}

.rail-button--preview:not(.is-disabled):hover {
  color: #2f6fd6;
  border-color: #b7d8ff;
  background: #edf7ff;
}

.rail-button--rename:not(.is-disabled):hover {
  color: #e67e22;
  border-color: #ffd2a6;
  background: #fff7ed;
}

.rail-arrow {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 62px;
  height: 62px;
  color: #1e9bff;
  border-radius: 24px;
  background: linear-gradient(135deg, rgba(30, 155, 255, 0.16), rgba(30, 155, 255, 0.08));
  border: 1px solid transparent;
}

.rail-arrow svg {
  width: 34px;
  height: 34px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.result-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 32px minmax(0, 1fr) auto;
}

.result-item__before,
.result-item__after {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-item__before {
  color: var(--text-secondary);
}

.result-item__after {
  color: #1f7a62;
}

.result-item__arrow {
  width: 24px;
  height: 24px;
  fill: none;
  stroke: #94a3b8;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.result-item__tag {
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 800;
}

.result-item__tag--preview {
  color: #2f6fd6;
  background: rgba(64, 158, 255, 0.1);
}

.result-item__tag--renamed {
  color: #1f7a62;
  background: rgba(32, 201, 151, 0.12);
}

.rule-dialog-shell {
  display: grid;
  grid-template-columns: 170px minmax(0, 1fr);
  gap: 14px;
  min-height: 430px;
}

.rule-dialog-nav,
.rule-config-panel,
.rule-set-dialog {
  border: 1px solid var(--border-color);
  border-radius: 18px;
  background: rgba(248, 250, 252, 0.78);
}

.rule-dialog-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 430px;
  padding: 8px;
  overflow-y: auto;
}

.rule-nav-item {
  width: 100%;
  min-height: 32px;
  padding: 0 10px;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 700;
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 10px;
  cursor: pointer;
  transition: color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.rule-nav-item:hover,
.rule-nav-item.is-active {
  color: #1e9bff;
  background: rgba(30, 155, 255, 0.11);
}

.rule-nav-item.is-active {
  box-shadow: inset 3px 0 0 #1e9bff;
}

.rule-config-panel {
  min-width: 0;
  padding: 18px;
}

.rule-config-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 18px;
}

.rule-config-panel__title {
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 800;
}

.metadata-button,
.create-rule-set-button {
  font-weight: 700;
}

.metadata-button .line-icon,
.create-rule-set-button .line-icon {
  margin-right: 6px;
}

.insert-config,
.delete-config {
  padding: 20px;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.78);
}

.insert-config__box {
  padding: 14px;
  margin-bottom: 18px;
  border: 1px solid rgba(226, 232, 240, 0.95);
  border-radius: 14px;
  background: rgba(248, 250, 252, 0.7);
}

.insert-config__box-title {
  margin-bottom: 10px;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 800;
}

.insert-position-line {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-primary);
  font-size: 13px;
  flex-wrap: wrap;
}

.insert-config__checks {
  margin-top: 10px;
}

.insert-config__hint {
  margin-top: 8px;
  color: var(--text-secondary);
  font-size: 12px;
}

.insert-text-input {
  max-width: 280px;
}

.inline-radio-control {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.inline-text-input {
  width: 180px;
}

.inline-text-input--short {
  width: 96px;
}

.delete-config__range {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
  margin-bottom: 18px;
}

.delete-config__box {
  padding: 14px;
  border: 1px solid rgba(226, 232, 240, 0.95);
  border-radius: 14px;
  background: rgba(248, 250, 252, 0.7);
}

.delete-config__box-title {
  margin-bottom: 10px;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 800;
}

.delete-radio-group {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
}

.delete-config__checks {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 22px;
  margin-bottom: 18px;
}

.rule-placeholder-panel {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 180px;
  padding: 24px;
  border: 1px dashed rgba(148, 163, 184, 0.38);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.62);
}

.rule-placeholder-panel__icon {
  flex: 0 0 44px;
  width: 44px;
  height: 44px;
  color: #94a3b8;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.rule-placeholder-panel__title {
  color: var(--text-primary);
  font-weight: 800;
}

.rule-placeholder-panel__desc,
.rule-set-dialog__hint,
.rule-set-item__desc,
.added-rule-item__desc {
  color: var(--text-secondary);
  font-size: 12px;
}

.added-rules-panel {
  margin-top: 16px;
  padding: 14px;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 16px;
  background: rgba(248, 250, 252, 0.62);
}

.added-rules-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.added-rules-panel__title-row {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.added-rules-panel__title {
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 800;
}

.added-rules-panel__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 48px;
  height: 26px;
  padding: 0 10px;
  color: #2f6fd6;
  font-size: 12px;
  font-weight: 800;
  border-radius: 999px;
  background: rgba(64, 158, 255, 0.1);
}

.added-rule-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.added-rule-item {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: flex-start;
  padding: 10px;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.78);
}

.added-rule-item__order {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  color: #1e9bff;
  font-size: 12px;
  font-weight: 800;
  border-radius: 999px;
  background: rgba(30, 155, 255, 0.12);
}

.added-rule-item__title {
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 800;
}

.added-rule-item__delete {
  align-self: center;
}

.rule-set-dialog {
  padding: 16px;
}

.rule-set-manager {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 14px;
  min-height: 420px;
}

.rule-set-manager__list,
.rule-set-manager__detail {
  min-width: 0;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.68);
}

.rule-set-manager__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  overflow-y: auto;
}

.rule-set-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 5px;
  width: 100%;
  padding: 12px;
  color: var(--text-primary);
  text-align: left;
  border: 1px solid transparent;
  border-radius: 14px;
  background: transparent;
  cursor: pointer;
  transition: color 0.2s ease, border-color 0.2s ease, background-color 0.2s ease;
}

.rule-set-card:hover,
.rule-set-card.is-active {
  color: #2f6fd6;
  border-color: rgba(64, 158, 255, 0.26);
  background: rgba(64, 158, 255, 0.08);
}

.rule-set-card__name {
  font-size: 13px;
  font-weight: 800;
}

.rule-set-card__count {
  color: var(--text-secondary);
  font-size: 12px;
}

.rule-set-manager__detail {
  padding: 14px;
}

.rule-set-editor__header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  margin-bottom: 10px;
}

.rule-set-dialog__hint {
  margin-bottom: 12px;
}

.rule-set-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rule-set-item {
  display: grid;
  grid-template-columns: 28px 28px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.76);
}

.rule-set-item__drag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #94a3b8;
  cursor: grab;
}

.rule-set-item__drag svg {
  width: 20px;
  height: 20px;
  fill: none;
  stroke: currentColor;
  stroke-width: 4;
  stroke-linecap: round;
}

.rule-set-item__order {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  color: #2f6fd6;
  font-size: 12px;
  font-weight: 800;
  border-radius: 999px;
  background: rgba(64, 158, 255, 0.1);
}

.rule-set-item__meta {
  min-width: 0;
}

.rule-set-item__title {
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 800;
}

.rule-set-item__actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.folder-options {
  padding: 2px 0 4px;
}

.folder-options__title {
  margin-bottom: 12px;
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 800;
}

.folder-option {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 34px;
  padding: 4px 2px;
}

.folder-option__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 24px;
  width: 24px;
  height: 24px;
  border-radius: 8px;
}

.folder-option__icon svg {
  width: 20px;
  height: 20px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.65;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.folder-option__icon--blue {
  color: #2f6fd6;
  background: rgba(64, 158, 255, 0.1);
}

.folder-option__icon--yellow {
  color: #b7791f;
  background: rgba(245, 210, 123, 0.18);
}

.folder-option__icon--cyan {
  color: #1f9d8b;
  background: rgba(31, 157, 139, 0.1);
}

.folder-option__icon--gray {
  color: #6b7280;
  background: rgba(107, 114, 128, 0.1);
}

.folder-option__icon--slate {
  color: #475569;
  background: rgba(71, 85, 105, 0.1);
}

.folder-option__icon--orange {
  color: #e67e22;
  background: rgba(230, 126, 34, 0.12);
}

.folder-option :deep(.el-checkbox) {
  height: auto;
  margin-right: 0;
}

.folder-option :deep(.el-checkbox__label) {
  color: var(--text-primary);
  font-weight: 700;
}

:deep(.el-card__header) {
  border-bottom-color: var(--border-color);
}

:deep(.el-empty) {
  min-height: 360px;
}

@media (max-width: 1280px) {
  .workshop-shell,
  .rule-dialog-shell {
    grid-template-columns: 1fr;
  }

  .control-rail {
    flex-direction: row;
    border-radius: 20px;
    padding: 14px 18px;
  }

  .rail-actions {
    flex-direction: row;
  }
}
</style>
