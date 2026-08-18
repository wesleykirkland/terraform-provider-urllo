// Copyright Wesley Kirkland-Daily 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// requiredDNSSchema is the computed required_dns_entries block for the host
// resource.
func requiredDNSSchema() rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		MarkdownDescription: "DNS records that must be configured for this host.",
		Computed:            true,
		PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
		Attributes: map[string]rschema.Attribute{
			"recommended": dnsValueResourceSchema("The recommended DNS record."),
			"alternatives": rschema.ListNestedAttribute{
				MarkdownDescription: "Alternative acceptable DNS records.",
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: rschema.NestedAttributeObject{
					Attributes: dnsValueResourceAttributes(),
				},
			},
			"verification": rschema.SingleNestedAttribute{
				MarkdownDescription: "Ownership verification record, when additional verification is required.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]rschema.Attribute{
					"type": rschema.StringAttribute{
						Computed: true, MarkdownDescription: "Record type, e.g. `TXT`.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"record": rschema.StringAttribute{
						Computed: true, MarkdownDescription: "Record name.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"value": rschema.StringAttribute{
						Computed: true, MarkdownDescription: "Challenge value.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				},
			},
		},
	}
}

// detectedDNSSchema is the computed detected_dns_entries block for the host
// resource.
func detectedDNSSchema() rschema.ListNestedAttribute {
	return rschema.ListNestedAttribute{
		MarkdownDescription: "Currently detected DNS records for this host.",
		Computed:            true,
		PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
		NestedObject: rschema.NestedAttributeObject{
			Attributes: dnsValueResourceAttributes(),
		},
	}
}

func dnsValueResourceSchema(desc string) rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		MarkdownDescription: desc,
		Computed:            true,
		PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
		Attributes:          dnsValueResourceAttributes(),
	}
}

func dnsValueResourceAttributes() map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		"type": rschema.StringAttribute{
			Computed: true, MarkdownDescription: "DNS record type (`A` or `CNAME`).",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"values": rschema.ListAttribute{
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "Record values.",
			PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
		},
	}
}
