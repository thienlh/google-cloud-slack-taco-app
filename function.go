// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nlopes/slack/slackevents"

	"github.com/nlopes/slack"
)

// slackToken Authentication token from slack
var slackToken = os.Getenv("SLACK_TOKEN")

// SlackVerification/Token Verification token from slack
var slackVerificationToken = os.Getenv("VERIFICATION_TOKEN")

// emojiName name of the emoji to use
var emojiName = fmt.Sprintf(":%s:", os.Getenv("EMOJI_NAME"))

// emojiNameLength Length of emoji name (for quickly rejecting too short messages
var emojiNameLength = len(emojiName)

// maxGivingPerDay Maximum number of emoji can be given everyday by each user
var maxGivingPerDay, _ = strconv.Atoi(os.Getenv("MAX_EVERYDAY"))

// locationVietnam Location name Vietnam
const locationVietnam = "Vietnam"

// slackDateTimeFormat Datetime format sent from Slack
const slackDateTimeFormat = "01/02/2006 15:04:05"

// GoogleSheetsTimeFormat Datetime format sent from Google Sheets
const GoogleSheetsTimeFormat = "02-Jan 2006"

// todayInVietnam Today in Vietnam time in special format '02-Jan 2006'
var todayInVietnam = timeIn(locationVietnam, time.Now()).Format(GoogleSheetsTimeFormat)

// compiledSlackUserIDPattern Compile Slack user id pattern first for better performance
var compiledSlackUserIDPattern = regexp.MustCompile(`<@[\w]*>`)

// compiledSlackEmojiPattern Compile Slack user id pattern first for better performance
var compiledSlackEmojiPattern = regexp.MustCompile(strings.Replace(fmt.Sprintf("(%s){1}", emojiName), "-", "\\-", -1))

var slackClient = slack.New(slackToken)

// appMentionResponseMessage Message responding to app mention events
var appMentionResponseMessage = fmt.Sprintf("Chào anh chị em e-pilot :thuan: :mama-thuy: :tung: Xem BXH tại %s", spreadsheetURL)

// emojiX Response to event giving_to_bot
const emojiX = "x"

// emojiPray Response to event giving_to_him_herself
const emojiPray = "pray"

// emojiNoGood Response to event user_has_given_maximum_today
const emojiNoGood = "no_good"

// googleSheetGivingSummaryReadRange Read range for the giving summary sheet
const googleSheetGivingSummaryReadRange = "Pivot Table 1!A3:D"

// googleSheetReceivingSummaryReadRange Read range for the receiving summary sheet
const googleSheetReceivingSummaryReadRange = "Pivot Table 2!A3:D"

// givingSummary model represents each Giving Summary row in Google Sheets
type givingSummary struct {
	Name  string
	Date  string
	Total string
}

const paramHelp = "help"
const paramChart = "chart"
const paramWeek = "week"
const paramScrumSprint = "sprint"
const paramMonth = "month"
const paramYear = "year"

const noRecordFoundInChartResponseMessage = "No record found! :quy-serious:"

const invalidAppMentionCommandResponseMessage = "Invalid command. Available commands are: ```chart day\nchart week\nchart sprint\nchart month\nchart year```"

// lastRecordSprintStartDate A start date of the sprint with layout dd MM yyyy
var lastRecordSprintStartDate, _ = time.Parse("02 01 2006", os.Getenv("SPRINT_START_DATE"))

// sprintDuration Sprint duration in day
var sprintDuration, _ = strconv.Atoi(os.Getenv("SPRINT_DURATION"))

// Handle handle every requests
func Handle(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error reading buffer from body.")
		return
	}

	log.Printf("Header: %v\n", r.Header)
	body := buf.String()
	log.Printf("Body: %v\n", body)

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: slackVerificationToken}))
	if err != nil {
		log.Printf("Unable to parse event. Error %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Event: %v\n", eventsAPIEvent)

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		responseSlackChallenge(body, w)
		break
	case slackevents.CallbackEvent:
		handleCallbackEvent(eventsAPIEvent)
		break
	}

	w.WriteHeader(http.StatusOK)
	log.Printf("Done")
	return
}

