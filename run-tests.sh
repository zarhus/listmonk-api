#!/bin/bash

existing_containers=$(docker ps -q)

docker compose up --quiet-pull -d demo-db demo-app || exit 1

new_containers=$(docker ps -q)

started_containers=$(comm -13 <(echo "$existing_containers" | sort) <(echo "$new_containers" | sort))

cd api

sleep 5

go test $1

docker stop $started_containers > /dev/null
