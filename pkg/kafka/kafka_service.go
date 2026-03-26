package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"be-modami-auth-service/config"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

type KafkaService struct {
	config    *KafkaConfig
	appConfig *config.Config
	client    *kgo.Client
	topics    *KafkaTopics
	mu        sync.RWMutex
	running   bool
}

func NewKafkaService(cfg *KafkaConfig, appCfg *config.Config) (*KafkaService, error) {
	if cfg == nil {
		cfg = GetDefaultKafkaConfig(appCfg)
	}

	opts, err := cfg.ToFranzGoOpts()
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka options: %w", err)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	return &KafkaService{
		config:    cfg,
		appConfig: appCfg,
		client:    client,
		topics:    GetKafkaTopics(),
		running:   false,
	}, nil
}

type ProducerMessage struct {
	Key     string                 `json:"key"`
	Value   interface{}            `json:"value"`
	Headers map[string]interface{} `json:"headers,omitempty"`
}

// Emit sends a message to Kafka synchronously with trace context propagation
func (k *KafkaService) Emit(ctx context.Context, topic string, message *ProducerMessage) error {
	topicName := GetTopicWithEnv(k.appConfig, topic)
	valueBytes, err := json.Marshal(message.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal message value: %w", err)
	}

	headers := k.buildHeaders(ctx, message.Headers)
	headers = InjectTraceContext(ctx, headers)

	record := &kgo.Record{
		Topic:   topicName,
		Key:     []byte(message.Key),
		Value:   valueBytes,
		Headers: headers,
	}

	if err := k.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		zap.L().Error("Failed to send message", zap.Error(err),
			zap.String("topic", topicName),
			zap.String("key", message.Key),
		)
		return fmt.Errorf("failed to send message to topic %s: %w", topicName, err)
	}

	zap.L().Info("Message sent successfully",
		zap.String("topic", topicName),
		zap.String("key", message.Key),
	)
	return nil
}

func (k *KafkaService) EnsureTopics(ctx context.Context) error {
	adm := kadm.NewClient(k.client)

	baseTopics := GetAllTopics()
	targetTopics := make([]string, 0, len(baseTopics))
	for _, t := range baseTopics {
		targetTopics = append(targetTopics, GetTopicWithEnv(k.appConfig, t))
	}

	zap.L().Info("Ensuring Kafka topics exist...", zap.Int("count", len(targetTopics)))

	metadata, err := adm.Metadata(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kafka metadata: %w", err)
	}

	brokerCount := len(metadata.Brokers)
	replicationFactor := int16(3)
	if brokerCount < 3 {
		replicationFactor = 1
		zap.L().Warn("Broker count is less than 3, falling back to lower replication factor",
			zap.Int("brokers", brokerCount),
			zap.Int16("fallback_replication", replicationFactor),
		)
	}

	existingTopics, err := adm.ListTopics(ctx)
	if err != nil {
		return fmt.Errorf("failed to list kafka topics: %w", err)
	}

	missingTopics := make([]string, 0)
	for _, t := range targetTopics {
		if !existingTopics.Has(t) {
			missingTopics = append(missingTopics, t)
		}
	}

	envPrefix := GetTopicWithEnv(k.appConfig, "")
	redundantTopics := make([]string, 0)
	for _, t := range existingTopics.Names() {
		if strings.HasPrefix(t, envPrefix) {
			isTarget := false
			for _, target := range targetTopics {
				if t == target {
					isTarget = true
					break
				}
			}
			if !isTarget {
				redundantTopics = append(redundantTopics, t)
			}
		}
	}

	if len(redundantTopics) > 0 {
		zap.L().Info("Deleting redundant Kafka topics...",
			zap.Int("count", len(redundantTopics)),
			zap.String("topics", strings.Join(redundantTopics, ",")),
		)
		delResp, err := adm.DeleteTopics(ctx, redundantTopics...)
		if err != nil {
			zap.L().Error("Failed to delete redundant topics", zap.Error(err))
		} else {
			for _, res := range delResp {
				if res.Err != nil {
					zap.L().Error("Failed to delete redundant topic", zap.Error(res.Err),
						zap.String("topic", res.Topic),
					)
				} else {
					zap.L().Info("Successfully deleted redundant topic", zap.String("topic", res.Topic))
				}
			}
		}
	}

	if len(missingTopics) == 0 {
		zap.L().Info("All required Kafka topics already exist")
		return nil
	}

	zap.L().Info("Creating missing Kafka topics...",
		zap.Int("missing_count", len(missingTopics)),
		zap.Int("partitions", 1),
		zap.Int16("replication", replicationFactor),
	)

	resp, err := adm.CreateTopics(ctx, 1, replicationFactor, nil, missingTopics...)
	if err != nil {
		return fmt.Errorf("failed to create missing topics: %w", err)
	}

	hasError := false
	for _, res := range resp {
		if res.Err != nil {
			zap.L().Error("Failed to create topic", zap.Error(res.Err), zap.String("topic", res.Topic))
			hasError = true
		} else {
			zap.L().Info("Successfully created topic", zap.String("topic", res.Topic))
		}
	}

	if hasError {
		return fmt.Errorf("some topics failed to be created")
	}
	return nil
}

