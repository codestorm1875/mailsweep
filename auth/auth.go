package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const tokenDir = ".mailsweep"
const tokenFile = "token.json"

// GetGmailClient logs you into Google, saves the token so you don't have
// to log in every time, and hands back a ready-to-use Gmail connection.
func GetGmailClient() (*gmail.Service, string, error) {
	ctx := context.Background()

	credPath, err := findCredentials()
	if err != nil {
		return nil, "", fmt.Errorf(`credentials.json not found

mailsweep needs a Google Cloud credentials file to access your Gmail.
Head to console.cloud.google.com and follow these steps:

1. Create a Google Cloud project
   Click the project dropdown (top left) → "New Project"
   Name it "mailsweep" → hit Create

2. Enable the Gmail API
   Navigate to APIs & Services → Library
   Search for "Gmail API" → select it → click Enable

3. Set up the OAuth consent screen
   Go to APIs & Services → OAuth consent screen
   - Select "External" as the user type → click Create
   - Set the app name to "mailsweep"
   - Enter your email address in both the "User support email" and
     "Developer contact" fields → Save and Continue
   - Skip the Scopes step → Save and Continue
   - Under Test Users, add your Gmail address → Save and Continue

4. Create your OAuth credentials
   Go to APIs & Services → Credentials
   - Click "+ Create Credentials" → choose "OAuth client ID"
   - Set application type to "Desktop app"
   - Give it any name → click Create
   - Download the JSON file from the dialog that appears

5. Move the credentials file into place
   mkdir -p ~/.mailsweep
   mv ~/Downloads/client_secret_*.json ~/.mailsweep/credentials.json

Once that's done, run mailsweep again.`)
	}

	b, err := os.ReadFile(credPath)
	if err != nil {
		return nil, "", fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailModifyScope)
	if err != nil {
		return nil, "", fmt.Errorf("unable to parse credentials: %w", err)
	}

	tokPath := tokenPath()
	tok, err := loadToken(tokPath)
	if err != nil {
		// No saved token — need to log in via browser
		tok, err = getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, "", fmt.Errorf("unable to get token: %w", err)
		}
		if err := saveToken(tokPath, tok); err != nil {
			return nil, "", fmt.Errorf("unable to save token: %w", err)
		}
	}

	client := config.Client(ctx, tok)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, "", fmt.Errorf("unable to create Gmail service: %w", err)
	}

	profile, err := srv.Users.GetProfile("me").Do()
	if err != nil {
		return nil, "", fmt.Errorf("unable to get user profile: %w", err)
	}

	return srv, profile.EmailAddress, nil
}

// Looks for credentials.json in ~/.mailsweep/ first, then the current folder
func findCredentials() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, tokenDir, "credentials.json")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	if _, err := os.Stat("credentials.json"); err == nil {
		return "credentials.json", nil
	}

	return "", fmt.Errorf("credentials.json not found in ~/.mailsweep/ or current directory")
}

func tokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return tokenFile
	}
	return filepath.Join(home, tokenDir, tokenFile)
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Spins up a tiny local web server so Google can send back the login code
func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	config.RedirectURL = "http://localhost:8085/callback"

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			fmt.Fprintln(w, "Error: no authorization code received.")
			return
		}
		codeChan <- code
		fmt.Fprintln(w, "Authorization successful! You can close this tab and return to the terminal.")
	})

	server := &http.Server{Addr: ":8085", Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("\nOpen this URL in your browser to authorize mailsweep:\n\n%s\n\n", authURL)

	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		server.Close()
		return nil, err
	}

	server.Close()

	tok, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}

	return tok, nil
}

func saveToken(path string, token *oauth2.Token) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}
	