package discovery

import (
	"fmt"
	"google-service/internal/config"
	"log"
	"slices"
	"strconv"

	"github.com/hashicorp/consul/api"
)

type ServiceRegistry struct {
	client *api.Client
	config *config.Config
}

var ServiceDiscovery *ServiceRegistry

// Initialize ServiceDiscovery - should be called after config is loaded
func InitServiceDiscovery(cfg *config.Config) error {
	var err error
	ServiceDiscovery, err = NewServiceRegistry(cfg)
	if err != nil {
		return fmt.Errorf("service Discovery Init Failed: %s", err)
	}

	if err := ServiceDiscovery.Register(); err != nil {
		return fmt.Errorf("failed to register service: %s", err)
	}

	log.Println("Service Discovery initialized successfully")
	return nil
}

func NewServiceRegistry(config *config.Config) (*ServiceRegistry, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.Consul.Address

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
	httpPort, err := strconv.Atoi(sr.config.Service.Port)
	if err != nil {
		return fmt.Errorf("invalid HTTP port: %v", err)
	}

	// Generate service ID based on service name and address
	serviceID := fmt.Sprintf("%s-%s", sr.config.Service.Name, sr.config.Service.Address)

	httpRegistration := &api.AgentServiceRegistration{
		ID:      serviceID + "-http",
		Name:    sr.config.Service.Name,
		Port:    httpPort,
		Address: sr.config.Service.Address,
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%s/health", sr.config.Service.Address, sr.config.Service.Port),
			Interval: "10s",
			Timeout:  "5s",
		},
		Tags: []string{"google", "oauth", "http", "api"},
		Meta: map[string]string{
			"protocol": "http",
			"version":  "1.0",
		},
	}

	if err := sr.client.Agent().ServiceRegister(httpRegistration); err != nil {
		return fmt.Errorf("failed to register HTTP service with Consul: %v", err)
	}

	log.Printf("Successfully registered HTTP service %s with Consul at %s:%d",
		sr.config.Service.Name, sr.config.Service.Address, httpPort)
	return nil
}

func (sr *ServiceRegistry) Deregister() error {
	serviceID := fmt.Sprintf("%s-%s", sr.config.Service.Name, sr.config.Service.Address)

	if err := sr.client.Agent().ServiceDeregister(serviceID + "-http"); err != nil {
		log.Printf("Error deregistering HTTP service: %v", err)
		return err
	}

	log.Printf("Successfully deregistered service %s from Consul", sr.config.Service.Name)
	return nil
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

func (sr *ServiceRegistry) GetServiceAddress(serviceName string, protocol string) (string, error) {
	// Default to HTTP if no protocol specified
	if protocol == "" {
		protocol = "http"
	}

	services, meta, err := sr.client.Health().Service(serviceName, "", true, &api.QueryOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to find service %s: %v", serviceName, err)
	}

	log.Printf("Found %d instances of service %s (ConsulIndex: %d)", len(services), serviceName, meta.LastIndex)

	if len(services) == 0 {
		return "", fmt.Errorf("no healthy instances of service %s found", serviceName)
	}

	var matchingServices []*api.ServiceEntry
	for _, service := range services {
		if proto, ok := service.Service.Meta["protocol"]; ok && proto == protocol {
			matchingServices = append(matchingServices, service)
		} else if len(service.Service.Tags) > 0 {
			if slices.Contains(service.Service.Tags, protocol) {
				matchingServices = append(matchingServices, service)
			}
		}
	}

	if len(matchingServices) == 0 {
		return "", fmt.Errorf("no healthy instances of service %s with protocol %s found", serviceName, protocol)
	}

	// Use the first matching service
	service := matchingServices[0]

	address := service.Service.Address
	if address == "" {
		address = service.Node.Address
	}

	fullAddress := fmt.Sprintf("%s:%d", address, service.Service.Port)

	log.Printf("Using service address: %s (protocol: %s)", fullAddress, protocol)
	return fullAddress, nil
}

// GetServiceURL returns a full URL for the service
func (sr *ServiceRegistry) GetServiceURL(serviceName string, protocol string) (string, error) {
	address, err := sr.GetServiceAddress(serviceName, protocol)
	if err != nil {
		return "", err
	}

	var scheme string
	switch protocol {
	case "http":
		scheme = "http"
	case "https":
		scheme = "https"
	case "grpc":
		scheme = "grpc"
	default:
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, address), nil
}

// HealthCheck performs a health check on the service
func (sr *ServiceRegistry) HealthCheck() error {
	// Check if we can connect to Consul
	_, err := sr.client.Status().Leader()
	if err != nil {
		return fmt.Errorf("consul connection failed: %v", err)
	}

	// Check if our service is registered
	services, err := sr.client.Agent().Services()
	if err != nil {
		return fmt.Errorf("failed to get services: %v", err)
	}

	serviceID := fmt.Sprintf("%s-%s-http", sr.config.Service.Name, sr.config.Service.Address)
	if _, exists := services[serviceID]; !exists {
		return fmt.Errorf("service %s not registered", serviceID)
	}

	return nil
}
