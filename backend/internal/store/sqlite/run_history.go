package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"nestify/backend/internal/model"
)

// 运行日志（run_history）的全部读写。
//
// 注意：这张表**不在主库 app.db 里**，它在独立的日志库文件上（s.logDB），
// 文件位置由 NESTIFY_LOG_DB_PATH 决定 —— 建表 / 迁移见 run_history_store.go。
// 只有保留策略要读的 settings 还在主库（s.GetSettings 走 s.db），这是唯一跨库的地方。
func (s *Store) UpsertRunHistory(item model.RunHistoryItem) error {
	var ruleID any
	if item.RuleID != nil {
		ruleID = *item.RuleID
	}

	finishedAt := ""
	if item.FinishedAt != nil {
		finishedAt = item.FinishedAt.UTC().Format(time.RFC3339)
	}

	// detail_size = detail_json 的字节数，**在写入时算好存成一列**。
	// 折叠组要挑「明细最完整的那一份」当代表行，原来直接 `ORDER BY LENGTH(detail_json)`：
	// 那会让每次列表查询都把全部行的 detail_json 读出来（一次执行上万行、每行明细几十 KB
	// 就是几百 MB，仪表盘还每 5 秒拉一次全量）。有了这一列，排序只碰一个整数。
	_, err := s.logDB.Exec(`
		INSERT INTO run_history (
			id, rule_id, rule_name, trigger_mode, archive_mode, link_mode, status,
			processed_files, success_count, skip_count, failure_count, deleted_count, size_bytes,
			summary, detail_json, detail_size, started_at, updated_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			rule_id = excluded.rule_id,
			rule_name = excluded.rule_name,
			trigger_mode = excluded.trigger_mode,
			archive_mode = excluded.archive_mode,
			link_mode = excluded.link_mode,
			status = excluded.status,
			processed_files = excluded.processed_files,
			success_count = excluded.success_count,
			skip_count = excluded.skip_count,
			failure_count = excluded.failure_count,
			deleted_count = excluded.deleted_count,
			size_bytes = excluded.size_bytes,
			summary = excluded.summary,
			detail_json = excluded.detail_json,
			detail_size = excluded.detail_size,
			started_at = excluded.started_at,
			updated_at = excluded.updated_at,
			finished_at = excluded.finished_at
	`,
		item.ID,
		ruleID,
		item.RuleName,
		item.TriggerMode,
		item.ArchiveMode,
		item.LinkMode,
		item.Status,
		item.ProcessedFiles,
		item.SuccessCount,
		item.SkipCount,
		item.FailureCount,
		item.DeletedCount,
		item.SizeBytes,
		item.Summary,
		item.DetailJSON,
		len(item.DetailJSON),
		item.StartedAt.UTC().Format(time.RFC3339),
		item.UpdatedAt.UTC().Format(time.RFC3339),
		finishedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert run history: %w", err)
	}

	if err := s.maybeApplyRunHistoryRetention(); err != nil {
		return err
	}

	return nil
}

// runHistoryRetentionInterval 是运行日志保留策略的最小执行间隔。
//
// 为什么必须节流：run_history 是「每处理一项写一行」，一次大执行会插上万行；
// 而保留策略里那条 `DELETE ... WHERE id NOT IN (SELECT id ... ORDER BY started_at DESC LIMIT N)`
// 每次都要扫一遍全表、并把最多 N 个 id 物化成临时 B 树。逐行跑的话代价是
// O(项数 × 表行数)，单轮上万项就是上亿次行访问 + 上万次临时表构建 ——
// 实测这会把容器的内存与 CPU 一起顶到几个 G。节流后代价恒定在「每 30 秒一次」，
// 裁剪结果最多晚一个间隔生效，没有正确性影响。
const runHistoryRetentionInterval = 30 * time.Second

// maybeApplyRunHistoryRetention 按时间节流地执行保留策略（见上）。
func (s *Store) maybeApplyRunHistoryRetention() error {
	s.retentionMu.Lock()
	now := time.Now()
	if !s.retentionAt.IsZero() && now.Sub(s.retentionAt) < runHistoryRetentionInterval {
		s.retentionMu.Unlock()
		return nil
	}
	s.retentionAt = now
	s.retentionMu.Unlock()

	return s.applyRunHistoryRetentionPolicy()
}

