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

func TestCreateSubscriber(t *testing.T) {
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)

	t.Run("correct input data", func(t *testing.T) {
		name := "John Doe"
		email := "john.doe@example.com"
		list_ids := []uint{1, 3}

		id, err := client.CreateSubscriber(name, email, list_ids)

		// Clean up
		defer deleteSubscriber(client, id)

		// Get actual subscriber ID
		getSubscribersService := client.Client.NewGetSubscribersService()
		getSubscribersService.Query(fmt.Sprintf("subscribers.email = '%s'", email))
		subscribers, err2 := getSubscribersService.Do(context.Background())

		assert.NoError(t, err)
		assert.NoError(t, err2)
		assert.Equal(t, subscribers[0].Id, id)
	})

	t.Run("incorrect e-mail", func(t *testing.T) {

		name := "John Doe"
		email := "not an e-mail"
		list_ids := []uint{1, 3}

		id, err := client.CreateSubscriber(name, email, list_ids)

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
		campaigns, err2 := getCampaignsService.Do(context.Background())

		assert.NoError(t, err2)

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
		err2 := client.deleteCampaign(campaign)

		assert.NoError(t, err2)

		// Make sure the campaign was actually deleted
		getCampaignService := client.Client.NewGetCampaignService()
		getCampaignService.Id(campaign.Id)
		_, err3 := getCampaignService.Do(context.Background())

		assert.Error(t, err3)

		// Delete campaign manually if it wasn't deleted
		if err3 == nil {
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

func TestResumeCampaign(t *testing.T) {
	username := ""
	password := ""
	client := NewAPIClient("http://0.0.0.0:9000", &username, &password)

	t.Run("correct", func(t *testing.T) {
		// Set up test - create and launch campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)
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

		resumed, err := client.ResumeCampaign(baseCampaign.Id)

		assert.NoError(t, err)
		assert.True(t, resumed)

		time.Sleep(5 * time.Second)

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
		assert.Equal(t, "finished", campaign.Status)
	})

	t.Run("no new subscribers", func(t *testing.T) {
		// Set up test - create and launch campaign with subscribers
		createListService := client.Client.NewCreateListService()
		createListService.Name("testlist")
		list, err := createListService.Do(context.Background())
		check(err)
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

    resumed, err := client.ResumeCampaign(baseCampaign.Id)

    assert.NoError(t, err)
    assert.False(t, resumed)
	})

  t.Run("no such campaign", func(t *testing.T) {
    resumed, err := client.ResumeCampaign(130)

    assert.Error(t, err)
    assert.False(t, resumed)
  })
}
