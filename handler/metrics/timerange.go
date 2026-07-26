package metrics

import "time"

// startOfDay 返回 t 所在日期的零点（保留时区）。
func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// startOfDaysAgo 返回 now 往前 days 天那一日的零点，作为「最近 days 天」的闭区间起点。
func startOfDaysAgo(now time.Time, days int) time.Time {
	return startOfDay(now).AddDate(0, 0, -days)
}
