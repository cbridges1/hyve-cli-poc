package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/charmbracelet/huh"
	"github.com/civo/civogo"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"

	"hyve/internal/providerconfig"
)

// fetchRegionGroups queries the provider API for available regions and groups
// them geographically. Returns nil on failure so callers fall back to manual entry.
func fetchRegionGroups(ctx context.Context, provider string) []optionGroup {
	switch provider {
	case "civo":
		return fetchCivoRegionGroups(ctx)
	case "aws":
		return fetchAWSRegionGroups(ctx)
	case "gcp":
		return fetchGCPRegionGroups(ctx)
	case "azure":
		return fetchAzureRegionGroups(ctx)
	}
	return nil
}

// fetchNodeGroups queries the provider API for available node/instance types.
// region is used by AWS/GCP/Azure to filter region-specific offerings.
func fetchNodeGroups(ctx context.Context, provider, region string) []optionGroup {
	switch provider {
	case "civo":
		return fetchCivoNodeGroups(ctx)
	case "aws":
		return fetchAWSNodeGroups(ctx, region)
	case "gcp":
		return fetchGCPNodeGroups(ctx, region)
	case "azure":
		return fetchAzureNodeGroups(ctx, region)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// groupsFromMap converts a map[category][]options into a sorted []optionGroup.
func groupsFromMap(m map[string][]huh.Option[string]) []optionGroup {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	groups := make([]optionGroup, 0, len(keys))
	for _, k := range keys {
		groups = append(groups, optionGroup{Name: k, Options: m[k]})
	}
	return groups
}

// newProviderConfigManager returns a providerconfig.Manager using the current
// repo path, or nil if no repo is configured.
func newProviderConfigManager() *providerconfig.Manager {
	repoPath := getRepoPath()
	if repoPath == "" {
		return nil
	}
	return providerconfig.NewManager(repoPath)
}

// ── Civo ──────────────────────────────────────────────────────────────────────

func getCivoClient() (*civogo.Client, error) {
	if pcm := newProviderConfigManager(); pcm != nil {
		if cfg, err := pcm.LoadCivoConfig(); err == nil && len(cfg.Organizations) > 0 {
			if token, err := pcm.GetCivoToken(cfg.Organizations[0].Name); err == nil && token != "" {
				if client, err := civogo.NewClient(token, ""); err == nil {
					return client, nil
				}
			}
		}
	}
	// fall back to env var
	token := os.Getenv("CIVO_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("no Civo credentials available")
	}
	return civogo.NewClient(token, "")
}

func fetchCivoRegionGroups(ctx context.Context) []optionGroup {
	client, err := getCivoClient()
	if err != nil {
		return nil
	}
	regions, err := client.ListRegions()
	if err != nil {
		return nil
	}
	byCountry := map[string][]huh.Option[string]{}
	for _, r := range regions {
		group := r.CountryName
		if group == "" {
			group = "Other"
		}
		label := fmt.Sprintf("%s  (%s)", r.Code, r.Name)
		byCountry[group] = append(byCountry[group], huh.NewOption(label, r.Code))
	}
	return groupsFromMap(byCountry)
}

func fetchCivoNodeGroups(ctx context.Context) []optionGroup {
	client, err := getCivoClient()
	if err != nil {
		return nil
	}
	sizes, err := client.ListInstanceSizes()
	if err != nil {
		return nil
	}
	byFamily := map[string][]huh.Option[string]{}
	for _, s := range sizes {
		if !s.Selectable {
			continue
		}
		// Only include kubernetes-suitable sizes (filter by name convention)
		lower := strings.ToLower(s.Name)
		if !strings.Contains(lower, "kube") && !strings.Contains(lower, "k3s") {
			continue
		}
		label := fmt.Sprintf("%-24s  (%d vCPU, %d MB RAM)", s.Name, s.CPUCores, s.RAMMegabytes)
		// group by size family prefix (e.g. "g4s" from "g4s.kube.small")
		parts := strings.SplitN(s.Name, ".", 2)
		family := parts[0]
		byFamily[family] = append(byFamily[family], huh.NewOption(label, s.Name))
	}
	if len(byFamily) == 0 {
		// If no kube-labelled sizes, include all selectable sizes
		byFamily = map[string][]huh.Option[string]{}
		for _, s := range sizes {
			if !s.Selectable {
				continue
			}
			label := fmt.Sprintf("%-24s  (%d vCPU, %d MB RAM)", s.Name, s.CPUCores, s.RAMMegabytes)
			parts := strings.SplitN(s.Name, ".", 2)
			family := parts[0]
			byFamily[family] = append(byFamily[family], huh.NewOption(label, s.Name))
		}
	}
	return groupsFromMap(byFamily)
}

// ── AWS ───────────────────────────────────────────────────────────────────────

func getAWSEC2Client(ctx context.Context, region string) (*ec2.Client, error) {
	if region == "" {
		region = "us-east-1"
	}
	var opts []func(*awsconfig.LoadOptions) error
	opts = append(opts, awsconfig.WithRegion(region))

	if pcm := newProviderConfigManager(); pcm != nil {
		if accounts, err := pcm.ListAWSAccounts(); err == nil && len(accounts) > 0 {
			keyID, secret, token, err := pcm.GetAWSCredentials(accounts[0].Name)
			if err == nil && keyID != "" {
				opts = append(opts, awsconfig.WithCredentialsProvider(
					awscredentials.NewStaticCredentialsProvider(keyID, secret, token),
				))
			}
		}
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return ec2.NewFromConfig(cfg), nil
}

// awsRegionContinent maps AWS region prefix to a continent/geography name.
func awsRegionContinent(name string) string {
	switch {
	case strings.HasPrefix(name, "us-") || strings.HasPrefix(name, "ca-"):
		return "Americas"
	case strings.HasPrefix(name, "eu-"):
		return "Europe"
	case strings.HasPrefix(name, "ap-"):
		return "Asia Pacific"
	case strings.HasPrefix(name, "sa-"):
		return "South America"
	case strings.HasPrefix(name, "me-"):
		return "Middle East"
	case strings.HasPrefix(name, "af-"):
		return "Africa"
	case strings.HasPrefix(name, "il-"):
		return "Israel"
	case strings.HasPrefix(name, "us-gov-"):
		return "US GovCloud"
	default:
		return "Other"
	}
}

func fetchAWSRegionGroups(ctx context.Context) []optionGroup {
	client, err := getAWSEC2Client(ctx, "us-east-1")
	if err != nil {
		return nil
	}
	out, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: boolPtr(true),
	})
	if err != nil {
		return nil
	}
	byCont := map[string][]huh.Option[string]{}
	for _, r := range out.Regions {
		name := stringVal(r.RegionName)
		continent := awsRegionContinent(name)
		label := fmt.Sprintf("%-20s  (%s)", name, stringVal(r.OptInStatus))
		byCont[continent] = append(byCont[continent], huh.NewOption(label, name))
	}
	// Sort options within each group
	for k := range byCont {
		opts := byCont[k]
		sort.Slice(opts, func(i, j int) bool {
			return opts[i].Value < opts[j].Value
		})
		byCont[k] = opts
	}
	return groupsFromMap(byCont)
}

// awsInstanceFamily returns the instance family prefix (e.g. "t3" from "t3.medium").
func awsInstanceFamily(name string) string {
	parts := strings.SplitN(name, ".", 2)
	if len(parts) == 0 {
		return "other"
	}
	// strip trailing digits for nice grouping: "m6i" stays "m6i", "t3" stays "t3"
	return parts[0]
}

func fetchAWSNodeGroups(ctx context.Context, region string) []optionGroup {
	if region == "" {
		region = "us-east-1"
	}
	client, err := getAWSEC2Client(ctx, region)
	if err != nil {
		return nil
	}
	// DescribeInstanceTypeOfferings is fast — it lists types available in the region.
	paginator := ec2.NewDescribeInstanceTypeOfferingsPaginator(client, &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: ec2types.LocationTypeRegion,
	})
	byFamily := map[string][]huh.Option[string]{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			break
		}
		for _, o := range page.InstanceTypeOfferings {
			name := string(o.InstanceType)
			family := awsInstanceFamily(name)
			byFamily[family] = append(byFamily[family], huh.NewOption(name, name))
		}
	}
	// Sort within each family
	for k := range byFamily {
		opts := byFamily[k]
		sort.Slice(opts, func(i, j int) bool {
			return opts[i].Value < opts[j].Value
		})
		byFamily[k] = opts
	}
	return groupsFromMap(byFamily)
}

