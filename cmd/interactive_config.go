package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runInteractiveConfig() error {
	for {
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
						huh.NewOption("← Back", "back"),
					).
					Value(&section),
			),
		).Run()
		if err != nil {
			return err
		}

		switch section {
		case "back":
			return errBack
		case "civo":
			if err := interactiveConfigCivo(); err != nil && err != errBack {
				return err
			}
		case "gcp":
			if err := interactiveConfigGCP(); err != nil && err != errBack {
				return err
			}
		case "aws":
			if err := interactiveConfigAWS(); err != nil && err != errBack {
				return err
			}
		case "azure":
			if err := interactiveConfigAzure(); err != nil && err != errBack {
				return err
			}
		}
	}
}

// ── Civo ─────────────────────────────────────────────────────────────────────

func interactiveConfigCivo() error {
	for {
		var action string
		err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Civo token — action").
					Options(
						huh.NewOption("Set token", "set"),
						huh.NewOption("Get token", "get"),
						huh.NewOption("Clear token", "clear"),
						huh.NewOption("List orgs", "list"),
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
		case "list":
			configCivoOrgListCmd.Run(configCivoOrgListCmd, nil)
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
			org := ""
			if err := selectFromList("Organization", fetchCivoOrgNames(), &org); err != nil {
				return err
			}
			configCivoGetTokenCmd.Flags().Set("org", org)
			configCivoGetTokenCmd.Run(configCivoGetTokenCmd, nil)
		case "clear":
			org := ""
			if err := selectFromList("Organization to clear token for", fetchCivoOrgNames(), &org); err != nil {
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
				continue
			}
			configCivoClearTokenCmd.Flags().Set("org", org)
			configCivoClearTokenCmd.Run(configCivoClearTokenCmd, nil)
		}
	}
}

// ── GCP ──────────────────────────────────────────────────────────────────────

func interactiveConfigGCP() error {
	for {
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
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
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
			alias := ""
			if err := selectFromList("Project alias", fetchGCPProjectNames(), &alias); err != nil {
				return err
			}
			configGCPGetProjectCmd.Run(configGCPGetProjectCmd, []string{alias})
		case "remove":
			alias := ""
			if err := selectFromList("Project alias to remove", fetchGCPProjectNames(), &alias); err != nil {
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
				continue
			}
			configGCPRemoveProjectCmd.Run(configGCPRemoveProjectCmd, []string{alias})
		}
	}
}

// ── AWS ──────────────────────────────────────────────────────────────────────

func interactiveConfigAWS() error {
	for {
		var resource string
		err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("AWS — what to configure?").
					Options(
						huh.NewOption("Account aliases", "account"),
						huh.NewOption("EKS role aliases", "eks-role"),
						huh.NewOption("VPC aliases", "vpc"),
						huh.NewOption("← Back", "back"),
					).
					Value(&resource),
			),
		).Run()
		if err != nil {
			return err
		}

		switch resource {
		case "back":
			return errBack
		case "account":
			if err := interactiveConfigAWSAccount(); err != nil && err != errBack {
				return err
			}
		case "eks-role":
			if err := interactiveConfigAWSEKSRole(); err != nil && err != errBack {
				return err
			}
		case "vpc":
			if err := interactiveConfigAWSVPC(); err != nil && err != errBack {
				return err
			}
		}
	}
}

func interactiveConfigAWSAccount() error {
	for {
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
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
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
			alias := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &alias); err != nil {
				return err
			}
			configAWSAccountGetCmd.Run(configAWSAccountGetCmd, []string{alias})
		case "remove":
			alias := ""
			if err := selectFromList("Account alias to remove", fetchAWSAccountNames(), &alias); err != nil {
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
				continue
			}
			configAWSAccountRemoveCmd.Run(configAWSAccountRemoveCmd, []string{alias})
		}
	}
}

