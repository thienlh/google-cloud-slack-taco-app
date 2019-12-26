package p

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

// givingSummary model represents each Giving Summary row in Google Sheets
type givingSummary struct {
	Name  string
	Date  string
	Total string
}

var today = timeIn(location, time.Now()).Format("02-Jan 2006")

//	writeRange Start range to write raw data
const writeRange = "A2"

//	spreadsheetID if of the spreadsheet
var spreadsheetID = os.Getenv("SPREADSHEET_ID")

//	spreadsheetURL Shareable link to the spreadsheet
var spreadsheetURL = os.Getenv("SPREADSHEET_URL")
var service = getService()

// givingSummaryReadRange Read range for the giving summary sheet
const givingSummaryReadRange = "Pivot Table 1!A3:D"

// receivingSummaryReadRange Read range for the receiving summary sheet
const receivingSummaryReadRange = "Pivot Table 2!A3:D"

// Get the google sheets service
func getService() *sheets.Service {
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)
	service, err := sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}
	return service
}

// getClient Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokenFilePath := "token.json"
	token, err := tokenFromFile(tokenFilePath)
	if err != nil {
		token = tokenFromConfig(config)
		saveToken(tokenFilePath, token)
	}
	return config.Client(context.Background(), token)
}

// Request a token from the web, then returns the retrieved token.
func tokenFromConfig(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}
	token, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return token
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// Read and print sample data from the sheet
func readRow(readRange string) [][]interface{} {
	response, err := service.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}
	if len(response.Values) == 0 {
		fmt.Println("No data found.")
		return nil
	}
	fmt.Printf("Data found: %v\n", response.Values)
	return response.Values
}

// write Write data to default range
func appendRow(values []interface{}) {
	var valueRange sheets.ValueRange
	valueRange.Values = append(valueRange.Values, values)
	_, err := service.Spreadsheets.Values.Append(spreadsheetID, writeRange, &valueRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Fatalf("Unable to write data %v to sheet. %v", values, err)
	}
}

func getRecords(from Date, to Date) ChartRecords {
	log.Printf("From: %v, to %v\n", from, to)
	records := readRow(receivingSummaryReadRange)
	chart := map[string]int{}
	for _, record := range records {
		// Skip Grand Total record
		if strings.Contains(fmt.Sprintf("%s %s %s %s", record[0], record[1], record[2], record[3]), "Grand Total") {
			log.Println("Grand Total record. Skip.")
			break
		}
		receivingSummary := givingSummary{record[0].(string), fmt.Sprintf("%s %s", record[1], record[2]), record[3].(string)}
		date, err := time.Parse("02-Jan 2006", receivingSummary.Date)
		if err != nil {
			log.Panicf("Unable to parse date %v from Google Sheets!", receivingSummary.Date)
			return nil
		}
		if isInRange(timeIn(location, date), from, to) {
			total, err := strconv.Atoi(receivingSummary.Total)
			if err != nil {
				log.Fatalf("Unable to parse total %v to int with error %v", receivingSummary.Total, err)
				return nil
			}
			chart[receivingSummary.Name] = total + chart[receivingSummary.Name]
			log.Printf("Chart: %v\n", chart)
		}
	}
	log.Printf("Chart: %v\n", chart)
	return rank(chart)
}
