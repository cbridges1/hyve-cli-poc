package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runInteractiveConfig() error {
	var section string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Config — which provider?").
				Options(
					huh.NewOption("Civo token management", "civo"),
					huh.NewOption("GCP project management", "gcp"),
					huh.NewOption("AWS configuration", "aws"),
					huh.NewOption("Azure configuration", "azure"),
				).
				Value(&section),
		),
	).Run()
	if err != nil {
		return err
	}

	switch section {
	case "civo":
		return interactiveConfigCivo()
	case "gcp":
		return interactiveConfigGCP()
	case "aws":
		return interactiveConfigAWS()
	case "azure":
		return interactiveConfigAzure()
	}
	return nil
}

// ── Civo ─────────────────────────────────────────────────────────────────────

func interactiveConfigCivo() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Civo token — action").
				Options(
					huh.NewOption("Set token", "set"),
					huh.NewOption("Get token", "get"),
					huh.NewOption("Clear token", "clear"),
					huh.NewOption("List tokens", "list"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		configCivoOrgListCmd.Run(configCivoOrgListCmd, nil)
		return nil
	case "set":
		var org, token string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Organization alias").Value(&org),
				huh.NewInput().Title("Token (leave blank to be prompted)").Value(&token),
			),
		).Run()
		if err != nil {
			return err
		}
		configCivoSetTokenCmd.Flags().Set("org", org)
		if token != "" {
			configCivoSetTokenCmd.Flags().Set("token", token)
		}
		configCivoSetTokenCmd.Run(configCivoSetTokenCmd, nil)
	case "get":
		var org string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Organization alias").Value(&org)),
		).Run()
		if err != nil {
			return err
		}
		configCivoGetTokenCmd.Flags().Set("org", org)
		configCivoGetTokenCmd.Run(configCivoGetTokenCmd, nil)
	case "clear":
		var org string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Organization alias to clear").Value(&org)),
		).Run()
		if err != nil {
			return err
		}
		var confirm bool
		err = newForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Clear Civo token for org '%s'?", org)).
					Value(&confirm),
			),
		).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
		configCivoClearTokenCmd.Flags().Set("org", org)
		configCivoClearTokenCmd.Run(configCivoClearTokenCmd, nil)
	}
	return nil
}

// ── GCP ──────────────────────────────────────────────────────────────────────

func interactiveConfigGCP() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("GCP project — action").
				Options(
					huh.NewOption("Add project alias", "add"),
					huh.NewOption("List projects", "list"),
					huh.NewOption("Get project", "get"),
					huh.NewOption("Remove project alias", "remove"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		configGCPListProjectsCmd.Run(configGCPListProjectsCmd, nil)
	case "add":
		var name, id string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Project alias").Placeholder("dev").Value(&name),
				huh.NewInput().Title("GCP project ID").Placeholder("my-project-123").Value(&id),
			),
		).Run()
		if err != nil {
			return err
		}
		configGCPAddProjectCmd.Flags().Set("name", name)
		configGCPAddProjectCmd.Flags().Set("id", id)
		configGCPAddProjectCmd.Run(configGCPAddProjectCmd, nil)
	case "get":
		var alias string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Project alias").Value(&alias)),
		).Run()
		if err != nil {
			return err
		}
		configGCPGetProjectCmd.Run(configGCPGetProjectCmd, []string{alias})
	case "remove":
		var alias string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Project alias to remove").Value(&alias)),
		).Run()
		if err != nil {
			return err
		}
		var confirm bool
		err = newForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove GCP project alias '%s'?", alias)).
					Value(&confirm),
			),
		).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
		configGCPRemoveProjectCmd.Run(configGCPRemoveProjectCmd, []string{alias})
	}
	return nil
}

// ── AWS ──────────────────────────────────────────────────────────────────────

