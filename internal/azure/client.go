package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/kubernetesconfiguration/armkubernetesconfiguration"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

// UserInfo represents the signed-in user.
type UserInfo struct {
	UPN      string
	Name     string
	TenantID string
}

// Subscription represents an Azure subscription.
type Subscription struct {
	ID       string
	Name     string
	TenantID string
}

// Cluster represents an AKS cluster with key properties.
type Cluster struct {
	Name              string
	ResourceGroup     string
	ResourceID        string
	Location          string
	SubscriptionID    string
	SubscriptionName  string
	KubernetesVersion string
	ProvisioningState string
	PowerState        string
	NodeCount         int
	FQDN              string
	NetworkPlugin     string
	NetworkPolicy     string
	NetworkDataplane  string
	ServiceCIDR       string
	DNSServiceIP      string
	PodCIDR           string
	SKU               string
	Tier              string
	Tags              map[string]string
	NodePools         []NodePool
	Addons            []string
	Extensions        []string
}

// NodePool represents an AKS node pool.
type NodePool struct {
	Name         string
	VMSize       string
	Count        int
	MinCount     int
	MaxCount     int
	Mode         string
	OSType       string
	OSDiskSizeGB int
	PowerState   string
	K8sVersion   string
	VnetSubnet   string
	PodSubnet    string
}

// Client wraps Azure SDK clients for AKS management.
type Client struct {
	cred *azidentity.DefaultAzureCredential
}

// NewClient creates a new Azure client using DefaultAzureCredential.
func NewClient() (*Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}
	return &Client{cred: cred}, nil
}

// GetUserInfo extracts UPN and name from the Azure access token claims.
func (c *Client) GetUserInfo(ctx context.Context) UserInfo {
	token, err := c.cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return UserInfo{UPN: "unknown", Name: "unknown"}
	}
	claims := parseJWTClaims(token.Token)
	upn := claims["upn"]
	if upn == "" {
		upn = claims["unique_name"]
	}
	if upn == "" {
		upn = claims["preferred_username"]
	}
	name := claims["name"]
	if upn == "" {
		upn = "unknown"
	}
	if name == "" {
		name = "unknown"
	}
	tenantID := claims["tid"]
	return UserInfo{UPN: upn, Name: name, TenantID: tenantID}
}

func parseJWTClaims(token string) map[string]string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload := parts[1]
	// Add padding if needed
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range raw {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

// ListSubscriptions returns all accessible Azure subscriptions.
func (c *Client) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	client, err := armsubscriptions.NewClient(c.cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	var subs []Subscription
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list subscriptions: %w", err)
		}
		for _, s := range page.Value {
			tid := ""
			if s.TenantID != nil {
				tid = *s.TenantID
			}
			subs = append(subs, Subscription{
				ID:       deref(s.SubscriptionID),
				Name:     deref(s.DisplayName),
				TenantID: tid,
			})
		}
	}
	return subs, nil
}

// ListClusters returns all AKS clusters across all subscriptions.
func (c *Client) ListClusters(ctx context.Context, subscriptions []Subscription) ([]Cluster, error) {
	var clusters []Cluster

	for _, sub := range subscriptions {
		subClusters, err := c.listClustersInSubscription(ctx, sub)
		if err != nil {
			continue // skip subscriptions where we lack permissions
		}
		clusters = append(clusters, subClusters...)
	}
	return clusters, nil
}

// StartCluster starts a stopped AKS cluster.
func (c *Client) StartCluster(ctx context.Context, subscriptionID, resourceGroup, clusterName string) error {
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, c.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	poller, err := client.BeginStart(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to start cluster: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed waiting for cluster start: %w", err)
	}
	return nil
}

// StopCluster stops a running AKS cluster.
func (c *Client) StopCluster(ctx context.Context, subscriptionID, resourceGroup, clusterName string) error {
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, c.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	poller, err := client.BeginStop(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to stop cluster: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed waiting for cluster stop: %w", err)
	}
	return nil
}

// DeleteCluster deletes an AKS cluster.
func (c *Client) DeleteCluster(ctx context.Context, subscriptionID, resourceGroup, clusterName string) error {
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, c.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	poller, err := client.BeginDelete(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed waiting for cluster deletion: %w", err)
	}
	return nil
}

func (c *Client) listClustersInSubscription(ctx context.Context, sub Subscription) ([]Cluster, error) {
	client, err := armcontainerservice.NewManagedClustersClient(sub.ID, c.cred, nil)
	if err != nil {
		return nil, err
	}

	var clusters []Cluster
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, mc := range page.Value {
			cluster := mapCluster(mc, sub)
			// Fetch extensions for this cluster
			extensions, _ := c.listExtensions(ctx, sub.ID, cluster.ResourceGroup, cluster.Name)
			cluster.Extensions = extensions
			clusters = append(clusters, cluster)
		}
	}
	return clusters, nil
}

