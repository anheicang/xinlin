// 文件用途：按上海时区解析快递轨迹文本，并提取最早和最晚的有效时间。

package kd100

import "time"

// shanghaiLocation 固定解释供应商的中国本地时间；系统缺少时区数据时回退到 UTC+8。
var shanghaiLocation = func() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}()

// TrackTimeUnix 按上海时区解析 yyyy-MM-dd HH:mm:ss 格式的轨迹时间。
// 解析失败时返回 false，成功时返回 Unix 秒时间戳。
func TrackTimeUnix(value string) (int64, bool) {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", value, shanghaiLocation)
	if err != nil {
		return 0, false
	}
	return t.Unix(), true
}

// FirstTrackUnix 从轨迹中选出最早的有效时间，忽略无法解析的时间项。
// 没有任何有效时间时返回 false。
func FirstTrackUnix(tracks []TrackData) (int64, bool) {
	var result int64
	found := false
	for _, track := range tracks {
		if unix, ok := TrackTimeUnix(track.Ftime); ok && (!found || unix < result) {
			result, found = unix, true
		}
	}
	return result, found
}

// LatestTrackUnix 从轨迹中选出最晚的有效时间，不依赖上游轨迹排列顺序。
// 没有任何有效时间时返回 false。
func LatestTrackUnix(tracks []TrackData) (int64, bool) {
	var result int64
	found := false
	for _, track := range tracks {
		if unix, ok := TrackTimeUnix(track.Ftime); ok && (!found || unix > result) {
			result, found = unix, true
		}
	}
	return result, found
}
