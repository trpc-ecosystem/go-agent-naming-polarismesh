/*
 * Tencent is pleased to support the open source community by making
 * trpc-agent-naming-polarismesh available.
 *
 * Copyright (C) 2026 Tencent. All rights reserved.
 *
 * trpc-agent-naming-polarismesh is licensed under the Apache License Version 2.0.
 */

// Package main runs a deterministic MCP server registered through Polaris.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/examples/internal/demo"
	adaptermcp "trpc.group/trpc-go/trpc-agent-naming-polarismesh/mcp"
	adapternaming "trpc.group/trpc-go/trpc-agent-naming-polarismesh/naming"
	trpcregistry "trpc.group/trpc-go/trpc-go/naming/registry"
	trpcmcp "trpc.group/trpc-go/trpc-mcp-go"
	polarisregistry "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
)

var (
	server          string
	grpcPort        int
	namespace       string
	serviceName     string
	host            string
	port            int
	protocolVersion string
	transport       string
	serverVersion   string
	sseVersion      string
	ssePort         int
	multiTransport  bool
	readyFile       string
	timeout         time.Duration
	debug           bool
)

func main() {
	initArgs()
	flag.Parse()
	if server == "" {
		log.Fatal("--server or POLARIS_SERVER is required")
	}
	if debug {
		if err := api.SetLoggersLevel(api.DebugLog); err != nil {
			log.Printf("[WARN] enable debug logs: %v", err)
		}
	}
	ctx, stop := demo.SignalContext()
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

type readyState struct {
	Service     string `json:"service"`
	Endpoint    string `json:"endpoint"`
	Contract    string `json:"contract"`
	SSEEndpoint string `json:"sseEndpoint,omitempty"`
	SSEContract string `json:"sseContract,omitempty"`
}

type registrar struct{}

func (registrar) RegisterMetadataProvider(
	service string,
	provider adapternaming.MetadataProvider,
) error {
	return polarisregistry.RegisterMetadataProvider(service, provider)
}

func (registrar) UnregisterMetadataProvider(service string) {
	polarisregistry.UnregisterMetadataProvider(service)
}

func run(ctx context.Context) error {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() { _ = listener.Close() }()
	actualPort := listener.Addr().(*net.TCPAddr).Port

	sdk, err := demo.NewSDK(net.JoinHostPort(server, fmt.Sprint(grpcPort)), "", false, timeout)
	if err != nil {
		return err
	}
	defer sdk.Destroy()
	sdk.GetConfig().GetProvider().SetReportServiceContractEnable(true)
	provider := api.NewProviderAPIByContext(sdk)

	mcpConfig := adaptermcp.DefaultConfig()
	mcpConfig.Namespace = namespace
	mcpConfig.Service = serviceName
	mcpConfig.ServerName = serviceName
	mcpConfig.ServerVersion = serverVersion
	mcpConfig.ProtocolVersion = protocolVersion
	mcpConfig.Transport = adaptermcp.Transport(transport)
	mcpConfig.Host = host
	mcpConfig.Port = actualPort
	mcpConfig.Timeout = timeout
	prepared, err := adaptermcp.PrepareServer(ctx, mcpConfig)
	if err != nil {
		return err
	}
	registryConfig := &polarisregistry.Config{
		Namespace:      namespace,
		ServiceName:    serviceName,
		EnableRegister: true,
		HeartBeat:      1,
		TTL:            2,
		ServiceToken:   os.Getenv("POLARIS_SERVICE_TOKEN"),
	}
	var second *adaptermcp.PreparedServer
	var secondDone <-chan error
	if multiTransport {
		if mcpConfig.Transport != adaptermcp.TransportStreamableHTTP {
			return errors.New("multi-transport requires streamable HTTP as the primary transport")
		}
		secondListener, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprint(ssePort)))
		if err != nil {
			return err
		}
		defer func() { _ = secondListener.Close() }()
		secondConfig := mcpConfig
		secondConfig.Transport = adaptermcp.TransportSSE
		secondConfig.Port = secondListener.Addr().(*net.TCPAddr).Port
		secondConfig.ServerVersion = sseVersion
		second, err = adaptermcp.PrepareServer(ctx, secondConfig)
		if err != nil {
			return err
		}
		registryConfig.TTL, registryConfig.HeartBeat, registryConfig.AutoHeartbeat = 6, 2, true
		registryConfig, err = adaptermcp.PrepareRegistryConfig(*registryConfig, mcpConfig, secondConfig)
		if err != nil {
			return err
		}
		for i, suffix := range []string{"http", "sse"} {
			weight := []int{80, 120}[i]
			registryConfig.Endpoints[i].InstanceID = serviceName + "-" + suffix
			registryConfig.Endpoints[i].Weight = &weight
		}
		var shutdown func()
		secondDone, shutdown = serveMCP(secondListener, secondConfig)
		defer shutdown()
	} else {
		registered, err := prepared.RegisterMetadata(registrar{})
		if err != nil {
			return err
		}
		if !registered {
			return errors.New("network MCP server did not register metadata")
		}
		defer polarisregistry.UnregisterMetadataProvider(serviceName)
	}
	serveDone, shutdown := serveMCP(listener, mcpConfig)
	defer shutdown()
	registry, err := polarisregistry.NewRegistry(provider, registryConfig)
	if err != nil {
		return err
	}
	trpcregistry.Register(serviceName, registry)
	if err := registry.Register(
		serviceName,
		trpcregistry.WithAddress(net.JoinHostPort(host, fmt.Sprint(actualPort))),
	); err != nil {
		return err
	}
	defer func() { _ = registry.Deregister(serviceName) }()

	client, err := adaptermcp.NewClient(ctx, mcpConfig, prepared.Endpoint)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	contract, err := adaptermcp.CollectContract(ctx, client, mcpConfig)
	if err != nil {
		return err
	}
	reported, err := adaptermcp.ReportContractWithConfig(ctx, provider, contract, mcpConfig)
	if err != nil {
		var sdkErr model.SDKError
		if errors.As(err, &sdkErr) {
			return fmt.Errorf(
				"report MCP contract: server code=%d info=%q: %w",
				sdkErr.ServerCode(),
				sdkErr.ServerInfo(),
				err,
			)
		}
		return err
	}
	ready := readyState{Service: serviceName, Endpoint: prepared.Endpoint, Contract: reported.ContentDigest}
	if second != nil {
		secondClient, err := adaptermcp.NewClient(ctx, second.Config, second.Endpoint)
		if err != nil {
			return err
		}
		defer func() { _ = secondClient.Close() }()
		secondContract, err := adaptermcp.CollectContract(ctx, secondClient, second.Config)
		if err != nil {
			return err
		}
		secondReport, err := adaptermcp.ReportContractWithConfig(ctx, provider, secondContract, second.Config)
		if err != nil {
			return err
		}
		ready.SSEEndpoint, ready.SSEContract = second.Endpoint, secondReport.ContentDigest
	}
	if readyFile != "" {
		raw, err := json.MarshalIndent(ready, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(readyFile, append(raw, '\n'), 0o600); err != nil {
			return err
		}
	}
	log.Printf("MCP provider ready: service=%s endpoint=%s contract=%s",
		serviceName, prepared.Endpoint, reported.ContentDigest)

	select {
	case <-ctx.Done():
	case err := <-serveDone:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case err := <-secondDone:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

func serveMCP(listener net.Listener, cfg adaptermcp.Config) (<-chan error, func()) {
	tool := trpcmcp.NewTool("echo", trpcmcp.WithDescription("Echo a deterministic value"))
	handler := func(_ context.Context, req *trpcmcp.CallToolRequest) (*trpcmcp.CallToolResult, error) {
		value := "hello"
		if req != nil {
			if candidate, ok := req.Params.Arguments["value"].(string); ok && candidate != "" {
				value = candidate
			}
		}
		return trpcmcp.NewTextResult("mcp|" + cfg.Service + "|" + value), nil
	}
	var httpHandler http.Handler
	if cfg.Transport == adaptermcp.TransportSSE {
		server := trpcmcp.NewSSEServer(cfg.Service, cfg.ServerVersion)
		server.RegisterTool(tool, handler)
		httpHandler = guardResponseLifetime(server)
	} else {
		server := trpcmcp.NewServer(cfg.Service, cfg.ServerVersion,
			trpcmcp.WithServerPath(adaptermcp.DefaultStreamableHTTPPath), trpcmcp.WithStatelessMode(true))
		server.RegisterTool(tool, handler)
		httpHandler = server.Handler()
	}
	server := &http.Server{Handler: httpHandler, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	return done, func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
	}
}

// The native SSE server stops its writers asynchronously. Drain active writes
// and reject late writes before net/http finishes the response.
func guardResponseLifetime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guard := &responseLifetimeWriter{writer: w}
		defer guard.close()
		next.ServeHTTP(guard, r)
	})
}

