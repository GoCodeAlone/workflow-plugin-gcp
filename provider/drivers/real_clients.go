package drivers

import (
	"context"
	"fmt"

	apigateway "cloud.google.com/go/apigateway/apiv1"
	apigatewpb "cloud.google.com/go/apigateway/apiv1/apigatewaypb"
	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	artifactregistrypb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	container "cloud.google.com/go/container/apiv1"
	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	redis "cloud.google.com/go/redis/apiv1"
	redispb "cloud.google.com/go/redis/apiv1/redispb"
	run "cloud.google.com/go/run/apiv2"
	runpb "cloud.google.com/go/run/apiv2/runpb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// --- Cloud Run ---

type realCloudRunClient struct {
	svc     *run.ServicesClient
	project string
	region  string
}

func NewRealCloudRunClient(ctx context.Context, project, region string, opts ...option.ClientOption) (CloudRunClient, error) {
	svc, err := run.NewServicesClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("cloud run client: %w", err)
	}
	return &realCloudRunClient{svc: svc, project: project, region: region}, nil
}

func (c *realCloudRunClient) CreateService(ctx context.Context, projectID, region string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "service"
	}
	image, _ := config["image"].(string)
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	req := &runpb.CreateServiceRequest{
		Parent:    parent,
		ServiceId: name,
		Service: &runpb.Service{
			Template: &runpb.RevisionTemplate{
				Containers: []*runpb.Container{{Image: image}},
			},
		},
	}
	op, err := c.svc.CreateService(ctx, req)
	if err != nil {
		return "", err
	}
	svc, err := op.Wait(ctx)
	if err != nil {
		return "", err
	}
	return svc.Name, nil
}

func (c *realCloudRunClient) GetService(ctx context.Context, projectID, region, serviceID string) (map[string]any, error) {
	name := fmt.Sprintf("projects/%s/locations/%s/services/%s", projectID, region, serviceID)
	svc, err := c.svc.GetService(ctx, &runpb.GetServiceRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":   svc.Name,
		"uri":    svc.Uri,
		"status": "running",
	}, nil
}

func (c *realCloudRunClient) UpdateService(ctx context.Context, projectID, region, serviceID string, config map[string]any) error {
	name := fmt.Sprintf("projects/%s/locations/%s/services/%s", projectID, region, serviceID)
	svc, err := c.svc.GetService(ctx, &runpb.GetServiceRequest{Name: name})
	if err != nil {
		return err
	}
	if image, ok := config["image"].(string); ok && len(svc.Template.Containers) > 0 {
		svc.Template.Containers[0].Image = image
	}
	op, opErr := c.svc.UpdateService(ctx, &runpb.UpdateServiceRequest{Service: svc})
	if opErr != nil {
		return opErr
	}
	_, err = op.Wait(ctx)
	return err
}

func (c *realCloudRunClient) DeleteService(ctx context.Context, projectID, region, serviceID string) error {
	name := fmt.Sprintf("projects/%s/locations/%s/services/%s", projectID, region, serviceID)
	op, err := c.svc.DeleteService(ctx, &runpb.DeleteServiceRequest{Name: name})
	if err != nil {
		return err
	}
	_, err = op.Wait(ctx)
	return err
}

// --- GKE ---

type realGKEClient struct {
	svc *container.ClusterManagerClient
}

func NewRealGKEClient(ctx context.Context, opts ...option.ClientOption) (GKEClient, error) {
	svc, err := container.NewClusterManagerClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gke client: %w", err)
	}
	return &realGKEClient{svc: svc}, nil
}

func (c *realGKEClient) CreateCluster(ctx context.Context, projectID, location string, config map[string]any) (string, error) {
	clusterName, _ := config["name"].(string)
	if clusterName == "" {
		clusterName = "cluster"
	}
	machineType, _ := config["machine_type"].(string)
	if machineType == "" {
		machineType = "e2-medium"
	}
	nodeCount := int32(3)
	if nc, ok := config["node_count"].(int); ok {
		nodeCount = int32(nc)
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, location)
	req := &containerpb.CreateClusterRequest{
		Parent: parent,
		Cluster: &containerpb.Cluster{
			Name:             clusterName,
			InitialNodeCount: nodeCount,
			NodeConfig: &containerpb.NodeConfig{
				MachineType: machineType,
			},
		},
	}
	op, err := c.svc.CreateCluster(ctx, req)
	if err != nil {
		return "", err
	}
	return op.Name, nil
}

