package util

import (
	"encoding/json"
	"fmt"
	"log"

	"yourapp/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *config.Config
}

type EmailMessage struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Type    string `json:"type"` // "otp", "reset_password", "verification"
}

const (
	EmailQueueName = "email_queue"
	EmailExchange  = "email_exchange"
)

func NewRabbitMQClient(cfg *config.Config) (*RabbitMQClient, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		cfg.RabbitMQUser,
		cfg.RabbitMQPassword,
		cfg.RabbitMQHost,
		cfg.RabbitMQPort,
	)

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = channel.ExchangeDeclare(
		EmailExchange, // name
		"direct",      // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue
	_, err = channel.QueueDeclare(
		EmailQueueName, // name
		true,           // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	err = channel.QueueBind(
		EmailQueueName, // queue name
		"email",        // routing key
		EmailExchange,  // exchange
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	return &RabbitMQClient{
		conn:    conn,
		channel: channel,
		config:  cfg,
	}, nil
}

// ensureConnection ensures the RabbitMQ connection and channel are open
// This is called only when needed (lazy reconnection), not continuously
func (r *RabbitMQClient) ensureConnection() error {
	// Check if connection is closed
	if r.conn == nil || r.conn.IsClosed() {
		log.Println("RabbitMQ connection closed, reconnecting...")
		if err := r.reconnect(); err != nil {
			return fmt.Errorf("failed to reconnect: %w", err)
		}
	}

	// Check if channel is closed
	if r.channel == nil || r.channel.IsClosed() {
		log.Println("RabbitMQ channel closed, recreating...")
		channel, err := r.conn.Channel()
		if err != nil {
			// If channel creation fails, try to reconnect the connection
			if r.conn.IsClosed() {
				if err := r.reconnect(); err != nil {
					return fmt.Errorf("failed to reconnect: %w", err)
				}
				channel, err = r.conn.Channel()
				if err != nil {
					return fmt.Errorf("failed to recreate channel: %w", err)
				}
			} else {
				return fmt.Errorf("failed to recreate channel: %w", err)
			}
		}

		// Re-declare exchange and queue
		if err := r.setupChannel(channel); err != nil {
			channel.Close()
			return fmt.Errorf("failed to setup channel: %w", err)
		}

		r.channel = channel
	}

	return nil
}

// reconnect reconnects to RabbitMQ
func (r *RabbitMQClient) reconnect() error {
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		r.config.RabbitMQUser,
		r.config.RabbitMQPassword,
		r.config.RabbitMQHost,
		r.config.RabbitMQPort,
	)

	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	if err := r.setupChannel(channel); err != nil {
		channel.Close()
		conn.Close()
		return err
	}

	r.conn = conn
	r.channel = channel
	log.Println("RabbitMQ reconnected successfully")
	return nil
}

// setupChannel sets up exchange, queue, and binding
func (r *RabbitMQClient) setupChannel(channel *amqp.Channel) error {
	// Declare exchange
	if err := channel.ExchangeDeclare(
		EmailExchange, // name
		"direct",      // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue
	if _, err := channel.QueueDeclare(
		EmailQueueName, // name
		true,           // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	); err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	if err := channel.QueueBind(
		EmailQueueName, // queue name
		"email",        // routing key
		EmailExchange,  // exchange
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	return nil
}

// PublishEmail publishes an email message to RabbitMQ
func (r *RabbitMQClient) PublishEmail(message EmailMessage) error {
	// Ensure connection is open
	if err := r.ensureConnection(); err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = r.channel.Publish(
		EmailExchange, // exchange
		"email",       // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // Make message persistent
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Printf("Email message published to queue: %s -> %s", message.Type, message.To)
	return nil
}

// Close closes the RabbitMQ connection
func (r *RabbitMQClient) Close() error {
	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			return err
		}
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// GetChannel returns the channel for consuming messages
func (r *RabbitMQClient) GetChannel() *amqp.Channel {
	return r.channel
}

// GetQueueName returns the email queue name
func (r *RabbitMQClient) GetQueueName() string {
	return EmailQueueName
}
