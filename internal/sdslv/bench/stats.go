package bench

import "sort"

type Statistics struct {
	Count  int    `json:"count"`
	Min    uint64 `json:"min"`
	Median uint64 `json:"median"`
	Max    uint64 `json:"max"`
	Mean   uint64 `json:"mean"`
}

// StatisticsFor uses the lower middle for even sample counts, preserving the
// integer-nanosecond canonical representation without rounding.
func StatisticsFor(samples []uint64) Statistics {
	if len(samples) == 0 {
		return Statistics{}
	}
	v := append([]uint64(nil), samples...)
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	var sum uint64
	for _, n := range v {
		sum += n
	}
	return Statistics{Count: len(v), Min: v[0], Median: v[(len(v)-1)/2], Max: v[len(v)-1], Mean: sum / uint64(len(v))}
}
