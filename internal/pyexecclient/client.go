package pyexecclient

import (
	"context"
	"fmt"
	"time"

	pb "ling-shu/internal/pyexecclient/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

const maxMessageBytes = 64 << 20

type Client struct {
	conn    *grpc.ClientConn
	api     pb.ResultAnalysisServiceClient
	timeout time.Duration
}

func Dial(ctx context.Context, addr string, timeout time.Duration) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("exec grpc addr is required")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := grpc.DialContext(
		dialCtx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMessageBytes),
			grpc.MaxCallSendMsgSize(maxMessageBytes),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("connect exec service addr=%s: %w", addr, err)
	}
	return &Client{conn: conn, api: pb.NewResultAnalysisServiceClient(conn), timeout: timeout}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) Analyze(ctx context.Context, input AnalyzeRequest) (*AnalyzeResponse, error) {
	if c == nil || c.api == nil {
		return nil, fmt.Errorf("exec client is not configured")
	}
	callCtx := ctx
	cancel := func() {}
	if c.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()
	resp, err := c.api.AnalyzeResultSets(withTrace(callCtx, input), toPBRequest(input))
	if err != nil {
		return nil, fmt.Errorf("exec AnalyzeResultSets failed: %w", err)
	}
	return fromPBResponse(resp), nil
}

func withTrace(ctx context.Context, input AnalyzeRequest) context.Context {
	return metadata.AppendToOutgoingContext(
		ctx,
		"request-id", input.RequestID,
		"tenant-id", fmt.Sprint(input.TenantID),
		"project-id", fmt.Sprint(input.ProjectID),
		"session-id", fmt.Sprint(input.SessionID),
		"user-id", fmt.Sprint(input.UserID),
	)
}

func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	if c == nil || c.api == nil {
		return nil, fmt.Errorf("exec client is not configured")
	}
	callCtx := ctx
	cancel := func() {}
	if c.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()
	resp, err := c.api.Health(callCtx, &pb.HealthRequest{})
	if err != nil {
		return nil, fmt.Errorf("exec Health failed: %w", err)
	}
	return &HealthStatus{OK: resp.Ok, Version: resp.Version, Capabilities: resp.Capabilities}, nil
}

func toPBRequest(input AnalyzeRequest) *pb.AnalyzeResultSetsRequest {
	req := &pb.AnalyzeResultSetsRequest{
		RequestId:    input.RequestID,
		TenantId:     input.TenantID,
		ProjectId:    input.ProjectID,
		SessionId:    input.SessionID,
		UserId:       input.UserID,
		Question:     input.Question,
		Mode:         input.Mode,
		AnalysisGoal: input.AnalysisGoal,
		TemplateName: input.TemplateName,
		Limits: &pb.AnalysisLimits{
			TimeoutMs:      int32(input.Limits.TimeoutMS),
			MaxInputRows:   int32(input.Limits.MaxInputRows),
			MaxOutputRows:  int32(input.Limits.MaxOutputRows),
			MaxStdoutChars: int32(input.Limits.MaxStdoutChars),
		},
	}
	if len(input.TemplateParams) > 0 {
		req.TemplateParams = mapToStruct(input.TemplateParams)
	}
	for _, dataset := range input.Datasets {
		item := &pb.AnalysisDataset{
			Name:           dataset.Name,
			DatasourceId:   dataset.DatasourceID,
			DatasourceName: dataset.DatasourceName,
			Purpose:        dataset.Purpose,
			ExecutionId:    dataset.ExecutionID,
			Columns:        append([]string(nil), dataset.Columns...),
		}
		for _, row := range dataset.Rows {
			item.Rows = append(item.Rows, mapToStruct(row))
		}
		req.Datasets = append(req.Datasets, item)
	}
	return req
}

func fromPBResponse(resp *pb.AnalyzeResultSetsResponse) *AnalyzeResponse {
	if resp == nil {
		return nil
	}
	out := &AnalyzeResponse{
		Success:        resp.Success,
		Summary:        resp.Summary,
		Observation:    resp.Observation,
		Warnings:       append([]string(nil), resp.Warnings...),
		StdoutPreview:  resp.StdoutPreview,
		StderrPreview:  resp.StderrPreview,
		Error:          resp.Error,
		DurationMS:     resp.DurationMs,
		InputRowCount:  int(resp.InputRowCount),
		OutputRowCount: int(resp.OutputRowCount),
		AnalysisKind:   resp.AnalysisKind,
		CodeHash:       resp.CodeHash,
		TemplateName:   resp.TemplateName,
	}
	for _, table := range resp.Tables {
		out.Tables = append(out.Tables, Table{
			Name:    table.Name,
			Columns: append([]string(nil), table.Columns...),
			Rows:    structsToRows(table.Rows),
		})
	}
	for _, chart := range resp.Charts {
		out.Charts = append(out.Charts, Chart{
			Type:       chart.Type,
			Title:      chart.Title,
			XField:     chart.XField,
			YFields:    append([]string(nil), chart.YFields...),
			NameField:  chart.NameField,
			ValueField: chart.ValueField,
			Reason:     chart.Reason,
			Rows:       structsToRows(chart.Rows),
		})
	}
	for _, metric := range resp.Metrics {
		out.Metrics = append(out.Metrics, Metric{
			Name:    metric.Name,
			Label:   metric.Label,
			Value:   metric.Value,
			Unit:    metric.Unit,
			Display: metric.Display,
		})
	}
	return out
}

func structsToRows(rows []*structpb.Struct) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, row.AsMap())
	}
	return out
}

func mapToStruct(row map[string]any) *structpb.Struct {
	value, err := structpb.NewStruct(row)
	if err != nil {
		return &structpb.Struct{}
	}
	return value
}