func (k *KafkaService) EmitAsync(ctx context.Context, topic string, message *ProducerMessage) {
	topicName := GetTopicWithEnv(k.appConfig, topic)
	valueBytes, err := json.Marshal(message.Value)
	if err != nil {
		zap.L().Error("Failed to marshal message value for async emit", zap.Error(err))
		return
	}

	headers := k.buildHeaders(ctx, message.Headers)
	headers = InjectTraceContext(ctx, headers)

	record := &kgo.Record{
		Topic:   topicName,
		Key:     []byte(message.Key),
		Value:   valueBytes,
		Headers: headers,
	}

	// Detach from the request context so the produce is not canceled
	// when the HTTP handler returns. Use a timeout to avoid leaking.
	produceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)

	k.client.Produce(produceCtx, record, func(r *kgo.Record, err error) {
		defer cancel()
		if err != nil {
			zap.L().Error("Failed to send async message", zap.Error(err),
				zap.String("topic", topicName),
				zap.String("key", message.Key),
			)
			return
		}
		zap.L().Debug("Async message sent successfully",
			zap.String("topic", topicName),
			zap.String("key", message.Key),
		)
	})
}

func (k *KafkaService) SendMessages(ctx context.Context, topic string, messages []*ProducerMessage) error {
	topicName := GetTopicWithEnv(k.appConfig, topic)
	records := make([]*kgo.Record, 0, len(messages))

	for _, message := range messages {
		valueBytes, err := json.Marshal(message.Value)
		if err != nil {
			zap.L().Error("Failed to marshal message value", zap.Error(err))
			continue
		}

		headers := k.buildHeaders(ctx, message.Headers)
		headers = InjectTraceContext(ctx, headers)

		records = append(records, &kgo.Record{
			Topic:   topicName,
			Key:     []byte(message.Key),
			Value:   valueBytes,
			Headers: headers,
		})
	}

	if err := k.client.ProduceSync(ctx, records...).FirstErr(); err != nil {
		zap.L().Error("Failed to send messages", zap.Error(err),
			zap.String("topic", topicName),
			zap.Int("count", len(messages)),
		)
		return fmt.Errorf("failed to send messages to topic %s: %w", topicName, err)
	}

	zap.L().Info("Messages sent successfully",
		zap.String("topic", topicName),
		zap.Int("count", len(messages)),
	)
	return nil
}

