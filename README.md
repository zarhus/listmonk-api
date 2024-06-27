# 3mdeb Listmonk API

A Golang tool for Listmonk.

## Installing

TBD

## Importing

```go
import (
    3mdeb/listmonk-api/api
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

Then use the object to call functions

```go
// Create new subscriber that subscribes to lists "list1" and "list2"
_, err := client.CreateSubscriber("Example", "something@example.com", []string{"list1", "list2"})
if err != nil {
panic(err)
}
```
