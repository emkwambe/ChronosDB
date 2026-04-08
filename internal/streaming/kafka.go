package streaming

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/IBM/sarama"
    "github.com/emkwambe/chronosdb/internal/query/executor"
    "github.com/emkwambe/chronosdb/internal/storage/temporal"
)

// KafkaConfig holds Kafka connection settings
type KafkaConfig struct {
    Brokers   []string
    Topic     string
    GroupID   string
    FromStart bool
}

// StreamMessage represents a message from Kafka
type StreamMessage struct {
    Type      string                 `json:"type"`      // "node", "edge", "update", "delete"
    ID        string                 `json:"id,omitempty"`
    Labels    []string               `json:"labels,omitempty"`
    EdgeType  string                 `json:"edge_type,omitempty"`
    SourceID  string                 `json:"source_id,omitempty"`
    TargetID  string                 `json:"target_id,omitempty"`
    Properties map[string]interface{} `json:"properties"`
    Timestamp int64                  `json:"timestamp,omitempty"`
}

// KafkaConsumer manages Kafka streaming
type KafkaConsumer struct {
    config   *KafkaConfig
    consumer sarama.ConsumerGroup
    executor *executor.Executor
    store    *temporal.TemporalStore
    wg       sync.WaitGroup
    ctx      context.Context
    cancel   context.CancelFunc
}

// NewKafkaConsumer creates a new Kafka consumer
func NewKafkaConsumer(config *KafkaConfig, exec *executor.Executor, store *temporal.TemporalStore) (*KafkaConsumer, error) {
    saramaConfig := sarama.NewConfig()
    saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
    saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
    
    if config.FromStart {
        saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
    }
    
    consumer, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, saramaConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create consumer group: %w", err)
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    
    return &KafkaConsumer{
        config:   config,
        consumer: consumer,
        executor: exec,
        store:    store,
        ctx:      ctx,
        cancel:   cancel,
    }, nil
}

// Start begins consuming messages from Kafka
func (kc *KafkaConsumer) Start() error {
    handler := &messageHandler{
        executor: kc.executor,
        store:    kc.store,
    }
    
    kc.wg.Add(1)
    go func() {
        defer kc.wg.Done()
        for {
            if err := kc.consumer.Consume(kc.ctx, []string{kc.config.Topic}, handler); err != nil {
                log.Printf("Consumer error: %v", err)
            }
            if kc.ctx.Err() != nil {
                return
            }
        }
    }()
    
    log.Printf("Kafka consumer started on topic: %s", kc.config.Topic)
    return nil
}

// Stop gracefully shuts down the consumer
func (kc *KafkaConsumer) Stop() {
    kc.cancel()
    kc.wg.Wait()
    kc.consumer.Close()
    log.Println("Kafka consumer stopped")
}

// messageHandler implements sarama.ConsumerGroupHandler
type messageHandler struct {
    executor *executor.Executor
    store    *temporal.TemporalStore
}

func (h *messageHandler) Setup(sarama.ConsumerGroupSession) error {
    log.Println("Kafka consumer session setup")
    return nil
}

func (h *messageHandler) Cleanup(sarama.ConsumerGroupSession) error {
    log.Println("Kafka consumer session cleanup")
    return nil
}

func (h *messageHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for message := range claim.Messages() {
        h.processMessage(message)
        session.MarkMessage(message, "")
    }
    return nil
}

func (h *messageHandler) processMessage(msg *sarama.ConsumerMessage) {
    var streamMsg StreamMessage
    if err := json.Unmarshal(msg.Value, &streamMsg); err != nil {
        log.Printf("Failed to parse message: %v", err)
        return
    }
    
    timestamp := streamMsg.Timestamp
    if timestamp == 0 {
        timestamp = time.Now().UnixMicro()
    }
    
    switch streamMsg.Type {
    case "node":
        id := streamMsg.ID
        if id == "" {
            id = fmt.Sprintf("stream_node_%d", time.Now().UnixNano())
        }
        err := h.store.CreateNode(id, streamMsg.Labels, streamMsg.Properties, timestamp, 0)
        if err != nil {
            log.Printf("Failed to create node from stream: %v", err)
        } else {
            log.Printf("Created node %s from Kafka stream", id)
        }
        
    case "edge":
        id := streamMsg.ID
        if id == "" {
            id = fmt.Sprintf("stream_edge_%d", time.Now().UnixNano())
        }
        err := h.store.CreateEdge(id, streamMsg.EdgeType, streamMsg.SourceID, streamMsg.TargetID, 
            streamMsg.Properties, timestamp, 0)
        if err != nil {
            log.Printf("Failed to create edge from stream: %v", err)
        } else {
            log.Printf("Created edge %s from Kafka stream", id)
        }
        
    case "update":
        err := h.store.UpdateNodeProperty(streamMsg.ID, "value", streamMsg.Properties["value"], timestamp)
        if err != nil {
            log.Printf("Failed to update node from stream: %v", err)
        } else {
            log.Printf("Updated node %s from Kafka stream", streamMsg.ID)
        }
        
    case "delete":
        err := h.store.SoftDeleteNode(streamMsg.ID, timestamp)
        if err != nil {
            log.Printf("Failed to delete node from stream: %v", err)
        } else {
            log.Printf("Deleted node %s from Kafka stream", streamMsg.ID)
        }
        
    default:
        log.Printf("Unknown message type: %s", streamMsg.Type)
    }
}
