package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	mode := flag.String("mode", "", "setup or publish")
	rabbitURL := flag.String("rabbit-url", "amqp://rjs:test@rabbit:5672/", "RabbitMQ URL")
	natsURL := flag.String("nats-url", "nats://nats:4222", "NATS URL")
	count := flag.Int("count", 3, "messages to publish")
	flag.Parse()
	if err := run(*mode, *rabbitURL, *natsURL, *count); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(mode, rabbitURL, natsURL string, count int) error {
	rabbit, err := amqp.Dial(rabbitURL)
	if err != nil {
		return err
	}
	defer rabbit.Close()
	channel, err := rabbit.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if err := channel.ExchangeDeclare("shadow.events", "direct", false, false, false, false, nil); err != nil {
		return err
	}
	if _, err := channel.QueueDeclare("shadow.capture", false, false, false, false, nil); err != nil {
		return err
	}
	if err := channel.QueueBind("shadow.capture", "events", "shadow.events", false, nil); err != nil {
		return err
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	if mode == "setup" {
		if _, err := js.AddStream(&nats.StreamConfig{Name: "SHADOW", Subjects: []string{"shadow.events"}, Storage: nats.MemoryStorage}); err != nil {
			return err
		}
		return nil
	}
	if mode != "publish" || count < 1 {
		return fmt.Errorf("invalid mode or count")
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		q, err := channel.QueueInspect("shadow.capture")
		if err == nil && q.Consumers > 0 {
			names := js.ConsumerNames("SHADOW")
			if _, ok := <-names; ok {
				break
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("shadow consumers did not become ready")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := channel.Confirm(false); err != nil {
		return err
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, count))
	for i := 1; i <= count; i++ {
		id := fmt.Sprintf("shadow-%04d", i)
		body := []byte(fmt.Sprintf("payload-%04d", i))
		if err := channel.PublishWithContext(context.Background(), "shadow.events", "events", false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, MessageId: id, Body: body}); err != nil {
			return err
		}
		msg := nats.NewMsg("shadow.events")
		msg.Header.Set("Nats-Msg-Id", id)
		msg.Data = body
		if _, err := js.PublishMsg(msg); err != nil {
			return err
		}
	}
	for i := 0; i < count; i++ {
		if confirmation := <-confirms; !confirmation.Ack {
			return fmt.Errorf("RabbitMQ publish not confirmed")
		}
	}
	return nil
}