func (c *realGKEClient) GetCluster(ctx context.Context, projectID, location, clusterID string) (map[string]any, error) {
	name := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, clusterID)
	cluster, err := c.svc.GetCluster(ctx, &containerpb.GetClusterRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":     cluster.Name,
		"endpoint": cluster.Endpoint,
		"status":   cluster.Status.String(),
	}, nil
}

func (c *realGKEClient) UpdateCluster(ctx context.Context, projectID, location, clusterID string, config map[string]any) error {
	name := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, clusterID)
	if nodeCount, ok := config["node_count"].(int); ok {
		_, err := c.svc.SetNodePoolSize(ctx, &containerpb.SetNodePoolSizeRequest{
			Name:      name + "/nodePools/default-pool",
			NodeCount: int32(nodeCount),
		})
		return err
	}
	return nil
}

func (c *realGKEClient) DeleteCluster(ctx context.Context, projectID, location, clusterID string) error {
	name := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, clusterID)
	_, err := c.svc.DeleteCluster(ctx, &containerpb.DeleteClusterRequest{Name: name})
	return err
}

// --- Cloud SQL ---

type realCloudSQLClient struct {
	svc *sqladmin.Service
}

func NewRealCloudSQLClient(ctx context.Context, opts ...option.ClientOption) (CloudSQLClient, error) {
	svc, err := sqladmin.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("cloud sql client: %w", err)
	}
	return &realCloudSQLClient{svc: svc}, nil
}

func (c *realCloudSQLClient) CreateInstance(ctx context.Context, projectID string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "sql-instance"
	}
	tier, _ := config["tier"].(string)
	if tier == "" {
		tier = "db-f1-micro"
	}
	dbVersion, _ := config["database_version"].(string)
	if dbVersion == "" {
		dbVersion = "POSTGRES_15"
	}
	region, _ := config["region"].(string)
	inst := &sqladmin.DatabaseInstance{
		Name:            name,
		DatabaseVersion: dbVersion,
		Region:          region,
		Settings: &sqladmin.Settings{
			Tier: tier,
		},
	}
	op, err := c.svc.Instances.Insert(projectID, inst).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return op.TargetId, nil
}

func (c *realCloudSQLClient) GetInstance(ctx context.Context, projectID, instanceID string) (map[string]any, error) {
	inst, err := c.svc.Instances.Get(projectID, instanceID).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":   inst.Name,
		"status": inst.State,
		"tier":   inst.Settings.Tier,
	}, nil
}

func (c *realCloudSQLClient) UpdateInstance(ctx context.Context, projectID, instanceID string, config map[string]any) error {
	inst := &sqladmin.DatabaseInstance{Settings: &sqladmin.Settings{}}
	if tier, ok := config["tier"].(string); ok {
		inst.Settings.Tier = tier
	}
	_, err := c.svc.Instances.Patch(projectID, instanceID, inst).Context(ctx).Do()
	return err
}

func (c *realCloudSQLClient) DeleteInstance(ctx context.Context, projectID, instanceID string) error {
	_, err := c.svc.Instances.Delete(projectID, instanceID).Context(ctx).Do()
	return err
}

// --- Memorystore ---

type realMemorystoreClient struct {
	svc *redis.CloudRedisClient
}

func NewRealMemorystoreClient(ctx context.Context, opts ...option.ClientOption) (MemorystoreClient, error) {
	svc, err := redis.NewCloudRedisClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("memorystore client: %w", err)
	}
	return &realMemorystoreClient{svc: svc}, nil
}

func (c *realMemorystoreClient) CreateInstance(ctx context.Context, projectID, region string, config map[string]any) (string, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	instanceID, _ := config["name"].(string)
	if instanceID == "" {
		instanceID = "redis-instance"
	}
	memorySizeGB := int32(1)
	if gb, ok := config["memory_size_gb"].(int); ok {
		memorySizeGB = int32(gb)
	}
	tier := redispb.Instance_BASIC
	if t, ok := config["tier"].(string); ok && t == "STANDARD_HA" {
		tier = redispb.Instance_STANDARD_HA
	}
	req := &redispb.CreateInstanceRequest{
		Parent:     parent,
		InstanceId: instanceID,
		Instance: &redispb.Instance{
			Tier:         tier,
			MemorySizeGb: memorySizeGB,
		},
	}
	op, err := c.svc.CreateInstance(ctx, req)
	if err != nil {
		return "", err
	}
	inst, err := op.Wait(ctx)
	if err != nil {
		return "", err
	}
	return inst.Name, nil
}

