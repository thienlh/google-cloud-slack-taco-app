package p

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
)

// mainEmoji name of the emoji to give
var mainEmoji = fmt.Sprintf(":%s:", os.Getenv("EMOJI_NAME"))

// dateTimeFormat Datetime format sent from Slack
const dateTimeFormat = "01/02/2006 15:04:05"

// userIDPattern Compile Slack user id pattern first for better performance
var userIDPattern = regexp.MustCompile(`<@[\w]*>`)

// mainEmojiPattern Compile Slack main emoji pattern first for better performance
var mainEmojiPattern = regexp.MustCompile(strings.Replace(fmt.Sprintf("(%s){1}", mainEmoji), "-", "\\-", -1))

// dayLimit Maximum number of emoji can be given everyday by each user
var dayLimit, _ = strconv.Atoi(os.Getenv("MAX_EVERYDAY"))

// location Location name Vietnam
const location = "Vietnam"

var greetingMessage = fmt.Sprintf("Chào anh chị em e-pilot :thuan: :mama-thuy: :tung: Xem BXH tại %s", spreadsheetURL)

const noRecordMessage = "No record found! :quy-serious:"
const invalidCommandMessage = "Invalid Command. Available commands are: ```help\nchart\nchart day\nchart week\nchart sprint\nchart month\nchart year```"

const resultMessageFormat = "Result from %v to %v:\n%s"

// sprintStart A start date of the sprint with layout dd MM yyyy
var sprintStart, _ = time.Parse("02 01 2006", os.Getenv("SPRINT_START_DATE"))

// sprintDuration Sprint Duration in day
var sprintDuration, _ = strconv.Atoi(os.Getenv("SPRINT_DURATION"))

var client = slack.New(os.Getenv("SLACK_TOKEN"))

type Emoji string

const (
	NotAllow Emoji = "x"
	Pray     Emoji = "pray"
	NoGood   Emoji = "no_good"
)

type Duration string

const (
	Day    Duration = "Duration"
	Week            = "Week"
	Sprint          = "Sprint"
	Month           = "Month"
	Year            = "Year"
)

type Command string

const (
	Help  Command = "help"
	Chart         = "chart"
)

// handleCallbackEvent Handle Callback events from Slack
func handleCallbackEvent(event slackevents.EventsAPIEvent) {
	log.Println("Callback event")
	switch event := event.InnerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Printf("AppMentionEvent %v\n", event)
		handleAppMention(event)
		return
	case *slackevents.MessageEvent:
		log.Printf("MessageEvent %v\n", event)
		handleMessage(event)
		return
	default:
		log.Printf("Strange message event %v\n", event)
		return
	}
}

func handleAppMention(event *slackevents.AppMentionEvent) {
	// Trim the mention part
	// format: <@app_id> which contains 12 characters
	text := strings.ToLower(strings.TrimSpace(event.Text[12:]))
	// @app help
	if Command(text) == Help || text == "" {
		go post(event.Channel, greetingMessage)
		return
	}
	// @app chart <day> (default)
	// @app chart week
	// @app chart sprint
	// @app chart month
	if strings.HasPrefix(text, Chart) {
		from, to, err := calculateRangeFrom(text)
		if err {
			return
		}
		records := getRecords(from, to)
		if len(records) > 0 {
			go post(event.Channel, fmt.Sprintf(resultMessageFormat, from, to, records.String()))
		} else {
			go post(event.Channel, noRecordMessage)
		}
		return
	}

	log.Println("Strange App Mention Event")
	go post(event.Channel, invalidCommandMessage)
}

