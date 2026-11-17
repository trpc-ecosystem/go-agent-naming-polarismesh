/*
 * Tencent is pleased to support the open source community by making
 * trpc-agent-naming-polarismesh available.
 *
 * Copyright (C) 2026 Tencent. All rights reserved.
 *
 * trpc-agent-naming-polarismesh is licensed under the Apache License Version 2.0.
 */

package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/internal/agentmodel"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/agentapi"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/agentcard"
	adapterconfig "trpc.group/trpc-go/trpc-agent-naming-polarismesh/config"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/naming"
)

func (s *suite) registration(ctx context.Context) error {
	c, err := s.start(ctx, s.main, "node-a", "/a2a", "valid", "")
	if err != nil {
		return err
	}
	s.first = c
	if err := s.wait(ctx, c, true); err != nil {
		return err
	}
	response, err := s.all(s.main)
	if err != nil {
		return err
	}
	instance := response.GetInstance(c.ready.InstanceID)
	if instance == nil {
		return errors.New("registered instance disappeared before metadata validation")
	}
	metadata := instance.GetMetadata()
	if metadata[agentcard.MetadataAgentName] != s.main ||
		metadata[agentcard.MetadataA2AVersion] != "1.0" {
		return errors.New("real registry did not inject the three-key metadata contract")
	}
	if instance.GetPort() != uint32(s.opts.RegistrationPort) || instance.GetProtocol() != "" ||
		instance.GetVersion() != "1.0.0" {
		return errors.New("registered instance port, protocol, or AgentCard version differs")
	}
	_, card, err := agentcard.ParseAgentInstanceMetadata(metadata)
	if err != nil {
		return err
	}
	if card.PrimaryURL() != c.ready.URL || card.Version != "1.0.0" || card.SupportedInterfaces[0].Tenant != s.opts.Tenant {
		return errors.New("real registry AgentCard differs from frozen Provider card")
	}
	// Wait beyond multiple TTL periods; initial registration alone cannot pass.
	timer := time.NewTimer(7 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if err := s.wait(ctx, c, true); err != nil {
		return fmt.Errorf("heartbeat did not keep the instance healthy: %w", err)
	}
	s.runtime, err = s.newRuntime(s.main)
	if err != nil {
		return err
	}
	s.resolver, err = s.runtime.Resolver()
	if err != nil {
		return err
	}
	s.remote, err = s.runtime.NewRemoteAgent(ctx, "sticky-caller", ClientFactory(s.transport, s.opts.Timeout, true, false))
	if err != nil {
		return err
	}
	resolved, err := s.resolver.Resolve(ctx, "sticky-caller")
	if err != nil {
		return err
	}
	s.boundID = resolved.InstanceID
	configuration, err := LoadConfiguration(s.opts, false)
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, s.opts.Timeout)
	projection, err := inspectServiceProjection(probeCtx, s.opts.Discover,
		configuration.GetGlobal().GetServerConnector().GetToken(), s.opts.Namespace, s.main, s.opts.RunID)
	cancel()
	if err != nil {
		return fmt.Errorf("inspect real SERVICES projection: %w", err)
	}
	if err := WriteJSON(filepath.Join(s.opts.RunDir, "service-projection.json"), projection); err != nil {
		return err
	}
	// Keep independently usable real fixtures available even if this server
	// does not implement the required service-level AI projection.
	return Eventually(ctx, 15*time.Second, func() error {
		request := &agentapi.GetServiceRequest{GetAgentServiceRequest: agentmodel.GetAgentServiceRequest{
			Namespace: s.opts.Namespace, Service: s.main,
		}}
		request.SetTimeout(s.opts.Timeout)
		service, err := s.agents.GetService(request)
		if err != nil {
			return err
		}
		return ValidateAIService(service)
	})
}

func (s *suite) multiInterface(ctx context.Context) error {
	previousVersion, previousMulti := s.opts.A2AVersion, s.opts.MultiInterface
	s.opts.A2AVersion, s.opts.MultiInterface = adapterconfig.A2AVersionV10, true
	defer func() {
		s.opts.A2AVersion, s.opts.MultiInterface = previousVersion, previousMulti
	}()
	service := s.service("multi-interface")
	c, err := s.start(ctx, service, "multi-interface", "/a2a", "valid", "")
	if err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.stop(stopCtx, c)
	}()
	if err := s.wait(ctx, c, true); err != nil {
		return err
	}
	response, err := s.all(service)
	if err != nil {
		return err
	}
	if len(response.GetInstances()) != 1 {
		return fmt.Errorf("multi-interface card produced %d instances, expected 1", len(response.GetInstances()))
	}
	instance := response.GetInstances()[0]
	_, card, err := agentcard.ParseAgentInstanceMetadata(instance.GetMetadata())
	if err != nil {
		return err
	}
	if len(card.SupportedInterfaces) != 2 {
		return fmt.Errorf("AgentCard exposes %d interfaces, expected 2", len(card.SupportedInterfaces))
	}
	if card.SupportedInterfaces[0].ProtocolBinding == card.SupportedInterfaces[1].ProtocolBinding {
		return errors.New("multi-interface card has duplicate protocol bindings")
	}
	return nil
}

