# 3mdeb Listmonk API

A Golang tool for Listmonk.

## Installing

```bash
go get github.com/zarhus/listmonk-api
```

## Importing

```go
import (
    listmonk "github.com/zarhus/listmonk-api/api"
)
```

## Usage

To perform operations on Listmonk, you must first initialize an `APIClient`
object.

```go
username := "username"
password := "password"
client := api.NewAPIClient("https://listmonk.3mdeb.com", &username, &password)
```

The URL should point to the Listmonk service (in our case it's
https://listmonk.3mdeb.com). The username and password should match an
administrator account.

Then use the object to call functions, for example:

```go
// Create new subscriber that subscribes to lists "list1" and "list2"
_, err := client.CreateSubscriber("Example", "something@example.com", []string{"list1", "list2"})
if err != nil {
panic(err)
}
```

## Documentation

There are several ways you can generate this API's documentation. The
recommended one is `pkgsite`. Follow the steps below:

1. Install `pkgsite`.

```bash
go install golang.org/x/pkgsite/cmd/pkgsite@latest
```

2. Generate the documentation.

```bash
pkgsite
```

By default the docs are hosted locally on port `8080`. If you want to use a
different port, specify it using the `-http` option.

```bash
pkgsite -http :1234
```

3. Open a web browser and go to
`localhost:<port>/git.3mdeb.com/3mdeb/listmonk-api`.

You will see the documentation there.


## Tests

The `api` package contains tests that cover most functionalities.

### Running the tests

You can run all tests using `run-tests.sh`.

```bash
./run-tests.sh
```

Passing arguments to the script will pass them to the `go test` command. For
example, the following command runs the tests and checks the test coverage.

```bash
./run-tests.sh -cover
```

Running the tests requires `docker` with `docker-compose`.

The script performs the following steps:

1. It hosts a local Listmonk instance with a PostgreSQL database using
containers.

1. It runs tests located in `api/client_test.go`.

1. It stops the containers.

### Adding tests

If you wish to add a new test, you need to modify `api/client_test.go`. Follow
the guidelines listed below:

- Each test must correspond to exactly one function in `api/client.go`.

- The test function's name must have the following form: `Test<FunctionName>`.

- The test must begin with the initialization of the `APIClient`.

```go
func TestCreateSubscriberListIDs(t *testing.T) {
    client := initAPIClient()
    t.Run("correct input data", func(t *testing.T) {
(...)
```

- The test must be divided into subtests that correspond to different input
data/expected outcomes. Even if there is only one test, it still must be defined
as a subtest.

```go

func TestAddSubscribersToList(t *testing.T) {
	client := initAPIClient()

    t.Run("correct input data", func(t *testing.T) {
        // Subtest 1
    }

    t.Run("no such user", func(t *testing.T) {
        // Subtest 2
    }
```

- If you use the `go-listmonk` module to set up the test, make sure to catch
errors using the `check` function and to clean up after the test.

```go
// Set up test - create subscribers and list
createListService := client.Client.NewCreateListService()
createListService.Name("tmp")
list, err := createListService.Do(context.Background())
check(err)
client.MailingListIDs[list.Name] = list.Id
defer deleteList(client, list.Id)
(...)
```
