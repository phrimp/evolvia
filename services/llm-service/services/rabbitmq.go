package services

import (
	"encoding/json"
	"log"

	"llm-service/configs"

	"github.com/streadway/amqp"
)

type RabbitMQService struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

var rabbitMQService *RabbitMQService

func InitRabbitMQ() error {
	conn, err := amqp.Dial(configs.AppConfig.RabbitMQURI)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	rabbitMQService = &RabbitMQService{
		Connection: conn,
		Channel:    ch,
	}

	// Declare queues
	err = rabbitMQService.declareQueues()
	if err != nil {
		return err
	}

	log.Println("Connected to RabbitMQ successfully")
	return nil
}

func GetRabbitMQService() *RabbitMQService {
	return rabbitMQService
}

func (r *RabbitMQService) IsConnected() bool {
	return r.Connection != nil && !r.Connection.IsClosed()
}

func (r *RabbitMQService) declareQueues() error {
	// Declare service registration queue
	_, err := r.Channel.QueueDeclare(
		"service.registration", // queue name
		true,                   // durable
		false,                  // delete when unused
		false,                  // exclusive
		false,                  // no-wait
		nil,                    // arguments
	)
	if err != nil {
		return err
	}

	// Declare LLM processing queue
	_, err = r.Channel.QueueDeclare(
		"llm.processing",
		true,
		false,
		false,
		false,
		nil,
	)
	return err
}

func (r *RabbitMQService) PublishServiceRegistration() error {
	serviceInfo := map[string]interface{}{
		"serviceName": configs.AppConfig.ServiceName,
		"version":     configs.AppConfig.ServiceVersion,
		"endpoint":    "http://localhost:" + configs.AppConfig.Port,
		"status":      "active",
		"capabilities": []string{
			"chat",
			"rag",
			"user-context",
		},
	}

	body, err := json.Marshal(serviceInfo)
	if err != nil {
		return err
	}

	err = r.Channel.Publish(
		"",                     // exchange
		"service.registration", // routing key
		false,                  // mandatory
		false,                  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	return err
}

func (r *RabbitMQService) PublishLLMEvent(eventType string, data interface{}) error {
	event := map[string]interface{}{
		"type":      eventType,
		"data":      data,
		"timestamp": "",
		"service":   configs.AppConfig.ServiceName,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = r.Channel.Publish(
		"",               // exchange
		"llm.processing", // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	return err
}

func (r *RabbitMQService) Close() error {
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.Connection != nil {
		return r.Connection.Close()
	}
	return nil
}
