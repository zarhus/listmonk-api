package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Exayn/go-listmonk"
	"github.com/stretchr/testify/assert"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)

	t.Run("correct input data", func(t *testing.T) {
		name := "John Doe"
		email := "john.doe@example.com"
		list_ids := []uint{1, 3}

		id, err := client.CreateSubscriberListIDs(name, email, list_ids)

		assert.NoError(t, err)

		// Clean up
		defer deleteSubscriber(client, id)

		// Get actual subscriber ID
		getSubscribersService := client.Client.NewGetSubscribersService()
		getSubscribersService.Query(fmt.Sprintf("subscribers.email = '%s'", email))
		subscribers, err := getSubscribersService.Do(context.Background())

		assert.NoError(t, err)
		assert.Equal(t, subscribers[0].Id, id)
	})

	t.Run("incorrect e-mail", func(t *testing.T) {

		name := "John Doe"
		email := "not an e-mail"
		list_ids := []uint{1, 3}

		id, err := client.CreateSubscriberListIDs(name, email, list_ids)

		assert.Error(t, err)
		assert.Equal(t, (uint)(0), id)
	})

}

func TestCreateCampaignHTML(t *testing.T) {
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
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
		lists := []uint{1}

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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create a campaign
		name := "My campaign"
		subject := "Subject of campaign"
		body := "Body"
		lists := []uint{1}
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create subscribers and list
		createListService := client.Client.NewCreateListService()
		createListService.Name("tmp")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
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
		// Set up test - create subscribers and list
		createListService := client.Client.NewCreateListService()
		createListService.Name("tmp")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
		defer deleteList(client, list.Id)

		// This subscriber does not exist
		subscribers := []*listmonk.Subscriber{
			{
				Id: 35,
			},
		}

		err = client.addSubscribersToList(subscribers, list)
		assert.Error(t, err)
	})
}

func TestCreateIncCampaign(t *testing.T) {
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)

	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create base campaign and temp list
		createCampaignService := client.Client.NewCreateCampaignService()
		createCampaignService.Name("Base campaign")
		createCampaignService.Subject("Subject")
		createCampaignService.Body("Campaign body")
		createCampaignService.Lists([]uint{1})
		baseCampaign, err := createCampaignService.Do(context.Background())
		check(err)
		defer deleteCampaign(client, baseCampaign.Id)

		createListService := client.Client.NewCreateListService()
		createListService.Name("tmp")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
		defer deleteList(client, list.Id)

		incCampaign, err := client.createIncCampaign(baseCampaign, list)

		assert.NoError(t, err)
		assert.Equal(t, baseCampaign.Name+"_inc", incCampaign.Name)
		assert.Equal(t, baseCampaign.Subject, incCampaign.Subject)
		assert.Equal(t, baseCampaign.Body, incCampaign.Body)

		listIDs := make([]uint, len(incCampaign.Lists))
		for index, l := range incCampaign.Lists {
			listIDs[index] = l.Id
		}
		assert.Equal(t, []uint{list.Id}, listIDs)
	})
}

func TestLaunchCampaign(t *testing.T) {
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)

	t.Run("resume with correct data", func(t *testing.T) {
		// Set up test - create and launch campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
    list, err := createListService.Do(context.Background())
    check(err)
    client.MailingListIDs[list.Name] = list.Id
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
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
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

		updateCampaignStatusService := client.Client.NewUpdateCampaignStatusService()
		updateCampaignStatusService.Id(baseCampaign.Id)
		updateCampaignStatusService.Status("running")
		_, err = updateCampaignStatusService.Do(context.Background())
		check(err)

		time.Sleep(5 * time.Second)

		resumed, err := client.LaunchCampaign(baseCampaign.Id)

		assert.NoError(t, err)
		assert.False(t, resumed)
	})

	t.Run("no such campaign", func(t *testing.T) {
		resumed, err := client.LaunchCampaign(130)

		assert.Error(t, err)
		assert.False(t, resumed)
	})

  t.Run("first launch correct", func(t *testing.T) {
		// Set up test - create campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
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

		// Check if the subscruber was deleted
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
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
		if err != nil {
			deleteSubscriber(client, sub.Id)
		}
	})

	t.Run("no such email", func(t *testing.T) {
		err := client.DeleteSubscriberEmail("no.such@email.com")

		assert.Error(t, err)
	})
}

func TestCreateCampaignHTMLOnListName(t *testing.T) {
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
	t.Run("correct input data", func(t *testing.T) {
		// Set up test - create list
		listName := "testlist"
		createListService := client.Client.NewCreateListService()
		createListService.Name(listName)
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
	t.Run("correct input data", func(t *testing.T) {
		// Set up test - Create subscriber and lists
		listIDs := make([]uint, 2)
		for i := 0; i < 2; i++ {
			createListService := client.Client.NewCreateListService()
			createListService.Name(fmt.Sprintf("list%d", i))
			list, err := createListService.Do(context.Background())
			check(err)
      client.MailingListIDs[list.Name] = list.Id
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
	t.Run("correct input data", func(t *testing.T) {
		// Set up test - Create subscriber and list
		createListService := client.Client.NewCreateListService()
		createListService.Name("list")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)
  t.Run("correct input data", func(t *testing.T) {
    // Set up test - create list, campaign and subscriber
	  createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
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
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)

  t.Run("correct data", func(t *testing.T) {
		// Set up test - create campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)
    client.MailingListIDs[list.Name] = list.Id
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