func (k *KafkaService) buildHeaders(ctx context.Context, customHeaders map[string]interface{}) []kgo.RecordHeader {
	headers := make([]kgo.RecordHeader, 0, len(customHeaders)+1)

	if requestID := getRequestIDFromContext(ctx); requestID != "" {
		headers = append(headers, kgo.RecordHeader{
			Key:   "request-id",
			Value: []byte(requestID),
		})
	}

	for key, value := range customHeaders {
		headerBytes, err := json.Marshal(value)
		if err != nil {
			zap.L().Warn("Failed to marshal header value",
				zap.String("key", key),
				zap.String("error", err.Error()),
			)
			continue
		}
		headers = append(headers, kgo.RecordHeader{
			Key:   key,
			Value: headerBytes,
		})
	}

	return headers
}

func (k *KafkaService) RegisterHandler(handler ConsumerHandler) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.running {
		zap.L().Error("Cannot register handler while service is running", zap.Error(fmt.Errorf("service running")))
		return
	}
	zap.L().Info("Registering consumer handler",
		zap.String("topics", strings.Join(handler.GetTopics(), ",")),
	)
}

func (k *KafkaService) StartConsumer(ctx context.Context, handlers []ConsumerHandler) error {
	k.mu.Lock()
	if k.running {
		k.mu.Unlock()
		return fmt.Errorf("consumer is already running")
	}
	k.running = true
	k.mu.Unlock()

	handlerMap := make(map[string][]ConsumerHandler)
	var topics []string
	topicSet := make(map[string]struct{})

	for _, handler := range handlers {
		for _, topic := range handler.GetTopics() {
			topicName := GetTopicWithEnv(k.appConfig, topic)
			if _, exists := topicSet[topicName]; !exists {
				topicSet[topicName] = struct{}{}
				topics = append(topics, topicName)
			}
			handlerMap[topicName] = append(handlerMap[topicName], handler)
		}
	}

	k.client.AddConsumeTopics(topics...)
	zap.L().Info("Starting consumer group", zap.String("topics", strings.Join(topics, ",")))

	for {
		fetches := k.client.PollFetches(ctx)
		if err := fetches.Err(); err != nil {
			if err == context.Canceled {
				zap.L().Info("Consumer context cancelled")
				k.mu.Lock()
				k.running = false
				k.mu.Unlock()
				return nil
			}
			zap.L().Error("Consumer poll error", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()

			msgCtx := k.extractContextFromHeaders(record.Headers)

			topicHandlers, exists := handlerMap[record.Topic]
			if !exists {
				zap.L().Warn("No handlers found for topic", zap.String("topic", record.Topic))
				continue
			}

			for _, handler := range topicHandlers {
				if err := handler.HandleMessage(msgCtx, record); err != nil {
					zap.L().Error("Failed to handle message", zap.Error(err),
						zap.String("topic", record.Topic),
					)
				}
			}
		}
	}
}

func (k *KafkaService) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.client.Close()
	k.running = false
	return nil
}

func (k *KafkaService) IsRunning() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.running
}

func (k *KafkaService) GetTopics() *KafkaTopics {
	return k.topics
}

func (k *KafkaService) extractContextFromHeaders(headers []kgo.RecordHeader) context.Context {
	ctx := context.Background()
	ctx = ExtractTraceContext(ctx, headers)
	for _, header := range headers {
		if header.Key == "request-id" {
			ctx = setRequestIDInContext(ctx, string(header.Value))
			break
		}
	}
	return ctx
}

type requestIDKeyType string

const requestIDCtxKey requestIDKeyType = "kafka_request_id"

func getRequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDCtxKey).(string); ok {
		return id
	}
	return ""
}

func setRequestIDInContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey, id)
}

func (k *KafkaService) Ping(ctx context.Context) error {
	if k.client == nil {
		return fmt.Errorf("kafka client is not initialized")
	}

	record := &kgo.Record{
		Topic: "__healthcheck",
		Key:   []byte("health"),
		Value: []byte("ping"),
	}

	if err := k.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("failed to send test message: %w", err)
	}
	return nil
}
