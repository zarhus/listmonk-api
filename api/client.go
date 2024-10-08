// File: client.go
package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	listmonk "github.com/Exayn/go-listmonk"
)

// Map subscription types to email templates
var subscriptionMap = map[string]string{
	"MSI":                   "desktop",
	"MSI_heads":             "desktop",
	"Optiplex_Dasharo_UEFI": "desktop",
	"novacustom_heads":      "laptop",
	"PCEngines":             "network",
	"PCEngines_seabios":     "network",
}

type APIClient struct {
	BaseURL        string
	Username       *string
	Password       *string
	Client         *listmonk.Client
	MailingListIDs sync.Map
}

type SubscriberInput struct {
	Name  string                 `json:"name"`
	Email string                 `json:"email"`
	Lists []string               `json:"lists"`
	Attrs map[string]interface{} `json:"attrs"`
}

func mapping[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i := range ts {
		us[i] = f(ts[i])
	}
	return us
}

func NewAPIClient(baseURL string, username, password *string) *APIClient {
	client := &APIClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Client:   listmonk.NewClient(baseURL, username, password),
	}

	err := client.setListIDs()
	if err != nil {
		panic(err)
	}
	return client
}

func (c *APIClient) setListIDs() error {
	getListsService := c.Client.NewGetListsService()
	lists, err := getListsService.Do(context.Background())
	if err != nil {
		return err
	}

	for _, list := range lists {
		c.MailingListIDs.Store(list.Name, list.Id)
	}

	return nil
}

// Create a new list and update sync.Map
func (c *APIClient) createList(name string) (*listmonk.List, error) {
	createListService := c.Client.NewCreateListService()
	createListService.Name(name)
	list, err := createListService.Do(context.Background())
	if err != nil {
		return nil, err
	}

	c.MailingListIDs.Store(list.Name, list.Id)
	return list, nil
}

func (c *APIClient) getListID(name string) (uint, error) {
	if val, ok := c.MailingListIDs.Load(name); ok {
		return val.(uint), nil
	}
	return 0, fmt.Errorf("list not found: %s", name)
}

// Create a new subscriber and add them to mailing lists with specified names, including attributes
func (c *APIClient) CreateSubscriber(name string, email string, lists []string, attrs map[string]interface{}) (uint, error) {
	listIDs := mapping(lists, func(listName string) uint {
		id, err := c.getListID(listName)
		if err != nil {
			panic(err)
		}
		return id
	})
	return c.CreateSubscriberListIDs(name, email, listIDs, attrs)
}

// Create a new subscriber and add them to mailing lists with specified IDs, including attributes
func (c *APIClient) CreateSubscriberListIDs(name string, email string, lists []uint, attrs map[string]interface{}) (uint, error) {
	service := c.Client.NewCreateSubscriberService()
	service.Email(email)
	service.Name(name)
	service.ListIds(lists)
	service.Attributes(attrs) // Set the attributes here
	LogInfof("Adding subscriber %s to Listmonk.\n", name)
	subscriber, err := service.Do(context.Background())
	if err != nil {
		return 0, err
	}
	LogOKln("Success")
	return subscriber.Id, nil
}

// Create a new subscriber from JSON data
func (c *APIClient) CreateSubscriberFromJSON(jsonData []byte) (uint, error) {
	var input SubscriberInput
	err := json.Unmarshal(jsonData, &input)
	if err != nil {
		return 0, err
	}
	return c.CreateSubscriber(input.Name, input.Email, input.Lists, input.Attrs)
}

func (c *APIClient) CreateCampaign(name, subject string, lists []uint, content, contentType string) (uint, error) {
	service := c.Client.NewCreateCampaignService()
	service.Name(name)
	service.Subject(subject)
	service.Lists(lists)
	service.Body(content)
	service.ContentType(contentType)
	LogInfof("Creating campaign: %s.\n", name)
	campaign, err := service.Do(context.Background())
	if err != nil {
		return 0, err
	}
	LogOKln("Campaign created.")
	return campaign.Id, nil
}

// Create a new campaign with HTML content
func (c *APIClient) CreateCampaignHTML(name string, subject string, lists []uint, content string) (uint, error) {
	return c.CreateCampaign(name, subject, lists, content, "html")
}

func (c *APIClient) deleteCampaign(campaign *listmonk.Campaign) error {
	deleteCampaignService := c.Client.NewDeleteCampaignService()
	deleteCampaignService.Id(campaign.Id)
	return deleteCampaignService.Do(context.Background())
}

