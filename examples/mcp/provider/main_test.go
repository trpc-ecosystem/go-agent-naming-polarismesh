/*
 * Tencent is pleased to support the open source community by making
 * trpc-agent-naming-polarismesh available.
 *
 * Copyright (C) 2026 Tencent. All rights reserved.
 *
 * trpc-agent-naming-polarismesh is licensed under the Apache License Version 2.0.
 */

package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	adaptermcp "trpc.group/trpc-go/trpc-agent-naming-polarismesh/mcp"
	trpcmcp "trpc.group/trpc-go/trpc-mcp-go"
)

func TestMCPDemoUsesNativeWireForBothTransports(t *testing.T) {
	for _, transport := range []adaptermcp.Transport{adaptermcp.TransportStreamableHTTP, adaptermcp.TransportSSE} {
		t.Run(string(transport), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { _ = listener.Close() })
			cfg := adaptermcp.DefaultConfig()
			cfg.Namespace, cfg.Service = "Test", "mcp-wire-demo"
			cfg.ProtocolVersion, cfg.Transport = trpcmcp.ProtocolVersion_2025_03_26, transport
			cfg.Host, cfg.Port = "127.0.0.1", listener.Addr().(*net.TCPAddr).Port
			done, shutdown := serveMCP(listener, cfg)
			t.Cleanup(func() { shutdown(); <-done })
			prepared, err := adaptermcp.PrepareServer(context.Background(), cfg)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, err := adaptermcp.NewClient(ctx, cfg, prepared.Endpoint)
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })
			request := &trpcmcp.CallToolRequest{}
			request.Params.Name, request.Params.Arguments = "echo", map[string]any{"value": "hello"}
			response, err := client.CallTool(ctx, request)
			require.NoError(t, err)
			require.False(t, response.IsError)
			require.Len(t, response.Content, 1)
			require.Equal(t, "mcp|mcp-wire-demo|hello", response.Content[0].(trpcmcp.TextContent).Text)
			contract, err := adaptermcp.CollectContract(ctx, client, cfg)
			require.NoError(t, err)
			require.Equal(t, string(transport), contract.Protocol)
			require.NotEmpty(t, contract.ContentDigest)
			require.NotEmpty(t, contract.ServiceFeatures)
		})
	}
}

func TestSSEWriterCannotOutliveHandler(t *testing.T) {
	response := httptest.NewRecorder()
	var retained http.ResponseWriter
	handler := guardResponseLifetime(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		retained = w
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("event: initial\n\n"))
		require.NoError(t, err)
		w.(http.Flusher).Flush()
	}))
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sse", nil))
	_, err := retained.Write([]byte("event: late\n\n"))
	require.ErrorIs(t, err, net.ErrClosed)
	retained.WriteHeader(http.StatusBadGateway)
	retained.(http.Flusher).Flush()
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "event: initial\n\n", response.Body.String())
}

func TestSSEWriterDrainsConcurrentWritesOnClose(t *testing.T) {
	response := httptest.NewRecorder()
	writer := &responseLifetimeWriter{writer: response}
	var writers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for index := 0; index < 200; index++ {
				_, _ = writer.Write([]byte("event: concurrent\n\n"))
				writer.Flush()
			}
		}()
	}
	writer.close()
	before := response.Body.String()
	// net/http may read and finish the underlying response as soon as close returns.
	writers.Wait()
	require.Equal(t, before, response.Body.String())
}
