// File: client_test.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Exayn/go-listmonk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func initAPIClient() *APIClient {
	username := ""
	password := ""
	hostname, exists := os.LookupEnv("LISTMONK_HOSTNAME")
	if !exists {
		hostname = "0.0.0.0"
	}
	return NewAPIClient(fmt.Sprintf("http://%s:9000", hostname), &username, &password)
}

func deleteCampaign(client *APIClient, id uint) {
	deleteCampaignService := client.Client.NewDeleteCampaignService()
	deleteCampaignService.Id(id)
	err := deleteCampaignService.Do(context.Background())
	check(err)
}

func deleteSubscriber(client *APIClient, id uint) {
	deleteSubscriberService := client.Client.NewDeleteSubscriberService()
	deleteSubscriberService.Id(id)
	_, err := deleteSubscriberService.Do(context.Background())
	check(err)
}

func deleteList(client *APIClient, id uint) {
	deleteListService := client.Client.NewDeleteListService()
	deleteListService.Id(id)
	err := deleteListService.Do(context.Background())
	check(err)
}

func TestCreateSubscriberListIDs(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		name := "John Doe"
		email := "john.doe@example.com"
		list_ids := []uint{1, 3}
		attrs := map[string]interface{}{
			"age":    30,
			"gender": "male",
		}

		id, err := client.CreateSubscriberListIDs(name, email, list_ids, attrs)

		assert.NoError(t, err)

		// Clean up
		defer deleteSubscriber(client, id)

		// Get actual subscriber ID
		getSubscribersService := client.Client.NewGetSubscribersService()
		getSubscribersService.Query(fmt.Sprintf("subscribers.email = '%s'", email))
		subscribers, err := getSubscribersService.Do(context.Background())

		assert.NoError(t, err)
		assert.Equal(t, subscribers[0].Id, id)

		// Convert actual "age" to int for comparison
		if ageFloat, ok := subscribers[0].Attributes["age"].(float64); ok {
			subscribers[0].Attributes["age"] = int(ageFloat)
		}

		assert.EqualValues(t, attrs, subscribers[0].Attributes)
	})

	t.Run("incorrect e-mail", func(t *testing.T) {

		name := "John Doe"
		email := "not an e-mail"
		list_ids := []uint{1, 3}
		attrs := map[string]interface{}{
			"age":    30,
			"gender": "male",
		}

		id, err := client.CreateSubscriberListIDs(name, email, list_ids, attrs)

		assert.Error(t, err)
		assert.Equal(t, uint(0), id)
	})

}

func TestCreateSubscriberFromJSON(t *testing.T) {
	client := initAPIClient()

	t.Run("valid JSON input", func(t *testing.T) {
		jsonData := []byte(`{
			"name": "Jane Doe",
			"email": "jane.doe@example.com",
			"lists": ["TestList"],
			"attrs": {
				"age": 28,
				"preferences": {
					"newsletter": true,
					"sms": false
				}
			}
		}`)

		// Create TestList if it doesn't exist
		listName := "TestList"
		_, err := client.getListID(listName)
		if err != nil {
			_, err := client.createList(listName)
			assert.NoError(t, err)
		}

		id, err := client.CreateSubscriberFromJSON(jsonData)
		assert.NoError(t, err)
		defer deleteSubscriber(client, id)

		// Fetch the subscriber and verify attributes
		getSubscriberService := client.Client.NewGetSubscriberService()
		getSubscriberService.Id(id)
		subscriber, err := getSubscriberService.Do(context.Background())
		assert.NoError(t, err)

		var expectedAttrs map[string]interface{}
		json.Unmarshal([]byte(`{
			"age": 28,
			"preferences": {
				"newsletter": true,
				"sms": false
			}
		}`), &expectedAttrs)

		assert.Equal(t, expectedAttrs, subscriber.Attributes)
	})

	t.Run("invalid JSON input", func(t *testing.T) {
		jsonData := []byte(`{
			"name": "Jane Doe",
			"email": "jane.doe@example.com",
			"lists": ["TestList"],
			"attrs": {
				"age": 28,
				"preferences": {
					"newsletter": true,
					"sms": false
				}
			`)

		id, err := client.CreateSubscriberFromJSON(jsonData)
		assert.Error(t, err)
		assert.Equal(t, uint(0), id)
	})
}

