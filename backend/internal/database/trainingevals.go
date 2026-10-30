package database

import (
	"database/sql"
	"fmt"
	"time"

	"training-record/internal/domain"
)

// CollapseLegacyBiweeklyEvaluations is an idempotent startup fixup that
// converts the legacy daily "biweekly" evaluation rows into the fixed
// two-week grid (日曜〜土曜・domain.BiweeklyPeriodOf).
//
// 旧バッチは毎日1行、period_from = 実行日-14日 / period_to = 実行日 の
// ローリング窓で保存していた（期間同士が13日重複する）。この fixup は:
//
//  1. legacy 行（period_to - period_from = 14日）を実行日（period_to）が
//     属する固定2週間期間ごとにグループ化する
//  2. 各グループの最新行（evaluated_at 最大）をその期間の代表として残し、
//     period_from / period_to を固定期間の境界（10/4〜10/17 など）に書き換える
//  3. グループ内の残りの行を削除する
//
// bimonthly 行と、すでに固定グリッド形式（期間の長さ = 13日）の biweekly 行は
// 触らない。以降のバッチは1期間1行（UpsertTrainingEvaluation）で保存するため、
// この fixup は legacy 行が無くなると no-op になる。
func CollapseLegacyBiweeklyEvaluations(db *sql.DB) error {
	rows, err := db.Query(
		`SELECT id, evaluated_at, period_from, period_to FROM training_evaluations
		 WHERE period_type = 'biweekly' ORDER BY evaluated_at, id`)
	if err != nil {
		return fmt.Errorf("load biweekly evaluations: %w", err)
	}
	defer rows.Close()

	type evalRow struct {
		id          int64
		evaluatedAt string
		periodFrom  string
		periodTo    string
	}
	// groups: 固定期間の from (YYYY-MM-DD) → その期間に実行された legacy 行（時系列順）。
	groups := map[string][]evalRow{}
	for rows.Next() {
		var r evalRow
		if err := rows.Scan(&r.id, &r.evaluatedAt, &r.periodFrom, &r.periodTo); err != nil {
			return fmt.Errorf("scan biweekly evaluation: %w", err)
		}
		from, err := time.ParseInLocation(domain.DateLayout, r.periodFrom, domain.JST)
		if err != nil {
			continue // 想定外の形式は触らない
		}
		to, err := time.ParseInLocation(domain.DateLayout, r.periodTo, domain.JST)
		if err != nil {
			continue
		}
		if int(to.Sub(from).Hours()/24) != domain.BiweeklyPeriodDays {
			continue // すでに固定グリッド形式（13日）の行
		}
		gFrom, _, ok := domain.BiweeklyPeriodOfDate(r.periodTo) // 実行日を含む固定期間
		if !ok {
			continue
		}
		groups[gFrom] = append(groups[gFrom], r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate biweekly evaluations: %w", err)
	}
	if len(groups) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for gFrom, rs := range groups {
		_, gTo, ok := domain.BiweeklyPeriodOfDate(gFrom)
		if !ok {
			continue
		}
		keep := rs[len(rs)-1] // evaluated_at 最大の行（クエリの時系列順）
		if _, err := tx.Exec(
			`UPDATE training_evaluations SET period_from = ?, period_to = ? WHERE id = ?`,
			gFrom, gTo, keep.id); err != nil {
			return fmt.Errorf("rewrite legacy biweekly row %d: %w", keep.id, err)
		}
		for _, r := range rs[:len(rs)-1] {
			if _, err := tx.Exec(`DELETE FROM training_evaluations WHERE id = ?`, r.id); err != nil {
				return fmt.Errorf("delete legacy biweekly row %d: %w", r.id, err)
			}
		}
	}
	return tx.Commit()
}