type responseLifetimeWriter struct {
	writer http.ResponseWriter
	mu     sync.Mutex
	closed bool
}

func (w *responseLifetimeWriter) Header() http.Header { return w.writer.Header() }

func (w *responseLifetimeWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.writer.WriteHeader(status)
	}
}

func (w *responseLifetimeWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, net.ErrClosed
	}
	return w.writer.Write(data)
}

func (w *responseLifetimeWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		if flusher, ok := w.writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func (w *responseLifetimeWriter) close() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
}

func initArgs() {
	flag.StringVar(&server, "server", os.Getenv("POLARIS_SERVER"), "Polaris host")
	flag.IntVar(&grpcPort, "polaris-port", 8091, "Polaris gRPC port")
	flag.StringVar(&namespace, "namespace", "go-naming-polarismesh-mcp-demo", "Polaris namespace")
	flag.StringVar(&serviceName, "service", "trpc.polaris.demo.mcp", "MCP service name")
	flag.StringVar(&host, "host", "127.0.0.1", "Provider listen and advertise host")
	flag.IntVar(&port, "port", 18494, "Provider listen port")
	flag.StringVar(&protocolVersion, "protocol-version", trpcmcp.ProtocolVersion_2025_03_26, "MCP protocol version")
	flag.StringVar(&transport, "transport", string(adaptermcp.TransportStreamableHTTP), "MCP transport")
	flag.StringVar(&serverVersion, "server-version", "1.0.0", "Primary server version")
	flag.StringVar(&sseVersion, "sse-version", "2.0.0", "Secondary SSE server version")
	flag.IntVar(&ssePort, "sse-port", 18495, "Secondary SSE listener port")
	flag.BoolVar(&multiTransport, "multi-transport", false, "Register streamable HTTP and SSE endpoints")
	flag.StringVar(&readyFile, "ready", "", "Readiness output file")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "Polaris operation timeout")
	flag.BoolVar(&debug, "debug", false, "enable Polaris debug logs")
}