func (c *realMemorystoreClient) GetInstance(ctx context.Context, projectID, region, instanceID string) (map[string]any, error) {
	name := fmt.Sprintf("projects/%s/locations/%s/instances/%s", projectID, region, instanceID)
	inst, err := c.svc.GetInstance(ctx, &redispb.GetInstanceRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":   inst.Name,
		"host":   inst.Host,
		"port":   inst.Port,
		"status": inst.State.String(),
	}, nil
}

func (c *realMemorystoreClient) UpdateInstance(ctx context.Context, projectID, region, instanceID string, config map[string]any) error {
	name := fmt.Sprintf("projects/%s/locations/%s/instances/%s", projectID, region, instanceID)
	inst := &redispb.Instance{Name: name}
	var paths []string
	if gb, ok := config["memory_size_gb"].(int); ok {
		inst.MemorySizeGb = int32(gb)
		paths = append(paths, "memory_size_gb")
	}
	req := &redispb.UpdateInstanceRequest{
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
		Instance:   inst,
	}
	op, err := c.svc.UpdateInstance(ctx, req)
	if err != nil {
		return err
	}
	_, err = op.Wait(ctx)
	return err
}

func (c *realMemorystoreClient) DeleteInstance(ctx context.Context, projectID, region, instanceID string) error {
	name := fmt.Sprintf("projects/%s/locations/%s/instances/%s", projectID, region, instanceID)
	op, err := c.svc.DeleteInstance(ctx, &redispb.DeleteInstanceRequest{Name: name})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

// --- VPC ---

type realVPCClient struct {
	networks *compute.NetworksClient
	subnets  *compute.SubnetworksClient
}

func NewRealVPCClient(ctx context.Context, opts ...option.ClientOption) (VPCClient, error) {
	networks, err := compute.NewNetworksRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("vpc networks client: %w", err)
	}
	subnets, err := compute.NewSubnetworksRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("vpc subnets client: %w", err)
	}
	return &realVPCClient{networks: networks, subnets: subnets}, nil
}

func (c *realVPCClient) CreateNetwork(ctx context.Context, projectID string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "vpc-network"
	}
	autoCreate := true
	if ac, ok := config["auto_create_subnetworks"].(bool); ok {
		autoCreate = ac
	}
	req := &computepb.InsertNetworkRequest{
		Project: projectID,
		NetworkResource: &computepb.Network{
			Name:                  &name,
			AutoCreateSubnetworks: &autoCreate,
		},
	}
	op, err := c.networks.Insert(ctx, req)
	if err != nil {
		return "", err
	}
	if err := op.Wait(ctx); err != nil {
		return "", err
	}
	return name, nil
}

