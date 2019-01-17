// Package p contains an HTTP Cloud Function.
package p

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"

	"github.com/nlopes/slack"
)

// HelloWorld prints the JSON encoded "message" field in the body
// of the request or "Hello, World!" if there isn't one.
func HelloWorld(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Message string `json:"message"`
		User    string `json:"user"`
	}

	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		fmt.Fprint(w, "Hello World!")
		return
	}

	if d.User == "" {
		fmt.Fprint(w, "Hello World!")
		return
	}

	fmt.Fprint(w, html.EscapeString(d.Message))

	api := slack.New(os.Getenv("SLACK_TOKEN"))
	user, err := api.GetUserInfo(d.User)
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}

	fmt.Printf("ID: %s, Fullname: %s, Email: %s\n", user.ID, user.Profile.RealName, user.Profile.Email)
}
