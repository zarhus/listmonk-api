package api

import (
	"context"
	"fmt"
	listmonk "github.com/Exayn/go-listmonk"
)

type APIClient struct {
	BaseURL  string
	Username *string
	Password *string
	Client   *listmonk.Client
}

func NewAPIClient(baseURL string, username, password *string) *APIClient {
	return &APIClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Client:   listmonk.NewClient(baseURL, username, password),
	}
}

// Create a new subscriber and add them to specified mailing lists
func (c *APIClient) CreateSubscriber(name string, email string, lists []uint) (uint, error) {
	service := c.Client.NewCreateSubscriberService()
	service.Email(email)
	service.Name(name)
	service.ListIds(lists)
	fmt.Println("Creating subscriber")
	subscriber, err := service.Do(context.Background())
	if err != nil {
		fmt.Println(err)
		return 0, err
	}
	return subscriber.Id, nil
}

// Create a new campaign with HTML content
func (c *APIClient) CreateCampaignHTML(name string, subject string, lists []uint, content string) (uint, error) {
  service := c.Client.NewCreateCampaignService()
  service.Name(name)
  service.Subject(subject)
  service.Lists(lists)
  service.Body(content)
	fmt.Println("Creating campaign")
  campaign, err := service.Do(context.Background())
	if err != nil {
    fmt.Println(err)
		return 0, err
	}
  // TODO: There is also campaign.CampaignID, check if it's the same
	return campaign.Id, err
}

// Send finished campaign to newly subscribed users
func (c *APIClient) ResumeCampaign(id int) (string, error) {
	// Fetch campaign launch date and mailing lists
  //TODO
	return "", nil
}

