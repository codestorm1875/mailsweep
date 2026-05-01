package gmail

import (
	"fmt"

	"google.golang.org/api/gmail/v1"
)

// BatchDelete nukes emails by ID. Gmail caps it at 1000 per request,
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
		req := &gmail.BatchDeleteMessagesRequest{
			Ids: batch,
		}

		err := c.Service.Users.Messages.BatchDelete(c.User, req).Do()
		if err != nil {
			return fmt.Errorf("failed to batch delete messages: %w", err)
		}
	}

	return nil
}