func TestAddSubscribersFromCSV(t *testing.T) {
	client := initAPIClient()

	t.Run("valid CSV input", func(t *testing.T) {
    listName := "MSI"
		list, err := client.createList(listName)
		assert.NoError(t, err)
		defer deleteList(client, list.Id)
		// Prepare a temporary CSV file
		csvContent := `duration,email,date_received,expiration
1,test1@example.com,2024-09-07,2025-09-07
2,test2@example.com,2024-05-12,2026-05-12
1,test3@example.com,2024-07-01,2025-07-01`
		csvFile, err := os.CreateTemp("", "subscribers_*.csv")
		assert.NoError(t, err)
		defer os.Remove(csvFile.Name())

		_, err = csvFile.WriteString(csvContent)
		assert.NoError(t, err)
		csvFile.Close()

    passwords := map[string]string{
      "test1@example.com": "password1",
      "test2@example.com": "password2",
      "test3@example.com": "password3",
    }
		err = client.AddSubscribersFromCSV(csvFile.Name(), listName, passwords)
		assert.NoError(t, err)

		// Cleanup subscribers
		subscriberEmails := []string{"test1@example.com", "test2@example.com", "test3@example.com"}
		for _, email := range subscriberEmails {
			id, err := client.getSubscriberID(email)
			assert.NoError(t, err)
			deleteSubscriber(client, id)
		}
	})
}

func TestCreateSubscriberWithAttributes(t *testing.T) {
	client := initAPIClient()

	t.Run("create subscriber with attributes", func(t *testing.T) {
		name := "Alice"
		email := fmt.Sprintf("alice+%d@example.com", time.Now().UnixNano())
		lists := []string{"TestList"}
		attrs := map[string]interface{}{
			"subscription_level": "gold",
			"signup_date":        "2023-10-10",
		}

		// Create TestList if it doesn't exist
		listName := "TestList"
		_, err := client.getListID(listName)
		if err != nil {
			_, err = client.createList(listName)
			require.NoError(t, err)
		}

		// Ensure no existing subscriber with the same email
		existingID, _ := client.getSubscriberID(email)
		if existingID != 0 {
			_ = client.DeleteSubscriberID(existingID)
		}

		id, err := client.CreateSubscriber(name, email, lists, attrs)
		require.NoError(t, err)
		defer func() {
			err := client.DeleteSubscriberID(id)
			require.NoError(t, err)
		}()

		// Fetch the subscriber and verify attributes
		getSubscriberService := client.Client.NewGetSubscriberService()
		getSubscriberService.Id(id)
		subscriber, err := getSubscriberService.Do(context.Background())
		require.NoError(t, err)
		assert.Equal(t, attrs, subscriber.Attributes)
	})
}

func TestCreateCampaignHTML(t *testing.T) {
	client := initAPIClient()
	t.Run("correct input data", func(t *testing.T) {
		htmlString := ` <!DOCTYPE html>
<html>
<body>

<h1>My First Heading</h1>
<p>My first paragraph.</p>

</body>
</html>`

		name := "My campaign"
		subject := "Subject of campaign"
		// Create a test list
		listName := "TestList"
		list, err := client.createList(listName)
		assert.NoError(t, err)
		defer deleteList(client, list.Id)

		lists := []uint{list.Id}

		id, err := client.CreateCampaignHTML(name, subject, lists, htmlString)

		assert.NoError(t, err)

		// Clean up
		defer deleteCampaign(client, id)

		// Check if campaign was added and compare content
		getCampaignsService := client.Client.NewGetCampaignsService()
		campaigns, err := getCampaignsService.Do(context.Background())

		assert.NoError(t, err)

		var campaign *listmonk.Campaign = nil
		for _, c := range campaigns {
			if c.Name == name {
				campaign = c
				break
			}
		}

		assert.Equal(t, id, campaign.Id)
		assert.Equal(t, htmlString, campaign.Body)
	})
}

