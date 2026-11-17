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
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/internal/agentmodel"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	adapteragent "trpc.group/trpc-go/trpc-agent-naming-polarismesh/agent"
	"trpc.group/trpc-go/trpc-agent-naming-polarismesh/agentapi"
	adapterconfig "trpc.group/trpc-go/trpc-agent-naming-polarismesh/config"
	polarisregistry "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
)

func (s *suite) typeConflict(ctx context.Context) error {
	name := s.service("ordinary")
	s.state.Services = appendUnique(s.state.Services, name)
	if err := s.save(); err != nil {
		return err
	}
	if _, err := s.console.Request(ctx, http.MethodPost, "/naming/v1/services", []map[string]any{{
		"namespace": s.opts.Namespace, "name": name, "service_type": "SERVICE_TYPE_MICROSERVICE",
		"metadata": map[string]string{OwnerLabel: Owner, RunLabel: s.opts.RunID},
	}}); err != nil {
		return err
	}
	cfg := s.providerConfig(name)
	cfg.EnsureService = true
	runtime, err := adapteragent.New(
		adapteragent.WithConfig(cfg),
		adapteragent.WithSDKContext(s.sdk),
		adapteragent.WithAgentAPI(s.agents),
	)
	if err != nil {
		return err
	}
	defer func() { _ = runtime.Close(context.Background()) }()
	err = runtime.Prepare(ctx)
	if sdkErrorCode(err) != model.ErrCodeServiceTypeConflict {
		return fmt.Errorf("expected ordinary-service type conflict, got %v", err)
	}
	if _, err := runtime.AgentCard(); !errors.Is(err, adapteragent.ErrNotPrepared) {
		return errors.New("failed type-conflict Prepare exposed a card")
	}
	return nil
}

func (s *suite) prepareRetry(ctx context.Context) error {
	name := s.service("prepare-retry")
	if err := s.ensureService(ctx, name); err != nil {
		return err
	}
	skill := SkillPrefix + s.opts.RunID + "-retry"
	cfg := s.providerConfig(name)
	cfg.SkillCacheDir = filepath.Join(s.opts.RunDir, "skill-cache")
	cfg.SkillRefs = []adapterconfig.SkillRef{s.ref(skill, "1.0.0")}
	runtime, err := adapteragent.New(adapteragent.WithConfig(cfg), adapteragent.WithSDKContext(s.sdk))
	if err != nil {
		return err
	}
	defer func() { _ = runtime.Close(context.Background()) }()
	if err := runtime.Prepare(ctx); sdkErrorCode(err) != model.ErrCodeSkillNotFound {
		return fmt.Errorf("prepare with a missing real Skill returned %v", err)
	}
	if _, err := runtime.SkillRepository(); !errors.Is(err, adapteragent.ErrNotPrepared) {
		return errors.New("failed Prepare returned a repository")
	}
	if err := polarisregistry.RegisterMetadataProvider(name, polarisregistry.MetadataProviderFunc(func() (map[string]string, error) { return nil, nil })); err != nil {
		return errors.New("failed Prepare left metadata provider registered")
	}
	polarisregistry.UnregisterMetadataProvider(name)
	if err := s.createSkill(ctx, skill, "1.0.0", true, false); err != nil {
		return err
	}
	if err := runtime.Prepare(ctx); err != nil {
		return fmt.Errorf("prepare could not retry after publishing the missing Skill: %w", err)
	}
	if err := runtime.Prepare(ctx); err != nil {
		return err
	}
	repository, err := runtime.SkillRepository()
	if err != nil || repository == nil || len(repository.Summaries()) != 1 {
		return errors.Join(err, errors.New("retry did not assemble the published Skill"))
	}
	if err := runtime.Close(ctx); err != nil {
		return err
	}
	if err := runtime.Close(ctx); err != nil {
		return err
	}
	if s.sdk.IsDestroyed() {
		return errors.New("Close destroyed the borrowed SDK context")
	}
	if _, err := runtime.NewRemoteAgent(ctx, "closed"); !errors.Is(err, adapteragent.ErrClosed) {
		return errors.New("closed Runtime accepted a new consumer")
	}
	_, err = s.skills.GetSkill(&api.GetSkillRequest{GetSkillRequest: model.GetSkillRequest{Namespace: s.opts.Namespace, Name: skill, Version: "1.0.0"}})
	return err
}

func (s *suite) ensureExisting(ctx context.Context) error {
	name := s.service("ensure-idempotent")
	if err := s.ensureService(ctx, name); err != nil {
		return err
	}
	before, err := s.mount(name, []adapterconfig.SkillRef{s.ref(s.skillB, "1.0.0")}, true)
	if err != nil {
		return err
	}
	after, err := s.agents.EnsureService(&agentapi.EnsureServiceRequest{EnsureAgentServiceRequest: agentmodel.EnsureAgentServiceRequest{
		Namespace: s.opts.Namespace,
		Service:   name,
	}})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before.Metadata, after.Metadata) || !reflect.DeepEqual(sortedIDs(before), sortedIDs(after)) {
		return errors.New("ensure overwrote existing service metadata or Skill references")
	}
	return ctx.Err()
}

