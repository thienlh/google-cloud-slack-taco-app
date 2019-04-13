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

// maxEveryday Maximum number of emoji can be given everyday by each user
var maxEveryday, _ = strconv.Atoi(os.Getenv("MAX_EVERYDAY"))

// locationVietnam Location name Vietnam
const locationVietnam = "Vietnam"

// slackDateTimeFormat Datetime format sent from Slack
const slackDateTimeFormat = "01/02/2006 15:04:05"

// todayInVietnam Today in Vietnam time in special format '02-Jan 2006'
var todayInVietnam = timeIn("Vietnam", time.Now()).Format(GoogleSheetsTimeFormat)

// compiledSlackUserIDPattern Compile Slack user id pattern first for better performance
var compiledSlackUserIDPattern = regexp.MustCompile(`<@[\w]*>`)

// compiledSlackEmojiPattern Compile Slack user id pattern first for better performance
var compiledSlackEmojiPattern = regexp.MustCompile(strings.Replace(fmt.Sprintf("(%s){1}", emojiName), "-", "\\-", -1))

// api Slack API
var api = slack.New(slackToken)

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
const paramBxh = "bxh"
const paramWeek = "week"
const paramSprint = "sprint"
const paramMonth = "month"
const paramYear = "year"

const leaderboardNoRecordFoundResponseMessage = "No record found! :quy-serious:"

const invalidAppMentionCommandResponseMessage = "Invalid command. Available commands are: ```bxh day\nbxh week\nbxh sprint\nbxh month\nbxh year```"

// sprintStartDate A start date of the sprint with layout dd MM yyyy
var sprintStartDate, _ = time.Parse("02 01 2006", os.Getenv("SPRINT_START_DATE"))

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
	body := buf.String()
	log.Printf("Body: %v\n", body)
	log.Printf("Header: %v\n", r.Header)

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: slackVerificationToken}))
	if err != nil {
		log.Printf("Unable to parse event. Error %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Event: %v\n", eventsAPIEvent)

	// Remember to break!
	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		responseToSlackChallenge(body, w)
		break
	case slackevents.CallbackEvent:
		handleCallbackEvent(eventsAPIEvent)
		break
	}

	log.Printf("Done")
	w.WriteHeader(http.StatusOK)
	return
}

// handleCallbackEvent Handle Callback events from Slack
func handleCallbackEvent(eventsAPIEvent slackevents.EventsAPIEvent) {
	log.Println("[Callback event]")
	innerEvent := eventsAPIEvent.InnerEvent
	log.Printf("Inner event %v\n", innerEvent)

	// Remember to return!
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Println("[AppMentionEvent]")

		// Trim the mention part
		// format: <@app_id> which contains 12 characters
		text := strings.ToLower(strings.TrimSpace(ev.Text[12:]))
		log.Printf("Text: [%s]\n", text)

		if text == paramHelp || text == "" {
			go postSlackMessage(ev.Channel, appMentionResponseMessage)
			return
		}

		// @app bxh <day> (default)
		// @app bxh week
		// @app bxh sprint
		// @app bxh month
		if strings.HasPrefix(text, paramBxh) {
			param := strings.Split(text, " ")[1]
			log.Printf("Param: %v\n", param)

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
			case paramSprint:
				sprintStart := sprintStartDate
				sprintEnd := sprintStartDate.AddDate(0, 0, sprintDuration).Add(-1)

				for !(nowInVietnam.After(sprintStart) && nowInVietnam.Before(sprintEnd)) {
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
				break
			}

			pairs := getLeaderboard(from, to)
			if len(pairs) > 0 {
				go postSlackMessage(ev.Channel, pairs.String())
			} else {
				go postSlackMessage(ev.Channel, leaderboardNoRecordFoundResponseMessage)
			}
			return
		}

		log.Println("Strange App Mention Event")
		go postSlackMessage(ev.Channel, invalidAppMentionCommandResponseMessage)
		return
	case *slackevents.MessageEvent:
		log.Println("[MessageEvent]")

		if !verifyMessageEvent(*ev) {
			return
		}

		log.Printf("Message text: %v\n", ev.Text)
		for _, row := range strings.Split(ev.Text, "\n") {
			log.Printf("Processing row: %s\n", row)
			go processText(ev, row)
		}

		log.Println("Finish handling MessageEvent")
		return
	}

	log.Printf("Strange message event %v\n", eventsAPIEvent)
}

