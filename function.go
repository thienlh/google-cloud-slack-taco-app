// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/nlopes/slack/slackevents"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nlopes/slack"
)

//	SlackToken Authentication token from slack
var SlackToken = os.Getenv("SLACK_TOKEN")

//	SlackVerificationToken Verification token from slack
var SlackVerificationToken = os.Getenv("VERIFICATION_TOKEN")

//	EmojiName name of the emoji to use
var EmojiName = fmt.Sprintf(":%s:", os.Getenv("EMOJI_NAME"))

//	EmojiNameLength Length of emoji name (for quickly rejecting too short messages
var EmojiNameLength = len(EmojiName)

//	MaxEveryday Maximum number of emoji can be given everyday by each user
var MaxEveryday, _ = strconv.Atoi(os.Getenv("MAX_EVERYDAY"))

//	LocationVietnam Location name Vietnam
const LocationVietnam = "Vietnam"

//	SlackDateTimeFormat Datetime format sent from Slack
const SlackDateTimeFormat = "01/02/2006 15:04:05"

//	Today Today in Vietnam time in special format '02-Jan 2006'
var Today = timeIn("Vietnam", time.Now()).Format("02-Jan 2006")

//	CompiledSlackUserIdPattern Compile Slack user id pattern first for better performance
var CompiledSlackUserIdPattern = regexp.MustCompile(`<@[\w]*>`)

//	CompiledSlackEmojiPattern Compile Slack user id pattern first for better performance
var CompiledSlackEmojiPattern = regexp.MustCompile(strings.Replace(fmt.Sprintf("(%s){1}", EmojiName), "-", "\\-", -1))

//	API Slack API
var API = slack.New(SlackToken)

//	SlackPostMessageParameters Just a dummy variable so that Slack won't complaint
var SlackPostMessageParameters slack.PostMessageParameters

//	AppMentionResponseMessage Message responding to app mention events
var AppMentionResponseMessage = fmt.Sprintf("Chào anh chị em e-pilot :thuan: :mama-thuy: :tung: Xem BXH tại %s", SpreadsheetURL)

//	Slack emoji for responding to events
//	EmojiX Response to event giving_to_bot
const EmojiX = "x"

//	EmojiPray Response to event giving_to_him_herself
const EmojiPray = "pray"

//	EmojiNoGood Response to event user_has_given_maximum_today
const EmojiNoGood = "no_good"

//	SheetGivingSummaryReadRange Read range for the giving summary sheet
const SheetGivingSummaryReadRange = "Pivot Table 1!A3:D"

//	GivingSummary model represents each Giving Summary row in Google Sheets
type GivingSummary struct {
	Name  string
	Date  string
	Total string
}

//	Handle handle every requests
func Handle(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		log.Fatal("Error reading buffer from body.")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body := buf.String()
	log.Printf("Body: %v\n", body)
	log.Printf("Header: %v\n", r.Header)

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: SlackVerificationToken}))
	if err != nil {
		log.Printf("Unable to parse event. Error %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Event: %v\n", eventsAPIEvent)

	//	Remember to break!
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

//	handleCallbackEvent Handle Callback events from Slack
func handleCallbackEvent(eventsAPIEvent slackevents.EventsAPIEvent) {
	log.Println("[Callback event]")
	innerEvent := eventsAPIEvent.InnerEvent
	log.Printf("Inner event %v\n", innerEvent)

	//	Remember to return!
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Println("[AppMentionEvent]")
		go postSlackMessage(ev.Channel, AppMentionResponseMessage)
		return
	case *slackevents.MessageEvent:
		log.Println("[MessageEvent]")

		if !verifyMessageEvent(*ev) {
			return
		}

		//	Get the emoji in the message
		//	return if no exact emoji found
		numOfEmojiMatches := findNumOfEmojiIn(ev.Text)
		if numOfEmojiMatches == 0 {
			log.Printf("No emoji %v found in message %v. Return.\n", EmojiName, ev.Text)
			return
		}

		// Find the receiver
		receiverID := findReceiverIDIn(ev.Text)

		if receiverID == "" {
			log.Printf("No receiver found. Return.\n")
			return
		}

		//	Get receiver information
		receiver, err := API.GetUserInfo(receiverID)
		if err != nil {
			log.Panicf("Error getting receiver %v info %v\n", ev.User, err)
			return
		}
		printUserInfo(receiver)

		if receiver.IsBot {
			log.Printf("Receiver %v is bot. Return.\n", receiver.Profile.RealName)
			go reactToSlackMessage(ev.Channel, ev.TimeStamp, EmojiX)
			return
		}

		//	Get user who posted the message
		//	return if error
		user, err := API.GetUserInfo(ev.User)
		if err != nil {
			log.Panicf("Error getting user %v info %v\n", ev.User, err)
			return
		}
		printUserInfo(user)

		//	Won't accept users giving for themself
		if user.ID == receiverID {
			log.Printf("UserID = receiverID = %v. Return.\n", user.ID)
			go reactToSlackMessage(ev.Channel, ev.TimeStamp, EmojiPray)
			return
		}

		go restrictNumOfEmojiCanBeGivenToday(ev, user, receiver, numOfEmojiMatches)
		return
	}

	log.Printf("Strange message event %v", eventsAPIEvent)
}