func (s *suite) agentServiceCRUD(ctx context.Context) error {
	name := s.service("agentapi-crud")
	s.state.Services = appendUnique(s.state.Services, name)
	if err := s.save(); err != nil {
		return err
	}
	skills := []model.AgentSkill{
		{ID: s.opts.Namespace + ":crud-a@1.0.0"},
		{ID: s.opts.Namespace + ":crud-b@1.0.0"},
	}
	service, err := s.agents.EnsureService(&agentapi.EnsureServiceRequest{
		EnsureAgentServiceRequest: agentmodel.EnsureAgentServiceRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
			Metadata: map[string]string{
				OwnerLabel: Owner,
				RunLabel:   s.opts.RunID,
				"crud":     "v1",
			},
			Skills: skills,
		},
	})
	if err != nil {
		return fmt.Errorf("ensure adapter Agent service: %w", err)
	}
	if service.Type != model.ServiceTypeAIAgent || service.Revision == "" ||
		len(service.Metadata) != 3 || len(service.ExtendedMetadata) != 2 {
		return fmt.Errorf("unexpected ensured Agent service: %+v", service)
	}

	read, err := s.agents.GetService(&agentapi.GetServiceRequest{
		GetAgentServiceRequest: agentmodel.GetAgentServiceRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
		},
	})
	if err != nil {
		return fmt.Errorf("get adapter Agent service: %w", err)
	}
	if read.Revision != service.Revision || !reflect.DeepEqual(sortedIDs(read), sortedIDs(service)) {
		return errors.New("adapter Agent Get did not return the ensured revision and Skills")
	}

	next := model.AgentSkill{ID: s.opts.Namespace + ":crud-c@1.0.0"}
	mounted, err := s.agents.MountSkills(&agentapi.MountSkillsRequest{
		MountAgentSkillsRequest: agentmodel.MountAgentSkillsRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
			Skills:    []model.AgentSkill{next},
		},
		Revision: read.Revision,
	})
	if err != nil {
		return fmt.Errorf("mount adapter Agent Skills: %w", err)
	}
	if len(mounted.ExtendedMetadata) != 3 {
		return fmt.Errorf("mount did not retain all Skills: %+v", sortedIDs(mounted))
	}

	unmounted, err := s.agents.UnmountSkills(&agentapi.UnmountSkillsRequest{
		UnmountAgentSkillsRequest: agentmodel.UnmountAgentSkillsRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
			SkillIDs:  []string{skills[0].ID},
		},
		Revision: mounted.Revision,
	})
	if err != nil {
		return fmt.Errorf("unmount adapter Agent Skill: %w", err)
	}
	if len(unmounted.ExtendedMetadata) != 2 {
		return fmt.Errorf("unmount retained the wrong Skills: %+v", sortedIDs(unmounted))
	}

	_, err = s.agents.MountSkills(&agentapi.MountSkillsRequest{
		MountAgentSkillsRequest: agentmodel.MountAgentSkillsRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
		},
		Revision: read.Revision,
	})
	if sdkErrorCode(err) != model.ErrCodeServicePreconditionFailed {
		return fmt.Errorf("stale Agent Service revision was not rejected: %v", err)
	}

	err = s.agents.DeleteService(&agentapi.DeleteServiceRequest{
		DeleteAgentServiceRequest: agentmodel.DeleteAgentServiceRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
			Revision:  unmounted.Revision,
			Owner:     "wrong-owner",
		},
	})
	if sdkErrorCode(err) != model.ErrCodeServicePreconditionFailed {
		return fmt.Errorf("wrong Agent Service owner was not rejected: %v", err)
	}
	if err = s.agents.DeleteService(&agentapi.DeleteServiceRequest{
		DeleteAgentServiceRequest: agentmodel.DeleteAgentServiceRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
			Revision:  unmounted.Revision,
		},
	}); err != nil {
		return fmt.Errorf("delete adapter Agent service: %w", err)
	}
	_, err = s.agents.GetService(&agentapi.GetServiceRequest{
		GetAgentServiceRequest: agentmodel.GetAgentServiceRequest{
			Namespace: s.opts.Namespace,
			Service:   name,
		},
	})
	if sdkErrorCode(err) != model.ErrCodeServiceNotFound {
		return fmt.Errorf("deleted Agent Service is still visible: %v", err)
	}
	return ctx.Err()
}

func (s *suite) cleanupSafety(ctx context.Context) error {
	statePath := filepath.Join(s.opts.RunDir, "state.json")
	state, err := LoadState(statePath)
	if err != nil {
		return err
	}
	bad := *state
	bad.Namespace = "default"
	if bad.Validate() == nil {
		return errors.New("cleanup accepted the default namespace")
	}
	bad = *state
	bad.Services = []string{"existing-production-service"}
	if bad.Validate() == nil {
		return errors.New("cleanup accepted an unowned service")
	}
	bad = *state
	bad.Console = "http://username:synthetic-test-secret@127.0.0.1:8090?access_token=test"
	if err := bad.Validate(); err == nil || strings.Contains(err.Error(), "synthetic-test-secret") {
		return errors.New("cleanup accepted or leaked access-bearing Console URL")
	}
	bad = *state
	bad.RunDir = "/"
	if bad.Validate() == nil {
		return errors.New("cleanup accepted an unconstrained artifact path")
	}
	dry := s.opts
	dry.Mode, dry.StateFile, dry.Apply, dry.DryRun = "cleanup", statePath, false, true
	if err := Cleanup(ctx, dry); err != nil {
		return err
	}
	service, err := s.console.Service(ctx, s.opts.Namespace, s.service("skill-provider"))
	if err != nil {
		return err
	}
	if service == nil || service.Metadata[RunLabel] != s.opts.RunID {
		return errors.New("dry-run modified or lost the owned service")
	}
	return nil
}

func sdkErrorCode(err error) model.ErrCode {
	if err == nil {
		return model.ErrCodeSuccess
	}
	var sdkErr model.SDKError
	if errors.As(err, &sdkErr) {
		return sdkErr.ErrorCode()
	}
	return model.ErrCodeUnknown
}