func (s *suite) agentCardV03(ctx context.Context) error {
	previousVersion := s.opts.A2AVersion
	previousTenant := s.opts.Tenant
	s.opts.A2AVersion = adapterconfig.A2AVersionV03
	s.opts.Tenant = ""
	defer func() {
		s.opts.A2AVersion = previousVersion
		s.opts.Tenant = previousTenant
	}()
	service := s.service("v03")
	c, err := s.start(ctx, service, "v03", "/a2a", "valid", "")
	if err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.stop(stopCtx, c)
	}()
	if err := s.wait(ctx, c, true); err != nil {
		return err
	}
	response, err := s.all(service)
	if err != nil {
		return err
	}
	instance := response.GetInstance(c.ready.InstanceID)
	if instance == nil {
		return errors.New("v0.3 instance is missing")
	}
	parsed, card, err := agentcard.ParseAgentInstanceMetadata(instance.GetMetadata())
	if err != nil {
		return err
	}
	if parsed.A2AVersion != agentcard.SchemaVersionV03 || card.URL == "" {
		return errors.New("v0.3 AgentCard schema was not preserved")
	}
	runtime, err := s.newRuntime(service)
	if err != nil {
		return err
	}
	defer func() { _ = runtime.Close(context.Background()) }()
	resolver, err := runtime.Resolver()
	if err != nil {
		return err
	}
	remote, err := resolver.NewRemoteAgent(ctx, "v03-caller", ClientFactory(s.transport, s.opts.Timeout, true, false))
	if err != nil {
		return err
	}
	return s.invoke(ctx, remote, "v03", c.ready.Node)
}

func (s *suite) registrationPorts(ctx context.Context) error {
	service := s.service("ports")
	previousPort, previousVersion, previousID := s.opts.RegistrationPort, s.opts.AgentVersion, s.opts.InstanceID
	defer func() {
		s.opts.RegistrationPort, s.opts.AgentVersion, s.opts.InstanceID = previousPort, previousVersion, previousID
	}()
	s.opts.RegistrationPort = 1
	s.opts.AgentVersion, s.opts.InstanceID = "1.0.0", s.opts.RunID+"-port-1"
	first, err := s.start(ctx, service, "port-1", "/a2a", "valid", "")
	if err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.stop(stopCtx, first)
	}()
	s.opts.RegistrationPort = 18080
	s.opts.AgentVersion, s.opts.InstanceID = "2.0.0", s.opts.RunID+"-port-18080"
	second, err := s.start(ctx, service, "port-18080", "/a2a", "valid", "")
	if err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.stop(stopCtx, second)
	}()
	if err := s.wait(ctx, first, true); err != nil {
		return err
	}
	if err := Eventually(ctx, 30*time.Second, func() error {
		response, err := s.all(service)
		if err != nil {
			return err
		}
		ports := make(map[uint32]bool)
		for _, instance := range response.GetInstances() {
			_, card, err := agentcard.ParseAgentInstanceMetadata(instance.GetMetadata())
			if err != nil {
				return err
			}
			if card.Version != instance.GetVersion() || instance.GetProtocol() != "" {
				return errors.New("instance version or protocol does not match AgentCard")
			}
			if instance.GetPort() == 1 && card.Version != "1.0.0" ||
				instance.GetPort() == 18080 && card.Version != "2.0.0" {
				return errors.New("different registration ports did not preserve independent AgentCard versions")
			}
			ports[instance.GetPort()] = true
		}
		if len(response.GetInstances()) != 2 || !ports[1] || !ports[18080] {
			return errors.New("two registration ports were not both visible")
		}
		return nil
	}); err != nil {
		return err
	}
	if err := syscall.Kill(first.process.PID, syscall.SIGSTOP); err != nil {
		return err
	}
	defer func() { _ = syscall.Kill(first.process.PID, syscall.SIGCONT) }()
	s.opts.RegistrationPort, s.opts.AgentVersion, s.opts.InstanceID = 1, "1.1.0", first.ready.InstanceID
	replacement, err := s.start(ctx, service, "port-1-replacement", "/replacement", "valid", "")
	if err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.stop(stopCtx, replacement)
	}()
	return Eventually(ctx, 30*time.Second, func() error {
		response, err := s.all(service)
		if err != nil {
			return err
		}
		if len(response.GetInstances()) != 2 {
			return errors.New("same host/port registration created an extra instance")
		}
		current := response.GetInstance(first.ready.InstanceID)
		if current == nil || current.GetHost() != replacement.ready.Host || current.GetPort() != 1 || current.GetVersion() != "1.1.0" || current.GetProtocol() != "" {
			return errors.New("same host/port replacement did not preserve instance identity and update its version")
		}
		_, card, err := agentcard.ParseAgentInstanceMetadata(current.GetMetadata())
		if err != nil {
			return err
		}
		if card.Version != "1.1.0" || card.PrimaryURL() != replacement.ready.URL {
			return errors.New("replacement AgentCard did not update atomically with the registration version")
		}
		parallel := response.GetInstance(second.ready.InstanceID)
		if parallel == nil || parallel.GetVersion() != "2.0.0" || parallel.GetPort() != 18080 {
			return errors.New("same port replacement changed the parallel version instance")
		}
		return nil
	})
}

