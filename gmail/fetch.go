package gmail

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/googleapi"
)

type Message struct {
	ID      string
	From    string
	Subject string
	Date    string
	Size    int64
}

type SenderGroup struct {
	Sender    string
	Email     string
	Messages  []Message
	TotalSize int64
	Count     int
}

type MailboxSnapshot struct {
	HistoryID   string        `json:"history_id"`
	ScannedAt   time.Time     `json:"scanned_at"`
	TotalEmails int           `json:"total_emails"`
	TotalSize   int64         `json:"total_size"`
	Senders     []SenderGroup `json:"senders"`
}

// FetchAllMessages grabs every email in your mailbox, pulls out who sent it
// and how big it is, then groups everything by sender so we can rank them.
func (c *Client) FetchAllMessages(progressFn func(fetched, total int)) (MailboxSnapshot, error) {
	historyID, err := c.CurrentHistoryID()
	if err != nil {
		return MailboxSnapshot{}, err
	}

	var allIDs []string
	pageToken := ""

	// Gmail gives us message IDs in pages of 500
	for {
		req := c.Service.Users.Messages.List(c.User).MaxResults(500)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, err := req.Do()
		if err != nil {
			return MailboxSnapshot{}, fmt.Errorf("failed to list messages: %w", err)
		}

		for _, msg := range resp.Messages {
			allIDs = append(allIDs, msg.Id)
		}

		if progressFn != nil {
			progressFn(0, len(allIDs))
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	totalMessages := len(allIDs)
	if totalMessages == 0 {
		return newMailboxSnapshot(nil, historyID, time.Now()), nil
	}

	messages := c.fetchMessagesByID(allIDs, progressFn, false)
	return newMailboxSnapshot(groupBySender(messages), historyID, time.Now()), nil
}

func (c *Client) SyncMailbox(cached MailboxSnapshot, progressFn func(fetched, total int)) (MailboxSnapshot, error) {
	if cached.HistoryID == "" || len(cached.Senders) == 0 {
		return c.FetchAllMessages(progressFn)
	}

	addedIDs, deletedIDs, latestHistoryID, err := c.fetchHistoryChanges(cached.HistoryID)
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && (gErr.Code == 404 || gErr.Code == 400) {
			return c.FetchAllMessages(progressFn)
		}
		return MailboxSnapshot{}, err
	}

	if progressFn != nil {
		progressFn(0, len(addedIDs))
	}

	if len(addedIDs) == 0 && len(deletedIDs) == 0 {
		cached.HistoryID = latestHistoryID
		cached.ScannedAt = time.Now()
		return newMailboxSnapshot(cached.Senders, cached.HistoryID, cached.ScannedAt), nil
	}

	existing := make(map[string]Message)
	for _, sender := range cached.Senders {
		for _, msg := range sender.Messages {
			existing[msg.ID] = msg
		}
	}

	for _, id := range deletedIDs {
		delete(existing, id)
	}

	addedMessages := c.fetchMessagesByID(addedIDs, progressFn, false)
	for _, msg := range addedMessages {
		existing[msg.ID] = msg
	}

	for _, id := range deletedIDs {
		delete(existing, id)
	}

	messages := make([]Message, 0, len(existing))
	for _, msg := range existing {
		messages = append(messages, msg)
	}

	return newMailboxSnapshot(groupBySender(messages), latestHistoryID, time.Now()), nil
}

// Groups emails by sender, then sorts so the biggest storage hogs come first
func groupBySender(messages []Message) []SenderGroup {
	groups := make(map[string]*SenderGroup)

	for _, msg := range messages {
		email := parseEmail(msg.From)
		key := strings.ToLower(email)

		g, ok := groups[key]
		if !ok {
			g = &SenderGroup{
				Sender: msg.From,
				Email:  email,
			}
			groups[key] = g
		}

		g.Messages = append(g.Messages, msg)
		g.TotalSize += msg.Size
		g.Count++
	}

	result := make([]SenderGroup, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g.Messages, func(i, j int) bool {
			return g.Messages[i].Size > g.Messages[j].Size
		})
		result = append(result, *g)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalSize > result[j].TotalSize
	})

	return result
}