// processText Process by custom text instead of entire message
func processText(event *slackevents.MessageEvent, text string) {
	// Get the emoji in the message
	// return if no exact emoji found
	numOfEmojiMatches := findNumOfEmojiIn(text)
	if numOfEmojiMatches == 0 {
		log.Printf("No emoji %v found in message %v. Return.\n", emojiName, text)
		return
	}

	// Find the receiver
	receiverID := findReceiverIDIn(text)

	if receiverID == "" {
		log.Printf("No receiver found. Return.\n")
		return
	}

	// Get receiver information
	receiver, err := api.GetUserInfo(receiverID)
	if err != nil {
		log.Panicf("Error getting receiver %v info %v\n", receiverID, err)
		return
	}
	printUserInfo(receiver)

	if receiver.IsBot {
		log.Printf("Receiver %v is bot. Return.\n", receiver.Profile.RealName)
		go reactToSlackMessage(event.Channel, event.TimeStamp, emojiX)
		return
	}

	// Get user who posted the message
	// return if error
	user, err := api.GetUserInfo(event.User)
	if err != nil {
		log.Panicf("Error getting user %v info %v\n", event.User, err)
		return
	}
	printUserInfo(user)

	// Won't accept users giving for themself
	if user.ID == receiverID {
		log.Printf("UserID = receiverID = %v. Return.\n", user.ID)
		go reactToSlackMessage(event.Channel, event.TimeStamp, emojiPray)
		return
	}

	go restrictNumOfEmojiCanBeGivenToday(event, user, receiver, numOfEmojiMatches)
	return
}

func getLeaderboard(from Date, to Date) PairList {
	log.Printf("From: %v, to %v\n", from, to)
	receivingSummaries := readFrom(googleSheetReceivingSummaryReadRange)
	leaderboard := map[string]int{}

	for _, row := range receivingSummaries {
		// Skip Grand Total row
		if strings.Contains(fmt.Sprintf("%s %s %s %s", row[0], row[1], row[2], row[3]), "Grand Total") {
			log.Println("Grand Total row. Skip.")
			break
		}

		receivingSummary := givingSummary{row[0].(string), fmt.Sprintf("%s %s", row[1], row[2]), row[3].(string)}
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

			leaderboard[receivingSummary.Name] = total + leaderboard[receivingSummary.Name]
			log.Printf("Leaderboard: %v\n", leaderboard)
		}
	}

	log.Printf("Leaderboard: %v\n", leaderboard)
	return rank(leaderboard)
}

// printUserInfo Print Slack user information
func printUserInfo(user *slack.User) {
	log.Printf("ID: %v, Fullname: %v, Email: %v\n", user.ID, user.Profile.RealName, user.Profile.Email)
}

// restrictNumOfEmojiCanBeGivenToday Check number of emoji user can give today
func restrictNumOfEmojiCanBeGivenToday(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, wantToGive int) {
	userRealName := user.Profile.RealName
	givingSummaries := readFrom(googleSheetGivingSummaryReadRange)
	// Is today record for user found?
	// (did user give anything today?)
	todayRecordFound := false

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

		// todayInVietnam record
		if userRealName == givingSummary.Name && givingSummary.Date == todayInVietnam {
			log.Printf("Today record for user %v found.\n", userRealName)
			todayRecordFound = true

			givenToday, err := strconv.Atoi(givingSummary.Total)
			if err != nil {
				log.Panicf("%v can not be convert to int\n", givingSummary.Total)
				return
			}

			if givenToday >= maxEveryday {
				log.Printf("User %s already gave %d today (maximum allowed: %d). Return.\n", givingSummary.Name, givenToday, maxEveryday)
				go reactToSlackMessage(event.Channel, event.TimeStamp, emojiNoGood)
				return
			}

			canBeGivenToday := maxEveryday - givenToday
			var toGive int

			if canBeGivenToday >= wantToGive {
				toGive = wantToGive
			} else {
				toGive = canBeGivenToday
			}

			log.Printf("Can be given today: %d, maximum to give everyday: %d,"+
				" user has given today: %d, want to give now: %d\n",
				canBeGivenToday, maxEveryday, givenToday, wantToGive)

			if toGive > 0 {
				recordGiving(event, user, receiver, toGive)
			} else {
				go reactToSlackMessage(event.Channel, event.TimeStamp, emojiX)
			}
		}
	}

	// No record today for user
	log.Printf("No record found today %v for user %v. Let he/she give at most %v.\n", todayInVietnam, userRealName, maxEveryday)
	if !todayRecordFound {
		if wantToGive >= maxEveryday {
			wantToGive = maxEveryday
		}

		recordGiving(event, user, receiver, wantToGive)
	}
}