func (s *suite) unaryAuth(ctx context.Context) error {
	if err := s.invoke(ctx, s.remote, "hello", "node-a|echo:hello"); err != nil {
		return err
	}
	unauthorized, err := s.runtime.NewRemoteAgent(ctx, "unauthorized", ClientFactory(s.transport, s.opts.Timeout, false, false))
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, s.opts.Timeout)
	defer cancel()
	if _, _, err := Invoke(callCtx, unauthorized, "denied"); err == nil {
		return errors.New("unauthenticated A2A request was accepted")
	}
	stats, err := s.stats(ctx, s.first)
	if err != nil {
		return err
	}
	if stats.Authenticated < 1 || stats.Denied < 1 || stats.TenantAccepted < 1 || stats.TenantRejected != 0 {
		return errors.New("real provider did not observe authenticated, denied and correct-tenant requests")
	}
	return nil
}

func (s *suite) stats(ctx context.Context, c *child) (Stats, error) {
	if c == nil {
		return Stats{}, errors.New("provider prerequisite is missing")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+fmt.Sprintf("%s:%d", c.ready.Host, c.ready.Port)+"/e2e/stats", nil)
	if err != nil {
		return Stats{}, err
	}
	response, err := s.http.Do(req)
	if err != nil {
		return Stats{}, err
	}
	defer func() { _ = response.Body.Close() }()
	var result Stats
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("provider stats HTTP %d", response.StatusCode)
	}
	err = json.NewDecoder(response.Body).Decode(&result)
	return result, err
}

func (s *suite) streaming(ctx context.Context) error {
	if s.runtime == nil {
		return errors.New("consumer prerequisite is missing")
	}
	remote, err := s.runtime.NewRemoteAgent(ctx, "stream-caller", ClientFactory(s.transport, 25*time.Second, true, true))
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, s.opts.Timeout)
	text, count, err := Invoke(callCtx, remote, "stream")
	cancel()
	if err != nil {
		return err
	}
	if !strings.Contains(text, "node-a|echo:stream") || count < 3 {
		return errors.New("real streaming did not deliver multiple successful events and full output")
	}
	before, err := s.stats(ctx, s.first)
	if err != nil {
		return err
	}
	slowCtx, stop := context.WithCancel(ctx)
	defer stop()
	events, err := remote.Run(slowCtx, Invocation("slow"))
	if err != nil {
		return err
	}
	deadline := time.NewTimer(s.opts.Timeout)
	defer deadline.Stop()
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return errors.New("slow stream closed before delivering output")
			}
			if evt != nil && evt.Response != nil && evt.Error == nil && len(evt.Choices) > 0 && evt.Choices[0].Delta.Content != "" {
				stop()
				return s.waitCanceled(ctx, events, before)
			}
		case <-deadline.C:
			return errors.New("slow stream delivered no output before deadline")
		}
	}
}

