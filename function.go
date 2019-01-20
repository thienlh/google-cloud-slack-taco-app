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
var EmojiNameLength = len(EmojiName)

var MaxEveryday, _ = strconv.Atoi(os.Getenv("MAX_EVERYDAY"))
var Today = timeIn("Vietnam", time.Now()).Format("02-Jan 2006")

//	API Slack API
var API = slack.New(SlackToken)

//	SlackPostMessageParameters Just a dummy variable so that Slack won't complaint
var SlackPostMessageParameters slack.PostMessageParameters

const AppMentionResponseMessage = "Chào anh chị em e-pilot :thuan: :mama-thuy:"
const SelfGivingResponseMessagePattern = "Bạn không thể tự tặng bản thân %s!"
const ReceivedResponseMessagePattern = "<@%s> đã nhận được %d %s từ <@%s>"
const GivingToBotResponseMessagePattern = "Bạn không thể cho bot %s!"

type GivingSummary struct {
	Name  string
	Date  string
	Total string
}

//	Handle handle every requests from Slack
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
	log.Printf("Header: %v", r.Header)

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: SlackVerificationToken}))
	if err != nil {
		log.Printf("Unable to parse event. Error %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Event: %v\n", eventsAPIEvent)

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		responseToSlackChallenge(body, w)
	case slackevents.CallbackEvent:
		handleCallbackEvent(eventsAPIEvent)
	}

	log.Printf("Finish")
	w.WriteHeader(http.StatusOK)
	return
}

//	handleCallbackEvent Handle Callback events from Slack
func handleCallbackEvent(eventsAPIEvent slackevents.EventsAPIEvent) {
	log.Printf("A message found %v\n", eventsAPIEvent)
	innerEvent := eventsAPIEvent.InnerEvent
	log.Printf("Inner event %v\n", innerEvent)

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
			log.Printf("No %v found in message %v. Return.\n", EmojiName, ev.Text)
			return
		}
		//	Never insert more than 5 at once
		if numOfEmojiMatches > 5 {
			numOfEmojiMatches = 5
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
			log.Printf("Error getting receiver %v info %v\n", ev.User, err)
			return
		}
		log.Printf("ID: %v, Fullname: %v, Email: %v\n", receiver.ID, receiver.Profile.RealName, receiver.Profile.Email)

		if receiver.IsBot {
			log.Printf("Receiver %v is bot. Return.\n", receiver.Profile.RealName)
			go reactToSlackMessage(ev.Channel, ev.TimeStamp, "x")
			return
		}

		//	Get user who posted the message
		//	return if error
		user, err := API.GetUserInfo(ev.User)
		if err != nil {
			log.Printf("Error getting user %v info %v\n", ev.User, err)
			return
		}
		log.Printf("ID: %v, Fullname: %v, Email: %v\n", user.ID, user.Profile.RealName, user.Profile.Email)

		//	Won't accept users giving for themself
		if user.ID == receiverID {
			log.Printf("UserID = receiverID = %v\n", user.ID)
			go reactToSlackMessage(ev.Channel, ev.TimeStamp, "pray")
			return
		}

		go restrictNumOfEmojiToday(ev, user, receiver, numOfEmojiMatches)
		go reactToSlackMessage(ev.Channel, ev.TimeStamp, "kiss")
		return
	}

	log.Printf("Strange message event %v", eventsAPIEvent)
}

func restrictNumOfEmojiToday(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, numOfEmojiMatches int) {
	summaries := readFrom("Pivot Table 1!A3:D")

	for _, row := range summaries {
		//	Skip Grand Total row
		if strings.Contains(fmt.Sprintf("%s %s %s %s", row[0], row[1], row[2], row[3]), "Grand Total") {
			log.Printf("Row %v contains Grand Total. Return.\n", row)
			break
		}

		giver := GivingSummary{row[0].(string), fmt.Sprintf("%s %s", row[1], row[2]), row[3].(string)}
		log.Printf("Giver: %v, today: %v\n", giver, Today)

		if user.Name == giver.Name && giver.Date == Today {
			log.Printf("Today record for user %v found.", user.Name)

			total, err := strconv.Atoi(giver.Total)
			if err != nil {
				log.Panicf("%v can not be convert to int\n", giver.Total)
				return
			}

			if total >= MaxEveryday {
				log.Printf("User %s already gave %d today (maximum allowed: %d). Return.\n", giver.Name, total, MaxEveryday)
				return
			}

			canBeGivenToday := MaxEveryday - total
			log.Printf("Can be given today: %d, MaxEveryday: %d,"+
				" total given today: %d, number of emoji matched: %d\n",
				canBeGivenToday, MaxEveryday, total, numOfEmojiMatches)
			var toGive int

			if canBeGivenToday >= numOfEmojiMatches {
				toGive = numOfEmojiMatches
			}

			if canBeGivenToday < numOfEmojiMatches {
				toGive = canBeGivenToday
			}

			log.Printf("To give: %d\n", toGive)

			go writeToGoogleSheets(event, user, receiver, toGive)
			go reactToSlackMessage(event.Channel, event.TimeStamp, getNumberEmoji(toGive))
		}

	}
}

//	writeToGoogleSheets Write value to Google Sheets using gsheets.go
func writeToGoogleSheets(event *slackevents.MessageEvent, user *slack.User, receiver *slack.User, toGive int) {
	//	Timestamp, Giver, Receiver, Quantity, Text, Date time
	//	format from Slack: 1547921475.007300
	var timestamp = timeIn("Vietnam", toDate(strings.Split(event.TimeStamp, ".")[0]))
	//	Using Google Sheets recognizable format
	var datetime = timestamp.Format("01/02/2006 15:04:05")
	var giverName = user.Profile.RealName
	var receiverName = receiver.Profile.RealName
	var quantity = toGive
	var message = event.Text
	valueToWrite := []interface{}{timestamp, datetime, giverName, receiverName, quantity, message}
	log.Printf("Value to write %v", valueToWrite)

	write(valueToWrite)
}

//	findReceiverIDIn Find id of the receiver in text message
func findReceiverIDIn(text string) string {
	rMentioned := regexp.MustCompile(`<@[\w]*>`)
	//	Only the first mention count as receiver
	//	Slack user format: <@USER_ID>
	receivers := rMentioned.FindAllString(text, -1)

	//	Return empty if no receiver found
	if len(receivers) == 0 {
		return ""
	}

	log.Printf("Matched receivers %v\n", receivers)
	var receiverRaw = receivers[0]
	var receiverID = receiverRaw[2 : len(receiverRaw)-1]

	return receiverID
}

//	findNumOfEmojiIn Find number of emoji EmojiName appeared in text message
func findNumOfEmojiIn(text string) int {
	emojiRegEx := strings.Replace(fmt.Sprintf("(%s){1}", EmojiName), "-", "\\-", -1)
	r := regexp.MustCompile(emojiRegEx)
	matchedEmoji := r.FindAllString(text, -1)
	var numOfMatches = len(matchedEmoji)
	log.Printf("%d matched %v found\n", numOfMatches, EmojiName)
	return numOfMatches
}

//	responseToSlackChallenge Response to Slack's URL verification challenge
func responseToSlackChallenge(body string, w http.ResponseWriter) {
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

//	getNumberEmoji Return number of given emoji
//	if < 1 then return ""
//	if 1 < number <= max then return emoji name
//	otherwise return emoji name for max value
func getNumberEmoji(number int) string {
	if number < 1 {
		return ""
	}
	switch number {
	case 1:
		return "one"
	case 2:
		return "two"
	case 3:
		return "three"
	case 4:
		return "four"
	default:
		return "five"
	}
}