// handleCallbackEvent Handle Callback events from Slack
func handleCallbackEvent(eventsAPIEvent slackevents.EventsAPIEvent) {
	log.Println("[Callback event]")
	innerEvent := eventsAPIEvent.InnerEvent
	log.Printf("Inner event %v\n", innerEvent)

	switch event := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Println("[AppMentionEvent]")

		handleAppMentionEvent(event)

		return
	case *slackevents.MessageEvent:
		log.Println("[MessageEvent]")

		handleMessageEvent(event)

		return
	default:
		log.Printf("Strange message event %v\n", eventsAPIEvent)

		return
	}

}

func handleAppMentionEvent(appMentionEvent *slackevents.AppMentionEvent) {
	// Trim the mention part
	// format: <@app_id> which contains 12 characters
	const slackAppIdLength = 12
	text := strings.ToLower(strings.TrimSpace(appMentionEvent.Text[slackAppIdLength:]))
	log.Printf("Text: [%s]\n", text)

	// @app help
	if text == paramHelp || text == "" {
		go postSlackMessage(appMentionEvent.Channel, appMentionResponseMessage)
		return
	}

	// @app chart <day> (default)
	// @app chart week
	// @app chart sprint
	// @app chart month
	if strings.HasPrefix(text, paramChart) {
		param := strings.Split(text, " ")[1]
		log.Printf("Param: %v\n", param)

		//	Calculate time in Vietnam
		nowInVietnam := timeIn(locationVietnam, time.Now())

		year, month, day := nowInVietnam.Date()
		today := Date{year, month, day}

		// Default
		from := today
		to := today

		switch param {
		case paramWeek:
			date := nowInVietnam

			// Iterate back to Monday
			for date.Weekday() != time.Monday {
				date = date.AddDate(0, 0, -1)
			}

			from = Date{date.Year(), date.Month(), date.Day()}
			break
		case paramScrumSprint:
			sprintStart := lastRecordSprintStartDate
			sprintEnd := lastRecordSprintStartDate.AddDate(0, 0, sprintDuration).Add(-1)

			inCurrentSprintPeriod := nowInVietnam.After(sprintStart) && nowInVietnam.Before(sprintEnd)
			for !inCurrentSprintPeriod {
				sprintStart = sprintEnd.Add(1)
				sprintEnd = sprintStart.AddDate(0, 0, sprintDuration).Add(-1)
			}

			from = Date{sprintStart.Year(), sprintStart.Month(), sprintStart.Day()}
			break
		case paramMonth:
			from = Date{year, month, 1}
			break
		case paramYear:
			from = Date{year, 1, 1}
		default:
			log.Printf("Strange param %s!", param)
			return
		}

		chartRecords := getChart(from, to)
		if len(chartRecords) > 0 {
			go postSlackMessage(appMentionEvent.Channel, chartRecords.String())
		} else {
			go postSlackMessage(appMentionEvent.Channel, noRecordFoundInChartResponseMessage)
		}
	}

	log.Println("Strange App Mention Event")
	go postSlackMessage(appMentionEvent.Channel, invalidAppMentionCommandResponseMessage)
}

func handleMessageEvent(event *slackevents.MessageEvent) {
	if !verifyMessageEvent(event) {
		return
	}
	log.Printf("Message text: %v\n", event.Text)

	for _, row := range strings.Split(event.Text, "\n") {
		log.Printf("Processing row: %s\n", row)

		go processMessageText(event, row)
	}
	log.Println("Finish handling MessageEvent")
}

// processMessageText Process by custom text instead of entire message
func processMessageText(event *slackevents.MessageEvent, text string) {
	// Get the emoji in the message
	// return if no exact emoji found
	numEmojiMatches := countNumEmoji(text)
	if numEmojiMatches == 0 {
		log.Printf("No emoji %v found in message %v. Return.\n", emojiName, text)
		return
	}

	// Find the receiver
	receiverId := getReceiverId(text)

	if receiverId == "" {
		log.Printf("No receiver found. Return.\n")
		return
	}

	// Get receiver information
	receiver, err := slackClient.GetUserInfo(receiverId)
	if err != nil {
		log.Panicf("Error getting receiver %v info %v\n", receiverId, err)
		return
	}
	printSlackUserInfo(receiver)

	if receiver.IsBot {
		log.Printf("Receiver %v is bot. Return.\n", receiver.Profile.RealName)
		go reactToSlackMessage(event.Channel, event.TimeStamp, emojiX)
		return
	}

	// Get the user who posted the message
	// return if error
	user, err := slackClient.GetUserInfo(event.User)
	if err != nil {
		log.Panicf("Error getting user %v info %v\n", event.User, err)
		return
	}
	printSlackUserInfo(user)

	// Won't accept users giving for themself
	if user.ID == receiverId {
		log.Printf("User with id %v is self-giving. Return.\n", user.ID)
		go reactToSlackMessage(event.Channel, event.TimeStamp, emojiPray)
		return
	}

	go checkGivingRestrictions(event, user, receiver, numEmojiMatches)
	return
}