func (s *Store) applyRunHistoryRetentionPolicy() error {
	settings, err := s.GetSettings()
	if err != nil {
		return fmt.Errorf("load settings for run history retention: %w", err)
	}
	if settings == nil {
		return nil
	}

	if settings.LogRetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -settings.LogRetentionDays).Format(time.RFC3339)
		if _, err := s.logDB.Exec(`DELETE FROM run_history WHERE started_at < ?`, cutoff); err != nil {
			return fmt.Errorf("delete expired run history: %w", err)
		}
	}

	if settings.LogRetentionMaxRecords > 0 {
		if _, err := s.logDB.Exec(`
			DELETE FROM run_history
			WHERE id NOT IN (
				SELECT id FROM run_history
				ORDER BY started_at DESC, id DESC
				LIMIT ?
			)
		`, settings.LogRetentionMaxRecords); err != nil {
			return fmt.Errorf("trim run history by max records: %w", err)
		}
	}

	return nil
}

// ErrRunHistoryNotFound 表示指定的运行日志记录不存在。
var ErrRunHistoryNotFound = errors.New("run history entry not found")

// GetRunHistoryByID 读取单条运行日志（含 detail_json 明细载荷）。
// 列表接口不带明细，详情弹窗按需调用它，避免日志列表响应体积失控。
func (s *Store) GetRunHistoryByID(id string) (model.RunHistoryItem, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return model.RunHistoryItem{}, fmt.Errorf("run history id is required")
	}

	row := s.logDB.QueryRow(`
		SELECT id, rule_id, rule_name, trigger_mode, archive_mode, link_mode, status,
		       processed_files, success_count, skip_count, failure_count, deleted_count, size_bytes,
		       summary, detail_json, started_at, updated_at, finished_at
		FROM run_history
		WHERE id = ?
	`, trimmed)

	item, err := scanRunHistory(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RunHistoryItem{}, ErrRunHistoryNotFound
		}
		return model.RunHistoryItem{}, err
	}

	return item, nil
}

// runHistoryListLimit 是「不分页」列表（GET /api/v1/run-history 不带分页参数）的硬上限。
//
// 这条路径是仪表盘「执行摘要」在用的：它只要最近几十条，但原来一次把**整张表**
// 读出来（还带着每行的 detail_json）。run_history 是「每处理一项写一行」，单表
// 上万行很正常，于是每次请求都要构造一个几 MB～几百 MB 的切片和 JSON 响应，
// 而仪表盘每 5 秒就拉一次 —— 容器内存就是这么顶上去的。
// 截到最近 runHistoryListLimit 条之后，这条路径的代价与库里积累了多少历史无关。
const runHistoryListLimit = 200

// runHistoryListColumns 是列表类查询的列清单。
//
// 刻意让 detail_json 列返回空字符串：列表从来不用明细（接口层 stripRunHistoryDetails
// 拿到就清空，详情弹窗按需走 /run-history/detail 单条拉取），但真去 SELECT detail_json
// 会把每行的明细全读进内存 —— 这是「打开网页内存暴涨」的主因之一。
const runHistoryListColumns = `id, rule_id, rule_name, trigger_mode, archive_mode, link_mode, status,
		       processed_files, success_count, skip_count, failure_count, deleted_count, size_bytes,
		       summary, '' AS detail_json, started_at, updated_at, finished_at`

func (s *Store) ListRunHistory() ([]model.RunHistoryItem, error) {
	rows, err := s.logDB.Query(`
		SELECT `+runHistoryListColumns+`
		FROM run_history
		ORDER BY started_at DESC, id DESC
		LIMIT ?
	`, runHistoryListLimit)
	if err != nil {
		return nil, fmt.Errorf("list run history: %w", err)
	}
	defer rows.Close()

	items := make([]model.RunHistoryItem, 0)
	for rows.Next() {
		item, scanErr := scanRunHistory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run history: %w", err)
	}

	return items, nil
}

