package discovery

import (
	"fmt"
	"log"
	"middleware/internal/config"
	"strconv"

	"github.com/hashicorp/consul/api"
)

type ServiceRegistry struct {
	client *api.Client
	config *config.Config
}

func NewServiceRegistry(config *config.Config) (*ServiceRegistry, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.ConsulAddress

	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %v", err)
	}

	return &ServiceRegistry{
		client: client,
		config: config,
	}, nil
}

func (sr *ServiceRegistry) Register() error {
	port, _ := strconv.Atoi(sr.config.Port)
	registration := &api.AgentServiceRegistration{
		ID:      sr.config.ServiceID,
		Name:    sr.config.ServiceName,
		Port:    port,
		Address: sr.config.ServiceAddress,
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%s/health", sr.config.ServiceAddress, sr.config.Port),
			Interval: "10s",
			Timeout:  "5s",
		},
		Tags: []string{"auth", "jwt"},
	}

	err := sr.client.Agent().ServiceRegister(registration)
	if err != nil {
		return fmt.Errorf("failed to register service with Consul: %v", err)
	}

	log.Println("Successfully registered service with Consul")
	return nil
}

// Deregister removes the service from Consul
func (sr *ServiceRegistry) Deregister() error {
	return sr.client.Agent().ServiceDeregister(sr.config.ServiceID)
}