func (c *realVPCClient) GetNetwork(ctx context.Context, projectID, networkID string) (map[string]any, error) {
	net, err := c.networks.Get(ctx, &computepb.GetNetworkRequest{
		Project: projectID,
		Network: networkID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":         net.GetName(),
		"network_id":   fmt.Sprintf("%d", net.GetId()),
		"self_link":    net.GetSelfLink(),
		"routing_mode": net.GetRoutingConfig().GetRoutingMode(),
	}, nil
}

func (c *realVPCClient) DeleteNetwork(ctx context.Context, projectID, networkID string) error {
	op, err := c.networks.Delete(ctx, &computepb.DeleteNetworkRequest{
		Project: projectID,
		Network: networkID,
	})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

func (c *realVPCClient) CreateSubnet(ctx context.Context, projectID, region string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "subnet"
	}
	ipRange, _ := config["ip_range"].(string)
	network, _ := config["network"].(string)
	networkURL := fmt.Sprintf("projects/%s/global/networks/%s", projectID, network)
	req := &computepb.InsertSubnetworkRequest{
		Project: projectID,
		Region:  region,
		SubnetworkResource: &computepb.Subnetwork{
			Name:        &name,
			IpCidrRange: &ipRange,
			Network:     &networkURL,
		},
	}
	op, err := c.subnets.Insert(ctx, req)
	if err != nil {
		return "", err
	}
	if err := op.Wait(ctx); err != nil {
		return "", err
	}
	return name, nil
}

func (c *realVPCClient) GetSubnet(ctx context.Context, projectID, region, subnetID string) (map[string]any, error) {
	sub, err := c.subnets.Get(ctx, &computepb.GetSubnetworkRequest{
		Project:    projectID,
		Region:     region,
		Subnetwork: subnetID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":     sub.GetName(),
		"ip_range": sub.GetIpCidrRange(),
	}, nil
}

func (c *realVPCClient) DeleteSubnet(ctx context.Context, projectID, region, subnetID string) error {
	op, err := c.subnets.Delete(ctx, &computepb.DeleteSubnetworkRequest{
		Project:    projectID,
		Region:     region,
		Subnetwork: subnetID,
	})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

// --- Load Balancer ---

type realLoadBalancerClient struct {
	forwarding *compute.GlobalForwardingRulesClient
}

func NewRealLoadBalancerClient(ctx context.Context, opts ...option.ClientOption) (LoadBalancerClient, error) {
	fwd, err := compute.NewGlobalForwardingRulesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load balancer client: %w", err)
	}
	return &realLoadBalancerClient{forwarding: fwd}, nil
}

func (c *realLoadBalancerClient) Create(ctx context.Context, projectID, _ string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "forwarding-rule"
	}
	target, _ := config["target"].(string)
	portRange, _ := config["port_range"].(string)
	if portRange == "" {
		portRange = "443"
	}
	req := &computepb.InsertGlobalForwardingRuleRequest{
		Project: projectID,
		ForwardingRuleResource: &computepb.ForwardingRule{
			Name:      &name,
			Target:    &target,
			PortRange: &portRange,
		},
	}
	op, err := c.forwarding.Insert(ctx, req)
	if err != nil {
		return "", err
	}
	if err := op.Wait(ctx); err != nil {
		return "", err
	}
	return name, nil
}

func (c *realLoadBalancerClient) Get(ctx context.Context, projectID, _ string, lbID string) (map[string]any, error) {
	rule, err := c.forwarding.Get(ctx, &computepb.GetGlobalForwardingRuleRequest{
		Project:        projectID,
		ForwardingRule: lbID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":   rule.GetName(),
		"ip":     rule.GetIPAddress(),
		"target": rule.GetTarget(),
		"status": "running",
	}, nil
}

func (c *realLoadBalancerClient) Update(ctx context.Context, projectID, _ string, lbID string, config map[string]any) error {
	target, _ := config["target"].(string)
	req := &computepb.SetTargetGlobalForwardingRuleRequest{
		Project:        projectID,
		ForwardingRule: lbID,
		TargetReferenceResource: &computepb.TargetReference{
			Target: &target,
		},
	}
	op, err := c.forwarding.SetTarget(ctx, req)
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

func (c *realLoadBalancerClient) Delete(ctx context.Context, projectID, _ string, lbID string) error {
	op, err := c.forwarding.Delete(ctx, &computepb.DeleteGlobalForwardingRuleRequest{
		Project:        projectID,
		ForwardingRule: lbID,
	})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

// --- DNS ---

type realDNSClient struct {
	svc *dns.Service
}

func NewRealDNSClient(ctx context.Context, opts ...option.ClientOption) (DNSClient, error) {
	svc, err := dns.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("dns client: %w", err)
	}
	return &realDNSClient{svc: svc}, nil
}

func (c *realDNSClient) CreateManagedZone(ctx context.Context, projectID string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "zone"
	}
	dnsName, _ := config["dns_name"].(string)
	zone := &dns.ManagedZone{
		Name:    name,
		DnsName: dnsName,
	}
	created, err := c.svc.ManagedZones.Create(projectID, zone).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

func (c *realDNSClient) GetManagedZone(ctx context.Context, projectID, zoneID string) (map[string]any, error) {
	zone, err := c.svc.ManagedZones.Get(projectID, zoneID).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":         zone.Name,
		"dns_name":     zone.DnsName,
		"name_servers": zone.NameServers,
	}, nil
}

func (c *realDNSClient) UpdateManagedZone(ctx context.Context, projectID, zoneID string, config map[string]any) error {
	zone := &dns.ManagedZone{}
	if desc, ok := config["description"].(string); ok {
		zone.Description = desc
	}
	_, err := c.svc.ManagedZones.Patch(projectID, zoneID, zone).Context(ctx).Do()
	return err
}

func (c *realDNSClient) DeleteManagedZone(ctx context.Context, projectID, zoneID string) error {
	return c.svc.ManagedZones.Delete(projectID, zoneID).Context(ctx).Do()
}

// --- Artifact Registry ---

type realArtifactRegistryClient struct {
	svc *artifactregistry.Client
}

func NewRealArtifactRegistryClient(ctx context.Context, opts ...option.ClientOption) (ArtifactRegistryClient, error) {
	svc, err := artifactregistry.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("artifact registry client: %w", err)
	}
	return &realArtifactRegistryClient{svc: svc}, nil
}

func (c *realArtifactRegistryClient) CreateRepository(ctx context.Context, projectID, location string, config map[string]any) (string, error) {
	repoID, _ := config["name"].(string)
	if repoID == "" {
		repoID = "repo"
	}
	format := artifactregistrypb.Repository_DOCKER
	if f, ok := config["format"].(string); ok && f == "MAVEN" {
		format = artifactregistrypb.Repository_MAVEN
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, location)
	req := &artifactregistrypb.CreateRepositoryRequest{
		Parent:       parent,
		RepositoryId: repoID,
		Repository: &artifactregistrypb.Repository{
			Format: format,
		},
	}
	op, err := c.svc.CreateRepository(ctx, req)
	if err != nil {
		return "", err
	}
	repo, err := op.Wait(ctx)
	if err != nil {
		return "", err
	}
	return repo.Name, nil
}

func (c *realArtifactRegistryClient) GetRepository(ctx context.Context, projectID, location, repoID string) (map[string]any, error) {
	name := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", projectID, location, repoID)
	repo, err := c.svc.GetRepository(ctx, &artifactregistrypb.GetRepositoryRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":   repo.Name,
		"format": repo.Format.String(),
	}, nil
}

func (c *realArtifactRegistryClient) UpdateRepository(ctx context.Context, projectID, location, repoID string, config map[string]any) error {
	name := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", projectID, location, repoID)
	desc, _ := config["description"].(string)
	req := &artifactregistrypb.UpdateRepositoryRequest{
		Repository: &artifactregistrypb.Repository{
			Name:        name,
			Description: desc,
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"description"}},
	}
	_, err := c.svc.UpdateRepository(ctx, req)
	return err
}

