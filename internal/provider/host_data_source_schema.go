// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Computed datasource-schema builders for the host nested blocks. They mirror
// the resource schema in host_schema.go but use datasource schema types.

func dsMatchOptionsSchema() dschema.SingleNestedAttribute {
	return dschema.SingleNestedAttribute{
		MarkdownDescription: "How source URLs are matched.",
		Computed:            true,
		Attributes: map[string]dschema.Attribute{
			"case_insensitive":  dschema.BoolAttribute{Computed: true, MarkdownDescription: "Ignore case."},
			"slash_insensitive": dschema.BoolAttribute{Computed: true, MarkdownDescription: "Ignore trailing slashes."},
		},
	}
}

func dsNotFoundActionSchema() dschema.SingleNestedAttribute {
	return dschema.SingleNestedAttribute{
		MarkdownDescription: "Behaviour when no matching redirect is found.",
		Computed:            true,
		Attributes: map[string]dschema.Attribute{
			"forward_params": dschema.BoolAttribute{Computed: true, MarkdownDescription: "Copy source query parameters."},
			"forward_path":   dschema.BoolAttribute{Computed: true, MarkdownDescription: "Copy the source path."},
			"response_code":  dschema.Int64Attribute{Computed: true, MarkdownDescription: "Response code (301/302/404)."},
			"response_url":   dschema.StringAttribute{Computed: true, MarkdownDescription: "Redirect target for 301/302."},
		},
	}
}

func dsSecuritySchema() dschema.SingleNestedAttribute {
	return dschema.SingleNestedAttribute{
		MarkdownDescription: "HTTPS and HSTS security settings.",
		Computed:            true,
		Attributes: map[string]dschema.Attribute{
			"https_upgrade":             dschema.BoolAttribute{Computed: true, MarkdownDescription: "Upgrade HTTP to HTTPS."},
			"prevent_foreign_embedding": dschema.BoolAttribute{Computed: true, MarkdownDescription: "Prevent foreign embedding."},
			"hsts_include_sub_domains":  dschema.BoolAttribute{Computed: true, MarkdownDescription: "Apply HSTS to subdomains."},
			"hsts_max_age":              dschema.Int64Attribute{Computed: true, MarkdownDescription: "HSTS max-age in seconds."},
			"hsts_preload":              dschema.BoolAttribute{Computed: true, MarkdownDescription: "Include HSTS preload directive."},
		},
	}
}

func dsDNSValueAttributes() map[string]dschema.Attribute {
	return map[string]dschema.Attribute{
		"type":   dschema.StringAttribute{Computed: true, MarkdownDescription: "DNS record type."},
		"values": dschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Record values."},
	}
}

func dsRequiredDNSSchema() dschema.SingleNestedAttribute {
	return dschema.SingleNestedAttribute{
		MarkdownDescription: "DNS records required for this host.",
		Computed:            true,
		Attributes: map[string]dschema.Attribute{
			"recommended": dschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Recommended DNS record.",
				Attributes:          dsDNSValueAttributes(),
			},
			"alternatives": dschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Alternative acceptable DNS records.",
				NestedObject:        dschema.NestedAttributeObject{Attributes: dsDNSValueAttributes()},
			},
			"verification": dschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Ownership verification record.",
				Attributes: map[string]dschema.Attribute{
					"type":   dschema.StringAttribute{Computed: true, MarkdownDescription: "Record type."},
					"record": dschema.StringAttribute{Computed: true, MarkdownDescription: "Record name."},
					"value":  dschema.StringAttribute{Computed: true, MarkdownDescription: "Challenge value."},
				},
			},
		},
	}
}

func dsDetectedDNSSchema() dschema.ListNestedAttribute {
	return dschema.ListNestedAttribute{
		MarkdownDescription: "Currently detected DNS records.",
		Computed:            true,
		NestedObject:        dschema.NestedAttributeObject{Attributes: dsDNSValueAttributes()},
	}
}