// runHistoryListView 决定「一次执行会写出多行记录」的运行历史按什么粒度回给前端。
//
// 背景：run_history 是「每处理一项写一行」，所以**记录行数不等于执行次数**。
// 三个视图的差别只在「一次执行的那几行怎么回」，查询条件与排序口径都是同一套。
type runHistoryListView int

const (
	// runHistoryListViewFlat：原样回记录行 —— 一行就是某次执行里的一条记录。
	runHistoryListViewFlat runHistoryListView = iota
	// runHistoryListViewGrouped：按折叠组分组，组内**每一行**都回。运行日志页要用它数
	// 「本次操作了几个文件 / 共几条明细」，所以必须拿全。
	runHistoryListViewGrouped
	// runHistoryListViewTask：每个折叠组只回**代表行**（组内明细最完整的那条），一行一个任务。
	// 仪表盘「执行摘要」用的是它 —— 它要的是「最近执行了哪些任务」，一行一个任务就够；
	// 若像 grouped 那样把组内每一行都带上，一次上万项执行的若干个组就是上万行，
	// 而它是 5 秒一次的轮询。
	runHistoryListViewTask
)

func (s *Store) ListRunHistoryPage(page, pageSize int, keyword, status, archiveMode, ruleType, sortBy, sortOrder string) ([]model.RunHistoryItem, int, error) {
	return s.listRunHistoryPage(page, pageSize, keyword, status, archiveMode, ruleType, sortBy, sortOrder, runHistoryListViewFlat)
}

func (s *Store) ListRunHistoryGroupPage(page, pageSize int, keyword, status, archiveMode, ruleType, sortBy, sortOrder string) ([]model.RunHistoryItem, int, error) {
	return s.listRunHistoryPage(page, pageSize, keyword, status, archiveMode, ruleType, sortBy, sortOrder, runHistoryListViewGrouped)
}

// ListRunHistoryTaskPage 是「一行一个任务」的分页：每个折叠组只回代表行。
func (s *Store) ListRunHistoryTaskPage(page, pageSize int, keyword, status, archiveMode, ruleType, sortBy, sortOrder string) ([]model.RunHistoryItem, int, error) {
	return s.listRunHistoryPage(page, pageSize, keyword, status, archiveMode, ruleType, sortBy, sortOrder, runHistoryListViewTask)
}

func (s *Store) listRunHistoryPage(page, pageSize int, keyword, status, archiveMode, ruleType, sortBy, sortOrder string, view runHistoryListView) ([]model.RunHistoryItem, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	whereClause, args := buildRunHistoryWhereClause(keyword, status, archiveMode, ruleType)
	switch view {
	case runHistoryListViewGrouped:
		return s.listRunHistoryGroupedPage(page, pageSize, whereClause, args, sortBy, sortOrder)
	case runHistoryListViewTask:
		return s.listRunHistoryTaskPage(page, pageSize, whereClause, args, sortBy, sortOrder)
	}

	countQuery := `SELECT COUNT(*) FROM run_history` + whereClause
	var total int
	if err := s.logDB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count run history: %w", err)
	}

	orderClause := buildRunHistoryOrderClause(sortBy, sortOrder)
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.logDB.Query(`
		SELECT `+runHistoryListColumns+`
		FROM run_history`+whereClause+`
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list run history page: %w", err)
	}
	defer rows.Close()

	items := make([]model.RunHistoryItem, 0, pageSize)
	for rows.Next() {
		item, scanErr := scanRunHistory(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate run history page: %w", err)
	}

	return items, total, nil
}

func (s *Store) listRunHistoryGroupedPage(page, pageSize int, whereClause string, args []any, sortBy, sortOrder string) ([]model.RunHistoryItem, int, error) {
	groupExpr := buildRunHistoryGroupExpression()
	total, err := s.countRunHistoryGroups(whereClause, args)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []model.RunHistoryItem{}, 0, nil
	}

	orderClause := buildRunHistoryGroupOrderClause(sortBy, sortOrder)
	groupArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	groupRows, err := s.logDB.Query(`
		SELECT `+groupExpr+` AS group_key
		FROM run_history`+whereClause+`
		GROUP BY group_key
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, groupArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list run history groups: %w", err)
	}
	defer groupRows.Close()

	groupKeys := make([]string, 0, pageSize)
	for groupRows.Next() {
		var key string
		if err := groupRows.Scan(&key); err != nil {
			return nil, 0, fmt.Errorf("scan run history group: %w", err)
		}
		groupKeys = append(groupKeys, key)
	}
	if err := groupRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate run history groups: %w", err)
	}
	if len(groupKeys) == 0 {
		return []model.RunHistoryItem{}, total, nil
	}

	placeholders := make([]string, len(groupKeys))
	queryArgs := make([]any, 0, len(groupKeys))
	for index, key := range groupKeys {
		placeholders[index] = "?"
		queryArgs = append(queryArgs, key)
	}

	rows, err := s.logDB.Query(`
		SELECT `+runHistoryListColumns+`
		FROM run_history
		WHERE `+groupExpr+` IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY `+buildRunHistoryOrderClause(sortBy, sortOrder)+`
	`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list grouped run history items: %w", err)
	}
	defer rows.Close()

	items := make([]model.RunHistoryItem, 0, len(groupKeys)*pageSize)
	for rows.Next() {
		item, scanErr := scanRunHistory(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate grouped run history items: %w", err)
	}

	return items, total, nil
}

