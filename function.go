// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/nlopes/slack/slackevents"

	"github.com/nlopes/slack"
)

// SlackToken Authentication token from slack
var SlackToken = os.Getenv("SLACK_TOKEN")

// SlackVerificationToken Verification token from slack
var SlackVerificationToken = os.Getenv("VERIFICATION_TOKEN")

// EmojiName name of the emoji to use
var EmojiName = fmt.Sprintf(":%s:", os.Getenv("EMOJI_NAME"))

// Api Slack API
var Api = slack.New(SlackToken)

var parameters slack.PostMessageParameters

const EventSubtypeBotMessage = "bot_message"

// HelloWorld prints the JSON encoded "message" field in the body
// of the request or "Hello, World!" if there isn't one.
func HelloWorld(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body := buf.String()
	fmt.Printf("Body=%s\n", body)

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: SlackVerificationToken}))
	if err != nil {
		fmt.Printf("Unable to parse event. Error %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	fmt.Printf("Event=%s\n", eventsAPIEvent)

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			fmt.Printf("Unable to unmarshal slack URL verification challenge. Error %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
	}

	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		fmt.Printf("A message found %s\n", eventsAPIEvent)
		innerEvent := eventsAPIEvent.InnerEvent

		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			fmt.Println("AppMentionEvent")
			var parameters slack.PostMessageParameters
			Api.PostMessage(ev.Channel, "Yes, hello.", parameters)
		case *slackevents.MessageEvent:
			fmt.Println("MessageEvent")

			if ev.SubType == EventSubtypeBotMessage {
				fmt.Printf("Bot message. Return.\n")
				return
			}

			if ev.IsEdited() {
				fmt.Printf("Edited message. Return.\n")
				return
			}

			if ev.PreviousMessage.TimeStamp == ev.TimeStamp {
				fmt.Printf("Message with the same timestamp as previous message. Maybe a duplicate. Return.\n")
				return
			}

			user, err := Api.GetUserInfo(ev.User)
			if err != nil {
				fmt.Printf("Error getting user %s info %s\n", ev.User, err)
				return
			}
			fmt.Printf("ID: %s, Fullname: %s, Email: %s\n", user.ID, user.Profile.RealName, user.Profile.Email)

			//	Get the emoji
			emojiRegEx := strings.Replace(fmt.Sprintf("(%s){1}", EmojiName), "-", "\\-", -1)
			r := regexp.MustCompile(emojiRegEx)
			matchedEmoji := r.FindAllString(ev.Text, -1)
			var numOfMatches = len(matchedEmoji)
			fmt.Printf("%d matched %s found\n", numOfMatches, EmojiName)

			if numOfMatches == 0 {
				fmt.Printf("No %s found. Return.\n", EmojiName)
				return
			}

			// Get the receiver
			rMentioned := regexp.MustCompile(`<@[\w]*>`)
			receivers := rMentioned.FindAllString(ev.Text, -1)
			fmt.Printf("Matched receivers %s\n", receivers)

			if len(receivers) == 0 {
				fmt.Printf("No receiver. Return.\n")
				return
			}

			var receiverRaw = receivers[0]
			var receiverID = receiverRaw[2 : len(receiverRaw)-1]

			if user.ID == receiverID {
				fmt.Printf("UserID = receiverID = %s\n", user.ID)
				Api.PostMessage(ev.Channel, fmt.Sprintf("Come on! It wouldn't be fair if you can give yourself %s!", EmojiName), parameters)
				return
			}

			receiver, err := Api.GetUserInfo(receiverID)
			if err != nil {
				fmt.Printf("Error getting receiver %s info %s\n", ev.User, err)
				return
			}
			fmt.Printf("ID: %s, Fullname: %s, Email: %s\n", receiver.ID, receiver.Profile.RealName, receiver.Profile.Email)

			// TODO: Uncomment this
			//if receiver.IsBot {
			//	fmt.Printf("Receiver %s is bot. Return.\n", receiver.Profile.RealName)
			//	Api.PostMessage(ev.Channel, fmt.Sprintf("You can not give bot %s!", EmojiName), parameters)
			//	return
			//}

			//	Timestamp, Giver, Receiver, Quantity
			var time = toDate(ev.TimeStamp)
			var giverName = user.Profile.RealName
			var receiverName = receiver.Profile.RealName
			var quantity = numOfMatches
			var message = ev.Text

			valueToWrite := []interface{}{time, giverName, receiverName, quantity, message}
			write(valueToWrite)

			Api.PostMessage(ev.Channel, fmt.Sprintf("<@%s> has received %d %s from <@%s>", receiver.ID, numOfMatches, EmojiName, user.ID), parameters)
		}
	}
}