func getChart(from Date, to Date) ChartRecords {
	log.Printf("From: %v, to %v\n", from, to)
	records := readFrom(googleSheetReceivingSummaryReadRange)
	chart := map[string]int{}

	for _, record := range records {
		// Skip Grand Total record
		if strings.Contains(fmt.Sprintf("%s %s %s %s", record[0], record[1], record[2], record[3]), "Grand Total") {
			log.Println("Grand Total record. Skip.")
			break
		}

		receivingSummary := givingSummary{record[0].(string), fmt.Sprintf("%s %s", record[1], record[2]), record[3].(string)}
		date, err := time.Parse(GoogleSheetsTimeFormat, receivingSummary.Date)
		if err != nil {
			log.Panicf("Unable to parse date %v from Google Sheets!", receivingSummary.Date)
			return nil
		}

		if isInRange(timeIn(locationVietnam, date), from, to) {
			total, err := strconv.Atoi(receivingSummary.Total)
			if err != nil {
				log.Fatalf("Unable to parse total %v to int with error %v", receivingSummary.Total, err)
				return nil
			}

			chart[receivingSummary.Name] = total + chart[receivingSummary.Name]
			log.Printf("Chart: %v\n", chart)
		}
	}

	log.Printf("Leaderboard: %v\n", chart)
	return rank(chart)
}

// printSlackUserInfo Print Slack user information
func printSlackUserInfo(user *slack.User) {
	log.Printf("Slack User { ID: %v, Fullname: %v, Email: %v }\n", user.ID, user.Profile.RealName, user.Profile.Email)
}

// checkGivingRestrictions Check number of emoji user can give today
func checkGivingRestrictions(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, numWantToGive int) {
	userRealName := user.Profile.RealName
	givingSummaries := readFrom(googleSheetGivingSummaryReadRange)

	// TODO: Filter by year first
	// maybe even month and then day
	// for better performance
	for _, row := range givingSummaries {
		// Skip Grand Total row
		if strings.Contains(fmt.Sprintf("%s %s %s %s", row[0], row[1], row[2], row[3]), "Grand Total") {
			log.Println("Grand Total row. Skip.")
			break
		}

		givingSummary := givingSummary{row[0].(string), fmt.Sprintf("%s %s", row[1], row[2]), row[3].(string)}
		log.Printf("Giving summary: %v, today: %v\n", givingSummary, todayInVietnam)

		usersTodayRecord := userRealName == givingSummary.Name && givingSummary.Date == todayInVietnam
		if usersTodayRecord {
			log.Printf("Today record for user %v found.\n", userRealName)

			numGivenToday, err := strconv.Atoi(givingSummary.Total)
			if err != nil {
				log.Panicf("%v can not be convert to int\n", givingSummary.Total)
				return
			}

			if numGivenToday >= maxGivingPerDay {
				log.Printf("User %s already gave %d today (maximum allowed: %d). Return.\n", givingSummary.Name, numGivenToday, maxGivingPerDay)
				go reactToSlackMessage(event.Channel, event.TimeStamp, emojiNoGood)

				return
			}

			maxToGiveToday := maxGivingPerDay - numGivenToday
			var numToGive int

			if numWantToGive <= maxToGiveToday {
				numToGive = numWantToGive
			} else {
				numToGive = maxToGiveToday
			}

			log.Printf("Can be given today: %d, maximum to give per day: %d,"+
				" user has given today: %d, want to give now: %d\n",
				maxToGiveToday, maxGivingPerDay, numGivenToday, numWantToGive)

			if numToGive > 0 {
				giveTaco(event, user, receiver, numToGive)
			} else {
				go reactToSlackMessage(event.Channel, event.TimeStamp, emojiX)
			}

			return
		} else {
			//	Not this row
			continue
		}
	}

	// No record today found for user
	log.Printf("No record found today %v for user %v. Let he/she give at most %v.\n", todayInVietnam, userRealName, maxGivingPerDay)
	if numWantToGive >= maxGivingPerDay {
		numWantToGive = maxGivingPerDay
	}

	giveTaco(event, user, receiver, numWantToGive)
}