func interactiveConfigAWS() error {
	var resource string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("AWS — what to configure?").
				Options(
					huh.NewOption("Account aliases", "account"),
					huh.NewOption("EKS role aliases", "eks-role"),
					huh.NewOption("VPC aliases", "vpc"),
				).
				Value(&resource),
		),
	).Run()
	if err != nil {
		return err
	}

	switch resource {
	case "account":
		return interactiveConfigAWSAccount()
	case "eks-role":
		return interactiveConfigAWSEKSRole()
	case "vpc":
		return interactiveConfigAWSVPC()
	}
	return nil
}

func interactiveConfigAWSAccount() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("AWS account — action").
				Options(
					huh.NewOption("Add", "add"),
					huh.NewOption("List", "list"),
					huh.NewOption("Get", "get"),
					huh.NewOption("Remove", "remove"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		configAWSAccountListCmd.Run(configAWSAccountListCmd, nil)
	case "add":
		var name, id string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Placeholder("prod").Value(&name),
				huh.NewInput().Title("AWS account ID").Placeholder("123456789012").Value(&id),
			),
		).Run()
		if err != nil {
			return err
		}
		configAWSAccountAddCmd.Flags().Set("name", name)
		configAWSAccountAddCmd.Flags().Set("id", id)
		configAWSAccountAddCmd.Run(configAWSAccountAddCmd, nil)
	case "get":
		var alias string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Account alias").Value(&alias)),
		).Run()
		if err != nil {
			return err
		}
		configAWSAccountGetCmd.Run(configAWSAccountGetCmd, []string{alias})
	case "remove":
		var alias string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Account alias to remove").Value(&alias)),
		).Run()
		if err != nil {
			return err
		}
		var confirm bool
		err = newForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove AWS account alias '%s'?", alias)).
					Value(&confirm),
			),
		).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
		configAWSAccountRemoveCmd.Run(configAWSAccountRemoveCmd, []string{alias})
	}
	return nil
}

func interactiveConfigAWSEKSRole() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("EKS role — action").
				Options(
					huh.NewOption("Add (register existing)", "add"),
					huh.NewOption("Create in AWS", "create"),
					huh.NewOption("List", "list"),
					huh.NewOption("Get", "get"),
					huh.NewOption("Remove alias", "remove"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		var account string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Account alias").Value(&account)),
		).Run()
		if err != nil {
			return err
		}
		configAWSEKSRoleListCmd.Flags().Set("account", account)
		configAWSEKSRoleListCmd.Run(configAWSEKSRoleListCmd, nil)
	case "add":
		var account, name, roleARN string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("Role alias").Value(&name),
				huh.NewInput().Title("IAM role ARN").Placeholder("arn:aws:iam::123456789012:role/...").Value(&roleARN),
			),
		).Run()
		if err != nil {
			return err
		}
		configAWSEKSRoleAddCmd.Flags().Set("account", account)
		configAWSEKSRoleAddCmd.Flags().Set("name", name)
		configAWSEKSRoleAddCmd.Flags().Set("role-arn", roleARN)
		configAWSEKSRoleAddCmd.Run(configAWSEKSRoleAddCmd, nil)
	case "create":
		var account, name, roleName, region string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("Role alias").Value(&name),
				huh.NewInput().Title("IAM role name in AWS").Placeholder("hyve-eks-role").Value(&roleName),
				huh.NewInput().Title("Region").Placeholder("us-east-1").Value(&region),
			),
		).Run()
		if err != nil {
			return err
		}
		configAWSEKSRoleCreateCmd.Flags().Set("account", account)
		configAWSEKSRoleCreateCmd.Flags().Set("name", name)
		configAWSEKSRoleCreateCmd.Flags().Set("role-name", roleName)
		configAWSEKSRoleCreateCmd.Flags().Set("region", region)
		configAWSEKSRoleCreateCmd.Run(configAWSEKSRoleCreateCmd, nil)
	case "get":
		var account, name string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("Role alias").Value(&name),
			),
		).Run()
		if err != nil {
			return err
		}
		configAWSEKSRoleGetCmd.Flags().Set("account", account)
		configAWSEKSRoleGetCmd.Flags().Set("name", name)
		configAWSEKSRoleGetCmd.Run(configAWSEKSRoleGetCmd, nil)
	case "remove":
		var account, name string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("Role alias to remove").Value(&name),
			),
		).Run()
		if err != nil {
			return err
		}
		var confirm bool
		err = newForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove EKS role alias '%s' from account '%s'?", name, account)).
					Value(&confirm),
			),
		).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
		configAWSEKSRoleRemoveCmd.Flags().Set("account", account)
		configAWSEKSRoleRemoveCmd.Flags().Set("name", name)
		configAWSEKSRoleRemoveCmd.Run(configAWSEKSRoleRemoveCmd, nil)
	}
	return nil
}

