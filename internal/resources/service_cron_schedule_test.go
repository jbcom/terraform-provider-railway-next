// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestServiceSchemaCronScheduleIsOptionalAndComputed(t *testing.T) {
	t.Parallel()

	service := NewService().(*Service)
	var response frameworkresource.SchemaResponse
	service.Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)
	attribute, ok := response.Schema.Attributes["cron_schedule"]
	if !ok {
		t.Fatal("service schema is missing cron_schedule")
	}
	cronSchedule, ok := attribute.(schema.StringAttribute)
	if !ok {
		t.Fatalf("cron_schedule attribute type = %T, want schema.StringAttribute", attribute)
	}
	if !cronSchedule.Optional || !cronSchedule.Computed {
		t.Errorf("cron_schedule optional=%t computed=%t, want both true", cronSchedule.Optional, cronSchedule.Computed)
	}
}