// giveTaco Record giving for user
// write to Google Sheets and react to Slack message
func giveTaco(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, numToGive int) {
	log.Printf("Record giving now for user %v, receiver %v, number %v\n", user, receiver, numToGive)

	writeToGoogleSheets(event, user, receiver, numToGive)

	numberEmojis := getNumberEmoji(numToGive)

	for _, e := range numberEmojis {
		go reactToSlackMessage(event.Channel, event.TimeStamp, e)
	}
}

// writeToGoogleSheets Write value to Google Sheets using gsheets.go
func writeToGoogleSheets(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, toGive int) {
	// Timestamp, Giver, Receiver, Quantity, Text, Date time
	// Format from Slack: 1547921475.007300
	var timestamp = timeIn(locationVietnam, toDate(strings.Split(event.TimeStamp, ".")[0]))
	// Using Google Sheets recognizable format
	var datetime = timestamp.Format(slackDateTimeFormat)
	var giverRealName = user.Profile.RealName
	var receiverRealName = receiver.Profile.RealName
	var message = event.Text
	valueToWrite := []interface{}{timestamp, datetime, giverRealName, receiverRealName, toGive, message}
	log.Printf("Value to write %v\n", valueToWrite)

	go appendValue(valueToWrite)
}

// getReceiverId Find id of the receiver in text message
func getReceiverId(text string) string {
	receivers := compiledSlackUserIDPattern.FindAllString(text, -1)
	log.Printf("Matched receivers %v\n", receivers)

	// Return empty if no receiver found
	if len(receivers) == 0 {
		return ""
	}

	// Only the first mention count as receiver
	var receiverRaw = receivers[0]

	// Slack user format: <@USER_ID>
	return receiverRaw[2 : len(receiverRaw)-1]
}

// countNumEmoji Find number of emoji emojiName appeared in text message
func countNumEmoji(text string) int {
	emojiMatches := compiledSlackEmojiPattern.FindAllString(text, -1)
	log.Printf("Matched emoji %v in text %v\n", emojiMatches, text)
	return len(emojiMatches)
}

// responseSlackChallenge Response to Slack's URL verification challenge
func responseSlackChallenge(body string, w http.ResponseWriter) {
	log.Println("[Slack URL Verification challenge event]")
	var response *slackevents.ChallengeResponse
	err := json.Unmarshal([]byte(body), &response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatalf("Unable to unmarshal slack URL verification challenge. Error %v\n", err)
	}

	w.Header().Set("Content-Type", "text")
	numWrittenBytes, err := w.Write([]byte(response.Challenge))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatalf("Unable to write response challenge with error %v\n", err)
	}
	log.Printf("%v bytes of Slack challenge response written\n", numWrittenBytes)
}

// verifyMessageEvent Check whether the message event is valid for processing
func verifyMessageEvent(event *slackevents.MessageEvent) bool {
	if event.SubType != "" {
		log.Printf("Event with subtype %v. Return.\n", event.SubType)
		return false
	}

	// TODO: Maybe handle edited messages someday ;)
	if event.IsEdited() {
		log.Printf("Edited message. Return.\n")
		return false
	}

	if len(event.Text) < emojiNameLength {
		log.Printf("Message too short. Return.\n")
		return false
	}

	return true
}

// postSlackMessage Post message to Slack
func postSlackMessage(channel string, text string) {
	var msgOptionText = slack.MsgOptionText(text, true)
	respChannel, respTimestamp, err := slackClient.PostMessage(channel, msgOptionText)
	if err != nil {
		log.Printf("Unable to post message to Slack with error %v\n", err)
		return
	}
	log.Printf("Message posted to channel %v at %v\n", respChannel, respTimestamp)
}

// reactToSlackMessage React to Slack message
func reactToSlackMessage(channel string, timestamp string, emoji string) {
	refToMessage := slack.NewRefToMessage(channel, timestamp)
	err := slackClient.AddReaction(emoji, refToMessage)
	if err != nil {
		log.Printf("Unable to react %v to comment %v with error %v\n", emoji, refToMessage, err)
		return
	}
}
