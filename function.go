// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/nlopes/slack/slackevents"

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
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body := buf.String()
	eventsAPIEvent, e := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: "TOKEN"}))
	if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
	}
}

func verifyToken(token string) bool {
	if token == VerificationToken {
		return true
	}

	fmt.Print("Not a valid token")
	return false
}