// ── GCP ───────────────────────────────────────────────────────────────────────

// getFirstGCPInfo returns (credentialsJSON, projectID) from the first configured
// GCP project, falling back to ("", "") so ADC is used.
func getFirstGCPInfo() (credJSON, projectID string) {
	if pcm := newProviderConfigManager(); pcm != nil {
		if projects, err := pcm.ListGCPProjects(); err == nil && len(projects) > 0 {
			name := projects[0].Name
			if id, err := pcm.GetGCPProjectID(name); err == nil {
				projectID = id
			}
			if j, err := pcm.GetGCPCredentialsJSON(name); err == nil {
				credJSON = j
			}
		}
	}
	return
}

func getGCPComputeService(ctx context.Context) (*compute.Service, string, error) {
	credJSON, projectID := getFirstGCPInfo()
	var svc *compute.Service
	var err error
	if credJSON != "" {
		svc, err = compute.NewService(ctx, option.WithCredentialsJSON([]byte(credJSON)))
	} else {
		svc, err = compute.NewService(ctx)
	}
	if err != nil {
		return nil, "", err
	}
	return svc, projectID, nil
}

// gcpRegionContinent maps GCP region prefix to a continent name.
func gcpRegionContinent(name string) string {
	switch {
	case strings.HasPrefix(name, "us-") || strings.HasPrefix(name, "northamerica-"):
		return "Americas"
	case strings.HasPrefix(name, "southamerica-"):
		return "South America"
	case strings.HasPrefix(name, "europe-"):
		return "Europe"
	case strings.HasPrefix(name, "asia-"):
		return "Asia Pacific"
	case strings.HasPrefix(name, "australia-"):
		return "Australia"
	case strings.HasPrefix(name, "me-"):
		return "Middle East"
	case strings.HasPrefix(name, "africa-"):
		return "Africa"
	default:
		return "Other"
	}
}

