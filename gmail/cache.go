package gmail

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheFile = "cache.json"
	prefsFile = "prefs.json"
)

type Preferences struct {
	SortMode      int    `json:"sort_mode"`
	FilterQuery   string `json:"filter_query"`
	LastAgeFilter int    `json:"last_age_filter"`
}

func cachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return cacheFile
	}
	return filepath.Join(home, ".mailsweep", cacheFile)
}

func prefsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return prefsFile
	}
	return filepath.Join(home, ".mailsweep", prefsFile)
}

func SaveCache(snapshot MailboxSnapshot) error {
	path := cachePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0600)
}

func LoadCache() (MailboxSnapshot, error) {
	path := cachePath()
	b, err := os.ReadFile(path)
	if err != nil {
		return MailboxSnapshot{}, err
	}

	var snapshot MailboxSnapshot
	if err := json.Unmarshal(b, &snapshot); err == nil && snapshot.Senders != nil {
		return newMailboxSnapshot(snapshot.Senders, snapshot.HistoryID, snapshot.ScannedAt), nil
	}

	var senders []SenderGroup
	if err := json.Unmarshal(b, &senders); err != nil {
		return MailboxSnapshot{}, err
	}

	return newMailboxSnapshot(senders, "", time.Time{}), nil
}

func SavePreferences(prefs Preferences) error {
	path := prefsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	b, err := json.Marshal(prefs)
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0600)
}

func LoadPreferences() (Preferences, error) {
	path := prefsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return Preferences{}, err
	}

	var prefs Preferences
	if err := json.Unmarshal(b, &prefs); err != nil {
		return Preferences{}, err
	}
	return prefs, nil
}