// Pulls the email address out of "John Doe <john@example.com>" format
func parseEmail(from string) string {
	if idx := strings.LastIndex(from, "<"); idx != -1 {
		end := strings.LastIndex(from, ">")
		if end > idx {
			return from[idx+1 : end]
		}
	}
	return strings.TrimSpace(from)
}

func newMailboxSnapshot(senders []SenderGroup, historyID string, scannedAt time.Time) MailboxSnapshot {
	totalEmails := 0
	var totalSize int64
	for _, sender := range senders {
		totalEmails += sender.Count
		totalSize += sender.TotalSize
	}

	return MailboxSnapshot{
		HistoryID:   historyID,
		ScannedAt:   scannedAt,
		TotalEmails: totalEmails,
		TotalSize:   totalSize,
		Senders:     senders,
	}
}

func (c *Client) CurrentHistoryID() (string, error) {
	profile, err := c.Service.Users.GetProfile(c.User).Do()
	if err != nil {
		return "", fmt.Errorf("failed to get mailbox profile: %w", err)
	}

	return strconv.FormatUint(profile.HistoryId, 10), nil
}

func (c *Client) fetchHistoryChanges(startHistoryID string) ([]string, []string, string, error) {
	startID, err := strconv.ParseUint(startHistoryID, 10, 64)
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid cached history id %q: %w", startHistoryID, err)
	}

	added := make(map[string]struct{})
	deleted := make(map[string]struct{})
	pageToken := ""
	latestHistoryID := startHistoryID

	for {
		req := c.Service.Users.History.List(c.User).StartHistoryId(startID).MaxResults(500)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, err := req.Do()
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to list mailbox history: %w", err)
		}

		if resp.HistoryId != 0 {
			latestHistoryID = strconv.FormatUint(resp.HistoryId, 10)
		}

		for _, history := range resp.History {
			for _, addedMsg := range history.MessagesAdded {
				if addedMsg.Message == nil || addedMsg.Message.Id == "" {
					continue
				}
				added[addedMsg.Message.Id] = struct{}{}
			}
			for _, deletedMsg := range history.MessagesDeleted {
				if deletedMsg.Message == nil || deletedMsg.Message.Id == "" {
					continue
				}
				deleted[deletedMsg.Message.Id] = struct{}{}
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return mapKeys(added), mapKeys(deleted), latestHistoryID, nil
}

func (c *Client) fetchMessagesByID(ids []string, progressFn func(fetched, total int), includeDetails bool) []Message {
	const workerCount = 10

	messages := make([]Message, 0, len(ids))
	var mu sync.Mutex
	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup
	fetched := 0

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}

		go func(msgID string) {
			defer wg.Done()
			defer func() { <-sem }()

			<-ticker.C

			req := c.Service.Users.Messages.Get(c.User, msgID).Format("metadata")
			if includeDetails {
				req = req.MetadataHeaders("From", "Subject", "Date")
			} else {
				req = req.MetadataHeaders("From")
			}

			msg, err := req.Do()
			if err != nil {
				return
			}

			m := Message{
				ID:   msgID,
				Size: msg.SizeEstimate,
			}

			for _, header := range msg.Payload.Headers {
				switch header.Name {
				case "From":
					m.From = header.Value
				case "Subject":
					if includeDetails {
						m.Subject = header.Value
					}
				case "Date":
					if includeDetails {
						m.Date = header.Value
					}
				}
			}

			mu.Lock()
			messages = append(messages, m)
			fetched++
			if progressFn != nil {
				progressFn(fetched, len(ids))
			}
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	return messages
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *Client) FetchMessagesForSender(senderEmail string) ([]Message, error) {
	query := fmt.Sprintf("from:%s", senderEmail)
	var allIDs []string
	pageToken := ""

	for {
		req := c.Service.Users.Messages.List(c.User).Q(query).MaxResults(500)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list messages for %s: %w", senderEmail, err)
		}

		for _, msg := range resp.Messages {
			allIDs = append(allIDs, msg.Id)
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	messages := c.fetchMessagesByID(allIDs, nil, true)

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Size > messages[j].Size
	})

	return messages, nil
}
