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
		fmt.Fprint(w, "Invalid input")

		var e interface{}
		err := json.NewDecoder(r.Body).Decode(&e)

		if err != nil {
			fmt.Fprintf(w, "Invalid input: %s", err)
			return
		}

		fmt.Fprintf(w, "Strange input: %s", e)
		var shit slack.PostMessageParameters
		api.PostMessage("test", e.(string), shit)
	}

	if !verifyToken(event.Token) {
		fmt.Fprint(w, "Invalid token")
		return
	}

	if event.Challenge != "" {
		// Respond to Slack event subscription URL verification challenge
		fmt.Fprintf(w, "{'challenge': %s}", event.Challenge)
		return
	}

	// user, err := api.GetUserInfo(event.User)
	// if err != nil {
	// 	fmt.Fprintf(w, "%s", err)
	// 	return
	// }

	// fmt.Fprintf(w, "ID: %s, Fullname: %s, Email: %s", user.ID, user.Profile.RealName, user.Profile.Email)
}

func verifyToken(token string) bool {
	if token == VerificationToken {
		return true
	}

	return false
}