// countRunHistoryGroups 数「折叠组」的个数，也就是**执行次数**。
// run_history 是「每处理一项写一行」，直接 COUNT(*) 数出来的是记录行数，不是执行次数。
func (s *Store) countRunHistoryGroups(whereClause string, args []any) (int, error) {
	groupExpr := buildRunHistoryGroupExpression()

	var total int
	if err := s.logDB.QueryRow(`SELECT COUNT(*) FROM (SELECT `+groupExpr+` AS group_key FROM run_history`+whereClause+` GROUP BY group_key)`, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count grouped run history: %w", err)
	}

	return total, nil
}

// listRunHistoryTaskPage 按**代表行**分页：一行一个任务。
//
// 代表行 = 组内明细最完整的那条（detail_size 最大，同长取 id 大者），与运行日志页
// 折叠组「取组内第一条」的口径一致。收尾那条带全量明细的记录因此就是任务条目本身，
// 它的计数也正好是该次执行的最终统计。前端点开这个任务，按它的 id 走
// /run-history/detail 就能拿到整份明细。
//
// 为什么不让调用方拿 grouped 的结果自己去重：grouped 会把组内**每一行**都回给前端。
// 一次执行每处理一个文件写一行，上万项的执行就有上万行，而仪表盘每 5 秒拉一次 ——
// 只回代表行之后，响应体量只跟「任务数」有关，跟一次执行处理了多少项无关。
func (s *Store) listRunHistoryTaskPage(page, pageSize int, whereClause string, args []any, sortBy, sortOrder string) ([]model.RunHistoryItem, int, error) {
	total, err := s.countRunHistoryGroups(whereClause, args)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []model.RunHistoryItem{}, 0, nil
	}

	groupExpr := buildRunHistoryGroupExpression()
	// ROW_NUMBER() 给组内每一行编号，编号 1 的那行就是代表行。
	// detail_size 只在窗口内部参与编号，不在结果集里（列表刻意不读明细，也不回这一列）。
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.logDB.Query(`
		SELECT `+runHistoryListColumns+`
		FROM (
			SELECT `+runHistoryListColumns+`,
			       ROW_NUMBER() OVER (PARTITION BY `+groupExpr+` ORDER BY detail_size DESC, id DESC) AS group_rank
			FROM run_history`+whereClause+`
		)
		WHERE group_rank = 1
		ORDER BY `+buildRunHistoryTaskOrderClause(sortBy, sortOrder)+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list run history tasks: %w", err)
	}
	defer rows.Close()

	items := make([]model.RunHistoryItem, 0, pageSize)
	for rows.Next() {
		item, scanErr := scanRunHistory(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate run history tasks: %w", err)
	}

	return items, total, nil
}

