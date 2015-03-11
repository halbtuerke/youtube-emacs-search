package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

type credentials struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

func main() {
	path := filepath.Join(userDir(), ".config", "youtube-emacs-search")

	var file *os.File
	var err error

	// Get value of last update from some file => ~/.config/youtube-emacs
	file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	defer file.Close()

	t := time.Now()
	// Subtract 2 days from current date
	twoDaysAgo := t.AddDate(0, 0, -2)

	if err == nil {
		fmt.Printf("%s does not exist! Creating it...\n", path)
		file.WriteString(twoDaysAgo.Format(time.RFC3339))
	} else {
		file, err = os.OpenFile(path, os.O_RDWR, 0666)
		check(err)
	}

	cred := loadOauthCredentials()

	if cred.ClientID == "YOUR-CLIENTID" || cred.ClientSecret == "YOUR-CLIENTSECRET" {
		fmt.Println("Please setup your YouTube OAuth credentials.")
		os.Exit(1)
	}

	conf := &oauth2.Config{
		ClientID:     cred.ClientID,
		ClientSecret: cred.ClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/youtube.readonly"},
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
	}

	tok, err := loadToken()

	if err != nil {
		url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
		fmt.Printf("Visit the URL for the auth dialog then come back here and paste the token:\n%v\n\n", url)

		var code string
		if _, err := fmt.Scan(&code); err != nil {
			check(err)
		}

		tok, err = conf.Exchange(oauth2.NoContext, code)
		if err != nil {
			check(err)
		}

		saveToken(tok)
	}

	client := conf.Client(oauth2.NoContext, tok)

	// Construct URL to query with value of last update
	resp, err := client.Get("https://www.googleapis.com/youtube/v3/search?part=snippet&order=date&publishedAfter=2015-02-17T00%3A00%3A00Z&q=emacs&type=video&maxResults=50")
	check(err)

	defer resp.Body.Close()

	htmlData, err := ioutil.ReadAll(resp.Body)
	check(err)

	// If everything went fine write the current time into update value
	file.WriteString(twoDaysAgo.Format(time.RFC3339))

	// Show the result of the search
	// fmt.Println(string(htmlData))
	ppJSON(htmlData)

	// Send results via email
}

func ppJSON(data []byte) {
	var dat map[string]interface{}

	if err := json.Unmarshal(data, &dat); err != nil {
		panic(err)
	}

	b, err := json.MarshalIndent(dat, "", "  ")
	if err != nil {
		log.Println(err)
	}
	fmt.Printf("%s\n", b)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func userDir() (userDir string) {
	currentUser, _ := user.Current()
	userDir = currentUser.HomeDir
	return
}

func loadOauthCredentials() (cred credentials) {
	credentialsFile, err := os.Open(filepath.Join(userDir(), ".config", "youtube-oauth-credentials"))
	defer credentialsFile.Close()

	if err != nil {
		fmt.Println("Please add your YouTube API credentials to ~/.config/youtube-oauth-credentials")
		os.Exit(1)
	}

	dec := json.NewDecoder(credentialsFile)
	err = dec.Decode(&cred)
	check(err)

	return
}

func saveToken(token *oauth2.Token) {
	tokenFile, err := os.Create(filepath.Join(userDir(), ".config", "youtube-oauth-token"))
	defer tokenFile.Close()
	check(err)

	tokenEncoder := gob.NewEncoder(tokenFile)
	tokenEncoder.Encode(token)
}

func loadToken() (token *oauth2.Token, err error) {
	tokenFile, err := os.Open(filepath.Join(userDir(), ".config", "youtube-oauth-token"))
	defer tokenFile.Close()

	if err != nil {
		return nil, err
	}

	tokenDecoder := gob.NewDecoder(tokenFile)
	err = tokenDecoder.Decode(&token)
	check(err)
	return
}