func TestDeleteCampaign(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create a campaign
		name := "My campaign"
		subject := "Subject of campaign"
		body := "Body"
		// Create a test list
		listName := "TestList"
		list, err := client.createList(listName)
		assert.NoError(t, err)
		defer deleteList(client, list.Id)

		lists := []uint{list.Id}
		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name(name)
		createCampaignService.Subject(subject)
		createCampaignService.Body(body)
		createCampaignService.Lists(lists)
		campaign, err := createCampaignService.Do(context.Background())
		check(err)

		// Delete campaign
		err = client.deleteCampaign(campaign)

		assert.NoError(t, err)

		// Make sure the campaign was actually deleted
		getCampaignService := client.Client.NewGetCampaignService()
		getCampaignService.Id(campaign.Id)
		_, err = getCampaignService.Do(context.Background())

		assert.Error(t, err)

		// Delete campaign manually if it wasn't deleted
		if err == nil {
			deleteCampaign(client, campaign.Id)
		}
	})
}

func TestAddSubscribersToList(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create subscribers and list
		createListService := client.Client.NewCreateListService()
		createListService.Name("tmp")
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list ID in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		subscribers := make([]*listmonk.Subscriber, 2)
		for i := 0; i < cap(subscribers); i++ {
			createSubscriberService := client.Client.NewCreateSubscriberService()
			createSubscriberService.Name(fmt.Sprintf("User %d", i))
			createSubscriberService.Email(fmt.Sprintf("test%d@example.com", i))
			createSubscriberService.Status("enabled")
			sub, err := createSubscriberService.Do(context.Background())
			check(err)
			subscribers[i] = sub
			defer deleteSubscriber(client, sub.Id)
		}

		// Add subscribers to list
		err = client.addSubscribersToList(subscribers, list)
		assert.NoError(t, err)

		//Check if subscribers were added
		for _, subscriber := range subscribers {
			getSubscriberService := client.Client.NewGetSubscriberService()
			getSubscriberService.Id(subscriber.Id)
			updatedSubscriber, _ := getSubscriberService.Do(context.Background())
			listIDs := make([]uint, len(updatedSubscriber.Lists))
			for index, l := range updatedSubscriber.Lists {
				listIDs[index] = l.Id
			}
			assert.Containsf(t, listIDs, list.Id, "Subscriber %s is not subscribed to list", updatedSubscriber.Name)
		}

	})

	t.Run("no such user", func(t *testing.T) {
		// Set up test - create list
		createListService := client.Client.NewCreateListService()
		createListService.Name("tmp")
		list, err := createListService.Do(context.Background())
		check(err)
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		// This subscriber does not exist
		subscribers := []*listmonk.Subscriber{
			{
				Id: 999999, // Assuming this ID doesn't exist
			},
		}

		err = client.addSubscribersToList(subscribers, list)
		assert.Error(t, err)
	})
}

func TestCreateIncCampaign(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create base campaign and temp list
		// Create a test list
		listName := "TestList"
		list, err := client.createList(listName)
		assert.NoError(t, err)
		defer deleteList(client, list.Id)

		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name("Base campaign")
		createCampaignService.Subject("Subject")
		createCampaignService.Body("Campaign body")
		createCampaignService.Lists([]uint{list.Id})
		baseCampaign, err := createCampaignService.Do(context.Background())
		check(err)
		defer deleteCampaign(client, baseCampaign.Id)

		createListService := client.Client.NewCreateListService()
		createListService.Name("tmp")
		tempList, err := createListService.Do(context.Background())
		check(err)

		// Store list in sync.Map
		client.MailingListIDs.Store(tempList.Name, tempList.Id)
		defer deleteList(client, tempList.Id)

		incCampaign, err := client.createIncCampaign(baseCampaign, tempList)

		assert.NoError(t, err)
		assert.Equal(t, baseCampaign.Name+"_inc", incCampaign.Name)
		assert.Equal(t, baseCampaign.Subject, incCampaign.Subject)
		assert.Equal(t, baseCampaign.Body, incCampaign.Body)

		listIDs := make([]uint, len(incCampaign.Lists))
		for index, l := range incCampaign.Lists {
			listIDs[index] = l.Id
		}
		assert.Equal(t, []uint{tempList.Id}, listIDs)
	})
}