func fetchGCPRegionGroups(ctx context.Context) []optionGroup {
	svc, projectID, err := getGCPComputeService(ctx)
	if err != nil || projectID == "" {
		return nil
	}
	resp, err := svc.Regions.List(projectID).Context(ctx).Do()
	if err != nil {
		return nil
	}
	byCont := map[string][]huh.Option[string]{}
	for _, r := range resp.Items {
		continent := gcpRegionContinent(r.Name)
		byCont[continent] = append(byCont[continent], huh.NewOption(r.Name, r.Name))
	}
	for k := range byCont {
		opts := byCont[k]
		sort.Slice(opts, func(i, j int) bool { return opts[i].Value < opts[j].Value })
		byCont[k] = opts
	}
	return groupsFromMap(byCont)
}

// gcpMachineFamily returns the machine family prefix (e.g. "n2" from "n2-standard-4").
func gcpMachineFamily(name string) string {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) == 0 {
		return "other"
	}
	return strings.ToUpper(parts[0])
}

func fetchGCPNodeGroups(ctx context.Context, region string) []optionGroup {
	svc, projectID, err := getGCPComputeService(ctx)
	if err != nil || projectID == "" {
		return nil
	}
	// Use a zone derived from the region (append "-b" as a reasonable default)
	zone := region
	if region != "" && !strings.Contains(region, "-b") && !strings.Contains(region, "-a") {
		zone = region + "-b"
	}
	resp, err := svc.MachineTypes.List(projectID, zone).Context(ctx).Do()
	if err != nil {
		return nil
	}
	byFamily := map[string][]huh.Option[string]{}
	for _, mt := range resp.Items {
		family := gcpMachineFamily(mt.Name)
		label := fmt.Sprintf("%-28s  (%d vCPU, %d MB RAM)", mt.Name, mt.GuestCpus, mt.MemoryMb)
		byFamily[family] = append(byFamily[family], huh.NewOption(label, mt.Name))
	}
	for k := range byFamily {
		opts := byFamily[k]
		sort.Slice(opts, func(i, j int) bool { return opts[i].Value < opts[j].Value })
		byFamily[k] = opts
	}
	return groupsFromMap(byFamily)
}

// ── Azure ─────────────────────────────────────────────────────────────────────

