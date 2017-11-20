package main

import "github.com/akaumov/nats-http"

func main() {

	server := nats_http.New(&nats_http.Config{
		NatsPoolSize:      10,
		NatsAddress:       "nats://localhost:4222",
		UrlPattern:        "/",
		ListenInterface:   "localhost:8080",
		PacketFormat:      "json",
		Timeout:           30000,
		NatsOutputSubject: "http",
		JwtSecret:         "123456",
	})

	server.Start()
}
