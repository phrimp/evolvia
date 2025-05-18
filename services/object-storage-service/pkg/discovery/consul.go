package discovery

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/consul/api"
)

// ServiceRegistry handles service registration and discovery with Consul
type ServiceRegistry struct {
	client      *api.Client
	serviceName string
	serviceID   string
	servicePort string
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(consulAddress, serviceName, serviceID, servicePort string) (*ServiceRegistry, error) {
	// Create Consul client config
	config := api.DefaultConfig()
	config.Address = consulAddress

	// Create the client
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %w", err)
	}

	return &ServiceRegistry{
		client:      client,
		serviceName: serviceName,
		serviceID:   serviceID,
		servicePort: servicePort,
	}, nil
}

// Register registers the service with Consul
func (sr *ServiceRegistry) Register() error {
	// Convert port string to int
	port, err := strconv.Atoi(sr.servicePort)
	if err != nil {
		return fmt.Errorf("invalid port: %s: %w", sr.servicePort, err)
	}

	// Create registration
	registration := &api.AgentServiceRegistration{
		ID:   sr.serviceID,
		Name: sr.serviceName,
		Port: port,
		Tags: []string{"storage", "file", "avatar", "minio"},
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%s/health", sr.serviceName, sr.servicePort),
			Interval: "10s",
			Timeout:  "5s",
		},
	}

	// Register the service
	if err := sr.client.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	log.Printf("Service %s registered with Consul", sr.serviceName)
	return nil
}

// Deregister deregisters the service from Consul
func (sr *ServiceRegistry) Deregister() error {
	if err := sr.client.Agent().ServiceDeregister(sr.serviceID); err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	log.Printf("Service %s deregistered from Consul", sr.serviceName)
	return nil
}

// GetService gets a service by name
func (sr *ServiceRegistry) GetService(name string) (string, error) {
	// Get all healthy service instances
	services, _, err := sr.client.Health().Service(name, "", true, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get service: %w", err)
	}

	if len(services) == 0 {
		return "", fmt.Errorf("no healthy instances of service %s found", name)
	}

	// Choose the first healthy instance
	service := services[0]
	address := service.Service.Address
	if address == "" {
		address = service.Node.Address
	}

	return fmt.Sprintf("%s:%d", address, service.Service.Port), nil
}

// GetAllServices gets all healthy service instances by name
func (sr *ServiceRegistry) GetAllServices(name string) ([]string, error) {
	// Get all healthy service instances
	services, _, err := sr.client.Health().Service(name, "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no healthy instances of service %s found", name)
	}

	// Extract addresses
	addresses := make([]string, 0, len(services))
	for _, service := range services {
		address := service.Service.Address
		if address == "" {
			address = service.Node.Address
		}
		addresses = append(addresses, fmt.Sprintf("%s:%d", address, service.Service.Port))
	}

	return addresses, nil
}