//	calculateRangeFrom Calculate the range from duration text
func calculateRangeFrom(text string) (Date, Date, bool) {
	year, month, day := timeIn(location, time.Now()).Date()
	today := Date{year, month, day}
	var from Date
	to := today

	duration := durationFrom(text)
	switch duration {
	case Day:
		from = today
		break
	case Week:
		date := timeIn(location, time.Now())
		// Iterate back to first day of the week, assuming it's Monday
		for date.Weekday() != time.Monday {
			date = date.AddDate(0, 0, -1)
		}
		from = Date{date.Year(), date.Month(), date.Day()}
		break
	case Sprint:
		sprintEnd := sprintStart.AddDate(0, 0, sprintDuration).Add(-1)
		duringSprint := timeIn(location, time.Now()).After(sprintStart) && timeIn(location, time.Now()).Before(sprintEnd)
		for !duringSprint {
			sprintStart = sprintEnd.Add(1)
			sprintEnd = sprintStart.AddDate(0, 0, sprintDuration).Add(-1)
		}
		from = Date{sprintStart.Year(), sprintStart.Month(), sprintStart.Day()}
		break
	case Month:
		from = Date{year, month, 1}
		break
	case Year:
		from = Date{year, 1, 1}
	default:
		log.Printf("Strange Duration %s!", duration)
		return Date{}, Date{}, true
	}
	return from, to, false
}

func durationFrom(text string) Duration {
	result := Day
	slices := strings.Split(text, " ")
	if len(slices) == 2 {
		result = Duration(slices[1])
	}
	log.Printf("Param: %v\n", result)
	return result
}

func handleMessage(messageEvent *slackevents.MessageEvent) {
	if !verifyMessageEvent(messageEvent) {
		return
	}
	log.Printf("Message text: %v\n", messageEvent.Text)
	//	Line by line
	for _, row := range strings.Split(messageEvent.Text, "\n") {
		log.Printf("Processing row: %s\n", row)
		go processMessageText(messageEvent, row)
	}
	log.Println("Finish handling MessageEvent")
}

// processMessageText Process by custom text instead of entire message
func processMessageText(event *slackevents.MessageEvent, text string) {
	numEmoji := len(mainEmojiPattern.FindAllString(text, -1))
	log.Printf("Matched emoji %v in text %v\n", numEmoji, text)
	if numEmoji == 0 {
		log.Printf("No emoji %v found in message %v. Return.\n", mainEmoji, text)
		return
	}

	// Find the receiver
	receiverID := findFirstUserIdIn(text)
	if receiverID == "" {
		log.Printf("No receiver found. Return.\n")
		return
	}
	receiver, err := client.GetUserInfo(receiverID)
	if err != nil {
		log.Panicf("Error getting receiver %v info %v\n", receiverID, err)
		return
	}
	printUserInfo(receiver)

	//	Human only, bitch!
	if receiver.IsBot {
		log.Printf("Receiver %v is bot. Return.\n", receiver.Profile.RealName)
		go react(event.Channel, event.TimeStamp, string(NotAllow))
		return
	}

	// Find the giver who posted the message
	giver, err := client.GetUserInfo(event.User)
	if err != nil {
		log.Panicf("Error getting giver %v info %v\n", event.User, err)
		return
	}
	printUserInfo(giver)

	// Won't accept users giving for themself
	if giver.ID == receiver.ID {
		log.Printf("User with id %v is self-giving. Return.\n", giver.ID)
		go react(event.Channel, event.TimeStamp, string(Pray))
		return
	}

	go give(event, giver, receiver, numEmoji)
	return
}

// printUserInfo Print Slack user information
func printUserInfo(user *slack.User) {
	log.Printf("Slack User { ID: %v, Fullname: %v, Email: %v }\n", user.ID, user.Profile.RealName, user.Profile.Email)
}

// findFirstUserIdIn Find first user id in text message
func findFirstUserIdIn(text string) string {
	ids := userIDPattern.FindAllString(text, -1)
	log.Printf("Matched ids %v\n", ids)
	if len(ids) == 0 {
		return ""
	}
	var firstUser = ids[0]
	// Slack user format: <@USER_ID>
	return firstUser[2 : len(firstUser)-1]
}

// handleURLVerificationEvent Response to Slack's URL verification challenge
func handleURLVerificationEvent(body string, w http.ResponseWriter) {
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
	//	Must at least contains <@USER_ID><space>:<mainEmoji>:<space>
	if len(event.Text) < len(mainEmoji)+16 {
		log.Printf("Message too short. Return.\n")
		return false
	}
	return true
}

// post Post message to Slack
func post(channel string, text string) {
	var msgOptionText = slack.MsgOptionText(text, true)
	respChannel, respTimestamp, err := client.PostMessage(channel, msgOptionText)
	if err != nil {
		log.Printf("Unable to post message to Slack with error %v\n", err)
		return
	}
	log.Printf("Message posted to channel %v at %v\n", respChannel, respTimestamp)
}

