package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/smtp"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"golang.org/x/oauth2"
)

type youtubeVideos struct {
	Etag  string `json:"etag"`
	Items []struct {
		Etag string `json:"etag"`
		ID   struct {
			Kind    string `json:"kind"`
			VideoID string `json:"videoId"`
		} `json:"id"`
		Kind    string `json:"kind"`
		Snippet struct {
			ChannelID            string `json:"channelId"`
			ChannelTitle         string `json:"channelTitle"`
			Description          string `json:"description"`
			LiveBroadcastContent string `json:"liveBroadcastContent"`
			PublishedAt          string `json:"publishedAt"`
			Thumbnails           struct {
				Default struct {
					URL string `json:"url"`
				} `json:"default"`
				High struct {
					URL string `json:"url"`
				} `json:"high"`
				Medium struct {
					URL string `json:"url"`
				} `json:"medium"`
			} `json:"thumbnails"`
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
	Kind          string `json:"kind"`
	NextPageToken string `json:"nextPageToken"`
	PageInfo      struct {
		ResultsPerPage float64 `json:"resultsPerPage"`
		TotalResults   float64 `json:"totalResults"`
	} `json:"pageInfo"`
}

type credentials struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPUserName string `json:"smtpUserName"`
	SMTPPassword string `json:"smtpPassword"`
}

type video struct {
	Title       string
	Description string
	Thumbnail   string
	ID          string
}

type data struct {
	Videos []video
}

var configPath = filepath.Join(userDir(), ".config", "youtube-emacs-search")

func main() {

	dirInfo, err := os.Stat(configPath)

	if err != nil || !dirInfo.IsDir() {
		fmt.Println("There's no config directory. Creating it now...")
		if err = os.MkdirAll(configPath, 0700); err != nil {
			fmt.Println("Something went wrong while creating the config directory")
			os.Exit(1)
		}
	}

	path := filepath.Join(configPath, "timestamp")

	var file *os.File

	file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	defer file.Close()

	t := time.Now()
	// Subtract 2 days from current date
	twoDaysAgo := t.AddDate(0, 0, -2)

	if err == nil {
		fmt.Printf("%s does not exist! Creating it...\n", path)
		file.WriteString(twoDaysAgo.Format(time.RFC3339))
	} else {
		dat, err := ioutil.ReadFile(path)
		check(err)
		twoDaysAgo, err = time.Parse(time.RFC3339, string(dat))
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
		fmt.Printf("Press any key to visit the URL for the auth dialog then come back here and paste the token:\n%v\n\n", url)

		var code string
		if _, err := fmt.Scanf("\n"); err != nil {
			check(err)
		}

		switch runtime.GOOS {
		case "darwin":
			cmd := exec.Command("open", url)
			cmd.Run()
		case "linux":
			cmd := exec.Command("xgdopen", url)
			cmd.Run()
		default:
			fmt.Println("Sorry but you have to open the URL manually")
		}

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

	date := url.QueryEscape(twoDaysAgo.Format(time.RFC3339))

	// Construct URL to query with value of last update
	resp, err := client.Get("https://www.googleapis.com/youtube/v3/search?part=snippet&order=date&publishedAfter=" + date + "&q=emacs&type=video&maxResults=50")
	check(err)

	defer resp.Body.Close()

	htmlData, err := ioutil.ReadAll(resp.Body)
	check(err)

	// If everything went fine write the current time into update value
	timeBuffer := bytes.NewBufferString(t.Format(time.RFC3339))
	ioutil.WriteFile(path, timeBuffer.Bytes(), 0700)

	videos := decodeYoutubeJSON(htmlData)

	videoData := make([]video, 10)

	for _, value := range videos.Items {
		snippet := value.Snippet
		videoData = append(videoData, video{snippet.Title, snippet.Description, snippet.Thumbnails.Medium.URL, value.ID.VideoID})
	}

	templateData := data{
		Videos: videoData,
	}

	buffer := new(bytes.Buffer)

	template := template.Must(template.New("videosTemplate").Parse(youtubeVideoTemplate()))
	err = template.Execute(buffer, &templateData)
	check(err)

	if len(videos.Items) == 0 {
		fmt.Println("There were no new videos")
	} else {
		// Send results via email
		sendEmail(
			cred.SMTPHost,
			cred.SMTPPort,
			cred.SMTPUserName,
			cred.SMTPPassword,
			[]string{cred.SMTPUserName},
			"YouTube Emacs Search",
			buffer.String())
	}

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
	credentialsFile, err := os.Open(filepath.Join(configPath, "youtube-oauth-credentials"))
	defer credentialsFile.Close()

	if err != nil {
		fmt.Printf("Please add your YouTube API credentials to %s\n", filepath.Join(configPath, "youtube-oauth-credentials"))
		os.Exit(1)
	}

	dec := json.NewDecoder(credentialsFile)
	err = dec.Decode(&cred)
	check(err)

	return
}

func decodeYoutubeJSON(jsonData []byte) (videos youtubeVideos) {
	dec := json.NewDecoder(bytes.NewReader(jsonData))
	err := dec.Decode(&videos)
	check(err)

	return
}

func saveToken(token *oauth2.Token) {
	tokenFile, err := os.Create(filepath.Join(configPath, "youtube-oauth-token"))
	defer tokenFile.Close()
	check(err)

	tokenEncoder := gob.NewEncoder(tokenFile)
	tokenEncoder.Encode(token)
}

func loadToken() (token *oauth2.Token, err error) {
	tokenFile, err := os.Open(filepath.Join(configPath, "youtube-oauth-token"))
	defer tokenFile.Close()

	if err != nil {
		return nil, err
	}

	tokenDecoder := gob.NewDecoder(tokenFile)
	err = tokenDecoder.Decode(&token)
	check(err)
	return
}

func sendEmail(host string, port int, userName string, password string, to []string, subject string, message string) (err error) {
	defer check(err)

	parameters := struct {
		From    string
		To      string
		Subject string
		Message string
	}{
		userName,
		strings.Join([]string(to), ","),
		subject,
		message,
	}

	buffer := new(bytes.Buffer)

	template := template.Must(template.New("emailTemplate").Parse(emailScript()))
	template.Execute(buffer, &parameters)

	auth := smtp.PlainAuth("", userName, password, host)

	err = smtp.SendMail(
		fmt.Sprintf("%s:%d", host, port),
		auth,
		userName,
		to,
		buffer.Bytes())

	return err
}

func emailScript() (script string) {
	return `From: {{.From}}
To: {{.To}}
Subject: {{.Subject}}
MIME-version: 1.0
Content-Type: text/html; charset="UTF-8"

{{.Message}}`
}

func youtubeVideoTemplate() (script string) {
	return `<html>
<body>
	<table>
	{{with .Videos}}
		{{range .}}
		<tr>
			<td><a href="https://youtube.com/watch?v={{.ID}}"><img src="{{.Thumbnail}}"</a></td>
			<td>{{.Title}}</td>
			<td>{{.Description}}</td>
		<tr>
		{{end}}
	{{end}}
	</table>
<body>
<html>`
}
