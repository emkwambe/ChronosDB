package main

import (
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/IBM/sarama"
)

type StreamMessage struct {
    Type       string                 `json:"type"`
    ID         string                 `json:"id,omitempty"`
    Labels     []string               `json:"labels,omitempty"`
    EdgeType   string                 `json:"edge_type,omitempty"`
    SourceID   string                 `json:"source_id,omitempty"`
    TargetID   string                 `json:"target_id,omitempty"`
    Properties map[string]interface{} `json:"properties"`
    Timestamp  int64                  `json:"timestamp,omitempty"`
}

func main() {
    // Kafka configuration
    brokers := []string{"localhost:9092"}
    topic := "chronosdb-stream"
    
    config := sarama.NewConfig()
    config.Producer.RequiredAcks = sarama.WaitForAll
    config.Producer.Retry.Max = 5
    config.Producer.Return.Successes = true
    
    producer, err := sarama.NewSyncProducer(brokers, config)
    if err != nil {
        log.Fatalf("Failed to create producer: %v", err)
    }
    defer producer.Close()
    
    // Send test messages
    messages := []StreamMessage{
        {
            Type:   "node",
            Labels: []string{"Sensor"},
            Properties: map[string]interface{}{
                "name":  "temperature_sensor",
                "value": 23.5,
                "unit":  "celsius",
            },
            Timestamp: time.Now().UnixMicro(),
        },
        {
            Type:   "node",
            Labels: []string{"Sensor"},
            Properties: map[string]interface{}{
                "name":  "humidity_sensor",
                "value": 65.0,
                "unit":  "percent",
            },
            Timestamp: time.Now().UnixMicro(),
        },
        {
            Type:     "edge",
            EdgeType: "CONNECTED_TO",
            SourceID: "sensor_1",
            TargetID: "gateway_1",
            Properties: map[string]interface{}{
                "signal_strength": -45,
            },
            Timestamp: time.Now().UnixMicro(),
        },
        {
            Type: "update",
            ID:   "sensor_1",
            Properties: map[string]interface{}{
                "value": 24.1,
            },
            Timestamp: time.Now().UnixMicro(),
        },
    }
    
    for _, msg := range messages {
        data, err := json.Marshal(msg)
        if err != nil {
            log.Printf("Failed to marshal message: %v", err)
            continue
        }
        
        partition, offset, err := producer.SendMessage(&sarama.ProducerMessage{
            Topic: topic,
            Value: sarama.StringEncoder(data),
        })
        if err != nil {
            log.Printf("Failed to send message: %v", err)
        } else {
            fmt.Printf("Message sent to partition %d at offset %d: %s\n", partition, offset, data)
        }
        
        time.Sleep(1 * time.Second)
    }
    
    fmt.Println("All test messages sent!")
}
