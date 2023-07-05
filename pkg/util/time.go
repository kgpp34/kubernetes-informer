package util

import "time"

func ConvertUTCToAsiaShanghai(utcTime time.Time) (time.Time, error) {
	// 设置时区为东八区（亚洲/上海）
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, err
	}

	// 使用指定时区将UTC时间转换为东八区时间
	asiaTime := utcTime.In(loc)
	return asiaTime, nil
}
