// Package tidal implements a limited client for the tidal API.
package tidal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/coaxial/tizinger/playlist"
	"github.com/coaxial/tizinger/utils/logger"
	"gopkg.in/yaml.v3"
)

// APIClient implements exporter.Client.
type APIClient struct{}

// baseURL can be overridden while testing to avoid live calls.
var baseURL = "https://api.tidalhifi.com/v1"

// jar is the cookie jar for the tidal client.
var jar http.CookieJar

// tidalClient is the client making requests to the Tidal API.
var tidalClient = &http.Client{
	Jar: jar,
}

// userData represents the data returned upon logging in that is necessary to
// compose authenticated requests.
type userData struct {
	SessionID   string
	CountryCode string
	UserID      string
}

// loginResponse is the JSON object returned from a successful login request.
type loginResponse struct {
	SessionID   string `json:"sessionId"`
	CountryCode string `json:"countryCode"`
	UserID      string `json:"userId"`
}

var tidalUserData userData

// composeHeaders adds the necessary headers to the request
func composeHeaders(req *http.Request) *http.Request {
	req.Header.Add("Origin", "https://listen.tidal.com")
	req.Header.Add("X-Tidal-SessionId", tidalUserData.SessionID)
	return req
}

// CreatePlaylist creates playlists on Tidal.
func (ac APIClient) CreatePlaylist(name string, tracks playlist.Tracklist) (ok bool, err error) {
	ok, err = setToken()
	if err != nil {
		logger.Error.Printf("could not fetch tokens: %v", err)
		return ok, err
	}
	return ok, err
}

// manifestURL is the tokens manifest's location.
// curtesy of https://github.com/yaronzz/Tidal-Media-Downloader
var manifestURL = "https://cdn.jsdelivr.net/gh/yaronzz/Tidal-Media-Downloader@latest/Else/tokens.json"

// tidalToken is the API token required for every request.
var tidalToken string

// setToken gets the currently valid token to send along with API requests.
// The purpose it to avoid hard-coding tokens so that the calls don't fail when
// Tidal rotates the token like they did in June 2020.
func setToken() (ok bool, err error) {
	type tokensResponse struct {
		Token      string `json:"token"`
		TokenPhone string `json:"token_phone"`
	}

	logger.Trace.Printf("getting tokens manifest at %q", manifestURL)
	resp, err := http.Get(manifestURL)
	logger.Trace.Printf(
		"Received response %q, %d bytes",
		resp.Header.Get("content-type"),
		resp.ContentLength,
	)
	if err != nil {
		logger.Error.Printf("error fetching API tokens from %q: %v", manifestURL, err)
		return ok, err
	}

	tokens, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		logger.Error.Printf("error reading response: %v", err)
		return ok, err
	}

	var JSONTokens tokensResponse
	err = json.Unmarshal(tokens, &JSONTokens)
	if err != nil {
		logger.Error.Printf("error unmarshalling token: %v", err)
		return ok, err
	}
	tidalToken = JSONTokens.Token
	ok = true
	logger.Info.Printf("successfully set API token")
	return ok, err
}

// credentialsYAML matches the format for individual credential entries in
// credentials.yml
type credentialsYAML struct {
	Tidal struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"tidal,inline,omitempty"`
}

var credentials credentialsYAML

// readCredentials reads the usernames and passwords from the credentials.yml
// file and unmarshals them into a Go struct
// TODO: extract credentials to own module
// TODO: access through getters and setters
func readCredentials() (ok bool, err error) {
	credentialsFile := "credentials.yml"
	content, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		logger.Error.Printf("could not read %q: %v", credentialsFile, err)
		return ok, err
	}

	err = yaml.Unmarshal(content, &credentials)
	if err != nil {
		logger.Error.Printf("could not parse %q: %v", credentialsFile, err)
		return ok, err
	}
	ok = true
	return ok, err
}

// login performs a login with the Tidal API.
func login() (ok bool, err error) {
	endpoint := "/login/username"
	var payload = fmt.Sprintf(`{"username":%q,"password":%q}`, credentials.Tidal.Username, credentials.Tidal.Password)
	uri := baseURL + endpoint
	loginRequest, err := http.NewRequest(http.MethodPost, uri, strings.NewReader(payload))
	if err != nil {
		logger.Error.Printf("error building login request: %v", err)
		return ok, err
	}
	composeHeaders(loginRequest)
	logger.Info.Printf("sending login request to %q", uri)
	resp, err := tidalClient.Do(loginRequest)
	logger.Info.Printf("received login response %q, %d bytes", resp.Header.Get("Content-Type"), resp.ContentLength)
	contents, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		logger.Error.Printf("error reading login response: %v", err)
		return ok, err
	}
	err = json.Unmarshal(contents, &tidalUserData)
	if err != nil {
		logger.Error.Printf("error unmarshalling login response: %v", err)
		return ok, err
	}

	ok = true
	logger.Info.Printf("successfully logged in")
	return ok, err
}
