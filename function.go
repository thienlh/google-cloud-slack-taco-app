// Package p contains an HTTP Cloud Function.
package p

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// SlackToken Authentication token from slack
var SlackToken = os.Getenv("SLACK_TOKEN")

// VerificationToken Verification token from slack
var VerificationToken = os.Getenv("VERIFICATION_TOKEN")

// HelloWorld prints the JSON encoded "message" field in the body
// of the request or "Hello, World!" if there isn't one.
func HelloWorld(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, r.Body)

	var event struct {
		Token     string `json:"token"`
		Challenge string `json:"challenge"`
	}

	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		fmt.Fprint(w, "Invalid input")
		return
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

	// if d.User == "" {
	// 	fmt.Fprint(w, "Hello World!")
	// 	return
	// }

	// api := slack.New(SlackToken)
	// user, err := api.GetUserInfo(d.User)
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