//	printUserInfo Print Slack user information
func printUserInfo(user *slack.User) {
	log.Printf("ID: %v, Fullname: %v, Email: %v\n", user.ID, user.Profile.RealName, user.Profile.Email)
}

//	restrictNumOfEmojiCanBeGivenToday Check number of emoji user can give today
func restrictNumOfEmojiCanBeGivenToday(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, wantToGive int) {
	userRealName := user.Profile.RealName
	givingSummaries := readFrom(SheetGivingSummaryReadRange)
	//	Is today record for user found?
	//	(did user give anything today?)
	todayRecordFound := false

	for _, row := range givingSummaries {
		//	Skip Grand Total row
		if strings.Contains(fmt.Sprintf("%s %s %s %s", row[0], row[1], row[2], row[3]), "Grand Total") {
			log.Println("Grand Total row. Skip.")
			break
		}

		givingSummary := GivingSummary{row[0].(string), fmt.Sprintf("%s %s", row[1], row[2]), row[3].(string)}
		log.Printf("Giving summary: %v, today: %v\n", givingSummary, Today)

		//	Today record
		if userRealName == givingSummary.Name && givingSummary.Date == Today {
			log.Printf("Today record for user %v found.", userRealName)
			todayRecordFound = true

			givenToday, err := strconv.Atoi(givingSummary.Total)
			if err != nil {
				log.Panicf("%v can not be convert to int\n", givingSummary.Total)
				return
			}

			if givenToday >= MaxEveryday {
				log.Printf("User %s already gave %d today (maximum allowed: %d). Return.\n", givingSummary.Name, givenToday, MaxEveryday)
				go reactToSlackMessage(event.Channel, event.TimeStamp, EmojiNoGood)
				return
			}

			canBeGivenToday := MaxEveryday - givenToday
			var toGive int

			if canBeGivenToday >= wantToGive {
				toGive = wantToGive
			} else {
				toGive = canBeGivenToday
			}

			log.Printf("Can be given today: %d, maximum to give everyday: %d,"+
				" user has given today: %d, want to give now: %d\n",
				canBeGivenToday, MaxEveryday, givenToday, wantToGive)

			if toGive > 0 {
				recordGiving(event, user, receiver, toGive)
			} else {
				go reactToSlackMessage(event.Channel, event.TimeStamp, EmojiX)
			}
		}
	}

	//	No record today for user
	log.Printf("No record found today %v for user %v. Let he/she give at most %v.\n", Today, userRealName, MaxEveryday)
	if !todayRecordFound {
		if wantToGive >= MaxEveryday {
			wantToGive = MaxEveryday
		}

		recordGiving(event, user, receiver, wantToGive)
	}
}