func TestLaunchCampaign(t *testing.T) {
	client := initAPIClient()

	t.Run("resume with correct data", func(t *testing.T) {
		// Set up test - create and launch campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		subscribers := make([]*listmonk.Subscriber, 4)
		for i := 0; i < cap(subscribers); i++ {
			createSubscriberService := client.Client.NewCreateSubscriberService()
			createSubscriberService.Name(fmt.Sprintf("User %d", i))
			createSubscriberService.Email(fmt.Sprintf("test%d@example.com", i))
			createSubscriberService.Status("enabled")
			// Only 2 subscribers will subscribe to the list now
			if i < 2 {
				createSubscriberService.ListIds([]uint{list.Id})
			}
			sub, err := createSubscriberService.Do(context.Background())
			check(err)
			subscribers[i] = sub
			defer deleteSubscriber(client, sub.Id)
		}

		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name("Base campaign")
		createCampaignService.Subject("Subject")
		createCampaignService.Body("Campaign body")
		createCampaignService.Lists([]uint{list.Id})
		baseCampaign, err := createCampaignService.Do(context.Background())
		check(err)
		defer deleteCampaign(client, baseCampaign.Id)

		updateCampaignStatusService := client.Client.NewUpdateCampaignStatusService()
		updateCampaignStatusService.Id(baseCampaign.Id)
		updateCampaignStatusService.Status("running")
		_, err = updateCampaignStatusService.Do(context.Background())
		check(err)

		// Wait for the campaign to finish
		time.Sleep(5 * time.Second)

		// The remaining 2 users will now subscribe to the list
		subscriberIDs := make([]uint, 2)
		for i := 2; i < len(subscribers); i++ {
			subscriberIDs[i-2] = subscribers[i].Id
		}

		subscribersListsService := client.Client.NewUpdateSubscribersListsService()
		subscribersListsService.ListIds([]uint{list.Id})
		subscribersListsService.Ids(subscriberIDs)
		subscribersListsService.Action("add")
		_, err = subscribersListsService.Do(context.Background())
		check(err)

		resumed, err := client.LaunchCampaign(baseCampaign.Id)

		assert.NoError(t, err)
		assert.True(t, resumed)

		// Check if incremental campaign has been created and sent
		getCampaignsService := client.Client.NewGetCampaignsService()
		campaigns, err := getCampaignsService.Do(context.Background())
		check(err)

		var campaign *listmonk.Campaign = nil
		for _, c := range campaigns {
			if c.Name == baseCampaign.Name+"_inc" {
				campaign = c
				break
			}
		}
		defer deleteCampaign(client, campaign.Id)

		assert.NotNil(t, campaign)
		assert.True(t, campaign.Status == "running" || campaign.Status == "finished")
		assert.Equal(t, baseCampaign.Body, campaign.Body)
	})

	t.Run("resume no new subscribers", func(t *testing.T) {
		// Set up test - create and launch campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist_no_new")
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		// Create subscribers and add them to the list
		for i := 0; i < 2; i++ {
			createSubscriberService := client.Client.NewCreateSubscriberService()
			createSubscriberService.Name(fmt.Sprintf("User_no_new %d", i))
			createSubscriberService.Email(fmt.Sprintf("test_no_new%d@example.com", i))
			createSubscriberService.Status("enabled")
			createSubscriberService.ListIds([]uint{list.Id})
			sub, err := createSubscriberService.Do(context.Background())
			check(err)
			defer deleteSubscriber(client, sub.Id)
		}

		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name("Base campaign no new")
		createCampaignService.Subject("Subject")
		createCampaignService.Body("Campaign body")
		createCampaignService.Lists([]uint{list.Id})
		baseCampaign, err := createCampaignService.Do(context.Background())
		check(err)
		defer deleteCampaign(client, baseCampaign.Id)

		updateCampaignStatusService := client.Client.NewUpdateCampaignStatusService()
		updateCampaignStatusService.Id(baseCampaign.Id)
		updateCampaignStatusService.Status("running")
		_, err = updateCampaignStatusService.Do(context.Background())
		check(err)

		// Wait for the campaign to finish
		time.Sleep(5 * time.Second)

		// No new subscribers added

		resumed, err := client.LaunchCampaign(baseCampaign.Id)

		assert.NoError(t, err)
		assert.False(t, resumed)
	})

	t.Run("no such campaign", func(t *testing.T) {
		resumed, err := client.LaunchCampaign(999999) // Assuming this ID doesn't exist

		assert.Error(t, err)
		assert.False(t, resumed)
	})

	t.Run("first launch correct", func(t *testing.T) {
		// Set up test - create campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist_first_launch")
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		// Create subscribers and add them to the list
		for i := 0; i < 2; i++ {
			createSubscriberService := client.Client.NewCreateSubscriberService()
			createSubscriberService.Name(fmt.Sprintf("User_first_launch %d", i))
			createSubscriberService.Email(fmt.Sprintf("test_first_launch%d@example.com", i))
			createSubscriberService.Status("enabled")
			createSubscriberService.ListIds([]uint{list.Id})
			sub, err := createSubscriberService.Do(context.Background())
			check(err)
			defer deleteSubscriber(client, sub.Id)
		}

		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name("Base campaign first launch")
		createCampaignService.Subject("Subject")
		createCampaignService.Body("Campaign body")
		createCampaignService.Lists([]uint{list.Id})
		baseCampaign, err := createCampaignService.Do(context.Background())
		check(err)
		defer deleteCampaign(client, baseCampaign.Id)

		// Launch campaign
		resumed, err := client.LaunchCampaign(baseCampaign.Id)

		assert.NoError(t, err)
		assert.True(t, resumed)

		// Check if campaign has been launched
		getCampaignsService := client.Client.NewGetCampaignsService()
		campaigns, err := getCampaignsService.Do(context.Background())
		check(err)

		var campaign *listmonk.Campaign = nil
		for _, c := range campaigns {
			if c.Name == baseCampaign.Name {
				campaign = c
				break
			}
		}

		assert.NotNil(t, campaign)
		assert.True(t, campaign.Status == "running" || campaign.Status == "finished")
		assert.Equal(t, baseCampaign.Body, campaign.Body)
	})
}

