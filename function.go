// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nlopes/slack/slackevents"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
)

//	SlackToken Authentication token from slack
var SlackToken = os.Getenv("SLACK_TOKEN")

//	SlackVerificationToken Verification token from slack
var SlackVerificationToken = os.Getenv("VERIFICATION_TOKEN")

//	EmojiName name of the emoji to use
var EmojiName = fmt.Sprintf(":%s:", os.Getenv("EMOJI_NAME"))

//	API Slack API
var API = slack.New(SlackToken)

//	SlackPostMessageParameters Just a dummy variable so that Slack won't complaint
var SlackPostMessageParameters slack.PostMessageParameters

const AppMentionResponseMessage = "Chào anh chị em e-pilot :thuan: :mama-thuy:"
const SelfGivingResponseMessagePattern = "Bạn không thể tự tặng bản thân %s!"
const ReceivedResponseMessagePattern = "<@%s> đã nhận được %d %s từ <@%s>"
const GivingToBotResponseMessagePattern = "Bạn không thể cho bot %s!"

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
	log.Printf("Body=%s\n", body)

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: SlackVerificationToken}))
	if err != nil {
		log.Printf("Unable to parse event. Error %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Event=%s\n", eventsAPIEvent)

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		responseToSlackChallenge(body, w)
	case slackevents.CallbackEvent:
		err := handleCallbackEvent(eventsAPIEvent)
		if err != nil {
			log.Fatalf("Error handling Slack callback event  with error %v. Return", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Printf("Finish")
		w.WriteHeader(http.StatusOK)
		return
	}
}

//	handleCallbackEvent Handle Callback events from Slack
func handleCallbackEvent(eventsAPIEvent slackevents.EventsAPIEvent) error {
	log.Printf("A message found %s\n", eventsAPIEvent)
	innerEvent := eventsAPIEvent.InnerEvent
	log.Printf("Inner event %s\n", innerEvent)

	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Println("[AppMentionEvent]")
		postSlackMessage(ev.Channel, AppMentionResponseMessage)
	case *slackevents.MessageEvent:
		log.Println("[MessageEvent]")

		if !verifyMessageEvent(*ev) {
			return errors.New("slack: not a valid message to process")
		}

		//	Get user who posted the message
		//	return if error
		user, err := API.GetUserInfo(ev.User)
		if err != nil {
			log.Printf("Error getting user %s info %s\n", ev.User, err)
			return errors.New("slack: unable to get user information")
		}
		log.Printf("ID: %s, Fullname: %s, Email: %s\n", user.ID, user.Profile.RealName, user.Profile.Email)

		//	Get the emoji in the message
		//	return if no exact emoji found
		numOfEmojiMatches := findNumOfEmojiIn(ev.Text)
		if numOfEmojiMatches == 0 {
			log.Fatalf("No %s found in message %s. Return.\n", EmojiName, ev.Text)
			return nil
		}

		// Find the receiver
		receiverID := findReceiverIDIn(ev.Text)

		if receiverID == "" {
			log.Fatalf("No receiver found. Return.\n")
			return nil
		}

		//	Won't accept users giving for themself
		if user.ID == receiverID {
			log.Fatalf("UserID = receiverID = %s\n", user.ID)
			postSlackMessage(ev.Channel, fmt.Sprintf(SelfGivingResponseMessagePattern, EmojiName))
			return nil
		}

		//	Get receiver information
		receiver, err := API.GetUserInfo(receiverID)
		if err != nil {
			log.Fatalf("Error getting receiver %s info %s\n", ev.User, err)
			return errors.New("slack: unable to get receiver information")
		}
		log.Printf("ID: %s, Fullname: %s, Email: %s\n", receiver.ID, receiver.Profile.RealName, receiver.Profile.Email)

		if receiver.IsBot {
			log.Panicf("Receiver %s is bot. Return.\n", receiver.Profile.RealName)
			postSlackMessage(ev.Channel, fmt.Sprintf(GivingToBotResponseMessagePattern, EmojiName))
			return nil
		}

		//	Write to Google sheets and post message
		writeToGoogleSheets(*ev, user, receiver, numOfEmojiMatches)

		refToMessage := slack.NewRefToMessage(ev.Channel, ev.TimeStamp)
		err = API.AddReaction("thuan", refToMessage)
		if err != nil {
			log.Panicf("Unable to react to comment %v with error %v", refToMessage, err)
			return nil
		}
	}

	log.Panicf("Strange message event %v", eventsAPIEvent)
	return errors.New("slack: strange event api event")
}

//	writeToGoogleSheets Write value to Google Sheets using gsheets.go
func writeToGoogleSheets(event slackevents.MessageEvent, user *slack.User, receiver *slack.User, numOfEmojiMatches int) {
	//	Timestamp, Giver, Receiver, Quantity, Text, Date time
	//	format from Slack: 1547921475.007300
	var timestamp = toDate(strings.Split(event.TimeStamp, ".")[0])
	//	Using Google Sheets recognizable format
	var datetime = timestamp.Format("Mon, 02 Jan 2006 15:04:05")
	var giverName = user.Profile.RealName
	var receiverName = receiver.Profile.RealName
	var quantity = numOfEmojiMatches
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

	log.Printf("Matched receivers %s\n", receivers)
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
	log.Printf("%d matched %s found\n", numOfMatches, EmojiName)
	return numOfMatches
}

//	responseToSlackChallenge Response to Slack's URL verification challenge
func responseToSlackChallenge(body string, w http.ResponseWriter) {
	var r *slackevents.ChallengeResponse
	err := json.Unmarshal([]byte(body), &r)
	if err != nil {
		log.Fatalf("Unable to unmarshal slack URL verification challenge. Error %s\n", err)
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
		log.Printf("Event with subtype=%v. Return.\n", ev.SubType)
		return false
	}

	// TODO: Maybe handle edited messages someday ;)
	if ev.IsEdited() {
		log.Printf("Edited message. Return.\n")
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
		log.Panicf("Unable to post message to Slack with error %v\n", err)
	}
	log.Printf("Message posted to channel %s at %s\n", respChannel, respTimestamp)
}
