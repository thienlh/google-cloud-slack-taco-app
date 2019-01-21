package p

import (
	"log"
	"strconv"
	"time"
)

//	CountryTz Supported locations
var CountryTz = map[string]string{
	"Hungary": "Europe/Budapest",
	"Egypt":   "Africa/Cairo",
	"Vietnam": "Asia/Ho_Chi_Minh",
}

//	timeIn Return time in location
func timeIn(name string, t time.Time) time.Time {
	loc, err := time.LoadLocation(CountryTz[name])
	if err != nil {
		log.Panicf("Error loading location %v", name)
	}
	return t.In(loc)
}

// toDate Convert epoch timestamp to time.Time
func toDate(timestamp string) time.Time {
	i, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Fatalf("Error parsing timestamp: %s", timestamp)
	}

	return time.Unix(i, 0)
}
