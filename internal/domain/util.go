package domain

import "time"

// NowUnix returns the current time as Unix seconds since epoch.
func NowUnix() int64 {
	return time.Now().Unix()
}
