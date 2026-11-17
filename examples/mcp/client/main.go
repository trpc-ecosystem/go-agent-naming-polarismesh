/*
 * Tencent is pleased to support the open source community by making
 * trpc-agent-naming-polarismesh available.
 *
 * Copyright (C) 2026 Tencent. All rights reserved.
 *
 * trpc-agent-naming-polarismesh is licensed under the Apache License Version 2.0.
 */

// Package main discovers and calls a deterministic MCP server through Polaris.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/examples/internal/demo"
	adaptermcp "trpc.group/trpc-go/trpc-agent-naming-polarismesh/mcp"
	trpcmcp "trpc.group/trpc-go/trpc-mcp-go"
)

type result struct {
	Success  bool   `json:"success"`
	Content  string `json:"content"`
	Contract string `json:"contract"`
	Error    string `json:"error,omitempty"`
}

var (
	server          string
	grpcPort        int
	namespace       string
	serviceName     string
	protocolVersion string
	transport       string
	serverVersion   string
	message         string
	timeout         time.Duration
	debug           bool
)

func main() {
	initArgs()
	flag.Parse()
	if server == "" {
		log.Fatal("--server or POLARIS_SERVER is required")
	}
	demo.ConfigureLogging(debug)
	if debug {
		if err := api.SetLoggersLevel(api.DebugLog); err != nil {
			log.Printf("[WARN] enable debug logs: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	item := run(ctx)
	raw, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(raw))
	if !item.Success {
		log.Fatal(item.Error)
	}
}

func run(ctx context.Context) result {
	sdk, err := demo.NewSDK(net.JoinHostPort(server, fmt.Sprint(grpcPort)), "", false, timeout)
	if err != nil {
		return result{Error: err.Error()}
	}
	defer sdk.Destroy()
	sdk.GetConfig().GetProvider().SetReportServiceContractEnable(true)
	config := adaptermcp.DefaultConfig()
	config.Namespace = namespace
	config.Service = serviceName
	config.ProtocolVersion = protocolVersion
	config.Transport = adaptermcp.Transport(transport)
	config.ServerVersion = serverVersion
	config.Timeout = timeout
	pool, err := adaptermcp.NewPool(adaptermcp.PoolConfig{
		Config:   config,
		Consumer: api.NewConsumerAPIByContext(sdk),
	})
	if err != nil {
		return result{Error: err.Error()}
	}
	defer func() { _ = pool.Close() }()
	if _, err := pool.Refresh(ctx); err != nil {
		return result{Error: err.Error()}
	}
	request := &trpcmcp.CallToolRequest{}
	request.Params.Name = "echo"
	request.Params.Arguments = map[string]any{"value": message}
	response, err := pool.CallTool(ctx, request)
	if err != nil {
		return result{Error: err.Error()}
	}
	var content strings.Builder
	for _, part := range response.Content {
		if text, ok := part.(trpcmcp.TextContent); ok {
			content.WriteString(text.Text)
		}
	}
	value := content.String()
	rule, err := adaptermcp.GetContractWithConfig(ctx, api.NewConsumerAPIByContext(sdk), config)
	if err != nil {
		return result{Error: err.Error()}
	}
	contract, ok := rule.Value.(*model.ServiceContract)
	if !ok || contract == nil || contract.Type != "mcp" || len(contract.Interfaces) == 0 {
		contractType, revision, digest := "", "", ""
		features, interfaces := 0, 0
		if contract != nil {
			contractType, revision, digest = contract.Type, contract.Revision, contract.ContentDigest
			features, interfaces = len(contract.ServiceFeatures), len(contract.Interfaces)
		}
		return result{Error: fmt.Sprintf(
			"MCP Service Contract query returned an incomplete contract: type=%q features=%d interfaces=%d revision=%q digest=%q",
			contractType, features, interfaces, revision, digest,
		)}
	}
	return result{
		Success:  strings.Contains(value, "mcp|"+serviceName+"|"+message),
		Content:  value,
		Contract: contract.ContentDigest,
		Error: func() string {
			if strings.Contains(value, "mcp|"+serviceName+"|"+message) {
				return ""
			}
			return "MCP response did not match the deterministic contract"
		}(),
	}
}

func initArgs() {
	flag.StringVar(&server, "server", os.Getenv("POLARIS_SERVER"), "Polaris host")
	flag.IntVar(&grpcPort, "polaris-port", 8091, "Polaris gRPC port")
	flag.StringVar(&namespace, "namespace", "go-naming-polarismesh-mcp-demo", "Polaris namespace")
	flag.StringVar(&serviceName, "service", "trpc.polaris.demo.mcp", "MCP service name")
	flag.StringVar(&protocolVersion, "protocol-version", trpcmcp.ProtocolVersion_2025_03_26, "MCP protocol version")
	flag.StringVar(&transport, "transport", string(adaptermcp.TransportStreamableHTTP), "MCP transport")
	flag.StringVar(&serverVersion, "server-version", "1.0.0", "Service Contract server version")
	flag.StringVar(&message, "message", "hello", "Tool input")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "Polaris operation timeout")
	flag.BoolVar(&debug, "debug", false, "enable Polaris debug logs")
}
