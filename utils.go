package p

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

const topLeaderboardEmoji = ":crown:"
const secondLeaderboardEmoji = ":rocket:"
const thirdLeaderboardEmoji = ":trident:"
const pairFormat = "*%-50s*\t\t%-5d"
const leaderboardFormat = "%15s\t%v"

type Date struct {
	Year  int
	Month time.Month
	Day   int
}

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

func isInRange(t time.Time, start Date, end Date) bool {
	timeInUTC := t.In(time.UTC).Add(10000)
	startTime := time.Date(start.Year, start.Month, start.Day, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(end.Year, end.Month, end.Day, 23, 59, 59, 999, time.UTC)

	return timeInUTC.After(startTime) && timeInUTC.Before(endTime)
}

func rank(m map[string]int) PairList {
	pl := make(PairList, len(m))
	i := 0
	for k, v := range m {
		pl[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(pl))
	return pl
}

type Pair struct {
	Key   string
	Value int
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Pair) String() string {
	return fmt.Sprintf(pairFormat, p.Key, p.Value)
}
func (p PairList) String() string {
	var arr []string

	for _, pair := range p {
		arr = append(arr, pair.String())
	}

	if len(arr) >= 1 {
		//	Add some emoji
		arr[0] = fmt.Sprintf(leaderboardFormat, arr[0], topLeaderboardEmoji)
	}

	if len(arr) >= 2 {
		arr[1] = fmt.Sprintf(leaderboardFormat, arr[1], secondLeaderboardEmoji)
	}

	if len(arr) >= 3 {
		arr[2] = fmt.Sprintf(leaderboardFormat, arr[2], thirdLeaderboardEmoji)
	}

	return strings.Join(arr, "\n")
}