//	recordGiving Record giving for user
//	write to Google Sheets and react to Slack message
func recordGiving(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, toGive int) {
	log.Printf("Record giving now for user %v, receiver %v, number %v\n", user, receiver, toGive)
	go writeToGoogleSheets(event, user, receiver, toGive)
	emoji := getNumberEmoji(toGive)

	for _, e := range emoji {
		go reactToSlackMessage(event.Channel, event.TimeStamp, e)
	}
}

//	writeToGoogleSheets Write value to Google Sheets using gsheets.go
func writeToGoogleSheets(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, toGive int) {
	//	Timestamp, Giver, Receiver, Quantity, Text, Date time
	//	Format from Slack: 1547921475.007300
	var timestamp = timeIn(LocationVietnam, toDate(strings.Split(event.TimeStamp, ".")[0]))
	//	Using Google Sheets recognizable format
	var datetime = timestamp.Format(SlackDateTimeFormat)
	var giverName = user.Profile.RealName
	var receiverName = receiver.Profile.RealName
	var message = event.Text
	valueToWrite := []interface{}{timestamp, datetime, giverName, receiverName, toGive, message}
	log.Printf("Value to write %v", valueToWrite)

	go appendValue(valueToWrite)
}

//	findReceiverIDIn Find id of the receiver in text message
func findReceiverIDIn(text string) string {
	//	Slack user format: <@USER_ID>
	receivers := CompiledSlackUserIdPattern.FindAllString(text, -1)
	log.Printf("Matched receivers %v\n", receivers)

	//	Return empty if no receiver found
	if len(receivers) == 0 {
		return ""
	}

	//	Only the first mention count as receiver
	var receiverRaw = receivers[0]
	return receiverRaw[2 : len(receiverRaw)-1]
}

//	findNumOfEmojiIn Find number of emoji EmojiName appeared in text message
func findNumOfEmojiIn(text string) int {
	matchedEmoji := CompiledSlackEmojiPattern.FindAllString(text, -1)
	log.Printf("Matched emoji %v in text %v\n", matchedEmoji, text)
	return len(matchedEmoji)
}

//	responseToSlackChallenge Response to Slack's URL verification challenge
func responseToSlackChallenge(body string, w http.ResponseWriter) {
	log.Println("[Slack URL Verification challenge event]")
	var r *slackevents.ChallengeResponse
	err := json.Unmarshal([]byte(body), &r)
	if err != nil {
		log.Fatalf("Unable to unmarshal slack URL verification challenge. Error %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text")
	numOfWrittenBytes, err := w.Write([]byte(r.Challenge))
	if err != nil {
		log.Fatalf("Unable to write response challenge with error %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	log.Printf("%v bytes of Slack challenge response written\n", numOfWrittenBytes)
}

//	verifyMessageEvent Check whether the message event is valid for processing
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

	if len(ev.Text) < EmojiNameLength {
		log.Printf("Message too short. Return.\n")
		return false
	}

	// TODO: Try to handle duplicate messages
	//if ev.PreviousMessage.TimeStamp == ev.TimeStamp {
	//	log.Printf("Message with the same timestamp as previous message. Maybe a duplicate. Return.\n")
	//	return false
	//}
	return true
}

//	postSlackMessage Post message to Slack
func postSlackMessage(channel string, text string) {
	respChannel, respTimestamp, err := API.PostMessage(channel, text, SlackPostMessageParameters)
	if err != nil {
		log.Printf("Unable to post message to Slack with error %v\n", err)
		return
	}
	log.Printf("Message posted to channel %v at %v\n", respChannel, respTimestamp)
}

//	reactToSlackMessage React to Slack message
func reactToSlackMessage(channel string, timestamp string, emoji string) {
	refToMessage := slack.NewRefToMessage(channel, timestamp)
	err := API.AddReaction(emoji, refToMessage)
	if err != nil {
		log.Printf("Unable to react %v to comment %v with error %v", emoji, refToMessage, err)
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

//	getNumberEmoji Return number of given emoji in text
//	character by character
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