func (s *suite) waitCanceled(ctx context.Context, events <-chan *event.Event, before Stats) error {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return Eventually(ctx, 5*time.Second, func() error {
					stats, err := s.stats(ctx, s.first)
					if err != nil {
						return err
					}
					if stats.Active != 0 || stats.Canceled <= before.Canceled {
						return errors.New("provider did not release the canceled runner")
					}
					return nil
				})
			}
		case <-timer.C:
			return errors.New("client event stream did not close after cancellation")
		}
	}
}

func (s *suite) exactEndpoints(ctx context.Context) error {
	for index, path := range []string{"/", "/a2a/", "/api/a2a", "/智能体/a2a"} {
		service, node := s.service(fmt.Sprintf("path-%d", index)), fmt.Sprintf("path-%d", index)
		c, err := s.start(ctx, service, node, path, "valid", "")
		if err != nil {
			return err
		}
		err = s.wait(ctx, c, true)
		if err == nil {
			runtime, createErr := s.newRuntime(service)
			if createErr != nil {
				err = createErr
			} else {
				remote, createErr := runtime.NewRemoteAgent(ctx, node, ClientFactory(s.transport, s.opts.Timeout, true, false))
				if createErr != nil {
					err = createErr
				} else {
					err = s.invoke(ctx, remote, "exact", node+"|echo:exact")
				}
				err = errors.Join(err, runtime.Close(context.Background()))
			}
		}
		err = errors.Join(err, s.stop(ctx, c))
		if err != nil {
			return fmt.Errorf("exact path %q: %w", path, err)
		}
	}
	return nil
}

func (s *suite) sticky(ctx context.Context) error {
	if s.resolver == nil {
		return errors.New("resolver prerequisite is missing")
	}
	c, err := s.start(ctx, s.main, "node-b", "/a2a", "valid", "")
	if err != nil {
		return err
	}
	s.second = c
	if err := s.wait(ctx, c, true); err != nil {
		return err
	}
	for i := 0; i < 12; i++ {
		resolved, err := s.resolver.Resolve(ctx, "sticky-caller")
		if err != nil {
			return err
		}
		if resolved.InstanceID != s.boundID {
			return errors.New("healthy sticky instance changed when a second node appeared")
		}
		if err := s.invoke(ctx, s.remote, "sticky", "node-a|echo:sticky"); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		resolved, err := s.resolver.Resolve(ctx, fmt.Sprintf("hash-%d", i))
		if err != nil {
			return err
		}
		seen[resolved.InstanceID] = true
	}
	if len(seen) != 2 {
		return errors.New("independent callers did not select both real instances")
	}
	return nil
}

func (s *suite) invalidCards(ctx context.Context) error {
	if s.resolver == nil {
		return errors.New("resolver prerequisite is missing")
	}
	missing := map[string]string{
		agentcard.MetadataAgentName:  s.main,
		agentcard.MetadataA2AVersion: "1.0",
	}
	if agentcard.ValidateAgentInstanceMetadata(missing) == nil {
		return errors.New("missing AgentCard unexpectedly passed metadata validation")
	}
	cfg := adapterconfig.DefaultConfig()
	cfg.Namespace, cfg.ServiceName, cfg.AgentName = s.opts.Namespace, s.main, s.main
	cfg.AgentVersion, cfg.AdvertiseHost, cfg.AdvertisePort = "1.0.0", "127.0.0.2", s.first.ready.Port
	cfg.A2APath = "/a2a"
	card, err := agentcard.NewAgentCard(cfg)
	if err != nil {
		return err
	}
	mismatch, err := agentcard.BuildAgentInstanceMetadata(cfg, card)
	if err != nil {
		return err
	}
	provider := api.NewProviderAPIByContext(s.sdk)
	fixtures := []struct {
		id       string
		port     int
		metadata map[string]string
	}{
		{id: s.opts.RunID + "-invalid-missing", port: 2, metadata: missing},
		{id: s.opts.RunID + "-invalid-ip", port: 3, metadata: mismatch},
	}
	for _, fixture := range fixtures {
		if err := registerRawFixture(provider, s.opts.Namespace, s.main, fixture.id, fixture.port, fixture.metadata); err != nil {
			return err
		}
		defer func(id string, port int) {
			_ = deregisterRawFixture(provider, s.opts.Namespace, s.main, id, port)
		}(fixture.id, fixture.port)
	}
	for i := 0; i < 16; i++ {
		resolved, err := s.resolver.Resolve(ctx, fmt.Sprintf("invalid-caller-%d", i))
		if err != nil {
			return err
		}
		if resolved.InstanceID != s.first.ready.InstanceID && resolved.InstanceID != s.second.ready.InstanceID {
			return errors.New("resolver selected real registered bad-card instance")
		}
	}
	return nil
}