func interactiveConfigAWSVPC() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("VPC — action").
				Options(
					huh.NewOption("Add (register existing)", "add"),
					huh.NewOption("Create in AWS", "create"),
					huh.NewOption("List", "list"),
					huh.NewOption("Get", "get"),
					huh.NewOption("Remove alias", "remove"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		var account string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Account alias").Value(&account)),
		).Run()
		if err != nil {
			return err
		}
		configAWSVPCListCmd.Flags().Set("account", account)
		configAWSVPCListCmd.Run(configAWSVPCListCmd, nil)
	case "add":
		var account, name, id string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("VPC alias").Value(&name),
				huh.NewInput().Title("VPC ID").Placeholder("vpc-0123456789abcdef0").Value(&id),
			),
		).Run()
		if err != nil {
			return err
		}
		configAWSVPCAddCmd.Flags().Set("account", account)
		configAWSVPCAddCmd.Flags().Set("name", name)
		configAWSVPCAddCmd.Flags().Set("id", id)
		configAWSVPCAddCmd.Run(configAWSVPCAddCmd, nil)
	case "create":
		var account, name, region, cidr, subnets string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("VPC alias").Value(&name),
				huh.NewInput().Title("Region").Placeholder("us-east-1").Value(&region),
				huh.NewInput().Title("CIDR block (optional)").Placeholder("10.0.0.0/16").Value(&cidr),
				huh.NewInput().Title("Subnet CIDRs, comma-separated (optional)").Placeholder("10.0.1.0/24,10.0.2.0/24").Value(&subnets),
			),
		).Run()
		if err != nil {
			return err
		}
		configAWSVPCCreateCmd.Flags().Set("account", account)
		configAWSVPCCreateCmd.Flags().Set("name", name)
		configAWSVPCCreateCmd.Flags().Set("region", region)
		if cidr != "" {
			configAWSVPCCreateCmd.Flags().Set("cidr", cidr)
		}
		if subnets != "" {
			configAWSVPCCreateCmd.Flags().Set("subnets", subnets)
		}
		configAWSVPCCreateCmd.Run(configAWSVPCCreateCmd, nil)
	case "get":
		var account, name string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("VPC alias").Value(&name),
			),
		).Run()
		if err != nil {
			return err
		}
		configAWSVPCGetCmd.Flags().Set("account", account)
		configAWSVPCGetCmd.Flags().Set("name", name)
		configAWSVPCGetCmd.Run(configAWSVPCGetCmd, nil)
	case "remove":
		var account, name string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Account alias").Value(&account),
				huh.NewInput().Title("VPC alias to remove").Value(&name),
			),
		).Run()
		if err != nil {
			return err
		}
		var confirm bool
		err = newForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove VPC alias '%s' from account '%s'?", name, account)).
					Value(&confirm),
			),
		).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
		configAWSVPCRemoveCmd.Flags().Set("account", account)
		configAWSVPCRemoveCmd.Flags().Set("name", name)
		configAWSVPCRemoveCmd.Run(configAWSVPCRemoveCmd, nil)
	}
	return nil
}

