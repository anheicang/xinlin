package kd100

import "time"

var shanghaiLocation = func() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}()

func TrackTimeUnix(value string) (int64, bool) {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", value, shanghaiLocation)
	if err != nil {
		return 0, false
	}
	return t.Unix(), true
}

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