func (c *Client) listExtensions(ctx context.Context, subscriptionID, resourceGroup, clusterName string) ([]string, error) {
	client, err := armkubernetesconfiguration.NewExtensionsClient(subscriptionID, c.cred, nil)
	if err != nil {
		return nil, err
	}

	var extensions []string
	pager := client.NewListPager(resourceGroup, "Microsoft.ContainerService", "managedClusters", clusterName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return extensions, nil // silently ignore errors
		}
		for _, ext := range page.Value {
			if ext.Name != nil {
				extensions = append(extensions, *ext.Name)
			}
		}
	}
	return extensions, nil
}

func mapCluster(mc *armcontainerservice.ManagedCluster, sub Subscription) Cluster {
	c := Cluster{
		Name:             deref(mc.Name),
		Location:         deref(mc.Location),
		SubscriptionID:   sub.ID,
		SubscriptionName: sub.Name,
	}

	// Extract resource group from ID
	if mc.ID != nil {
		c.ResourceID = deref(mc.ID)
		c.ResourceGroup = extractResourceGroup(c.ResourceID)
	}

	if mc.Properties != nil {
		props := mc.Properties
		c.KubernetesVersion = deref(props.CurrentKubernetesVersion)
		if c.KubernetesVersion == "" {
			c.KubernetesVersion = deref(props.KubernetesVersion)
		}
		c.ProvisioningState = deref(props.ProvisioningState)
		c.FQDN = deref(props.Fqdn)

		if props.PowerState != nil && props.PowerState.Code != nil {
			c.PowerState = string(*props.PowerState.Code)
		}

		if props.NetworkProfile != nil {
			np := props.NetworkProfile
			if np.NetworkPlugin != nil {
				c.NetworkPlugin = string(*np.NetworkPlugin)
			}
			if np.NetworkPolicy != nil {
				c.NetworkPolicy = string(*np.NetworkPolicy)
			}
			if np.NetworkDataplane != nil {
				c.NetworkDataplane = string(*np.NetworkDataplane)
			}
			c.ServiceCIDR = deref(np.ServiceCidr)
			c.DNSServiceIP = deref(np.DNSServiceIP)
			c.PodCIDR = deref(np.PodCidr)
		}

		if props.AgentPoolProfiles != nil {
			for _, pool := range props.AgentPoolProfiles {
				np := NodePool{
					Name:   deref(pool.Name),
					VMSize: deref(pool.VMSize),
				}
				if pool.Count != nil {
					np.Count = int(*pool.Count)
					c.NodeCount += np.Count
				}
				if pool.MinCount != nil {
					np.MinCount = int(*pool.MinCount)
				}
				if pool.MaxCount != nil {
					np.MaxCount = int(*pool.MaxCount)
				}
				if pool.Mode != nil {
					np.Mode = string(*pool.Mode)
				}
				if pool.OSType != nil {
					np.OSType = string(*pool.OSType)
				}
				if pool.OSDiskSizeGB != nil {
					np.OSDiskSizeGB = int(*pool.OSDiskSizeGB)
				}
				if pool.PowerState != nil && pool.PowerState.Code != nil {
					np.PowerState = string(*pool.PowerState.Code)
				}
				np.K8sVersion = deref(pool.CurrentOrchestratorVersion)
				if np.K8sVersion == "" {
					np.K8sVersion = deref(pool.OrchestratorVersion)
				}
				np.VnetSubnet = deref(pool.VnetSubnetID)
				np.PodSubnet = deref(pool.PodSubnetID)
				c.NodePools = append(c.NodePools, np)
			}
		}
	}

	if mc.SKU != nil {
		if mc.SKU.Name != nil {
			c.SKU = string(*mc.SKU.Name)
		}
		if mc.SKU.Tier != nil {
			c.Tier = string(*mc.SKU.Tier)
		}
	}

	if mc.Tags != nil {
		c.Tags = make(map[string]string)
		for k, v := range mc.Tags {
			c.Tags[k] = deref(v)
		}
	}

	// Extract enabled addons
	if mc.Properties != nil && mc.Properties.AddonProfiles != nil {
		for name, profile := range mc.Properties.AddonProfiles {
			if profile != nil && profile.Enabled != nil && *profile.Enabled {
				c.Addons = append(c.Addons, name)
			}
		}
	}

	return c
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func extractResourceGroup(id string) string {
	// /subscriptions/.../resourceGroups/RG_NAME/providers/...
	parts := splitPath(id)
	for i, p := range parts {
		if (p == "resourceGroups" || p == "resourcegroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
