// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/nlopes/slack/slackevents"

	"github.com/nlopes/slack"
)

// SlackToken Authentication token from slack
var SlackToken = os.Getenv("SLACK_TOKEN")

// SlackVerificationToken Verification token from slack
var SlackVerificationToken = os.Getenv("VERIFICATION_TOKEN")

var api = slack.New(SlackToken)

var parameters slack.PostMessageParameters

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
			fmt.Printf("AppMentionEvent")
			var parameters slack.PostMessageParameters
			api.PostMessage(ev.Channel, "Yes, hello.", parameters)
		case *slackevents.MessageEvent:
			fmt.Printf("MessageEvent")

			if ev.SubType == "bot_message" {
				fmt.Printf("Bot message. Return.")
				return
			}

			user, err := api.GetUserInfo(ev.User)
			if err != nil {
				fmt.Printf("Error getting user %s info %s\n", ev.User, err)
				return
			}
			fmt.Printf("ID: %s, Fullname: %s, Email: %s\n", user.ID, user.Profile.RealName, user.Profile.Email)

			//	Get the emoji
			r := regexp.MustCompile(`:[thac\-mo]*:`)
			matchedEmojies := r.FindAllString(ev.Text, -1)
			var numOfMatches = len(matchedEmojies)
			fmt.Printf("%d matched :thac-mo: found", numOfMatches)

			// Get the receiver
			rMentioned := regexp.MustCompile(`<@[\w]*>`)
			receivers := rMentioned.FindAllString(ev.Text, -1)
			fmt.Printf("Matched receivers %s", receivers)

			if len(receivers) == 0 {
				fmt.Printf("No receiver. Return.")
				return
			}

			var receiverRaw = receivers[0]
			var receiverID = receiverRaw[2 : len(receiverRaw)-1]

			if user.ID == receiverID {
				fmt.Printf("UserID = receiverID = %s", user.ID)
				api.PostMessage(ev.Channel, "Come on! It wouldn't be fair if you can give yourself :thac-mo:!", parameters)
				return
			}

			receiver, err := api.GetUserInfo(receiverID)
			if err != nil {
				fmt.Printf("Error getting receiver %s info %s\n", ev.User, err)
				return
			}
			fmt.Printf("ID: %s, Fullname: %s, Email: %s\n", receiver.ID, receiver.Profile.RealName, receiver.Profile.Email)

			api.PostMessage(ev.Channel, fmt.Sprintf("<@%s> has received %d :thac-mo: from <@%s>", receiver.ID, numOfMatches, user.Profile.ID), parameters)
		}
	}
}
