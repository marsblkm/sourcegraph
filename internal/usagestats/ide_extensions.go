package usagestats

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func GetIdeExtensionsUsageStatistics(ctx context.Context, db database.DB) (*types.IdeExtensionsUsage, error) {
	// Only getting stats by month
	// TODO: group by month, day, week
	// TODO: add backward compatible for vsce
	stats := types.IdeExtensionsUsage{}

	ideExtensionsPeriodQuery := `
	SELECT
		DATE_TRUNC('month', TIMEZONE('UTC', $1::timestamp)) as current_month,
		DATE_TRUNC('week', TIMEZONE('UTC', $1::timestamp)) as current_week,
		DATE_TRUNC('day', TIMEZONE('UTC', $1::timestamp)) as current_day
	FROM event_logs
	WHERE timestamp >= DATE_TRUNC('month', $1::timestamp) AND source = 'IDEEXTENSION';
	`

	if err := db.QueryRowContext(ctx, ideExtensionsPeriodQuery, timeNow()).Scan(
		&stats.Month.StartTime,
		&stats.Week.StartTime,
		&stats.Day.StartTime,
	); err != nil {
		return nil, err
	}

	ideExtensionsPeriodUsageQuery := `
	SELECT
		argument ->> 'platform'::text AS ide_kind,
		COUNT(DISTINCT user_id) AS user_count,
		COUNT(*) FILTER (WHERE name = 'IDESearchSubmitted'),
		COUNT(DISTINCT user_id) FILTER (WHERE name = 'IDESearchSubmitted'),
		COUNT(*) FILTER (WHERE name = 'IDERedirects')
	FROM event_logs
	WHERE
		source = 'IDEEXTENSION' AND timestamp >= DATE_TRUNC('month', $1::timestamp)
	GROUP BY ide_kind;
	`
	usageStatisticsByIde := []*types.IdeExtensionsUsageStatistics{}

	rows, err := db.QueryContext(ctx, ideExtensionsPeriodUsageQuery, timeNow())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		ideaExtensionUsageStatistics := types.IdeExtensionsUsageStatistics{}

		if err := rows.Scan(
			&ideaExtensionUsageStatistics.IdeKind,
			&ideaExtensionUsageStatistics.UserCount,
			&ideaExtensionUsageStatistics.SearchPerformed.TotalCount,
			&ideaExtensionUsageStatistics.SearchPerformed.UniqueCount,
			&ideaExtensionUsageStatistics.RedirectCount,
		); err != nil {
			return nil, err
		}

		usageStatisticsByIde = append(usageStatisticsByIde, &ideaExtensionUsageStatistics)
	}

	stats.Month.IDEs = usageStatisticsByIde

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &stats, nil

	// TODO: Monthly Installs

	// TODO: Monthly Uninstalls
}

// USERCOUNT
// SELECT COUNT(DISTINCT user_id) FROM event_logs WHERE name LIKE 'VSCE%' OR source = 'IDEEXTENSION';

// SEARCHPERFORMED
// SELECT COUNT(DISTINCT user_id) FROM event_logs WHERE name LIKE 'VSCESearchSubmitted' OR name LIKE 'IDESearchSubmitted';

// TOTALSEARCHPERFORMED
// SELECT COUNT(name) FROM event_logs WHERE name LIKE 'VSCESearchSubmitted' OR name LIKE 'IDESearchSubmitted';

// Redirects
// SELECT * FROM event_logs WHERE name = 'IDERedirects' OR (url LIKE '%&utm_source=VSCode-%' AND name = 'ViewBlob');

// Installs

// Uninstalls
