package main

import (
	"context"
	"errors"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type fakeUpstreamClient struct {
	startCalls      int
	initializeCalls int
	pingCalls       int
	callToolCalls   int
	closeCalls      int
	startErrs       []error
	initializeErrs  []error
	pingErr         error
	pingWaitForDone bool
	callToolErr     error
	callToolResult  *mcp.CallToolResult
}

func (f *fakeUpstreamClient) Start(context.Context) error {
	f.startCalls++
	if len(f.startErrs) > 0 {
		err := f.startErrs[0]
		f.startErrs = f.startErrs[1:]
		return err
	}
	return nil
}

func (f *fakeUpstreamClient) Initialize(context.Context, mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	f.initializeCalls++
	if len(f.initializeErrs) > 0 {
		err := f.initializeErrs[0]
		f.initializeErrs = f.initializeErrs[1:]
		return nil, err
	}
	return &mcp.InitializeResult{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		Capabilities:    mcp.ServerCapabilities{},
		ServerInfo:      mcp.Implementation{Name: "fake", Version: "1.0.0"},
	}, nil
}

func (f *fakeUpstreamClient) Ping(ctx context.Context) error {
	f.pingCalls++
	if f.pingWaitForDone {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.pingErr
}

func (f *fakeUpstreamClient) ListTools(context.Context, mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{}, nil
}

func (f *fakeUpstreamClient) CallTool(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	f.callToolCalls++
	if f.callToolErr != nil {
		return nil, f.callToolErr
	}
	return f.callToolResult, nil
}

func (f *fakeUpstreamClient) ListPrompts(context.Context, mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	return &mcp.ListPromptsResult{}, nil
}

func (f *fakeUpstreamClient) GetPrompt(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{}, nil
}

func (f *fakeUpstreamClient) ListResources(context.Context, mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	return &mcp.ListResourcesResult{}, nil
}

func (f *fakeUpstreamClient) ReadResource(context.Context, mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{}, nil
}

func (f *fakeUpstreamClient) ListResourceTemplates(context.Context, mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	return &mcp.ListResourceTemplatesResult{}, nil
}

func (f *fakeUpstreamClient) Complete(context.Context, mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return &mcp.CompleteResult{}, nil
}

func (f *fakeUpstreamClient) Close() error {
	f.closeCalls++
	return nil
}

func (f *fakeUpstreamClient) OnConnectionLost(func(error)) {}

func TestCallToolReconnectsAfterStaleSessionError(t *testing.T) {
	oldClient := &fakeUpstreamClient{
		callToolErr: errors.New("request failed with status 503: No active SSE connection"),
	}
	newClient := &fakeUpstreamClient{
		callToolResult: &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("ok")},
		},
	}

	var factoryCalls int
	proxyClient := &Client{
		name:            "dms",
		client:          oldClient,
		needManualStart: true,
		newClient: func() (upstreamClient, error) {
			factoryCalls++
			return newClient, nil
		},
		initRequest: &mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo:      mcp.Implementation{Name: "proxy", Version: "1.0.0"},
			},
		},
	}

	result, err := proxyClient.callTool(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected reconnect retry to succeed, got error: %v", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("expected one reconnect, got %d", factoryCalls)
	}
	if oldClient.closeCalls != 1 {
		t.Fatalf("expected stale client to be closed once, got %d", oldClient.closeCalls)
	}
	if newClient.startCalls != 1 || newClient.initializeCalls != 1 {
		t.Fatalf("expected replacement client to start and initialize once, got start=%d init=%d", newClient.startCalls, newClient.initializeCalls)
	}
	if newClient.callToolCalls != 1 {
		t.Fatalf("expected replacement client to handle retried tool call once, got %d", newClient.callToolCalls)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected tool content after reconnect, got %#v", result.Content)
	}
}