// buildRunHistoryTaskOrderClause 是「任务视图」的外层排序。
//
// 结果里一行就是一个任务，而同一次执行的记录行共享同一个 started_at
// （见 executor 的 recordHistory：用的是 run.StartedAt），所以直接按 started_at 排即可，
// 不需要像分组查询那样再套一层 MAX(started_at)。
func buildRunHistoryTaskOrderClause(sortBy, sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		direction = "ASC"
	}

	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "name":
		return "LOWER(COALESCE(rule_name, '')) " + direction + ", started_at DESC, id DESC"
	case "modified_at":
		return "started_at " + direction + ", id DESC"
	default:
		return "started_at DESC, id DESC"
	}
}

func buildRunHistoryGroupExpression() string {
	return `COALESCE(CAST(rule_id AS TEXT), 'manual') || '|' || COALESCE(rule_name, '') || '|' || COALESCE(trigger_mode, '') || '|' || COALESCE(archive_mode, '') || '|' || COALESCE(link_mode, '') || '|' || COALESCE(started_at, '')`
}

func buildRunHistoryGroupOrderClause(sortBy, sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		direction = "ASC"
	}

	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "name":
		return "LOWER(COALESCE(rule_name, '')) " + direction + ", MAX(started_at) DESC, group_key DESC"
	case "modified_at":
		return "MAX(started_at) " + direction + ", group_key DESC"
	default:
		return "MAX(started_at) DESC, group_key DESC"
	}
}

// runHistoryDetailTieBreak 是同一次执行内部各行的排序兜底。
//
// 一次执行会写出**多行**历史（每处理一个文件 / 文件夹落一行），这些行共享同一个
// started_at，而 id 是随机十六进制 —— 只按 `started_at DESC, id DESC` 排的话，
// 组内谁排在第一条完全是随机的。前端「折叠任务」组取的正是**组内第一条**的 id
// （再走 /run-history/detail 按需拉明细），随机就意味着详情有时只显示前几个文件。
//
// 明细最完整的那一份长度最大，所以按明细长度降序排就能稳定拿到它，长度相同时再按 id 兜底。
//
// 这里排的是 `detail_size` **列**，不是 `LENGTH(detail_json)`：后者要求 SQLite 把
// 待排序的每一行的 detail_json 都读出来（字符串可能落在溢出页里），一次上万行的
// 列表查询就会把几百 MB 明细拉进内存。detail_size 在写入时算好、跟着行首页一起读，
// 排序代价与明细体积无关。存量行由 backfillRunHistoryDetailSize 补齐。
const runHistoryDetailTieBreak = "detail_size DESC, id DESC"

func buildRunHistoryOrderClause(sortBy, sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		direction = "ASC"
	}

	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "name":
		return "LOWER(COALESCE(rule_name, '')) " + direction + ", started_at DESC, " + runHistoryDetailTieBreak
	case "modified_at":
		return "started_at " + direction + ", " + runHistoryDetailTieBreak
	default:
		return "started_at DESC, " + runHistoryDetailTieBreak
	}
}