// Get users who subscribed after campaign was launched
func (c *APIClient) getSubscribersAfterLaunch(campaign *listmonk.Campaign) ([]*listmonk.Subscriber, error) {
	LogInfof("Checking for already-existing incremental campaign.\n")
	getCampaignsService := c.Client.NewGetCampaignsService()
	campaigns, err := getCampaignsService.Do(context.Background())
	if err != nil {
		return nil, err
	}

	var incCampaign *listmonk.Campaign = nil
	var incCampaignLaunchDate string
	for _, camp := range campaigns {
		if camp.Name == campaign.Name+"_inc" {
			incCampaign = camp
			incCampaignLaunchDate = incCampaign.StartedAt.Format("2006-01-02T15:04:05.999999-07:00")
			c.deleteCampaign(camp)
			break
		}
	}

	m := func(l listmonk.CampaignList) string { return strconv.Itoa(int(l.Id)) }
	listIDs := mapping(campaign.Lists, m)

	launchDate := campaign.StartedAt.Format("2006-01-02T15:04:05.999999-07:00")

	var query string
	if incCampaign != nil {
		query = fmt.Sprintf("id IN (SELECT subscriber_id FROM subscriber_lists WHERE created_at > '%v' AND list_id IN (%s))", incCampaignLaunchDate, strings.Join(listIDs, ","))
	} else {
		query = fmt.Sprintf("id IN (SELECT subscriber_id FROM subscriber_lists WHERE created_at >'%v' AND list_id IN (%s))", launchDate, strings.Join(listIDs, ","))
	}

	getSubscribersService := c.Client.NewGetSubscribersService()
	getSubscribersService.Query(query)

	LogInfoln("Fetching new subscribers.")
	return getSubscribersService.Do(context.Background())
}

func (c *APIClient) addSubscribersToList(subscribers []*listmonk.Subscriber, list *listmonk.List) error {
	subscribersListsService := c.Client.NewUpdateSubscribersListsService()

	m := func(s *listmonk.Subscriber) uint { return s.Id }

	subscriberIDs := mapping(subscribers, m)
	subscribersListsService.ListIds([]uint{list.Id})
	subscribersListsService.Ids(subscriberIDs)
	subscribersListsService.Action("add")
	_, err := subscribersListsService.Do(context.Background())
	return err
}

// Create incremental campaign from an existing one
func (c *APIClient) createIncCampaign(campaign *listmonk.Campaign, tempList *listmonk.List) (*listmonk.Campaign, error) {
	createCampaignService := c.Client.NewCreateCampaignService()
	createCampaignService.Name(campaign.Name + "_inc")

	// Copy fields from original campaign
	createCampaignService.Subject(campaign.Subject)
	createCampaignService.Type(campaign.Type)
	createCampaignService.Body(campaign.Body)
	createCampaignService.Lists([]uint{tempList.Id})
	createCampaignService.ContentType(campaign.ContentType)
	createCampaignService.FromEmail(campaign.FromEmail)
	createCampaignService.Messenger(campaign.Messenger)
	createCampaignService.TemplateId(campaign.TemplateId)
	createCampaignService.Tags(campaign.Tags)

	LogInfoln("Creating incremental campaign.")
	return createCampaignService.Do(context.Background())
}

// Launch campaign or send finished campaign to newly subscribed users
func (c *APIClient) LaunchCampaign(id uint) (bool, error) {
	// Fetch campaign launch date and mailing lists
	getCampaignService := c.Client.NewGetCampaignService()
	getCampaignService.Id(id)

	LogInfoln("Fetching campaign data.")

	campaign, err := getCampaignService.Do(context.Background())
	if err != nil {
		return false, err
	}

	if len(campaign.Lists) == 0 {
		LogWarningln("The campaign targets no mailing lists! Aborting.")
		return false, nil
	}

	// If campaign has never been launched - launch it
	if campaign.StartedAt.IsZero() {
		updateCampaignStatusService := c.Client.NewUpdateCampaignStatusService()
		updateCampaignStatusService.Id(id)
		updateCampaignStatusService.Status("running")

		LogInfoln("The campaign has not been launched before. Launching now.")
		_, err := updateCampaignStatusService.Do(context.Background())
		if err != nil {
			return false, err
		}
		LogOKf("Successfully launched campaign %s.\n", campaign.Name)
		return true, nil
	}

	subscribers, err := c.getSubscribersAfterLaunch(campaign)
	if err != nil {
		return false, err
	}

	if len(subscribers) == 0 {
		LogWarningln("No new subscribers since last launch! Aborting.")
		return false, nil
	}

	// Create temporary list
	tempList, err := c.createList("temp_list")
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

	// Launch incremental campaign
	updateCampaignStatusService := c.Client.NewUpdateCampaignStatusService()
	updateCampaignStatusService.Id(incCampaign.Id)
	updateCampaignStatusService.Status("running")

	LogInfoln("Launching incremental campaign.")
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

	LogOKf("Successfully resumed campaign %s.\n", campaign.Name)
	return true, nil
}