func (c *realArtifactRegistryClient) DeleteRepository(ctx context.Context, projectID, location, repoID string) error {
	name := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", projectID, location, repoID)
	op, err := c.svc.DeleteRepository(ctx, &artifactregistrypb.DeleteRepositoryRequest{Name: name})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

// --- API Gateway ---

type realAPIGatewayClient struct {
	svc *apigateway.Client
}

func NewRealAPIGatewayClient(ctx context.Context, opts ...option.ClientOption) (APIGatewayClient, error) {
	svc, err := apigateway.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("api gateway client: %w", err)
	}
	return &realAPIGatewayClient{svc: svc}, nil
}

func (c *realAPIGatewayClient) CreateGateway(ctx context.Context, projectID, region string, config map[string]any) (string, error) {
	gatewayID, _ := config["name"].(string)
	if gatewayID == "" {
		gatewayID = "gateway"
	}
	apiConfig, _ := config["api_config"].(string)
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	req := &apigatewpb.CreateGatewayRequest{
		Parent:    parent,
		GatewayId: gatewayID,
		Gateway: &apigatewpb.Gateway{
			ApiConfig: apiConfig,
		},
	}
	op, err := c.svc.CreateGateway(ctx, req)
	if err != nil {
		return "", err
	}
	gw, err := op.Wait(ctx)
	if err != nil {
		return "", err
	}
	return gw.Name, nil
}

func (c *realAPIGatewayClient) GetGateway(ctx context.Context, projectID, region, gatewayID string) (map[string]any, error) {
	name := fmt.Sprintf("projects/%s/locations/%s/gateways/%s", projectID, region, gatewayID)
	gw, err := c.svc.GetGateway(ctx, &apigatewpb.GetGatewayRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":       gw.Name,
		"state":      gw.State.String(),
		"api_config": gw.ApiConfig,
		"status":     "running",
	}, nil
}