func TestCallToolReconnectsAfterTransportClosedError(t *testing.T) {
	oldClient := &fakeUpstreamClient{
		callToolErr: errors.New("transport error: transport closed"),
	}
	newClient := &fakeUpstreamClient{
		callToolResult: &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("ok")},
		},
	}

	var factoryCalls int
	proxyClient := &Client{
		name:            "nebula",
		client:          oldClient,
		needManualStart: false,
		newClient: func() (upstreamClient, error) {
			factoryCalls++
			return newClient, nil
		},
		initRequest: &mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo:      mcp.Implementation{Name: "proxy", Version: "1.0.0"},
			},
		},
	}

	result, err := proxyClient.callTool(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected reconnect retry to succeed, got error: %v", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("expected one reconnect, got %d", factoryCalls)
	}
	if oldClient.closeCalls != 1 {
		t.Fatalf("expected stale client to be closed once, got %d", oldClient.closeCalls)
	}
	if newClient.initializeCalls != 1 {
		t.Fatalf("expected replacement client to initialize once, got init=%d", newClient.initializeCalls)
	}
	if newClient.callToolCalls != 1 {
		t.Fatalf("expected replacement client to handle retried tool call once, got %d", newClient.callToolCalls)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected tool content after reconnect, got %#v", result.Content)
	}
}

func TestPingOnceReconnectsAfterStaleSessionError(t *testing.T) {
	oldClient := &fakeUpstreamClient{
		pingErr: errors.New("request failed with status 503: No active SSE connection"),
	}
	newClient := &fakeUpstreamClient{}

	var factoryCalls int
	proxyClient := &Client{
		name:            "dms",
		client:          oldClient,
		needManualStart: true,
		newClient: func() (upstreamClient, error) {
			factoryCalls++
			return newClient, nil
		},
		initRequest: &mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo:      mcp.Implementation{Name: "proxy", Version: "1.0.0"},
			},
		},
	}

	failCount := 0
	proxyClient.pingOnce(context.Background(), &failCount)

	if factoryCalls != 1 {
		t.Fatalf("expected one reconnect on ping failure, got %d", factoryCalls)
	}
	if failCount != 0 {
		t.Fatalf("expected fail count to reset after successful reconnect, got %d", failCount)
	}
	if oldClient.closeCalls != 1 {
		t.Fatalf("expected stale client to be closed once, got %d", oldClient.closeCalls)
	}
	if newClient.startCalls != 1 || newClient.initializeCalls != 1 {
		t.Fatalf("expected replacement client to start and initialize once, got start=%d init=%d", newClient.startCalls, newClient.initializeCalls)
	}
}

func TestPingOnceReconnectsAfterUnauthorizedError(t *testing.T) {
	oldClient := &fakeUpstreamClient{
		pingErr: errors.New("transport error: unauthorized (401)"),
	}
	newClient := &fakeUpstreamClient{}

	var factoryCalls int
	proxyClient := &Client{
		name:            "dms",
		client:          oldClient,
		needManualStart: true,
		newClient: func() (upstreamClient, error) {
			factoryCalls++
			return newClient, nil
		},
		initRequest: &mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo:      mcp.Implementation{Name: "proxy", Version: "1.0.0"},
			},
		},
	}

	failCount := 0
	proxyClient.pingOnce(context.Background(), &failCount)

	if factoryCalls != 1 {
		t.Fatalf("expected one reconnect on unauthorized ping failure, got %d", factoryCalls)
	}
	if failCount != 0 {
		t.Fatalf("expected fail count to reset after successful reconnect, got %d", failCount)
	}
	if oldClient.closeCalls != 1 {
		t.Fatalf("expected unauthorized client to be closed once, got %d", oldClient.closeCalls)
	}
	if newClient.startCalls != 1 || newClient.initializeCalls != 1 {
		t.Fatalf("expected replacement client to start and initialize once, got start=%d init=%d", newClient.startCalls, newClient.initializeCalls)
	}
}

