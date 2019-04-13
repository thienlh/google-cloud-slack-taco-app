package p

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

const topChartEmoji = ":crown:"
const runnerUpEmoji = ":rocket:"
const thirdChartEmoji = ":trident:"
const pairFormat = "*%s* (%d)"
const chartFormat = "%s %v"

type Date struct {
	Year  int
	Month time.Month
	Day   int
}

//	countryTz Supported locations
var countryTz = map[string]string{
	"Hungary": "Europe/Budapest",
	"Egypt":   "Africa/Cairo",
	"Vietnam": "Asia/Ho_Chi_Minh",
}

//	timeIn Return time in location
func timeIn(name string, t time.Time) time.Time {
	loc, err := time.LoadLocation(countryTz[name])
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

func rank(m map[string]int) ChartRecords {
	pl := make(ChartRecords, len(m))
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

type ChartRecords []Pair

func (p ChartRecords) Len() int           { return len(p) }
func (p ChartRecords) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p ChartRecords) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Pair) String() string {
	return fmt.Sprintf(pairFormat, p.Key, p.Value)
}
func (p ChartRecords) String() string {
	var arr []string

	for _, pair := range p {
		arr = append(arr, pair.String())
	}

	if len(arr) >= 1 {
		//	Add some emoji
		arr[0] = fmt.Sprintf(chartFormat, arr[0], topChartEmoji)
	}

	if len(arr) >= 2 {
		arr[1] = fmt.Sprintf(chartFormat, arr[1], runnerUpEmoji)
	}

	if len(arr) >= 3 {
		arr[2] = fmt.Sprintf(chartFormat, arr[2], thirdChartEmoji)
	}

	return strings.Join(arr, "\n")
}

var emojiTexts = map[int]string{
	0:  "zero",
	1:  "one",
	2:  "two",
	3:  "three",
	4:  "four",
	5:  "five",
	6:  "six",
	7:  "seven",
	8:  "eight",
	9:  "nine",
	10: "keycap_ten",
}

// getNumberEmoji Return number of given emoji in text
// character by character
func getNumberEmoji(number int) []string {
	if number < 1 {
		return nil
	}

	var results []string

	if number > 0 && number <= 10 {
		return append(results, emojiTexts[number])
	}

	str := strconv.Itoa(number)

	for _, r := range str {
		var c = string(r)
		n, _ := strconv.Atoi(c)
		results = append(results, emojiTexts[n])
	}

	return results
}
