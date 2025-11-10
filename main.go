package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

type ChangeNotification struct {
	Table     string          `json:"table"`
	Operation string          `json:"operation"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

type TableChangeHandler interface {
	HandleChange(operation string, data json.RawMessage) error
}

type ConfigManager struct{}

func (cm *ConfigManager) HandleChange(operation string, data json.RawMessage) error {
	log.Printf("[s_config] %s: %s", operation, string(data))
	return nil
}

type UserManager struct{}

func (um *UserManager) HandleChange(operation string, data json.RawMessage) error {
	log.Printf("[s_user] %s: %s", operation, string(data))
	return nil
}

type DataListener struct {
	db       *sql.DB
	handlers map[string]TableChangeHandler
}

func NewDataListener(connStr string) (*DataListener, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DataListener{
		db:       db,
		handlers: make(map[string]TableChangeHandler),
	}, nil
}

func (dl *DataListener) RegisterHandler(tableName string, handler TableChangeHandler) {
	dl.handlers[tableName] = handler
}

func (dl *DataListener) handleNotification(payload string) error {
	var notification ChangeNotification
	if err := json.Unmarshal([]byte(payload), &notification); err != nil {
		return fmt.Errorf("failed to parse notification: %v", err)
	}

	handler, ok := dl.handlers[notification.Table]
	if !ok {
		return nil
	}

	return handler.HandleChange(notification.Operation, notification.Data)
}

func (dl *DataListener) Start(connStr string) error {
	eventCallback := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("Listener event: %s, error: %v", ev, err)
		}
	}

	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, eventCallback)
	defer listener.Close()

	if err := listener.Listen("data_changes"); err != nil {
		return err
	}

	log.Println("Listening on channel: data_changes")

	for {
		select {
		case notification := <-listener.Notify:
			if notification != nil {
				if err := dl.handleNotification(notification.Extra); err != nil {
					log.Printf("Error: %v", err)
				}
			}
		case <-time.After(15 * time.Second):
			err := listener.Ping()
			if err != nil {

				return err
			}
		}
	}
}

func (dl *DataListener) Close() error {
	return dl.db.Close()
}

func main() {
	connStr := "host=localhost port=5433 user=postgres password=post123 dbname=data_listener sslmode=disable"

	listener, err := NewDataListener(connStr)
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	listener.RegisterHandler("s_config", &ConfigManager{})
	listener.RegisterHandler("s_user", &UserManager{})

	log.Println("Starting listener...")
	if err := listener.Start(connStr); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
}