func TestPingOnceReconnectsAfterHealthCheckTimeout(t *testing.T) {
	oldTimeout := healthCheckTimeout
	healthCheckTimeout = time.Nanosecond
	defer func() {
		healthCheckTimeout = oldTimeout
	}()

	oldClient := &fakeUpstreamClient{
		pingWaitForDone: true,
	}
	newClient := &fakeUpstreamClient{}

	var factoryCalls int
	proxyClient := &Client{
		name:            "log-search",
		client:          oldClient,
		needManualStart: false,
		newClient: func() (upstreamClient, error) {
			factoryCalls++
			return newClient, nil
		},
		initRequest: &mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo:      mcp.Implementation{Name: "proxy", Version: "1.0.0"},
			},
		},
	}

	failCount := 0
	proxyClient.pingOnce(context.Background(), &failCount)

	if factoryCalls != 1 {
		t.Fatalf("expected one reconnect on health-check timeout, got %d", factoryCalls)
	}
	if failCount != 0 {
		t.Fatalf("expected fail count to reset after successful reconnect, got %d", failCount)
	}
	if oldClient.closeCalls != 1 {
		t.Fatalf("expected timed-out client to be closed once, got %d", oldClient.closeCalls)
	}
	if newClient.initializeCalls != 1 {
		t.Fatalf("expected replacement stdio client to initialize once, got %d", newClient.initializeCalls)
	}
	if newClient.startCalls != 0 {
		t.Fatalf("expected replacement stdio client to preserve automatic start behavior, got start=%d", newClient.startCalls)
	}
}

func TestReconnectRetriesTransientStartError(t *testing.T) {
	oldDelay := transientConnectRetryDelay
	oldAttempts := transientConnectMaxAttempts
	transientConnectRetryDelay = 0
	transientConnectMaxAttempts = 3
	defer func() {
		transientConnectRetryDelay = oldDelay
		transientConnectMaxAttempts = oldAttempts
	}()

	oldClient := &fakeUpstreamClient{
		callToolErr: errors.New("request failed with status 503: No active SSE connection"),
	}
	newClient := &fakeUpstreamClient{
		startErrs: []error{
			errors.New("unexpected status code: 429"),
		},
		callToolResult: &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("ok")},
		},
	}

	proxyClient := &Client{
		name:            "dms",
		client:          oldClient,
		needManualStart: true,
		newClient: func() (upstreamClient, error) {
			return newClient, nil
		},
		initRequest: &mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo:      mcp.Implementation{Name: "proxy", Version: "1.0.0"},
			},
		},
	}

	result, err := proxyClient.callTool(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected reconnect retry to recover from transient start error, got: %v", err)
	}
	if newClient.startCalls != 2 {
		t.Fatalf("expected replacement client start to retry once, got %d calls", newClient.startCalls)
	}
	if newClient.initializeCalls != 1 {
		t.Fatalf("expected replacement client initialize once after start recovered, got %d", newClient.initializeCalls)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected tool result after reconnect retry, got %#v", result.Content)
	}
}

func TestAddToMCPServerRetriesTransientStartError(t *testing.T) {
	oldDelay := transientConnectRetryDelay
	oldAttempts := transientConnectMaxAttempts
	transientConnectRetryDelay = 0
	transientConnectMaxAttempts = 3
	defer func() {
		transientConnectRetryDelay = oldDelay
		transientConnectMaxAttempts = oldAttempts
	}()

	startupClient := &fakeUpstreamClient{
		startErrs: []error{
			errors.New("unexpected status code: 429"),
		},
	}

	proxyClient := &Client{
		name:            "dms",
		client:          startupClient,
		needManualStart: true,
	}

	mcpServer := server.NewMCPServer("proxy", "1.0.0")
	err := proxyClient.addToMCPServer(context.Background(), mcp.Implementation{Name: "proxy", Version: "1.0.0"}, mcpServer)
	if err != nil {
		t.Fatalf("expected startup retry to recover from transient start error, got: %v", err)
	}
	if startupClient.startCalls != 2 {
		t.Fatalf("expected startup client start to retry once, got %d calls", startupClient.startCalls)
	}
	if startupClient.initializeCalls != 1 {
		t.Fatalf("expected startup client initialize once after start recovered, got %d", startupClient.initializeCalls)
	}
}

func TestStdioClientNeedsPingHealthCheck(t *testing.T) {
	_, needPing, needManualStart, err := buildUpstreamClient(&MCPClientConfigV2{
		Command: "true",
	})
	if err != nil {
		t.Fatalf("expected stdio client to build, got error: %v", err)
	}
	if !needPing {
		t.Fatal("expected stdio client to enable ping health checks")
	}
	if needManualStart {
		t.Fatal("expected stdio client to keep existing automatic start behavior")
	}
}

var _ upstreamClient = (*fakeUpstreamClient)(nil)
var _ interface {
	OnConnectionLost(func(error))
} = (*mcpclient.Client)(nil)
