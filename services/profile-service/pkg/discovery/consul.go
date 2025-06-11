package discovery

import (
	"fmt"
	"log"
	"profile-service/internal/config"
	"slices"
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
	ServiceDiscovery.Register()
}

func NewServiceRegistry(config *config.Config) (*ServiceRegistry, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.Consul.ConsulAddress

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
	httpPort, _ := strconv.Atoi(sr.config.Server.Port)
	// grpcPort, _ := strconv.Atoi(sr.config.Server.GRPCPort)

	httpRegistration := &api.AgentServiceRegistration{
		ID:      sr.config.Server.ServiceID + "-http",
		Name:    sr.config.Server.ServiceName,
		Port:    httpPort,
		Address: sr.config.Server.ServiceAddress,
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%s/health", sr.config.Server.ServiceAddress, sr.config.Server.Port),
			Interval: "10s",
			Timeout:  "5s",
		},
		Tags: []string{"auth", "jwt", "http"},
		Meta: map[string]string{
			"protocol": "http",
		},
	}

	//grpcRegistration := &api.AgentServiceRegistration{
	//	ID:      sr.config.Server.ServiceID + "-grpc",
	//	Name:    sr.config.Server.ServiceName,
	//	Port:    grpcPort,
	//	Address: sr.config.Server.ServiceAddress,
	//	Check: &api.AgentServiceCheck{
	//		TCP:      fmt.Sprintf("%s:%s", sr.config.Server.ServiceAddress, sr.config.Server.GRPCPort),
	//		Interval: "10s",
	//		Timeout:  "5s",
	//	},
	//	Tags: []string{"auth", "jwt", "grpc"},
	//	Meta: map[string]string{
	//		"protocol": "grpc",
	//	},
	//}

	if err := sr.client.Agent().ServiceRegister(httpRegistration); err != nil {
		return fmt.Errorf("failed to register HTTP service with Consul: %v", err)
	}

	//if err := sr.client.Agent().ServiceRegister(grpcRegistration); err != nil {
	//	return fmt.Errorf("failed to register gRPC service with Consul: %v", err)
	//}

	log.Println("Successfully registered HTTP and gRPC services with Consul")
	return nil
}

func (sr *ServiceRegistry) Deregister() error {
	if err := sr.client.Agent().ServiceDeregister(sr.config.Server.ServiceID + "-http"); err != nil {
		log.Printf("Error deregistering HTTP service: %v", err)
	}

	if err := sr.client.Agent().ServiceDeregister(sr.config.Server.ServiceID + "-grpc"); err != nil {
		log.Printf("Error deregistering gRPC service: %v", err)
	}

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
