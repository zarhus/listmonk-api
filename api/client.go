package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
  service.ContentType("html")
	fmt.Println("Creating campaign")
  campaign, err := service.Do(context.Background())
	if err != nil {
		return 0, err
	}
	return campaign.Id, err
}

func (c *APIClient) deleteCampaign(campaign *listmonk.Campaign) error{
  deleteCampaignService := c.Client.NewDeleteCampaignService()
  deleteCampaignService.Id(campaign.Id)
  return deleteCampaignService.Do(context.Background())
}

// Get users who subscribed after campaign was launched
func (c *APIClient) getSubscribersAfterLaunch(campaign *listmonk.Campaign) ([] *listmonk.Subscriber, error){
  // Check if there already was an incremental campaign. If so, only search for
  // subscribers who subscribed after inc. campaign launch.
  fmt.Println("Checking for already-existing incremental campaign")
  getCampaignsService := c.Client.NewGetCampaignsService()
  campaigns, err := getCampaignsService.Do(context.Background())

  if err != nil {
    return nil, err
  }

  var inc_campaign *listmonk.Campaign = nil
  var incCampaignLaunchDate string
  for _, camp := range campaigns {
    if camp.Name == campaign.Name + "_inc"{
      inc_campaign = camp
      incCampaignLaunchDate = inc_campaign.StartedAt.Format("2006-01-02T15:04:05.999999-07:00")
      c.deleteCampaign(camp) 
      break
    }
  }
  
  listIDs := make([]string, len(campaign.Lists))
  for i, list := range campaign.Lists {
    listIDs[i] = strconv.Itoa(int(list.Id))
  }

  launchDate := campaign.StartedAt.Format("2006-01-02T15:04:05.999999-07:00")

  var query string
  if inc_campaign != nil{
    query = fmt.Sprintf("id IN (SELECT subscriber_id FROM subscriber_lists WHERE created_at > '%v' AND list_id IN (%s))", incCampaignLaunchDate, strings.Join(listIDs, ","))
  }else
  {
    query = fmt.Sprintf("id IN (SELECT subscriber_id FROM subscriber_lists WHERE created_at >'%v' AND list_id IN (%s))", launchDate, strings.Join(listIDs, ","))
  }

  getSubscribersService := c.Client.NewGetSubscribersService()
  getSubscribersService.Query(query)

  fmt.Println("Fetching new subscribers")
  return getSubscribersService.Do(context.Background())
}

func (c *APIClient) addSubscribersToList(subscribers [] *listmonk.Subscriber, list *listmonk.List) error { 
  subscribersListsService := c.Client.NewUpdateSubscribersListsService()
  subscriberIDs := make([]uint, len(subscribers))
  for i, subscriber := range subscribers {
    subscriberIDs[i] = subscriber.Id
  }
  subscribersListsService.ListIds([]uint{list.Id})
  subscribersListsService.Ids(subscriberIDs)
  subscribersListsService.Action("add")
  _, err := subscribersListsService.Do(context.Background())
  return err
}

// Create incremental campaign from an existing one
func (c *APIClient) createIncCampaign(campaign *listmonk.Campaign, tempList *listmonk.List) (*listmonk.Campaign, error){
  createCampaignService := c.Client.NewCreateCampaignService()
  createCampaignService.Name(campaign.Name + "_inc")

  //Copy fields from original campaign
  createCampaignService.Subject(campaign.Subject)
  createCampaignService.Type(campaign.Type)
  createCampaignService.Body(campaign.Body)
  createCampaignService.Lists([]uint{tempList.Id})
  createCampaignService.ContentType(campaign.ContentType)
  createCampaignService.FromEmail(campaign.FromEmail)
  createCampaignService.Messenger(campaign.Messenger)
  createCampaignService.TemplateId(campaign.TemplateId)
  createCampaignService.Tags(campaign.Tags)

  fmt.Println("Creating incremental campaign")
  return createCampaignService.Do(context.Background())

}
// Send finished campaign to newly subscribed users. Return bool indicating
// whether the operation was performed.
func (c *APIClient) ResumeCampaign(id uint) (bool, error) {
	// Fetch campaign launch date and mailing lists
  getCampaignService := c.Client.NewGetCampaignService()
  getCampaignService.Id(id)

  fmt.Println("Fetching campaign data")
  campaign, err := getCampaignService.Do(context.Background())

  if err != nil {
    return false, err
  }

  if len(campaign.Lists) == 0 {
    fmt.Println("The campaign targets no mailing lists")
    return false, nil
  }

  subscribers, err := c.getSubscribersAfterLaunch(campaign)
  
  if err != nil {
    return false, err
  }

  if len(subscribers) == 0 {
    fmt.Println("No new subscribers since last launch")
    return false, nil
  }
  
  // Create temporary list 
  createListService := c.Client.NewCreateListService()
  createListService.Name("temp_list")
  tempList, err := createListService.Do(context.Background())

  if err != nil {
    return false, err
  }

  // Add subscribers to temporary list
  err = c.addSubscribersToList(subscribers, tempList) 

  if err != nil {
    return false, err
  }

  incCampaign, err := c.createIncCampaign(campaign, tempList)

  if err != nil {
    return false, err
  }

  //Launch incremental campaign
  updateCampaignStatusService := c.Client.NewUpdateCampaignStatusService()
  updateCampaignStatusService.Id(incCampaign.Id)
  updateCampaignStatusService.Status("running")

  fmt.Println("Launching incremental campaign")
  _, err = updateCampaignStatusService.Do(context.Background())
  
  if err != nil {
    return false, err
  }
  
  // Remove temporary list
  deleteListService := c.Client.NewDeleteListService()
  deleteListService.Id(tempList.Id)
  err = deleteListService.Do(context.Background())

  if err != nil {
    return false, err
  }

	return true, nil
}

