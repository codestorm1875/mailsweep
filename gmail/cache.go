package gmail

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const cacheFile = "cache.json"

func cachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return cacheFile
	}
	return filepath.Join(home, ".mailsweep", cacheFile)
}

func SaveCache(senders []SenderGroup) error {
	path := cachePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	b, err := json.Marshal(senders)
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0600)
}

func LoadCache() ([]SenderGroup, error) {
	path := cachePath()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var senders []SenderGroup
	err = json.Unmarshal(b, &senders)
	return senders, err
}
