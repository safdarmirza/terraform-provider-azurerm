package azurerm

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2018-02-14/keyvault"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/set"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func dataSourceArmKeyVault() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceArmKeyVaultRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateKeyVaultName,
			},

			"resource_group_name": resourceGroupNameForDataSourceSchema(),

			"location": locationForDataSourceSchema(),

			"sku": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},

			"vault_uri": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"tenant_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"access_policy": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"tenant_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"object_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"application_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"certificate_permissions": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"key_permissions": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"secret_permissions": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},

			"enabled_for_deployment": {
				Type:     schema.TypeBool,
				Computed: true,
			},

			"enabled_for_disk_encryption": {
				Type:     schema.TypeBool,
				Computed: true,
			},

			"enabled_for_template_deployment": {
				Type:     schema.TypeBool,
				Computed: true,
			},

			"network_acls": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"default_action": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"bypass": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"ip_rules": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Set:      schema.HashString,
						},
						"virtual_network_subnet_ids": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Set:      set.HashStringIgnoreCase,
						},
					},
				},
			},

			"tags": tagsForDataSourceSchema(),
		},
	}
}

func dataSourceArmKeyVaultRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).keyVaultClient
	ctx := meta.(*ArmClient).StopContext

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	resp, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			return fmt.Errorf("KeyVault %q (Resource Group %q) does not exist", name, resourceGroup)
		}
		return fmt.Errorf("Error making Read request on KeyVault %q: %+v", name, err)
	}

	d.SetId(*resp.ID)

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resourceGroup)
	if location := resp.Location; location != nil {
		d.Set("location", azureRMNormalizeLocation(*location))
	}

	if props := resp.Properties; props != nil {
		d.Set("tenant_id", props.TenantID.String())
		d.Set("enabled_for_deployment", props.EnabledForDeployment)
		d.Set("enabled_for_disk_encryption", props.EnabledForDiskEncryption)
		d.Set("enabled_for_template_deployment", props.EnabledForTemplateDeployment)
		d.Set("vault_uri", props.VaultURI)

		if err := d.Set("sku", flattenKeyVaultDataSourceSku(props.Sku)); err != nil {
			return fmt.Errorf("Error flattening `sku` for KeyVault %q: %+v", *resp.Name, err)
		}

		flattenedPolicies := azure.FlattenKeyVaultAccessPolicies(props.AccessPolicies)
		if err := d.Set("access_policy", flattenedPolicies); err != nil {
			return fmt.Errorf("Error flattening `access_policy` for KeyVault %q: %+v", *resp.Name, err)
		}

		if err := d.Set("network_acls", flattenKeyVaultDataSourceNetworkAcls(props.NetworkAcls)); err != nil {
			return fmt.Errorf("Error flattening `network_acls` for KeyVault %q: %+v", *resp.Name, err)
		}
	}

	flattenAndSetTags(d, resp.Tags)

	return nil
}

func flattenKeyVaultDataSourceSku(sku *keyvault.Sku) []interface{} {
	result := map[string]interface{}{
		"name": string(sku.Name),
	}

	return []interface{}{result}
}

func flattenKeyVaultDataSourceNetworkAcls(input *keyvault.NetworkRuleSet) []interface{} {
	if input == nil {
		return []interface{}{}
	}

	output := make(map[string]interface{}, 0)

	output["bypass"] = string(input.Bypass)
	output["default_action"] = string(input.DefaultAction)

	ipRules := make([]interface{}, 0)
	if input.IPRules != nil {
		for _, v := range *input.IPRules {
			if v.Value == nil {
				continue
			}

			ipRules = append(ipRules, *v.Value)
		}
	}
	output["ip_rules"] = schema.NewSet(schema.HashString, ipRules)

	virtualNetworkRules := make([]interface{}, 0)
	if input.VirtualNetworkRules != nil {
		for _, v := range *input.VirtualNetworkRules {
			if v.ID == nil {
				continue
			}

			virtualNetworkRules = append(virtualNetworkRules, *v.ID)
		}
	}
	output["virtual_network_subnet_ids"] = schema.NewSet(schema.HashString, virtualNetworkRules)

	return []interface{}{output}
}
