#!/bin/bash

sudo docker stop postgres-test
sudo docker rm postgres-test

sudo docker run --name postgres-test \
-e POSTGRES_PASSWORD=123 \
-p 5432:5432 \
-v ./initTest.sql:/docker-entrypoint-initdb.d/init.sql \
-d postgres
until sudo docker exec postgres-test pg_isready -U postgres; do
    echo 'Waiting for postgres'
    sleep 0.5
done

npm run dev