// react React to Slack message
func react(channel string, timestamp string, emoji string) {
	refToMessage := slack.NewRefToMessage(channel, timestamp)
	err := client.AddReaction(emoji, refToMessage)
	if err != nil {
		log.Printf("Unable to react %v to comment %v with error %v\n", emoji, refToMessage, err)
		return
	}
	log.Printf("Reacted %v to message with timestamp %v in channel %v\n", emoji, timestamp, channel)
}

func give(event *slackevents.MessageEvent, giver *slack.User, receiver *slack.User, numToGive int) {
	giverRealName := giver.Profile.RealName
	givingSummaries := readRow(givingSummaryReadRange)
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
		log.Printf("Giving summary: %v, today: %v\n", givingSummary, today)
		//	TODO: Use user id instead of real name since real name can be changed
		givenToday := giverRealName == givingSummary.Name && givingSummary.Date == today
		if givenToday {
			log.Printf("Today record for user %v found.\n", giverRealName)
			numGivenToday, err := strconv.Atoi(givingSummary.Total)
			if err != nil {
				log.Panicf("%v can not be convert to int\n", givingSummary.Total)
				return
			}
			if numGivenToday >= dayLimit {
				log.Printf("User %s already gave %d today (maximum allowed: %d). Return.\n", givingSummary.Name, numGivenToday, dayLimit)
				go react(event.Channel, event.TimeStamp, string(NoGood))
				return
			}
			remainingToGiveToday := dayLimit - numGivenToday
			if numToGive > remainingToGiveToday {
				numToGive = remainingToGiveToday
			}
			log.Printf("Can be given today: %d, maximum to give per day: %d,"+
				" user has given today: %d, want to give now: %d\n",
				remainingToGiveToday, dayLimit, numGivenToday, numToGive)
			if numToGive > 0 {
				record(event, giver, receiver, numToGive)
			} else {
				go react(event.Channel, event.TimeStamp, string(NotAllow))
			}
			return
		}
	}
	// Haven't give today
	log.Printf("No record found today %v for user %v. Let he/she give at most %v.\n", today, giverRealName, dayLimit)
	if numToGive >= dayLimit {
		numToGive = dayLimit
	}
	record(event, giver, receiver, numToGive)
}

// record Record giving for  giver
func record(event *slackevents.MessageEvent, giver *slack.User, receiver *slack.User, numToGive int) {
	log.Printf("Record giving now for user %v, receiver %v, number %v\n", giver, receiver, numToGive)
	write(event.Text, event.TimeStamp, giver, receiver, numToGive)
	emoji := getNumberEmoji(numToGive)
	for _, e := range emoji {
		go react(event.Channel, event.TimeStamp, e)
	}
}

// write Write value to Google Sheets
func write(message string, timestamp string, giver *slack.User, receiver *slack.User, toGive int) {
	go appendRow(prepareRecord(timestamp, giver, receiver, toGive, message))
}

func prepareRecord(strTimestamp string, giver *slack.User, receiver *slack.User, toGive int, message string) []interface{} {
	// Timestamp, Giver, Receiver, Quantity, Text, Date timestamp
	// Format from Slack: 1547921475.007300
	var timestamp = timeIn(location, toDate(strings.Split(strTimestamp, ".")[0]))
	// Using Google Sheets recognizable format
	var datetime = timestamp.Format(dateTimeFormat)
	var giverRealName = giver.Profile.RealName
	var receiverRealName = receiver.Profile.RealName
	row := []interface{}{timestamp, datetime, giverRealName, receiverRealName, toGive, message}
	log.Printf("Value to write %v\n", row)
	return row
}

func parseEvent(body string, w http.ResponseWriter) bool {
	event, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: os.Getenv("VERIFICATION_TOKEN")}))
	if err != nil {
		return false
	}
	log.Printf("Event: %v\n", event)
	switch event.Type {
	case slackevents.URLVerification:
		handleURLVerificationEvent(body, w)
		break
	case slackevents.CallbackEvent:
		handleCallbackEvent(event)
		break
	}
	return true
}