func TestDeleteSubscriberID(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create subscribers
		subscribers := make([]*listmonk.Subscriber, 2)
		for i := 0; i < cap(subscribers); i++ {
			createSubscriberService := client.Client.NewCreateSubscriberService()
			createSubscriberService.Name(fmt.Sprintf("User %d", i))
			createSubscriberService.Email(fmt.Sprintf("test%d@example.com", i))
			createSubscriberService.Status("enabled")
			sub, err := createSubscriberService.Do(context.Background())
			check(err)
			subscribers[i] = sub
			defer deleteSubscriber(client, sub.Id)
		}
		err := client.DeleteSubscriberID(subscribers[0].Id)

		assert.NoError(t, err)

		// Check if the subscriber was deleted
		getSubscribersService := client.Client.NewGetSubscribersService()
		updatedSubscribers, err := getSubscribersService.Do(context.Background())

		subscriberIDs := make([]uint, len(updatedSubscribers))
		for i, subscriber := range updatedSubscribers {
			subscriberIDs[i] = subscriber.Id
		}

		assert.NotContains(t, subscriberIDs, subscribers[0].Id)
	})
}

func TestDeleteSubscriberEmail(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create subscriber
		email := "user1@test.com"
		createSubscriberService := client.Client.NewCreateSubscriberService()
		createSubscriberService.Name("User 1")
		createSubscriberService.Email(email)
		createSubscriberService.Status("enabled")
		sub, err := createSubscriberService.Do(context.Background())
		check(err)

		// Delete subscriber
		err = client.DeleteSubscriberEmail(email)

		assert.NoError(t, err)

		// If failed - delete subscriber manually to clean up
		if err == nil {
			deleteSubscriber(client, sub.Id)
		}
	})

	t.Run("no such email", func(t *testing.T) {
		err := client.DeleteSubscriberEmail("no.such@email.com")

		assert.Error(t, err)
	})
}

func TestCreateCampaignHTMLOnListName(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create list
		listName := "testlist"
		createListService := client.Client.NewCreateListService()
		createListService.Name(listName)
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list ID in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		htmlString := ` <!DOCTYPE html>
<html>
<body>

<h1>My First Heading</h1>
<p>My first paragraph.</p>

</body>
</html>`

		campaignName := "My campaign"
		subject := "Subject of campaign"
		campaignID, err := client.CreateCampaignHTMLOnListName(campaignName, subject, listName, htmlString)
		assert.NoError(t, err)

		// Clean up
		defer deleteCampaign(client, campaignID)

		// Check if campaign was added and compare content
		getCampaignsService := client.Client.NewGetCampaignsService()
		campaigns, err := getCampaignsService.Do(context.Background())

		assert.NoError(t, err)

		var campaign *listmonk.Campaign = nil
		for _, c := range campaigns {
			if c.Name == campaignName {
				campaign = c
				break
			}
		}

		assert.Equal(t, campaignID, campaign.Id)
		assert.Equal(t, htmlString, campaign.Body)
	})
}