// Delete subscriber by ID
func (c *APIClient) DeleteSubscriberID(id uint) error {
	deleteSubscriberService := c.Client.NewDeleteSubscriberService()
	deleteSubscriberService.Id(id)
	_, err := deleteSubscriberService.Do(context.Background())
	LogOKln("Successfully deleted subscriber.")
	return err
}

// Get ID of subscriber with given email
func (c *APIClient) getSubscriberID(email string) (uint, error) {
	getSubscribersService := c.Client.NewGetSubscribersService()
	getSubscribersService.Query(fmt.Sprintf("subscribers.email = '%s'", email))
	subscribers, err := getSubscribersService.Do(context.Background())
	if err != nil {
		return 0, err
	}

	if len(subscribers) == 0 {
		return 0, fmt.Errorf("Could not find subscriber with email %s", email)
	}
	if len(subscribers) > 1 {
		return 0, fmt.Errorf("Query returned too many results")
	}
	return subscribers[0].Id, nil
}

// Delete subscriber by email
func (c *APIClient) DeleteSubscriberEmail(email string) error {
	LogInfof("Deleting subscriber %s from Listmonk.\n", email)
	subscriberID, err := c.getSubscriberID(email)
	if err != nil {
		return err
	}

	return c.DeleteSubscriberID(subscriberID)
}

// Add subscribers from CSV file.
// Assumes CSV has columns: Duration (years), Email, Date received, Expiration date
func (c *APIClient) AddSubscribersFromCSV(path, list string, passwords map[string]string) error {
	LogInfoln("Adding subscribers from CSV to Listmonk.")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	if len(records) < 1 {
		return fmt.Errorf("no records found")
	}

	// Skip the header row
	records = records[1:]

	for _, record := range records {
		if len(record) < 4 {
			return fmt.Errorf("invalid record length: %v", record)
		}

		duration := record[0]
		email := record[1]
		received := record[2]
		expiration := record[3]
		attrs := map[string]interface{}{
			"key": passwords[email],
			fmt.Sprintf("duration_%s", strings.ToLower(list)):        duration,
			fmt.Sprintf("created_%s", strings.ToLower(list)):         received,
			fmt.Sprintf("expiration_date_%s", strings.ToLower(list)): expiration,
		}

		// If subscriber does not already exists
		if _, err := c.getSubscriberID(email); err != nil {
			LogInfof("Subscriber %s does not exist in Listmonk. Adding now.\n", email)
			_, err = c.CreateSubscriber(email, email, []string{list}, attrs)
			if err != nil {
				return err
			}
			LogOKf("Added subscriber %s.\n", email)
			continue
		}
		err := c.AddToList(email, list)
		if err != nil {
			return err
		}
		LogInfof("Adding new subscription for subscriber %s.\n", email)
		err = c.SetAttribute(email, fmt.Sprintf("duration_%s", strings.ToLower(list)), duration)
		if err != nil {
			return err
		}
		err = c.SetAttribute(email, fmt.Sprintf("created_%s", strings.ToLower(list)), received)
		if err != nil {
			return err
		}
		err = c.SetAttribute(email, fmt.Sprintf("expiration_date_%s", strings.ToLower(list)), expiration)
		if err != nil {
			return err
		}
		LogOKln("Success.")
	}
	return nil
}

// Create campaign from HTML on a list given by name.
func (c *APIClient) CreateCampaignHTMLOnListName(campaignName string, subject string, listName string, content string) (uint, error) {
	getListsService := c.Client.NewGetListsService()
	lists, err := getListsService.Do(context.Background())
	if err != nil {
		return 0, err
	}

	for _, list := range lists {
		if list.Name == listName {
			return c.CreateCampaignHTML(campaignName, subject, []uint{list.Id}, content)
		}
	}
	return 0, fmt.Errorf("Could not find list %s", listName)
}

