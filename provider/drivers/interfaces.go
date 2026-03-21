package drivers

import "context"

// CloudRunClient abstracts Cloud Run service operations.
type CloudRunClient interface {
	CreateService(ctx context.Context, projectID, region string, config map[string]any) (string, error)
	GetService(ctx context.Context, projectID, region, serviceID string) (map[string]any, error)
	UpdateService(ctx context.Context, projectID, region, serviceID string, config map[string]any) error
	DeleteService(ctx context.Context, projectID, region, serviceID string) error
}

// GKEClient abstracts GKE cluster operations.
type GKEClient interface {
	CreateCluster(ctx context.Context, projectID, location string, config map[string]any) (string, error)
	GetCluster(ctx context.Context, projectID, location, clusterID string) (map[string]any, error)
	UpdateCluster(ctx context.Context, projectID, location, clusterID string, config map[string]any) error
	DeleteCluster(ctx context.Context, projectID, location, clusterID string) error
}

// CloudSQLClient abstracts Cloud SQL instance operations.
type CloudSQLClient interface {
	CreateInstance(ctx context.Context, projectID string, config map[string]any) (string, error)
	GetInstance(ctx context.Context, projectID, instanceID string) (map[string]any, error)
	UpdateInstance(ctx context.Context, projectID, instanceID string, config map[string]any) error
	DeleteInstance(ctx context.Context, projectID, instanceID string) error
}

// MemorystoreClient abstracts Memorystore Redis operations.
type MemorystoreClient interface {
	CreateInstance(ctx context.Context, projectID, region string, config map[string]any) (string, error)
	GetInstance(ctx context.Context, projectID, region, instanceID string) (map[string]any, error)
	UpdateInstance(ctx context.Context, projectID, region, instanceID string, config map[string]any) error
	DeleteInstance(ctx context.Context, projectID, region, instanceID string) error
}

// VPCClient abstracts VPC network and subnet operations.
type VPCClient interface {
	CreateNetwork(ctx context.Context, projectID string, config map[string]any) (string, error)
	GetNetwork(ctx context.Context, projectID, networkID string) (map[string]any, error)
	DeleteNetwork(ctx context.Context, projectID, networkID string) error
	CreateSubnet(ctx context.Context, projectID, region string, config map[string]any) (string, error)
	GetSubnet(ctx context.Context, projectID, region, subnetID string) (map[string]any, error)
	DeleteSubnet(ctx context.Context, projectID, region, subnetID string) error
}

// LoadBalancerClient abstracts GCP load balancer operations.
type LoadBalancerClient interface {
	Create(ctx context.Context, projectID, region string, config map[string]any) (string, error)
	Get(ctx context.Context, projectID, region, lbID string) (map[string]any, error)
	Update(ctx context.Context, projectID, region, lbID string, config map[string]any) error
	Delete(ctx context.Context, projectID, region, lbID string) error
}

// DNSClient abstracts Cloud DNS operations.
type DNSClient interface {
	CreateManagedZone(ctx context.Context, projectID string, config map[string]any) (string, error)
	GetManagedZone(ctx context.Context, projectID, zoneID string) (map[string]any, error)
	UpdateManagedZone(ctx context.Context, projectID, zoneID string, config map[string]any) error
	DeleteManagedZone(ctx context.Context, projectID, zoneID string) error
}

// ArtifactRegistryClient abstracts Artifact Registry operations.
type ArtifactRegistryClient interface {
	CreateRepository(ctx context.Context, projectID, location string, config map[string]any) (string, error)
	GetRepository(ctx context.Context, projectID, location, repoID string) (map[string]any, error)
	UpdateRepository(ctx context.Context, projectID, location, repoID string, config map[string]any) error
	DeleteRepository(ctx context.Context, projectID, location, repoID string) error
}

// APIGatewayClient abstracts API Gateway operations.
type APIGatewayClient interface {
	CreateGateway(ctx context.Context, projectID, region string, config map[string]any) (string, error)
	GetGateway(ctx context.Context, projectID, region, gatewayID string) (map[string]any, error)
	UpdateGateway(ctx context.Context, projectID, region, gatewayID string, config map[string]any) error
	DeleteGateway(ctx context.Context, projectID, region, gatewayID string) error
}

// FirewallClient abstracts VPC firewall rule operations.
type FirewallClient interface {
	CreateRule(ctx context.Context, projectID string, config map[string]any) (string, error)
	GetRule(ctx context.Context, projectID, ruleID string) (map[string]any, error)
	UpdateRule(ctx context.Context, projectID, ruleID string, config map[string]any) error
	DeleteRule(ctx context.Context, projectID, ruleID string) error
}

// IAMClient abstracts IAM role/binding operations.
type IAMClient interface {
	CreateRole(ctx context.Context, projectID string, config map[string]any) (string, error)
	GetRole(ctx context.Context, projectID, roleID string) (map[string]any, error)
	UpdateRole(ctx context.Context, projectID, roleID string, config map[string]any) error
	DeleteRole(ctx context.Context, projectID, roleID string) error
}

// GCSClient abstracts Cloud Storage bucket operations.
type GCSClient interface {
	CreateBucket(ctx context.Context, projectID string, config map[string]any) (string, error)
	GetBucket(ctx context.Context, bucketID string) (map[string]any, error)
	UpdateBucket(ctx context.Context, bucketID string, config map[string]any) error
	DeleteBucket(ctx context.Context, bucketID string) error
}

// SSLCertificateClient abstracts SSL certificate operations.
type SSLCertificateClient interface {
	CreateCertificate(ctx context.Context, projectID string, config map[string]any) (string, error)
	GetCertificate(ctx context.Context, projectID, certID string) (map[string]any, error)
	UpdateCertificate(ctx context.Context, projectID, certID string, config map[string]any) error
	DeleteCertificate(ctx context.Context, projectID, certID string) error
}