func (c *realAPIGatewayClient) UpdateGateway(ctx context.Context, projectID, region, gatewayID string, config map[string]any) error {
	name := fmt.Sprintf("projects/%s/locations/%s/gateways/%s", projectID, region, gatewayID)
	gw := &apigatewpb.Gateway{Name: name}
	var paths []string
	if ac, ok := config["api_config"].(string); ok {
		gw.ApiConfig = ac
		paths = append(paths, "api_config")
	}
	req := &apigatewpb.UpdateGatewayRequest{
		Gateway:    gw,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
	}
	op, err := c.svc.UpdateGateway(ctx, req)
	if err != nil {
		return err
	}
	_, err = op.Wait(ctx)
	return err
}

func (c *realAPIGatewayClient) DeleteGateway(ctx context.Context, projectID, region, gatewayID string) error {
	name := fmt.Sprintf("projects/%s/locations/%s/gateways/%s", projectID, region, gatewayID)
	op, err := c.svc.DeleteGateway(ctx, &apigatewpb.DeleteGatewayRequest{Name: name})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

// --- Firewall ---

type realFirewallClient struct {
	svc *compute.FirewallsClient
}

func NewRealFirewallClient(ctx context.Context, opts ...option.ClientOption) (FirewallClient, error) {
	svc, err := compute.NewFirewallsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("firewall client: %w", err)
	}
	return &realFirewallClient{svc: svc}, nil
}

func (c *realFirewallClient) CreateRule(ctx context.Context, projectID string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "fw-rule"
	}
	network, _ := config["network"].(string)
	if network == "" {
		network = "default"
	}
	networkURL := fmt.Sprintf("projects/%s/global/networks/%s", projectID, network)
	direction := "INGRESS"
	if d, ok := config["direction"].(string); ok && d == "EGRESS" {
		direction = "EGRESS"
	}
	req := &computepb.InsertFirewallRequest{
		Project: projectID,
		FirewallResource: &computepb.Firewall{
			Name:      &name,
			Network:   &networkURL,
			Direction: &direction,
		},
	}
	op, err := c.svc.Insert(ctx, req)
	if err != nil {
		return "", err
	}
	if err := op.Wait(ctx); err != nil {
		return "", err
	}
	return name, nil
}

func (c *realFirewallClient) GetRule(ctx context.Context, projectID, ruleID string) (map[string]any, error) {
	rule, err := c.svc.Get(ctx, &computepb.GetFirewallRequest{
		Project:  projectID,
		Firewall: ruleID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":      rule.GetName(),
		"direction": rule.GetDirection(),
		"network":   rule.GetNetwork(),
	}, nil
}

func (c *realFirewallClient) UpdateRule(ctx context.Context, projectID, ruleID string, config map[string]any) error {
	rule := &computepb.Firewall{Name: &ruleID}
	req := &computepb.PatchFirewallRequest{
		Project:          projectID,
		Firewall:         ruleID,
		FirewallResource: rule,
	}
	op, err := c.svc.Patch(ctx, req)
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

func (c *realFirewallClient) DeleteRule(ctx context.Context, projectID, ruleID string) error {
	op, err := c.svc.Delete(ctx, &computepb.DeleteFirewallRequest{
		Project:  projectID,
		Firewall: ruleID,
	})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

// --- IAM ---

type realIAMClient struct {
	svc *iam.Service
}

func NewRealIAMClient(ctx context.Context, opts ...option.ClientOption) (IAMClient, error) {
	svc, err := iam.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("iam client: %w", err)
	}
	return &realIAMClient{svc: svc}, nil
}

func (c *realIAMClient) CreateRole(ctx context.Context, projectID string, config map[string]any) (string, error) {
	roleID, _ := config["role_id"].(string)
	if roleID == "" {
		roleID = "customRole"
	}
	title, _ := config["title"].(string)
	var permissions []string
	if perms, ok := config["permissions"].([]string); ok {
		permissions = perms
	}
	req := &iam.CreateRoleRequest{
		RoleId: roleID,
		Role: &iam.Role{
			Title:               title,
			IncludedPermissions: permissions,
		},
	}
	role, err := c.svc.Projects.Roles.Create("projects/"+projectID, req).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return role.Name, nil
}

func (c *realIAMClient) GetRole(ctx context.Context, projectID, roleID string) (map[string]any, error) {
	name := fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
	role, err := c.svc.Projects.Roles.Get(name).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":        role.Name,
		"title":       role.Title,
		"permissions": role.IncludedPermissions,
	}, nil
}

func (c *realIAMClient) UpdateRole(ctx context.Context, projectID, roleID string, config map[string]any) error {
	name := fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
	role := &iam.Role{}
	if title, ok := config["title"].(string); ok {
		role.Title = title
	}
	_, err := c.svc.Projects.Roles.Patch(name, role).Context(ctx).Do()
	return err
}

func (c *realIAMClient) DeleteRole(ctx context.Context, projectID, roleID string) error {
	name := fmt.Sprintf("projects/%s/roles/%s", projectID, roleID)
	_, err := c.svc.Projects.Roles.Delete(name).Context(ctx).Do()
	return err
}

// --- GCS ---

type realGCSClient struct {
	svc *storage.Client
}

func NewRealGCSClient(ctx context.Context, opts ...option.ClientOption) (GCSClient, error) {
	svc, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcs client: %w", err)
	}
	return &realGCSClient{svc: svc}, nil
}

