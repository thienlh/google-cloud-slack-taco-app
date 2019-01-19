package p

import (
	"log"
	"strconv"
	"time"
)

func toDate(timestamp string) time.Time {
	i, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Fatalf("Error parsing timestamp: %s", timestamp)
	}

	return time.Unix(i, 0)
}
