package pyexecclient

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestWithTraceAppendsOutgoingMetadata(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "existing", "value")

	ctx = withTrace(ctx, AnalyzeRequest{
		RequestID: "rid-1",
		TenantID:  2,
		ProjectID: 3,
		SessionID: 4,
		UserID:    5,
	})

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	assertMetadataValue(t, md, "existing", "value")
	assertMetadataValue(t, md, "request-id", "rid-1")
	assertMetadataValue(t, md, "tenant-id", "2")
	assertMetadataValue(t, md, "project-id", "3")
	assertMetadataValue(t, md, "session-id", "4")
	assertMetadataValue(t, md, "user-id", "5")
}

func TestToPBRequestIncludesTemplateFields(t *testing.T) {
	req := toPBRequest(AnalyzeRequest{
		Mode:         "template",
		TemplateName: "category_analysis",
		TemplateParams: map[string]any{
			"category_field": "city",
			"value_field":    "gmv",
			"dataset_index":  float64(1),
		},
		Limits: Limits{MaxInputRows: 10},
	})

	if req.Mode != "template" || req.TemplateName != "category_analysis" {
		t.Fatalf("expected template request fields, got %+v", req)
	}
	params := req.TemplateParams.AsMap()
	if params["category_field"] != "city" || params["value_field"] != "gmv" || params["dataset_index"] != float64(1) {
		t.Fatalf("unexpected template params: %+v", params)
	}
}

func assertMetadataValue(t *testing.T, md metadata.MD, key string, want string) {
	t.Helper()
	values := md.Get(key)
	if len(values) == 0 || values[len(values)-1] != want {
		t.Fatalf("expected metadata %s=%s, got %v", key, want, values)
	}
}
