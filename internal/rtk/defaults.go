package rtk

import (
	"strconv"
	"time"
)

const DefaultTimeout = 10 * time.Minute
const DefaultTimeoutMS = int(DefaultTimeout / time.Millisecond)

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func roundLimitExceeded(limit *int, round int) bool {
	return limit != nil && round > *limit
}

func roundLimitExhausted(limit *int, completedRounds int) bool {
	return limit != nil && completedRounds >= *limit
}

func roundLimitText(limit *int) string {
	if limit == nil {
		return "unbounded"
	}
	return strconv.Itoa(*limit)
}

func roundProgressText(round int, limit *int) string {
	return strconv.Itoa(round) + "/" + roundLimitText(limit)
}