func TestRemoveFromList(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - Create subscriber and lists
		listIDs := make([]uint, 2)
		for i := 0; i < 2; i++ {
			createListService := client.Client.NewCreateListService()
			createListService.Name(fmt.Sprintf("list%d", i))
			list, err := createListService.Do(context.Background())
			check(err)

			// Store the list in sync.Map
			client.MailingListIDs.Store(list.Name, list.Id)
			listIDs[i] = list.Id
			defer deleteList(client, list.Id)
		}
		email := "user@test.com"
		createSubscriberService := client.Client.NewCreateSubscriberService()
		createSubscriberService.Name("User")
		createSubscriberService.Email(email)
		createSubscriberService.Status("enabled")
		createSubscriberService.ListIds(listIDs)
		subscriber, err := createSubscriberService.Do(context.Background())
		check(err)
		defer deleteSubscriber(client, subscriber.Id)

		// Remove subscriber from list0
		err = client.RemoveFromList(email, "list0")

		assert.NoError(t, err)

		// Check if subscriber was removed from list
		getSubscriberService := client.Client.NewGetSubscriberService()
		getSubscriberService.Id(subscriber.Id)
		sub, err := getSubscriberService.Do(context.Background())
		check(err)
		assert.Equal(t, 1, len(sub.Lists))
		assert.Equal(t, "list1", sub.Lists[0].Name)
		assert.Equal(t, listIDs[1], sub.Lists[0].Id)
	})
}

func TestAddToList(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - Create subscriber and list
		createListService := client.Client.NewCreateListService()
		createListService.Name("list")
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		email := "user@test.com"
		createSubscriberService := client.Client.NewCreateSubscriberService()
		createSubscriberService.Name("User")
		createSubscriberService.Email(email)
		createSubscriberService.Status("enabled")
		subscriber, err := createSubscriberService.Do(context.Background())
		check(err)
		defer deleteSubscriber(client, subscriber.Id)

		// Add subscriber to list
		err = client.AddToList(email, "list")

		assert.NoError(t, err)

		// Check if subscriber was added to list
		getSubscriberService := client.Client.NewGetSubscriberService()
		getSubscriberService.Id(subscriber.Id)
		sub, err := getSubscriberService.Do(context.Background())
		check(err)
		assert.Equal(t, 1, len(sub.Lists))
		assert.Equal(t, "list", sub.Lists[0].Name)
		assert.Equal(t, list.Id, sub.Lists[0].Id)
	})
}

func TestAddAndSendCampaign(t *testing.T) {
	client := initAPIClient()

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create list, campaign and subscriber
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name("Campaign name")
		createCampaignService.Subject("Subject")
		createCampaignService.Body("Campaign body")
		createCampaignService.Lists([]uint{list.Id})
		baseCampaign, err := createCampaignService.Do(context.Background())
		check(err)
		defer deleteCampaign(client, baseCampaign.Id)

		email := "test@example.com"
		createSubscriberService := client.Client.NewCreateSubscriberService()
		createSubscriberService.Name("User")
		createSubscriberService.Email(email)
		createSubscriberService.Status("enabled")
		subscriber, err := createSubscriberService.Do(context.Background())
		check(err)
		defer deleteSubscriber(client, subscriber.Id)

		launched, err := client.AddAndSendCampaign(email, list.Name)

		assert.NoError(t, err)
		assert.True(t, launched)

		// Check if campaign has been launched
		getCampaignsService := client.Client.NewGetCampaignsService()
		campaigns, err := getCampaignsService.Do(context.Background())
		check(err)

		var campaign *listmonk.Campaign = nil
		for _, c := range campaigns {
			if c.Name == baseCampaign.Name {
				campaign = c
				break
			}
		}

		assert.NotNil(t, campaign)
		assert.True(t, campaign.Status == "running" || campaign.Status == "finished")
		assert.Equal(t, baseCampaign.Body, campaign.Body)
	})
}

