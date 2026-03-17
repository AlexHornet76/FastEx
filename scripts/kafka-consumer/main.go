package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	broker := getEnv("KAFKA_BROKER", "localhost:29092")
	topic := getEnv("KAFKA_TOPIC", "trade.executed")
	groupID := getEnv("KAFKA_GROUP_ID", "trade-consumer-dev")

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})

	defer r.Close()

	log.Printf("consuming topic=%s broker=%s group=%s", topic, broker, groupID)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			log.Printf("read stopped: %v", err)
			return
		}

		log.Printf("partition=%d offset=%d key=%s value=%s time=%s",
			msg.Partition, msg.Offset, string(msg.Key), string(msg.Value), msg.Time.Format(time.RFC3339))
	}
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
