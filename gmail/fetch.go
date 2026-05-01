package gmail

import (
	"fmt"
	"sort"
	"strings"
	"sync"
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

// FetchAllMessages grabs every email in your mailbox, pulls out who sent it
// and how big it is, then groups everything by sender so we can rank them.
func (c *Client) FetchAllMessages(progressFn func(fetched, total int)) ([]SenderGroup, error) {
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
			return nil, fmt.Errorf("failed to list messages: %w", err)
		}

		for _, msg := range resp.Messages {
			allIDs = append(allIDs, msg.Id)
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	totalMessages := len(allIDs)
	if totalMessages == 0 {
		return nil, nil
	}

	// Fetch the details (sender, subject, size) for each email
	const workerCount = 10
	messages := make([]Message, 0, totalMessages)
	var mu sync.Mutex
	var fetchErr error

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup
	fetched := 0

	for _, id := range allIDs {
		wg.Add(1)
		sem <- struct{}{}

		go func(msgID string) {
			defer wg.Done()
			defer func() { <-sem }()

			msg, err := c.Service.Users.Messages.Get(c.User, msgID).
				Format("metadata").
				MetadataHeaders("From", "Subject", "Date").
				Do()
			if err != nil {
				mu.Lock()
				if fetchErr == nil {
					fetchErr = fmt.Errorf("failed to fetch message %s: %w", msgID, err)
				}
				mu.Unlock()
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
					m.Subject = header.Value
				case "Date":
					m.Date = header.Value
				}
			}

			mu.Lock()
			messages = append(messages, m)
			fetched++
			if progressFn != nil {
				progressFn(fetched, totalMessages)
			}
			mu.Unlock()
		}(id)
	}

	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	return groupBySender(messages), nil
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

	const workerCount = 10
	messages := make([]Message, 0, len(allIDs))
	var mu sync.Mutex
	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for _, id := range allIDs {
		wg.Add(1)
		sem <- struct{}{}

		go func(msgID string) {
			defer wg.Done()
			defer func() { <-sem }()

			msg, err := c.Service.Users.Messages.Get(c.User, msgID).
				Format("metadata").
				MetadataHeaders("From", "Subject", "Date").
				Do()
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
					m.Subject = header.Value
				case "Date":
					m.Date = header.Value
				}
			}

			mu.Lock()
			messages = append(messages, m)
			mu.Unlock()
		}(id)
	}

	wg.Wait()

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Size > messages[j].Size
	})

	return messages, nil
}
