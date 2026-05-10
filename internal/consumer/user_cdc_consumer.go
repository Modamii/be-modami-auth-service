package consumer

import (
	"context"
	"encoding/json"

	"be-modami-auth-service/pkg/events"

	"github.com/twmb/franz-go/pkg/kgo"
	pkgkafka "gitlab.com/lifegoeson-libs/pkg-gokit/kafka"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

// cdcTopic is the Kafka topic Debezium publishes user_entity row changes to.
// Format: <topic.prefix>.<schema>.<table>
const cdcTopic = "keycloak-cdc.public.user_entity"

type debeziumEnvelope struct {
	Before *userEntityRow `json:"before"`
	After  *userEntityRow `json:"after"`
	// Op values: "c" = INSERT, "u" = UPDATE, "d" = DELETE, "r" = snapshot READ
	Op   string `json:"op"`
	TsMs int64  `json:"ts_ms"`
}

type userEntityRow struct {
	ID                       string  `json:"id"`
	Email                    string  `json:"email"`
	EmailConstraint          string  `json:"email_constraint"`
	EmailVerified            bool    `json:"email_verified"`
	Enabled                  bool    `json:"enabled"`
	FederationLink           *string `json:"federation_link"`
	FirstName                string  `json:"first_name"`
	LastName                 string  `json:"last_name"`
	RealmID                  string  `json:"realm_id"`
	Username                 string  `json:"username"`
	CreatedTimestamp         int64   `json:"created_timestamp"`
	ServiceAccountClientLink *string `json:"service_account_client_link"`
	NotBefore                int     `json:"not_before"`
}

// UserCDCConsumer reads Debezium CDC events from the user_entity topic and
// re-publishes them as application-level user events to Kafka.
type UserCDCConsumer struct {
	client   *kgo.Client
	producer pkgkafka.Producer
	logger   logging.Logger
	env      string
	cancel   context.CancelFunc
}

// NewUserCDCConsumer creates a new CDC consumer connected to the given brokers.
// groupID should be unique per consumer group (e.g. "be-modami-auth-service-cdc").
func NewUserCDCConsumer(
	brokers []string,
	groupID string,
	producer pkgkafka.Producer,
	env string,
	logger logging.Logger,
) (*UserCDCConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(cdcTopic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}
	return &UserCDCConsumer{
		client:   client,
		producer: producer,
		logger:   logger,
		env:      env,
	}, nil
}

// Start launches the CDC consumer loop in a background goroutine.
// Call Close() to stop it.
func (c *UserCDCConsumer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.run(ctx)
}

func (c *UserCDCConsumer) run(ctx context.Context) {
	c.logger.Info("CDC consumer started", logging.String("topic", cdcTopic))
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			c.logger.Info("CDC consumer stopped")
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				c.logger.Error("CDC fetch error", fe.Err,
					logging.String("topic", fe.Topic),
					logging.Int("partition", int(fe.Partition)),
				)
			}
			continue
		}

		fetches.EachRecord(func(r *kgo.Record) {
			c.processRecord(ctx, r)
		})

		if err := c.client.CommitUncommittedOffsets(ctx); err != nil && ctx.Err() == nil {
			c.logger.Error("CDC offset commit error", err)
		}
	}
}

func (c *UserCDCConsumer) processRecord(ctx context.Context, r *kgo.Record) {
	// Debezium sends a null value tombstone after DELETE — skip it.
	if r.Value == nil {
		return
	}

	var envelope debeziumEnvelope
	if err := json.Unmarshal(r.Value, &envelope); err != nil {
		c.logger.Error("CDC: failed to unmarshal debezium envelope", err)
		return
	}

	switch envelope.Op {
	case "c", "r":
		// INSERT or initial snapshot read → emit user.created
		if envelope.After == nil {
			return
		}
		c.logger.Info("CDC: processing user_entity change event", logging.Any("test", envelope))
		row := envelope.After
		topic := events.GetTopicWithEnv(c.env, events.TopicUserCreated)
		c.producer.EmitAsync(ctx, topic, &pkgkafka.ProducerMessage{
			Key: row.ID,
			Value: events.NewUserCreatedPayload(events.UserEntityFields{
				ID:                       row.ID,
				Email:                    row.Email,
				EmailConstraint:          row.EmailConstraint,
				EmailVerified:            row.EmailVerified,
				Enabled:                  row.Enabled,
				FederationLink:           row.FederationLink,
				FirstName:                row.FirstName,
				LastName:                 row.LastName,
				RealmID:                  row.RealmID,
				Username:                 row.Username,
				CreatedTimestamp:         row.CreatedTimestamp,
				ServiceAccountClientLink: row.ServiceAccountClientLink,
				NotBefore:                row.NotBefore,
			}),
		})
		c.logger.Info("CDC: emitted user.created",
			logging.String("user_id", row.ID),
			logging.String("topic", topic),
		)

	case "u":
		// UPDATE → emit user.updated
		if envelope.After == nil {
			return
		}
		row := envelope.After
		topic := events.GetTopicWithEnv(c.env, events.TopicUserUpdated)
		c.producer.EmitAsync(ctx, topic, &pkgkafka.ProducerMessage{
			Key: row.ID,
			Value: events.NewUserUpdatedPayload(events.UserEntityFields{
				ID:                       row.ID,
				Email:                    row.Email,
				EmailConstraint:          row.EmailConstraint,
				EmailVerified:            row.EmailVerified,
				Enabled:                  row.Enabled,
				FederationLink:           row.FederationLink,
				FirstName:                row.FirstName,
				LastName:                 row.LastName,
				RealmID:                  row.RealmID,
				Username:                 row.Username,
				CreatedTimestamp:         row.CreatedTimestamp,
				ServiceAccountClientLink: row.ServiceAccountClientLink,
				NotBefore:                row.NotBefore,
			}),
		})
		c.logger.Info("CDC: emitted user.updated",
			logging.String("user_id", row.ID),
			logging.String("topic", topic),
		)
	// "d" (DELETE) — no application event emitted.
	}
}

// Close stops the consumer loop and releases the Kafka client.
func (c *UserCDCConsumer) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	c.client.CloseAllowingRebalance()
}