// getFirstAzureInfo returns (credential, subscriptionID) using the first configured
// Azure subscription. Falls back to DefaultAzureCredential.
func getFirstAzureInfo() (cred *azidentity.ClientSecretCredential, subscriptionID string, defCred *azidentity.DefaultAzureCredential) {
	if pcm := newProviderConfigManager(); pcm != nil {
		if subs, err := pcm.ListAzureSubscriptions(); err == nil && len(subs) > 0 {
			name := subs[0].Name
			if id, err := pcm.GetAzureSubscriptionID(name); err == nil {
				subscriptionID = id
			}
			if tenantID, clientID, clientSecret, err := pcm.GetAzureCredentials(name); err == nil {
				c, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
				if err == nil {
					cred = c
					return
				}
			}
		}
	}
	// Fall back to default Azure credential chain
	dc, _ := azidentity.NewDefaultAzureCredential(nil)
	defCred = dc
	return
}

func fetchAzureRegionGroups(ctx context.Context) []optionGroup {
	cred, subID, defCred := getFirstAzureInfo()
	if subID == "" {
		return nil
	}
	var tokenCred interface {
		GetToken(context.Context, interface{}) (interface{}, error)
	}
	_ = tokenCred

	// Build the subscriptions client
	var subsClient *armsubscriptions.Client
	var err error
	if cred != nil {
		subsClient, err = armsubscriptions.NewClient(cred, nil)
	} else if defCred != nil {
		subsClient, err = armsubscriptions.NewClient(defCred, nil)
	} else {
		return nil
	}
	if err != nil {
		return nil
	}

	pager := subsClient.NewListLocationsPager(subID, nil)
	byGroup := map[string][]huh.Option[string]{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			break
		}
		for _, loc := range page.Value {
			name := stringVal(loc.Name)
			display := stringVal(loc.DisplayName)
			group := "Other"
			if loc.Metadata != nil && loc.Metadata.GeographyGroup != nil {
				group = *loc.Metadata.GeographyGroup
			}
			label := fmt.Sprintf("%-24s  (%s)", name, display)
			byGroup[group] = append(byGroup[group], huh.NewOption(label, name))
		}
	}
	for k := range byGroup {
		opts := byGroup[k]
		sort.Slice(opts, func(i, j int) bool { return opts[i].Value < opts[j].Value })
		byGroup[k] = opts
	}
	return groupsFromMap(byGroup)
}

// azureVMFamily returns the VM size family (e.g. "Standard_D" from "Standard_D2s_v3").
func azureVMFamily(name string) string {
	// Azure VM size format: Standard_<family><size>[_<extra>]
	// e.g. Standard_D2s_v3, Standard_B2s, Standard_F4s_v2
	parts := strings.SplitN(name, "_", 3)
	if len(parts) < 2 {
		return "Other"
	}
	// Strip trailing digits from the family part to get the base family
	family := strings.TrimRight(parts[1], "0123456789")
	if family == "" {
		family = parts[1]
	}
	return family
}

func fetchAzureNodeGroups(ctx context.Context, location string) []optionGroup {
	if location == "" {
		return nil
	}
	cred, subID, defCred := getFirstAzureInfo()
	if subID == "" {
		return nil
	}

	var vmClient *armcompute.VirtualMachineSizesClient
	var err error
	if cred != nil {
		vmClient, err = armcompute.NewVirtualMachineSizesClient(subID, cred, nil)
	} else if defCred != nil {
		vmClient, err = armcompute.NewVirtualMachineSizesClient(subID, defCred, nil)
	} else {
		return nil
	}
	if err != nil {
		return nil
	}

	pager := vmClient.NewListPager(location, nil)
	byFamily := map[string][]huh.Option[string]{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			break
		}
		for _, s := range page.Value {
			name := stringVal(s.Name)
			family := azureVMFamily(name)
			label := fmt.Sprintf("%-32s  (%d vCPU, %d MB RAM)", name, int32Val(s.NumberOfCores), int32Val(s.MemoryInMB))
			byFamily[family] = append(byFamily[family], huh.NewOption(label, name))
		}
	}
	for k := range byFamily {
		opts := byFamily[k]
		sort.Slice(opts, func(i, j int) bool { return opts[i].Value < opts[j].Value })
		byFamily[k] = opts
	}
	return groupsFromMap(byFamily)
}

// ── pointer helpers ───────────────────────────────────────────────────────────

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func int32Val(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

func boolPtr(b bool) *bool {
	return &b
}
