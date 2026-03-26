package kafka

import (
	"be-modami-auth-service/config"
)

type KafkaTopics struct {
	User struct {
		Created string
		Updated string
	}
}

func GetKafkaTopics() *KafkaTopics {
	return &KafkaTopics{
		User: struct {
			Created string
			Updated string
		}{
			Created: "modami.auth.user.created",
			Updated: "modami.auth.user.updated",
		},
	}
}

func GetTopicWithEnv(cfg *config.Config, topic string) string {
	env := cfg.App.Environment
	if env == "" {
		env = "local"
	}
	return env + "." + topic
}

func GetAllTopics() []string {
	topics := GetKafkaTopics()
	return []string{
		topics.User.Created,
		topics.User.Updated,
	}
}

func TopicExists(topic string) bool {
	for _, t := range GetAllTopics() {
		if t == topic {
			return true
		}
	}
	return false
}
