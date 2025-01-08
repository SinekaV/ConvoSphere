package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ChatRoom manages clients and broadcasts messages.
type ChatRoom struct {
	clients   map[string]chan string // Map of clientID to their message channels
	broadcast chan string            // Channel for broadcasting messages
	leave     chan string            // Channel for clients leaving the chat room
	mutex     sync.Mutex             // Ensures thread-safe access to clients map
}

func NewChatRoom() *ChatRoom {
	return &ChatRoom{
		clients:   make(map[string]chan string),
		broadcast: make(chan string),
		leave:     make(chan string),
	}
}

func (cr *ChatRoom) AddClient(clientID string) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()
	cr.clients[clientID] = make(chan string)
}

func (cr *ChatRoom) RemoveClient(clientID string) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()
	if _, exists := cr.clients[clientID]; exists {
		close(cr.clients[clientID])
		delete(cr.clients, clientID)
	}
}

func (cr *ChatRoom) BroadcastMessages() {
	for msg := range cr.broadcast {
		cr.mutex.Lock()
		for _, ch := range cr.clients {
			select {
			case ch <- msg:
			default:
			}
		}
		cr.mutex.Unlock()
	}
}

func (cr *ChatRoom) HandleJoin(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}
	cr.AddClient(clientID)
	fmt.Fprintf(w, "Client %s joined the chat", clientID)
}

func (cr *ChatRoom) HandleSend(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("id")
	message := r.URL.Query().Get("message")

	if clientID == "" || message == "" {
		http.Error(w, "Client ID and message are required", http.StatusBadRequest)
		return
	}

	cr.mutex.Lock()
	_, exists := cr.clients[clientID]
	cr.mutex.Unlock()
	if !exists {
		http.Error(w, "Invalid client ID", http.StatusNotFound)
		return
	}

	cr.broadcast <- fmt.Sprintf("%s: %s", clientID, message)
	fmt.Fprintf(w, "Message from %s sent", clientID)
}

func (cr *ChatRoom) HandleLeave(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}
	cr.RemoveClient(clientID)
	fmt.Fprintf(w, "Client %s left the chat", clientID)
}

func (cr *ChatRoom) HandleMessages(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	cr.mutex.Lock()
	ch, exists := cr.clients[clientID]
	cr.mutex.Unlock()
	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	timeout := time.After(30 * time.Second)
	select {
	case msg, ok := <-ch:
		if !ok {
			http.Error(w, "Client has left the chat", http.StatusGone)
			return
		}
		fmt.Fprintln(w, msg)
	case <-timeout:
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}

func (cr *ChatRoom) RunServer() {
	http.HandleFunc("/join", cr.HandleJoin)
	http.HandleFunc("/send", cr.HandleSend)
	http.HandleFunc("/leave", cr.HandleLeave)
	http.HandleFunc("/messages", cr.HandleMessages)
	log.Println("Chat server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	chatRoom := NewChatRoom()
	go chatRoom.BroadcastMessages()
	chatRoom.RunServer()
}