func TestLaunchCampaignListName(t *testing.T) {
	client := initAPIClient()

	t.Run("correct data", func(t *testing.T) {
		// Set up test - create campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)

		// Store the list in sync.Map
		client.MailingListIDs.Store(list.Name, list.Id)
		defer deleteList(client, list.Id)

		subscribers := make([]*listmonk.Subscriber, 2)
		for i := 0; i < cap(subscribers); i++ {
			createSubscriberService := client.Client.NewCreateSubscriberService()
			createSubscriberService.Name(fmt.Sprintf("User %d", i))
			createSubscriberService.Email(fmt.Sprintf("test%d@example.com", i))
			createSubscriberService.Status("enabled")
			sub, err := createSubscriberService.Do(context.Background())
			check(err)
			subscribers[i] = sub
			defer deleteSubscriber(client, sub.Id)
		}

		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name("Base campaign")
		createCampaignService.Subject("Subject")
		createCampaignService.Body("Campaign body")
		createCampaignService.Lists([]uint{list.Id})
		baseCampaign, err := createCampaignService.Do(context.Background())
		check(err)
		defer deleteCampaign(client, baseCampaign.Id)

		// Launch campaign
		launched, err := client.LaunchCampaignListName(list.Name)

		assert.NoError(t, err)
		assert.True(t, launched)

		// Check if campaign has been launched
		getCampaignsService := client.Client.NewGetCampaignsService()
		campaigns, err := getCampaignsService.Do(context.Background())
		check(err)

		var campaign *listmonk.Campaign = nil
		for _, c := range campaigns {
			if c.Name == baseCampaign.Name {
				campaign = c
				break
			}
		}

		assert.NotNil(t, campaign)
		assert.True(t, campaign.Status == "running" || campaign.Status == "finished")
		assert.Equal(t, baseCampaign.Body, campaign.Body)
	})

	t.Run("no such list", func(t *testing.T) {
		launched, _ := client.LaunchCampaignListName("no such list")
		assert.False(t, launched)
	})
}

func TestMailingListIDsConcurrency(t *testing.T) {
	client := initAPIClient()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Generate unique list names
	listNames := make([]string, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		listNames[i] = fmt.Sprintf("concurrent-test-list-%d", i)
	}

	createAndGetListID := func(listName string) {
		defer wg.Done()

		// Create list
		list, err := client.createList(listName)
		if err != nil {
			t.Errorf("Error creating list %s: %v", listName, err)
			return
		}
		defer deleteList(client, list.Id)

		// Get list ID
		id, err := client.getListID(listName)
		if err != nil {
			t.Errorf("Error getting list ID for %s: %v", listName, err)
			return
		}

		if id != list.Id {
			t.Errorf("List ID mismatch for %s: expected %d, got %d", listName, list.Id, id)
		}

		// Simulate concurrent reads and writes
		for j := 0; j < 5; j++ {
			go func(j int) {
				// Concurrently get list ID
				concurrentID, err := client.getListID(listName)
				if err != nil {
					t.Errorf("Goroutine %d: Error getting list ID for %s: %v", j, listName, err)
				} else if concurrentID != list.Id {
					t.Errorf("Goroutine %d: List ID mismatch for %s: expected %d, got %d", j, listName, list.Id, concurrentID)
				}
			}(j)
		}
	}

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go createAndGetListID(listNames[i])
	}

	wg.Wait()
}

