package discovery

import (
	"auth_service/internal/config"
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/consul/api"
)

type ServiceRegistry struct {
	client *api.Client
	config *config.Config
}

var ServiceDiscovery *ServiceRegistry

func init() {
	var err error
	ServiceDiscovery, err = NewServiceRegistry(config.ServiceConfig)
	if err != nil {
		log.Fatalf("Service Discovery Init Failed: %s", err)
	}
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

// FindService looks up a service by name in Consul
func (sr *ServiceRegistry) FindService(serviceName string) ([]*api.ServiceEntry, error) {
	services, meta, err := sr.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find service %s: %v", serviceName, err)
	}

	log.Printf("Found %d instances of service %s (ConsulIndex: %d)", len(services), serviceName, meta.LastIndex)

	if len(services) == 0 {
		return nil, fmt.Errorf("no healthy instances of service %s found", serviceName)
	}

	return services, nil
}

func (sr *ServiceRegistry) GetServiceAddress(serviceName string) (string, error) {
	services, err := sr.FindService(serviceName)
	if err != nil {
		return "", err
	}

	service := services[0]

	address := service.Service.Address
	if address == "" {
		address = service.Node.Address
	}

	fullAddress := fmt.Sprintf("%s:%d", address, service.Service.Port)

	log.Printf("Using service address: %s", fullAddress)
	return fullAddress, nil
}