// Modify subscriber list memberships.
func (c *APIClient) updateSubscriberLists(email string, listNames []string, action string) error {
	subscriberID, err := c.getSubscriberID(email)
	if err != nil {
		return err
	}

	subscriber, err := c.GetSubscriber(subscriberID)
	if len(subscriber.Lists) == 1 && action == "remove" {
		return c.DeleteSubscriberID(subscriberID)
	}

	listIDs := make([]uint, len(listNames))
	for i, listName := range listNames {
		listID, err := c.getListID(listName)
		if err != nil {
			return err
		}
		listIDs[i] = listID
	}

	subscribersListsService := c.Client.NewUpdateSubscribersListsService()
	subscribersListsService.Ids([]uint{subscriberID})
	subscribersListsService.ListIds(listIDs)
	subscribersListsService.Action(action)
	_, err = subscribersListsService.Do(context.Background())
	if err == nil {
		LogOKln("Success")
	}
	return err
}

// Remove subscriber from a list. Deletes subscriber if removed from all lists.
func (c *APIClient) RemoveFromList(email string, listName string) error {
	LogInfof("Removing subscriber %s from list %s.\n", email, listName)
	return c.updateSubscriberLists(email, []string{listName}, "remove")
}

// Add subscriber to list
func (c *APIClient) AddToList(email string, listName string) error {
	LogInfof("Adding subscriber %s to list %s.\n", email, listName)
	return c.updateSubscriberLists(email, []string{listName}, "add")
}

// Launch campaign on list
func (c *APIClient) LaunchCampaignListName(listName string) (bool, error) {
	getCampaignsService := c.Client.NewGetCampaignsService()
	campaigns, err := getCampaignsService.Do(context.Background())
	if err != nil {
		return false, err
	}

	for _, campaign := range campaigns {
		for _, list := range campaign.Lists {
			if list.Name == listName {
				return c.LaunchCampaign(campaign.Id)
			}
		}
	}
	return false, fmt.Errorf("Could not find list %s", listName)
}

// Add subscriber to list and launch campaign
func (c *APIClient) AddAndSendCampaign(email string, listName string) (bool, error) {
	err := c.AddToList(email, listName)
	if err != nil {
		return false, err
	}

	return c.LaunchCampaignListName(listName)
}

// Add subscribers from CSV and launch campaigns of affected lists. Return true
// if all campaigns were launched successfully
func (c *APIClient) AddCSVAndSendCampaign(path, list string, passwords map[string]string) (bool, error) {
	err := c.AddSubscribersFromCSV(path, list, passwords)
	if err != nil {
		return false, err
	}

	return c.LaunchCampaignListName(list)
}

// GetSubscriber retrieves a subscriber by ID
func (c *APIClient) GetSubscriber(subscriberID uint) (*listmonk.Subscriber, error) {
	service := c.Client.NewGetSubscriberService()
	service.Id(subscriberID)
	subscriber, err := service.Do(context.Background())
	if err != nil {
		return nil, err
	}
	return subscriber, nil
}

// GetSubscriberAttributes retrieves a subscriber's attributes by ID
func (c *APIClient) GetSubscriberAttributes(subscriberID uint) (map[string]interface{}, error) {
	subscriber, err := c.GetSubscriber(subscriberID)
	if err != nil {
		return nil, err
	}
	return subscriber.Attributes, nil
}

func (c *APIClient) GetSubscriberAttributesEmail(email string) (map[string]interface{}, error) {
	subscriberID, err := c.getSubscriberID(email)
	if err != nil {
		return nil, err
	}
	return c.GetSubscriberAttributes(subscriberID)
}

// UpdateSubscriberAttributes updates a subscriber's attributes
func (c *APIClient) UpdateSubscriberAttributes(subscriberID uint, attrs map[string]interface{}) error {
	// Get the current subscriber details
	subscriber, err := c.GetSubscriber(subscriberID)
	if err != nil {
		return err
	}

	service := c.Client.NewUpdateSubscriberService()
	service.Id(subscriberID)
	service.Email(subscriber.Email) // Set the email
	service.Name(subscriber.Name)   // Set the name
	service.Status(subscriber.Status)
	// Extract list IDs from subscriber's lists
	listIDs := mapping(subscriber.Lists, func(l listmonk.SubscriberList) uint { return l.Id })
	service.ListIds(listIDs)
	service.Attributes(attrs) // Use Attribs instead of Attrs

	_, err = service.Do(context.Background())
	return err
}