func TestSubscriberAttributes(t *testing.T) {
	client := initAPIClient()

	t.Run("Update subscriber's attributes", func(t *testing.T) {
		// First, create a subscriber
		name := "Test User Update"
		email := fmt.Sprintf("testuserupdate+%d@example.com", time.Now().UnixNano())
		lists := []string{"TestList"}

		// Create TestList if it doesn't exist
		listName := "TestList"
		_, err := client.getListID(listName)
		if err != nil {
			_, err = client.createList(listName)
			require.NoError(t, err)
		}

		attrs := map[string]interface{}{
			"expiration_date":        "2023-12-31",
			"key_creation_timestamp": time.Now().Format(time.RFC3339),
			"key":                    "initial-key-value",
		}

		id, err := client.CreateSubscriber(name, email, lists, attrs)
		require.NoError(t, err)
		defer func() {
			err := client.DeleteSubscriberID(id)
			require.NoError(t, err)
		}()

		// Update the subscriber's attributes
		updatedAttrs := map[string]interface{}{
			"expiration_date":          "2024-12-31",
			"key_creation_timestamp":   "2023-11-01T10:00:00Z",
			"key":                      "updated-key-value",
			"new_custom_field":         "new field value",
			"additional_field_example": "updated value",
		}

		err = client.UpdateSubscriberAttributes(id, updatedAttrs)
		require.NoError(t, err)

		// Fetch the subscriber and verify updated attributes
		fetchedAttrs, err := client.GetSubscriberAttributes(id)
		require.NoError(t, err)

		// Convert any float64 to int if necessary for comparison
		for k, v := range fetchedAttrs {
			switch val := v.(type) {
			case float64:
				if val == float64(int(val)) {
					fetchedAttrs[k] = int(val)
				}
			}
		}

		assert.Equal(t, updatedAttrs["expiration_date"], fetchedAttrs["expiration_date"])
		assert.Equal(t, updatedAttrs["key"], fetchedAttrs["key"])
		assert.Equal(t, updatedAttrs["new_custom_field"], fetchedAttrs["new_custom_field"])
		assert.Equal(t, updatedAttrs["additional_field_example"], fetchedAttrs["additional_field_example"])
	})

	t.Run("Set specific attribute", func(t *testing.T) {
		// First, create a subscriber
		name := "Test User Set Attribute"
		email := fmt.Sprintf("testusersetattr+%d@example.com", time.Now().UnixNano())
		lists := []string{"TestList"}

		// Create TestList if it doesn't exist
		listName := "TestList"
		_, err := client.getListID(listName)
		if err != nil {
			_, err = client.createList(listName)
			require.NoError(t, err)
		}

		attrs := map[string]interface{}{
			"key": "initial-key-value",
		}

		id, err := client.CreateSubscriber(name, email, lists, attrs)
		require.NoError(t, err)
		defer func() {
			err := client.DeleteSubscriberID(id)
			require.NoError(t, err)
		}()

		// Retrieve current attributes
		fetchedAttrs, err := client.GetSubscriberAttributes(id)
		require.NoError(t, err)

		// Modify the 'key' attribute
		fetchedAttrs["key"] = "updated-key-value"

		// Update the subscriber's attributes
		err = client.UpdateSubscriberAttributes(id, fetchedAttrs)
		require.NoError(t, err)

		// Fetch the subscriber and verify the updated 'key' attribute
		updatedAttrs, err := client.GetSubscriberAttributes(id)
		require.NoError(t, err)
		assert.Equal(t, "updated-key-value", updatedAttrs["key"])
	})
}

func TestListSubscribers(t *testing.T) {
client := initAPIClient()

	t.Run("Correct data", func(t *testing.T) {
    listName := "sometestlist"
    // Create list and subscribers
		list, err := client.createList(listName)
    check(err)
    defer deleteList(client, list.Id) 

		subscribers := make([]*listmonk.Subscriber, 2)
    expriationKey := fmt.Sprintf("expiration_date_%s", strings.ToLower(listName))

		for i := 0; i < cap(subscribers); i++ {
		attrs := map[string]interface{}{
			expriationKey: "2025-09-07",
      "key": "some_key",
		}
			createSubscriberService := client.Client.NewCreateSubscriberService()
			createSubscriberService.Name(fmt.Sprintf("User %d", i))
			createSubscriberService.Email(fmt.Sprintf("test%d@example.com", i))
			createSubscriberService.Status("enabled")
      createSubscriberService.ListIds([]uint{list.Id})
      createSubscriberService.Attributes(attrs)
			sub, err := createSubscriberService.Do(context.Background())
			check(err)
			subscribers[i] = sub
			defer deleteSubscriber(client, sub.Id)
		}

    result, err := client.ListSubscribers(listName)

    if assert.NoError(t, err) {
      assert.Contains(t, result, map[string]string{
          "id": strconv.Itoa(int(subscribers[0].Id)),
          "email": "test0@example.com",
          "expiration_date":  "2025-09-07",
        })
      assert.Contains(t, result, map[string]string{
          "id": strconv.Itoa(int(subscribers[1].Id)),
          "email": "test1@example.com",
          "expiration_date":  "2025-09-07",
        })
    }
  })
}