// recordGiving Record giving for user
// write to Google Sheets and react to Slack message
func recordGiving(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, toGive int) {
	log.Printf("Record giving now for user %v, receiver %v, number %v\n", user, receiver, toGive)
	go writeToGoogleSheets(event, user, receiver, toGive)
	emoji := getNumberEmoji(toGive)

	for _, e := range emoji {
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
	var giverName = user.Profile.RealName
	var receiverName = receiver.Profile.RealName
	var message = event.Text
	valueToWrite := []interface{}{timestamp, datetime, giverName, receiverName, toGive, message}
	log.Printf("Value to write %v\n", valueToWrite)

	go appendValue(valueToWrite)
}

// findReceiverIDIn Find id of the receiver in text message
func findReceiverIDIn(text string) string {
	// Slack user format: <@USER_ID>
	receivers := compiledSlackUserIDPattern.FindAllString(text, -1)
	log.Printf("Matched receivers %v\n", receivers)

	// Return empty if no receiver found
	if len(receivers) == 0 {
		return ""
	}

	// Only the first mention count as receiver
	var receiverRaw = receivers[0]
	return receiverRaw[2 : len(receiverRaw)-1]
}

// findNumOfEmojiIn Find number of emoji emojiName appeared in text message
func findNumOfEmojiIn(text string) int {
	matchedEmoji := compiledSlackEmojiPattern.FindAllString(text, -1)
	log.Printf("Matched emoji %v in text %v\n", matchedEmoji, text)
	return len(matchedEmoji)
}

// responseToSlackChallenge Response to Slack's URL verification challenge
func responseToSlackChallenge(body string, w http.ResponseWriter) {
	log.Println("[Slack URL Verification challenge event]")
	var r *slackevents.ChallengeResponse
	err := json.Unmarshal([]byte(body), &r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatalf("Unable to unmarshal slack URL verification challenge. Error %v\n", err)
	}

	w.Header().Set("Content-Type", "text")
	numOfWrittenBytes, err := w.Write([]byte(r.Challenge))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatalf("Unable to write response challenge with error %v\n", err)
	}
	log.Printf("%v bytes of Slack challenge response written\n", numOfWrittenBytes)
}

// verifyMessageEvent Check whether the message event is valid for processing
func verifyMessageEvent(ev slackevents.MessageEvent) bool {
	if ev.SubType != "" {
		log.Printf("Event with subtype %v. Return.\n", ev.SubType)
		return false
	}

	// TODO: Maybe handle edited messages someday ;)
	if ev.IsEdited() {
		log.Printf("Edited message. Return.\n")
		return false
	}

	if len(ev.Text) < emojiNameLength {
		log.Printf("Message too short. Return.\n")
		return false
	}

	return true
}

// postSlackMessage Post message to Slack
func postSlackMessage(channel string, text string) {
	var msgOptionText = slack.MsgOptionText(text, true)
	respChannel, respTimestamp, err := api.PostMessage(channel, msgOptionText)
	if err != nil {
		log.Printf("Unable to post message to Slack with error %v\n", err)
		return
	}
	log.Printf("Message posted to channel %v at %v\n", respChannel, respTimestamp)
}

// reactToSlackMessage React to Slack message
func reactToSlackMessage(channel string, timestamp string, emoji string) {
	refToMessage := slack.NewRefToMessage(channel, timestamp)
	err := api.AddReaction(emoji, refToMessage)
	if err != nil {
		log.Printf("Unable to react %v to comment %v with error %v\n", emoji, refToMessage, err)
		return
	}
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
