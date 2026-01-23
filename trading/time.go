package trading

import "time"

// 中国时区
var cst = time.FixedZone("CST", 8*3600)

// TimeRange 时间范围
type TimeRange struct {
	StartHour   int
	StartMinute int
	EndHour     int
	EndMinute   int
}

// A股交易时间段
var stockTradingHours = []TimeRange{
	{9, 30, 11, 30},  // 上午 9:30-11:30
	{13, 0, 15, 0},   // 下午 13:00-15:00
}

// 期货交易时间段（日盘）
var futuresDayHours = []TimeRange{
	{9, 0, 10, 15},   // 上午第一节 9:00-10:15
	{10, 30, 11, 30}, // 上午第二节 10:30-11:30
	{13, 30, 15, 0},  // 下午 13:30-15:00
}

// 期货交易时间段（夜盘）- 不同品种不同，这里取最大范围
var futuresNightHours = []TimeRange{
	{21, 0, 23, 59}, // 夜盘第一段 21:00-23:59
	{0, 0, 2, 30},   // 夜盘第二段（部分品种）00:00-02:30
}

// IsStockTradingTime 判断当前是否为A股交易时间
func IsStockTradingTime() bool {
	return IsStockTradingTimeAt(time.Now())
}

// IsStockTradingTimeAt 判断指定时间是否为A股交易时间
func IsStockTradingTimeAt(t time.Time) bool {
	t = t.In(cst)

	// 检查是否为工作日（周一到周五）
	weekday := t.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// 检查是否在交易时间段内
	return isInTimeRanges(t, stockTradingHours)
}

// IsFuturesTradingTime 判断当前是否为期货交易时间
func IsFuturesTradingTime() bool {
	return IsFuturesTradingTimeAt(time.Now())
}

// IsFuturesTradingTimeAt 判断指定时间是否为期货交易时间
func IsFuturesTradingTimeAt(t time.Time) bool {
	t = t.In(cst)

	weekday := t.Weekday()
	hour := t.Hour()

	// 夜盘检查（周一到周五晚上，周六凌晨）
	if hour >= 21 {
		// 周五晚上有夜盘
		if weekday >= time.Monday && weekday <= time.Friday {
			if isInTimeRanges(t, futuresNightHours) {
				return true
			}
		}
	} else if hour < 3 {
		// 凌晨时段：周二到周六凌晨（对应周一到周五的夜盘）
		if weekday >= time.Tuesday && weekday <= time.Saturday {
			if isInTimeRanges(t, futuresNightHours) {
				return true
			}
		}
	}

	// 日盘检查（周一到周五白天）
	if weekday >= time.Monday && weekday <= time.Friday {
		if isInTimeRanges(t, futuresDayHours) {
			return true
		}
	}

	return false
}

// IsTradingTime 判断当前是否为任意市场的交易时间
func IsTradingTime() bool {
	return IsStockTradingTime() || IsFuturesTradingTime()
}

// isInTimeRanges 检查时间是否在指定的时间范围内
func isInTimeRanges(t time.Time, ranges []TimeRange) bool {
	hour := t.Hour()
	minute := t.Minute()
	currentMinutes := hour*60 + minute

	for _, r := range ranges {
		startMinutes := r.StartHour*60 + r.StartMinute
		endMinutes := r.EndHour*60 + r.EndMinute

		// 处理跨午夜的情况
		if startMinutes <= endMinutes {
			if currentMinutes >= startMinutes && currentMinutes <= endMinutes {
				return true
			}
		} else {
			// 跨午夜：23:00-02:30
			if currentMinutes >= startMinutes || currentMinutes <= endMinutes {
				return true
			}
		}
	}
	return false
}

// GetNextTradingTime 获取下一个交易时间
func GetNextTradingTime() time.Time {
	now := time.Now().In(cst)

	// 简化实现：返回下一个可能的交易时间点
	// 实际使用中可以根据需要优化
	for i := 0; i < 7*24*60; i++ {
		checkTime := now.Add(time.Duration(i) * time.Minute)
		if IsStockTradingTimeAt(checkTime) || IsFuturesTradingTimeAt(checkTime) {
			return checkTime
		}
	}
	return now.Add(24 * time.Hour)
}
