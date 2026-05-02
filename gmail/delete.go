package gmail

import (
	"fmt"
	"sort"
	"time"

	"google.golang.org/api/gmail/v1"
)

// BatchDelete moves emails to the trash by ID. Gmail caps it at 1000 per request,
// so we chunk them up if there are more.
func (c *Client) BatchDelete(messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	const batchSize = 1000

	for i := 0; i < len(messageIDs); i += batchSize {
		end := i + batchSize
		if end > len(messageIDs) {
			end = len(messageIDs)
		}

		batch := messageIDs[i:end]
		req := &gmail.BatchModifyMessagesRequest{
			Ids:         batch,
			AddLabelIds: []string{"TRASH"},
		}

		err := c.Service.Users.Messages.BatchModify(c.User, req).Do()
		if err != nil {
			return fmt.Errorf("failed to batch trash messages: %w", err)
		}
	}

	return nil
}

// RemoveMessages updates the grouped sender data after a successful delete so
// the UI and cache can be refreshed without a full mailbox re-scan.
func RemoveMessages(senders []SenderGroup, messageIDs []string) []SenderGroup {
	if len(senders) == 0 || len(messageIDs) == 0 {
		return senders
	}

	toDelete := make(map[string]struct{}, len(messageIDs))
	for _, id := range messageIDs {
		toDelete[id] = struct{}{}
	}

	updated := make([]SenderGroup, 0, len(senders))
	for _, sender := range senders {
		messages := make([]Message, 0, len(sender.Messages))
		var totalSize int64

		for _, msg := range sender.Messages {
			if _, ok := toDelete[msg.ID]; ok {
				continue
			}
			messages = append(messages, msg)
			totalSize += msg.Size
		}

		if len(messages) == 0 {
			continue
		}

		sender.Messages = messages
		sender.Count = len(messages)
		sender.TotalSize = totalSize
		updated = append(updated, sender)
	}

	sort.Slice(updated, func(i, j int) bool {
		return updated[i].TotalSize > updated[j].TotalSize
	})

	return updated
}

func UpdateSnapshotAfterDelete(snapshot MailboxSnapshot, messageIDs []string) MailboxSnapshot {
	return newMailboxSnapshot(RemoveMessages(snapshot.Senders, messageIDs), snapshot.HistoryID, time.Now())
}
