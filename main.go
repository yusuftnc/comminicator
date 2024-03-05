package main

import (
	"agent/protocol"
	"bufio"
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

type ServiceConfig struct {
	Name    string `yaml:"name"`
	Id      int    `yaml:"id"`
	Port    int    `yaml:"port"`
	Type    string `yaml:"type"`    // SURGUARD or ADM-CID or DC09
	EndChar byte   `yaml:"endChar"` // Delimiter byte
}

type MonitoringCenter struct {
	Name   string `yaml:"name"`
	Serial int    `yaml:"serial"`
}

type Config struct {
	Center          MonitoringCenter `yaml:"monitoringCenter"`
	ListenServices  []ServiceConfig  `yaml:"listenServices"`
	ConnectServices []ServiceConfig  `yaml:"connectServices"`
}

var (
	pubsubClient *pubsub.Client
	conf         Config
)

func initPubSubClient(projectID string) (*pubsub.Client, error) {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub.NewClient: %w", err)
	}
	return client, nil
}

func main() {
	// Load and parse the YAML configuration file
	configFile, err := os.ReadFile("conf.yaml")
	if err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal(configFile, &conf); err != nil {
		panic(err)
	}

	// Initialize the Pub/Sub client

	pubsubClient, err = initPubSubClient("bulutalarm") // Replace with your actual project ID
	if err != nil {
		fmt.Println("Failed to create Pub/Sub client:", err)
		return
	}
	defer pubsubClient.Close()

	// Start listeners for services that this app listens to
	for _, service := range conf.ListenServices {
		go startListener(service)
	}

	// Start connecting to services that this app needs to connect to
	for _, service := range conf.ConnectServices {
		go startConnector(service)
	}

	// Block forever (or until the program is manually stopped)
	select {}
}

func startListener(service ServiceConfig) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", service.Port))
	if err != nil {
		fmt.Printf("Error starting listener on port %d for service %s: %v\n", service.Port, service.Name, err)
		return
	}
	defer ln.Close()
	fmt.Printf("Listening on port %d for service %s with type %s\n", service.Port, service.Name, service.Type)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection for service %s: %v\n", service.Name, err)
			continue
		}
		go handleConnection(conn, service)
	}
}

func startConnector(service ServiceConfig) {
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", service.Port))
		if err != nil {
			fmt.Printf("Error connecting to service %s on port %d: %v\n", service.Name, service.Port, err)
			time.Sleep(time.Second * 5) // Wait before retrying
			continue
		}
		fmt.Printf("Connected to service %s on port %d\n", service.Name, service.Port)

		handleConnection(conn, service)

		fmt.Printf("Disconnected from service %s on port %d, attempting to reconnect...\n", service.Name, service.Port)
		// The connection will attempt to reconnect after handling in handleConnection completes
	}
}

func handleConnection(conn net.Conn, service ServiceConfig) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	for {
		var data []byte
		var err error

		data, err = r.ReadBytes(service.EndChar)
		//fmt.Println("ReadLine", data)

		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading for type %s: %v\n", service.Type, err)
			}
			break
		}
		if len(data) > 1 {
			data = data[:len(data)-1] // Remove the delimiter
		} else {
			continue // No actual data to process
		}

		ack, handleDataErr := handleData(data, service)
		fmt.Println("ACK:", ack)
		if handleDataErr == nil {
			if _, err := conn.Write([]byte(ack)); err != nil {
				fmt.Printf("Error sending ack for type %s: %v\n", service.Type, err)
				break
			}
		} else {
			fmt.Printf("Error handling data for type %s: %v. Retrying...\n", service.Type, handleDataErr)
			time.Sleep(1 * time.Second) // Retry logic can be more sophisticated
		}
	}
}

func handleData(data []byte, service ServiceConfig) (ack string, err error) {
	dataType := service.Type
	receiverId := strconv.Itoa(service.Id)
	event := []interface{}(nil)
	// Simulate processing data differently based on the type
	fmt.Printf("Processing %s data: %s", dataType, string(data))

	switch dataType {
	case "SURGUARD":
		event, ack, err = protocol.ParseSurguard(string(data), receiverId)
		if err != nil {
			return "", err
		}
	case "ADEMCO":
		event, ack, err = protocol.ParseAdemco(string(data), receiverId)
		if err != nil {
			return "", err
		}
	case "DC09":
		event, ack, err = protocol.ParseDc09(string(data), receiverId)
		if err != nil {
			return "", err
		}
	case "TEKNIM":
		event, ack, err = protocol.ParseTeknim(string(data), receiverId)
		if err != nil {
			return "", err
		}
	case "FONRI":
		event, ack, err = protocol.ParseFonri(string(data), receiverId)
		if err != nil {
			return "", err
		}

	default:
		fmt.Printf("Unknown service type: %s\n", dataType)
	}

	if event == nil {
		fmt.Println("Event is nil")
		return "", nil
	}

	topic := pubsubClient.Topic("event")
	ctx := context.Background()

	for _, e := range event {
		jsonData, err := json.Marshal(e)
		if err != nil {
			log.Fatalf("JSON marshalling hatası: %v", err)
		}

		if jsonData == nil {
			fmt.Println("JSON is empty, jsonData: ", string(jsonData))
			return "", nil
		}
		empJSON, err := json.MarshalIndent(string(jsonData), "", " ")
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("--(:)--")
		fmt.Println(string(empJSON))

		// Create a message
		msg := &pubsub.Message{
			Data: jsonData,
		}

		result := topic.Publish(ctx, msg)

		_, err = result.Get(ctx)
		if err != nil {
			fmt.Println("failed to publish to topic: %v", err)
			return "", err
		}

	}

	//fmt.Println("Published a message to the topic for", dataType, event)
	return ack, nil
}
