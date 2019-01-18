// Package p contains an HTTP Cloud Function.
package p

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/nlopes/slack"
)

// SlackToken Authentication token from slack
var SlackToken = os.Getenv("SLACK_TOKEN")

// VerificationToken Verification token from slack
var VerificationToken = os.Getenv("VERIFICATION_TOKEN")

var api = slack.New(SlackToken)

// HelloWorld prints the JSON encoded "message" field in the body
// of the request or "Hello, World!" if there isn't one.
func HelloWorld(w http.ResponseWriter, r *http.Request) {
	var event struct {
		Token     string `json:"token"`
		Challenge string `json:"challenge"`
	}

	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		fmt.Printf("Not a challenge request. err=%s", err)
	} else {
		if !verifyToken(event.Token) {
			fmt.Fprint(w, "Invalid token")
			return
		}

		if event.Challenge != "" {
			// Respond to Slack event subscription URL verification challenge
			fmt.Fprintf(w, "{'challenge': %s}", event.Challenge)
			return
		}
	}

	var messageEvent slack.MessageEvent

	if err := json.NewDecoder(r.Body).Decode(&messageEvent); err != nil {
		fmt.Printf("Not a message event. err=%s", err)
	} else {
		fmt.Print(messageEvent)
		fmt.Fprint(w, messageEvent)
		var parameters slack.PostMessageParameters
		channelID, timestamp, err := api.PostMessage("test", messageEvent.Text, parameters)

		if err != nil {
			fmt.Printf("%s\n", err)
			return
		}

		fmt.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	}

	var body interface{}
	json.NewDecoder(r.Body).Decode(&body)

	fmt.Printf("Strange event [%s]. Ignore.", body)
}

func verifyToken(token string) bool {
	if token == VerificationToken {
		return true
	}

	fmt.Print("Not a valid token")
	return false
}
