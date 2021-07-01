package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func HandleConnections(s *Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client := &Client{}
		var err error
		client.Ws, err = upgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Println(err.Error())
		}

		open := time.Now()

		defer func() {
			s.Mu.Lock()
			delete(s.Clients, client)
			s.Mu.Unlock()
			client.Ws.Close()
		}()
		s.Mu.Lock()
		s.Clients[client] = true
		s.Mu.Unlock()
		fmt.Println(s.Clients)

		go func() {
			for {
				client.Mu.Lock()
				err := client.Ws.WriteJSON(map[string]interface{}{
					"op": "0",
					"d": map[string]int64{
						"t": time.Since(open).Milliseconds(),
					},
				})
				client.Mu.Unlock()
				if err != nil {
					break
				}
				time.Sleep(5 * time.Second)
			}
		}()

		for {
			var request Request
			client.Ws.SetReadDeadline(time.Now().Add(3 * time.Minute)) // change the time here :uganda"
			err := client.Ws.ReadJSON(&request)
			if err != nil {
				log.Printf("error: %v", err)
				break
			}

			callback, err := s.UseCommand(request.OperationCode)
			if err != nil {
				continue
			}

			go callback(client, request)
		}
	}
}

// need better handlers
func HandleChallenges(s *Server) {
	var ctx = context.Background()

	pubsub := s.RedisClient.Subscribe(ctx, "events")
	_, err := pubsub.Receive(ctx)
	if err != nil {
		panic(err)
	}

	redisChan := pubsub.Channel()
	var subEvent SubEvent

	for {
		msg := <-redisChan
		err = json.Unmarshal([]byte(msg.Payload), &subEvent)
		if err != nil {
			log.Println(err)
		}

		for client := range s.Clients {
			client.Mu.Lock()
			err := client.Ws.WriteJSON(subEvent)
			client.Mu.Unlock()
			if err != nil {
				log.Printf("error: %v", err)
				client.Ws.Close()
			}
		}
	}
}