func registerRawFixture(
	provider api.ProviderAPI,
	namespace, service, instanceID string,
	port int,
	metadata map[string]string,
) error {
	request := &api.InstanceRegisterRequest{InstanceRegisterRequest: model.InstanceRegisterRequest{
		Namespace:  namespace,
		Service:    service,
		Host:       "127.0.0.1",
		Port:       port,
		Metadata:   metadata,
		InstanceId: instanceID,
	}}
	request.SetTTL(30)
	_, err := provider.RegisterInstance(request)
	return err
}

func deregisterRawFixture(provider api.ProviderAPI, namespace, service, instanceID string, port int) error {
	return provider.Deregister(&api.InstanceDeRegisterRequest{InstanceDeRegisterRequest: model.InstanceDeRegisterRequest{
		Namespace:  namespace,
		Service:    service,
		Host:       "127.0.0.1",
		Port:       port,
		InstanceID: instanceID,
	}})
}

func (s *suite) setHealth(ctx context.Context, c *child, isolate, healthy bool) error {
	return Eventually(ctx, 30*time.Second, func() error {
		_, err := s.console.Request(ctx, http.MethodPut, "/naming/v1/instances", []map[string]any{{
			"id": c.ready.InstanceID, "namespace": s.opts.Namespace, "service": c.ready.Service,
			"host": c.ready.Host, "port": s.opts.RegistrationPort, "isolate": isolate, "healthy": healthy,
		}})
		return err
	})
}

func (s *suite) healthFiltering(ctx context.Context) (result error) {
	if s.first == nil || s.second == nil || s.resolver == nil {
		return errors.New("two healthy provider prerequisites are missing")
	}
	defer func() {
		restore, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result = errors.Join(result, signalProvider(s.first.process, syscall.SIGCONT))
		result = errors.Join(result, s.setHealth(restore, s.first, false, true))
	}()
	if err := s.setHealth(ctx, s.first, true, true); err != nil {
		return err
	}
	if err := Eventually(ctx, 15*time.Second, func() error {
		resolved, err := s.resolver.Resolve(ctx, "isolated-caller")
		if err != nil {
			return err
		}
		if resolved.InstanceID != s.second.ready.InstanceID {
			return errors.New("isolated node is still selected")
		}
		return nil
	}); err != nil {
		return err
	}
	if err := signalProvider(s.first.process, syscall.SIGSTOP); err != nil {
		return err
	}
	if err := s.setHealth(ctx, s.first, false, true); err != nil {
		return err
	}
	// Pause this run's heartbeat, then observe server TTL expiry independently.
	if err := Eventually(ctx, 30*time.Second, func() error {
		instances, err := s.console.Instances(ctx, s.opts.Namespace, s.main)
		if err != nil {
			return err
		}
		for _, instance := range instances {
			if instance.ID == s.first.ready.InstanceID && !instance.Healthy && !instance.Isolate {
				all, err := s.all(s.main)
				if err != nil {
					return err
				}
				observed := all.GetInstance(s.first.ready.InstanceID)
				if observed == nil || observed.IsHealthy() || observed.IsIsolated() {
					return errors.New("discovery has not observed the non-isolated unhealthy node")
				}
				resolved, err := s.resolver.Resolve(ctx, "unhealthy-caller")
				if err != nil {
					return err
				}
				if resolved.InstanceID == s.first.ready.InstanceID {
					return errors.New("unhealthy instance was selected")
				}
				return nil
			}
		}
		return errors.New("unhealthy transition has not become visible")
	}); err != nil {
		return err
	}
	if err := signalProvider(s.first.process, syscall.SIGCONT); err != nil {
		return err
	}
	return s.wait(ctx, s.first, true)
}

func (s *suite) concurrent(ctx context.Context) error {
	if s.runtime == nil {
		return errors.New("consumer prerequisite is missing")
	}
	return runConcurrent(24, func(index int) error {
		remote, err := s.runtime.NewRemoteAgent(ctx, fmt.Sprintf("parallel-%d", index), ClientFactory(s.transport, s.opts.Timeout, true, index%2 == 0))
		if err != nil {
			return err
		}
		return s.invoke(ctx, remote, "parallel", "|echo:parallel")
	})
}