func (s *Store) GetRunHistorySummary() (model.RunHistorySummary, error) {
	var summary model.RunHistorySummary
	err := s.logDB.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN date(started_at, 'localtime') = date('now', 'localtime') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'skip' THEN 1 ELSE 0 END), 0)
		FROM run_history
	`).Scan(&summary.Total, &summary.Today, &summary.Success, &summary.Failed, &summary.Skipped)
	if err != nil {
		return model.RunHistorySummary{}, fmt.Errorf("get run history summary: %w", err)
	}

	return summary, nil
}

func (s *Store) DeleteRunHistoryByID(id string) error {
	if _, err := s.logDB.Exec(`DELETE FROM run_history WHERE id = ?`, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("delete run history by id: %w", err)
	}

	return nil
}

func (s *Store) DeleteRunHistoryByStatus(status string) error {
	if _, err := s.logDB.Exec(`DELETE FROM run_history WHERE status = ?`, strings.TrimSpace(status)); err != nil {
		return fmt.Errorf("delete run history by status: %w", err)
	}

	return nil
}

// ClearRunHistory 清空运行日志，并回收日志库文件空间。
//
// 为什么要顺带 VACUUM：DELETE 只是把页还给 freelist，logs.db 文件本身不会缩小，
// 而之后任何一次全表扫描（列表 / 汇总，仪表盘每 5 秒就有一次）都会把这个大文件
// 读进系统的 page cache —— docker stats 把 page cache 也算进容器内存，
// 于是「什么都没干内存也下不来」。清空是用户主动发起的低频操作，同步做一次彻底
// 回收是值得的；回收失败只记日志，清空本身已经生效。
func (s *Store) ClearRunHistory() error {
	if _, err := s.logDB.Exec(`DELETE FROM run_history`); err != nil {
		return fmt.Errorf("clear run history: %w", err)
	}

	if _, err := s.logDB.Exec(`VACUUM;`); err != nil {
		log.Printf("sqlite:runhistory: vacuum log store after clear skipped: %v", err)
	}

	return nil
}

func buildRunHistoryWhereClause(keyword, status, archiveMode, ruleType string) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 8)

	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus != "" {
		clauses = append(clauses, `status = ?`)
		args = append(args, trimmedStatus)
	}

	trimmedArchiveMode := strings.TrimSpace(archiveMode)
	if trimmedArchiveMode != "" {
		clauses = append(clauses, `archive_mode = ?`)
		args = append(args, trimmedArchiveMode)
	}

	trimmedRuleType := strings.TrimSpace(ruleType)
	if trimmedRuleType != "" {
		switch trimmedRuleType {
		case "archive":
			clauses = append(clauses, `(archive_mode = 'package' OR archive_mode = 'collect')`)
		case "cleanup":
			clauses = append(clauses, `(archive_mode = 'cleanup' OR archive_mode = 'transform')`)
		case "link":
			clauses = append(clauses, `archive_mode = 'link'`)
		case "naming":
			clauses = append(clauses, `archive_mode = 'naming'`)
		case "backup":
			clauses = append(clauses, `archive_mode = 'backup'`)
		}
	}

	trimmedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if trimmedKeyword != "" {
		like := "%" + trimmedKeyword + "%"
		clauses = append(clauses, `(
			LOWER(COALESCE(rule_name, '')) LIKE ? OR
			LOWER(COALESCE(summary, '')) LIKE ? OR
			LOWER(COALESCE(status, '')) LIKE ? OR
			LOWER(COALESCE(trigger_mode, '')) LIKE ? OR
			LOWER(COALESCE(archive_mode, '')) LIKE ?
		)`)
		args = append(args, like, like, like, like, like)
	}

	if len(clauses) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

type runHistoryScanner interface {
	Scan(dest ...any) error
}

func scanRunHistory(s runHistoryScanner) (model.RunHistoryItem, error) {
	var item model.RunHistoryItem
	var ruleID sql.NullInt64
	var startedAt string
	var updatedAt string
	var finishedAt string
	var linkMode string
	var detailJSON string

	err := s.Scan(
		&item.ID,
		&ruleID,
		&item.RuleName,
		&item.TriggerMode,
		&item.ArchiveMode,
		&linkMode,
		&item.Status,
		&item.ProcessedFiles,
		&item.SuccessCount,
		&item.SkipCount,
		&item.FailureCount,
		&item.DeletedCount,
		&item.SizeBytes,
		&item.Summary,
		&detailJSON,
		&startedAt,
		&updatedAt,
		&finishedAt,
	)
	if err != nil {
		return model.RunHistoryItem{}, fmt.Errorf("scan run history: %w", err)
	}

	if ruleID.Valid {
		v := ruleID.Int64
		item.RuleID = &v
	}
	item.LinkMode = strings.TrimSpace(linkMode)
	item.DetailJSON = strings.TrimSpace(detailJSON)
	item.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if finishedAt != "" {
		if t, parseErr := time.Parse(time.RFC3339, finishedAt); parseErr == nil {
			item.FinishedAt = &t
		}
	}

	return item, nil
}