func (c *APIClient) UpdateSubscriberAttributesEmail(email string, attrs map[string]interface{}) error {
	subscriberID, err := c.getSubscriberID(email)
	if err != nil {
		return err
	}
	return c.UpdateSubscriberAttributes(subscriberID, attrs)
}

// Set a single attribute for subscriber
func (c *APIClient) SetAttribute(email, key, value string) error {
	attrs, err := c.GetSubscriberAttributesEmail(email)
	if err != nil {
		return err
	}
	attrs[key] = value
	return c.UpdateSubscriberAttributesEmail(email, attrs)
}

func (c *APIClient) ListSubscribers(listName string) ([]map[string]string, error) {
	LogInfof("Fetching subscribers of list %s.\n", listName)
	var result []map[string]string
	listID, err := c.getListID(listName)
	getSubscribersService := c.Client.NewGetSubscribersService()
	subscribers, err := getSubscribersService.Do(context.Background())
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("expiration_date_%s", strings.ToLower(listName))
	for _, subscriber := range subscribers {
		for _, list := range subscriber.Lists {
			if list.Id == listID {
				expirationString, ok := subscriber.Attributes[key].(string)
				if !ok {
					return nil, fmt.Errorf("expiration_date is not a string")
				}

				result = append(result, map[string]string{
					"id":              strconv.Itoa(int(subscriber.Id)),
					"email":           subscriber.Email,
					"expiration_date": expirationString,
				})
				break
			}
		}
	}
	LogOKln("Success")
	return result, nil
}

func (c *APIClient) DeleteList(name string) error {
	LogInfof("Deleting list: %s.\n", name)
	listID, err := c.getListID(name)
	if err != nil {
		return fmt.Errorf("Could not delete list: %s", name)
	}
	deleteListService := c.Client.NewDeleteListService()
	deleteListService.Id(listID)
	err = deleteListService.Do(context.Background())
	if err == nil {
		LogOKln("Success")
	}
	return err
}

func (c *APIClient) formatEmailTemplate(emailType, name, password, expiration_date string) (string, error) {
	var filePath string
	switch emailType {
	case "desktop":
		filePath = "../config/dpp_desktop"
	case "laptop":
		filePath = "../config/dpp_laptop"
	case "network":
		filePath = "../config/dpp_network"
	default:
		return "", fmt.Errorf("Wrong email type! Available types: desktop, laptop, network")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("Could not read file: %w", err)
	}

	text := string(content)
	text = strings.ReplaceAll(text, "${key}", password)
	text = strings.ReplaceAll(text, "${expiration_date}", expiration_date)
	text = strings.ReplaceAll(text, "${name}", name)
	return text, nil
}

func (c *APIClient) SendEmail(subscriptionType, subscriberEmail, name string) error {
	LogInfof("Sending email to subscriber %s.\n", subscriberEmail)
	attrs, err := c.GetSubscriberAttributesEmail(subscriberEmail)
	if err != nil {
		return err
	}
	password, ok := attrs["key"].(string)
	if !ok {
		return fmt.Errorf("User key is not a string or does not exit")
	}
	expiration_date, ok := attrs[fmt.Sprintf("expiration_date_%s", strings.ToLower(subscriptionType))].(string)
	if !ok {
		return fmt.Errorf("Expiration date is not a string or does not exist")
	}
	emailType := subscriptionMap[subscriptionType]
	content, err := c.formatEmailTemplate(emailType, name, password, expiration_date)
	if err != nil {
		return err
	}
	listname := "tmplist"
	list, err := c.createList(listname)
	if err != nil {
		return err
	}
	err = c.AddToList(subscriberEmail, listname)
	if err != nil {
		return err
	}
	campaignID, err := c.CreateCampaign("tmpcampaign", "TBD", []uint{list.Id}, content, "plain")
	if err != nil {
		return err
	}
	_, err = c.LaunchCampaign(campaignID)
	if err != nil {
		return err
	}
	getCampaignService := c.Client.NewGetCampaignService()
	getCampaignService.Id(campaignID)
	campaign, err := getCampaignService.Do(context.Background())
	if err != nil {
		return err
	}
	err = c.deleteCampaign(campaign)
	if err != nil {
		return err
	}
	return c.DeleteList(listname)
}