func (s *suite) failover(ctx context.Context) error {
	if s.first == nil || s.second == nil || s.resolver == nil {
		return errors.New("failover prerequisites are missing")
	}
	if err := s.stop(ctx, s.first); err != nil {
		return err
	}
	if err := s.wait(ctx, s.first, false); err != nil {
		return err
	}
	if err := s.wait(ctx, s.second, true); err != nil {
		return err
	}
	resolved, err := s.resolver.Resolve(ctx, "sticky-caller")
	if err != nil {
		return err
	}
	if resolved.InstanceID != s.second.ready.InstanceID {
		return errors.New("sticky caller did not switch to surviving real node")
	}
	if err := s.invoke(ctx, s.remote, "failover", "node-b|echo:failover"); err != nil {
		return err
	}
	data, err := os.ReadFile(s.first.process.ReadyFile)
	if err != nil {
		return err
	}
	var ready Ready
	if err := json.Unmarshal(data, &ready); err != nil {
		return err
	}
	if !ready.Stopped || !ready.CloseIdempotent || !ready.ProviderRemoved || !ready.BorrowedContextAlive {
		return errors.New("provider shutdown did not prove correct lifecycle ownership")
	}
	return nil
}

func (s *suite) emptySet(ctx context.Context) error {
	if s.second == nil || s.resolver == nil {
		return errors.New("empty-set prerequisites are missing")
	}
	var result error
	for _, c := range append(append([]*child(nil), s.invalid...), s.second) {
		result = errors.Join(result, s.stop(ctx, c), s.wait(ctx, c, false))
	}
	if result != nil {
		return result
	}
	_, err := s.resolver.Resolve(ctx, "sticky-caller")
	if !errors.Is(err, naming.ErrNoAvailableInstance) {
		return fmt.Errorf("expected empty healthy set, got %v", err)
	}
	if _, err := s.remote.Run(ctx, Invocation("must-not-replay")); !errors.Is(err, naming.ErrNoAvailableInstance) {
		return fmt.Errorf("dynamic Agent replayed old endpoint after empty discovery: %v", err)
	}
	return nil
}

func (s *suite) fallback(ctx context.Context) (result error) {
	service := s.service("fault")
	c, err := s.start(ctx, service, "fault-provider", "/a2a", "valid", "")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.stop(context.Background(), c)) }()
	if err := s.wait(ctx, c, true); err != nil {
		return err
	}
	gate, err := NewTCPGate(s.opts.Discover)
	if err != nil {
		return err
	}
	defer gate.Close()
	cfg, err := LoadConfiguration(s.opts, false)
	if err != nil {
		return err
	}
	lister, err := NewLiveLister(gate.Address(), cfg.GetGlobal().GetServerConnector().GetToken())
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, lister.Close()) }()
	resolver := naming.NewResolver(naming.ResolverOptions{Namespace: s.opts.Namespace, Service: service, Timeout: s.opts.Timeout, Lister: lister})
	defer func() { result = errors.Join(result, resolver.Close()) }()
	var remote agent.Agent
	if err := Eventually(ctx, 25*time.Second, func() error {
		var err error
		remote, err = resolver.NewRemoteAgent(ctx, "fault-caller", ClientFactory(s.transport, s.opts.Timeout, true, false))
		return err
	}); err != nil {
		return err
	}
	gate.Pause()
	failuresBefore := lister.Failures.Load()
	if err := s.invoke(ctx, remote, "fallback", "fault-provider|echo:fallback"); err != nil {
		return err
	}
	if lister.Failures.Load() <= failuresBefore {
		return errors.New("fallback passed without a real network error")
	}
	timer := time.NewTimer(31 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if _, err := resolver.Resolve(ctx, "fault-caller"); !errors.Is(err, naming.ErrNoAvailableInstance) {
		return fmt.Errorf("expired fallback remained available: %v", err)
	}
	gate.Resume()
	if err := Eventually(ctx, 20*time.Second, func() error {
		return s.invoke(ctx, remote, "recover", "fault-provider|echo:recover")
	}); err != nil {
		return err
	}
	if err := s.stop(ctx, c); err != nil {
		return err
	}
	if err := s.wait(ctx, c, false); err != nil {
		return err
	}
	if _, err := resolver.Resolve(ctx, "fault-caller"); !errors.Is(err, naming.ErrNoAvailableInstance) {
		return fmt.Errorf("real empty response did not invalidate fallback: %v", err)
	}
	gate.Pause()
	if _, err := resolver.Resolve(ctx, "fault-caller"); !errors.Is(err, naming.ErrNoAvailableInstance) {
		return fmt.Errorf("network failure resurrected an invalidated cache: %v", err)
	}
	return nil
}