// ── Azure ─────────────────────────────────────────────────────────────────────

func interactiveConfigAzure() error {
	var resource string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Azure — what to configure?").
				Options(
					huh.NewOption("Subscription aliases", "subscription"),
					huh.NewOption("Resource groups", "resource-group"),
				).
				Value(&resource),
		),
	).Run()
	if err != nil {
		return err
	}

	switch resource {
	case "subscription":
		return interactiveConfigAzureSubscription()
	case "resource-group":
		return interactiveConfigAzureResourceGroup()
	}
	return nil
}

func interactiveConfigAzureSubscription() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Subscription — action").
				Options(
					huh.NewOption("Add", "add"),
					huh.NewOption("List", "list"),
					huh.NewOption("Remove", "remove"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		configAzureListSubscriptionIDsCmd.Run(configAzureListSubscriptionIDsCmd, nil)
	case "add":
		var name, id string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Subscription alias").Placeholder("prod-sub").Value(&name),
				huh.NewInput().Title("Azure subscription ID").Placeholder("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx").Value(&id),
			),
		).Run()
		if err != nil {
			return err
		}
		configAzureAddSubscriptionIDsCmd.Flags().Set("name", name)
		configAzureAddSubscriptionIDsCmd.Flags().Set("id", id)
		configAzureAddSubscriptionIDsCmd.Run(configAzureAddSubscriptionIDsCmd, nil)
	case "remove":
		var name string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Subscription name to remove").Value(&name)),
		).Run()
		if err != nil {
			return err
		}
		var confirm bool
		err = newForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove Azure subscription '%s'?", name)).
					Value(&confirm),
			),
		).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
		configAzureRemoveSubscriptionIDsCmd.Run(configAzureRemoveSubscriptionIDsCmd, []string{name})
	}
	return nil
}

func interactiveConfigAzureResourceGroup() error {
	var action string
	err := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Resource group — action").
				Options(
					huh.NewOption("Add (create in Azure)", "add"),
					huh.NewOption("List", "list"),
					huh.NewOption("Delete", "delete"),
				).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	switch action {
	case "list":
		var subscription string
		err = newForm(
			huh.NewGroup(huh.NewInput().Title("Subscription alias").Value(&subscription)),
		).Run()
		if err != nil {
			return err
		}
		configAzureListResourceGroupsCmd.Flags().Set("subscription", subscription)
		configAzureListResourceGroupsCmd.Run(configAzureListResourceGroupsCmd, nil)
	case "add":
		var subscription, name, location string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Subscription alias").Value(&subscription),
				huh.NewInput().Title("Resource group name").Placeholder("hyve-rg").Value(&name),
				huh.NewInput().Title("Location/region").Placeholder("eastus").Value(&location),
			),
		).Run()
		if err != nil {
			return err
		}
		configAzureAddResourceGroupCmd.Flags().Set("subscription", subscription)
		configAzureAddResourceGroupCmd.Flags().Set("name", name)
		configAzureAddResourceGroupCmd.Flags().Set("location", location)
		configAzureAddResourceGroupCmd.Run(configAzureAddResourceGroupCmd, nil)
	case "delete":
		var subscription, name string
		err = newForm(
			huh.NewGroup(
				huh.NewInput().Title("Subscription alias").Value(&subscription),
				huh.NewInput().Title("Resource group name to delete").Value(&name),
			),
		).Run()
		if err != nil {
			return err
		}
		var confirm bool
		err = newForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Delete resource group '%s' from Azure subscription '%s'? This cannot be undone.", name, subscription)).
					Value(&confirm),
			),
		).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
		configAzureDeleteResourceGroupCmd.Flags().Set("subscription", subscription)
		configAzureDeleteResourceGroupCmd.Flags().Set("name", name)
		configAzureDeleteResourceGroupCmd.Run(configAzureDeleteResourceGroupCmd, nil)
	}
	return nil
}