func interactiveConfigAWSEKSRole() error {
	for {
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
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
		case "list":
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			configAWSEKSRoleListCmd.Flags().Set("account", account)
			configAWSEKSRoleListCmd.Run(configAWSEKSRoleListCmd, nil)
		case "add":
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			var name, roleARN string
			err = newForm(
				huh.NewGroup(
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
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			var name, roleName, region string
			err = newForm(
				huh.NewGroup(
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
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			name := ""
			if err := selectFromList("Role alias", fetchAWSEKSRoleNames(account), &name); err != nil {
				return err
			}
			configAWSEKSRoleGetCmd.Flags().Set("account", account)
			configAWSEKSRoleGetCmd.Flags().Set("name", name)
			configAWSEKSRoleGetCmd.Run(configAWSEKSRoleGetCmd, nil)
		case "remove":
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			name := ""
			if err := selectFromList("Role alias to remove", fetchAWSEKSRoleNames(account), &name); err != nil {
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
				continue
			}
			configAWSEKSRoleRemoveCmd.Flags().Set("account", account)
			configAWSEKSRoleRemoveCmd.Flags().Set("name", name)
			configAWSEKSRoleRemoveCmd.Run(configAWSEKSRoleRemoveCmd, nil)
		}
	}
}

func interactiveConfigAWSVPC() error {
	for {
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
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
		case "list":
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			configAWSVPCListCmd.Flags().Set("account", account)
			configAWSVPCListCmd.Run(configAWSVPCListCmd, nil)
		case "add":
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			var name, id string
			err = newForm(
				huh.NewGroup(
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
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			var name, region, cidr, subnets string
			err = newForm(
				huh.NewGroup(
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
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			name := ""
			if err := selectFromList("VPC alias", fetchAWSVPCNames(account), &name); err != nil {
				return err
			}
			configAWSVPCGetCmd.Flags().Set("account", account)
			configAWSVPCGetCmd.Flags().Set("name", name)
			configAWSVPCGetCmd.Run(configAWSVPCGetCmd, nil)
		case "remove":
			account := ""
			if err := selectFromList("Account alias", fetchAWSAccountNames(), &account); err != nil {
				return err
			}
			name := ""
			if err := selectFromList("VPC alias to remove", fetchAWSVPCNames(account), &name); err != nil {
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
				continue
			}
			configAWSVPCRemoveCmd.Flags().Set("account", account)
			configAWSVPCRemoveCmd.Flags().Set("name", name)
			configAWSVPCRemoveCmd.Run(configAWSVPCRemoveCmd, nil)
		}
	}
}

// ── Azure ─────────────────────────────────────────────────────────────────────

func interactiveConfigAzure() error {
	for {
		var resource string
		err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Azure — what to configure?").
					Options(
						huh.NewOption("Subscription aliases", "subscription"),
						huh.NewOption("Resource groups", "resource-group"),
						huh.NewOption("← Back", "back"),
					).
					Value(&resource),
			),
		).Run()
		if err != nil {
			return err
		}

		switch resource {
		case "back":
			return errBack
		case "subscription":
			if err := interactiveConfigAzureSubscription(); err != nil && err != errBack {
				return err
			}
		case "resource-group":
			if err := interactiveConfigAzureResourceGroup(); err != nil && err != errBack {
				return err
			}
		}
	}
}

func interactiveConfigAzureSubscription() error {
	for {
		var action string
		err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Subscription — action").
					Options(
						huh.NewOption("Add", "add"),
						huh.NewOption("List", "list"),
						huh.NewOption("Remove", "remove"),
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
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
			name := ""
			if err := selectFromList("Subscription to remove", fetchAzureSubscriptionNames(), &name); err != nil {
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
				continue
			}
			configAzureRemoveSubscriptionIDsCmd.Run(configAzureRemoveSubscriptionIDsCmd, []string{name})
		}
	}
}

func interactiveConfigAzureResourceGroup() error {
	for {
		var action string
		err := newForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Resource group — action").
					Options(
						huh.NewOption("Add (create in Azure)", "add"),
						huh.NewOption("List", "list"),
						huh.NewOption("Delete", "delete"),
						huh.NewOption("← Back", "back"),
					).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}

		switch action {
		case "back":
			return errBack
		case "list":
			sub := ""
			if err := selectFromList("Subscription alias", fetchAzureSubscriptionNames(), &sub); err != nil {
				return err
			}
			configAzureListResourceGroupsCmd.Flags().Set("subscription", sub)
			configAzureListResourceGroupsCmd.Run(configAzureListResourceGroupsCmd, nil)
		case "add":
			sub := ""
			if err := selectFromList("Subscription alias", fetchAzureSubscriptionNames(), &sub); err != nil {
				return err
			}
			var name, location string
			err = newForm(
				huh.NewGroup(
					huh.NewInput().Title("Resource group name").Placeholder("hyve-rg").Value(&name),
					huh.NewInput().Title("Location/region").Placeholder("eastus").Value(&location),
				),
			).Run()
			if err != nil {
				return err
			}
			configAzureAddResourceGroupCmd.Flags().Set("subscription", sub)
			configAzureAddResourceGroupCmd.Flags().Set("name", name)
			configAzureAddResourceGroupCmd.Flags().Set("location", location)
			configAzureAddResourceGroupCmd.Run(configAzureAddResourceGroupCmd, nil)
		case "delete":
			sub := ""
			if err := selectFromList("Subscription alias", fetchAzureSubscriptionNames(), &sub); err != nil {
				return err
			}
			name := ""
			if err := selectFromList("Resource group to delete", fetchAzureResourceGroupNames(sub), &name); err != nil {
				return err
			}
			var confirm bool
			err = newForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("Delete resource group '%s' from subscription '%s'? This cannot be undone.", name, sub)).
						Value(&confirm),
				),
			).Run()
			if err != nil {
				return err
			}
			if !confirm {
				continue
			}
			configAzureDeleteResourceGroupCmd.Flags().Set("subscription", sub)
			configAzureDeleteResourceGroupCmd.Flags().Set("name", name)
			configAzureDeleteResourceGroupCmd.Run(configAzureDeleteResourceGroupCmd, nil)
		}
	}
}
