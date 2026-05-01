package gmail

import (
	"google.golang.org/api/gmail/v1"
)

type Client struct {
	Service *gmail.Service
	User    string
}

func NewClient(service *gmail.Service) *Client {
	return &Client{
		Service: service,
		User:    "me",
	}
}