func (c *realGCSClient) CreateBucket(ctx context.Context, projectID string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "bucket"
	}
	location, _ := config["location"].(string)
	if location == "" {
		location = "US"
	}
	attrs := &storage.BucketAttrs{
		Location: location,
	}
	if sc, ok := config["storage_class"].(string); ok {
		attrs.StorageClass = sc
	}
	if err := c.svc.Bucket(name).Create(ctx, projectID, attrs); err != nil {
		return "", err
	}
	return name, nil
}

func (c *realGCSClient) GetBucket(ctx context.Context, bucketID string) (map[string]any, error) {
	attrs, err := c.svc.Bucket(bucketID).Attrs(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":          attrs.Name,
		"location":      attrs.Location,
		"storage_class": attrs.StorageClass,
	}, nil
}

func (c *realGCSClient) UpdateBucket(ctx context.Context, bucketID string, config map[string]any) error {
	update := storage.BucketAttrsToUpdate{}
	if sc, ok := config["storage_class"].(string); ok {
		update.StorageClass = sc
	}
	_, err := c.svc.Bucket(bucketID).Update(ctx, update)
	return err
}

func (c *realGCSClient) DeleteBucket(ctx context.Context, bucketID string) error {
	return c.svc.Bucket(bucketID).Delete(ctx)
}

// --- SSL Certificate ---

type realSSLClient struct {
	svc *compute.SslCertificatesClient
}

func NewRealSSLClient(ctx context.Context, opts ...option.ClientOption) (SSLCertificateClient, error) {
	svc, err := compute.NewSslCertificatesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("ssl client: %w", err)
	}
	return &realSSLClient{svc: svc}, nil
}

func (c *realSSLClient) CreateCertificate(ctx context.Context, projectID string, config map[string]any) (string, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "ssl-cert"
	}
	var domains []string
	if d, ok := config["domains"].(string); ok {
		domains = []string{d}
	}
	if ds, ok := config["domains"].([]string); ok {
		domains = ds
	}
	req := &computepb.InsertSslCertificateRequest{
		Project: projectID,
		SslCertificateResource: &computepb.SslCertificate{
			Name: &name,
			Managed: &computepb.SslCertificateManagedSslCertificate{
				Domains: domains,
			},
		},
	}
	op, err := c.svc.Insert(ctx, req)
	if err != nil {
		return "", err
	}
	if err := op.Wait(ctx); err != nil {
		return "", err
	}
	return name, nil
}

func (c *realSSLClient) GetCertificate(ctx context.Context, projectID, certID string) (map[string]any, error) {
	cert, err := c.svc.Get(ctx, &computepb.GetSslCertificateRequest{
		Project:        projectID,
		SslCertificate: certID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":    cert.GetName(),
		"status":  "ACTIVE",
		"domains": cert.GetManaged().GetDomains(),
	}, nil
}

func (c *realSSLClient) UpdateCertificate(_ context.Context, _, _ string, _ map[string]any) error {
	// Managed SSL certificates are immutable; updates require replacement.
	return fmt.Errorf("managed ssl certificates are immutable; delete and recreate instead")
}

func (c *realSSLClient) DeleteCertificate(ctx context.Context, projectID, certID string) error {
	op, err := c.svc.Delete(ctx, &computepb.DeleteSslCertificateRequest{
		Project:        projectID,
		SslCertificate: certID,
	})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}
