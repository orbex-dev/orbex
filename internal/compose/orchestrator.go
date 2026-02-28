package compose

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/docker"
)

// Orchestrator manages multi-container compose deployments.
type Orchestrator struct {
	docker *docker.Client
}

// NewOrchestrator creates a new compose orchestrator.
func NewOrchestrator(dockerClient *docker.Client) *Orchestrator {
	return &Orchestrator{docker: dockerClient}
}

// RunResult holds the result of running a compose deployment.
type RunResult struct {
	ExitCode    int
	Logs        map[string]string // service name -> logs
	MainService string
	Error       error
}

// Run executes a parsed compose file: creates network, starts services, waits for main service.
func (o *Orchestrator) Run(ctx context.Context, cf *ComposeFile, jobID uuid.UUID, env map[string]string) *RunResult {
	result := &RunResult{
		Logs:     make(map[string]string),
		ExitCode: -1,
	}

	// Determine start order and main service
	order, err := cf.StartOrder()
	if err != nil {
		result.Error = fmt.Errorf("dependency resolution: %w", err)
		return result
	}

	mainService := cf.MainService()
	result.MainService = mainService
	log.Printf("[compose] Main service: %s, start order: %v", mainService, order)

	// Create a Docker network for this job
	networkName := fmt.Sprintf("orbex-%s", jobID.String()[:8])
	networkID, err := o.docker.CreateNetwork(ctx, networkName)
	if err != nil {
		result.Error = fmt.Errorf("create network: %w", err)
		return result
	}
	defer func() {
		if err := o.docker.RemoveNetwork(ctx, networkID); err != nil {
			log.Printf("[compose] Warning: failed to remove network %s: %v", networkName, err)
		}
	}()

	// Track created containers for cleanup
	var containers []string
	defer func() {
		for _, cid := range containers {
			_ = o.docker.RemoveContainer(ctx, cid)
		}
	}()

	// Start services in dependency order
	containerIDs := make(map[string]string) // service name -> container ID
	for _, svcName := range order {
		svc := cf.Services[svcName]

		// Determine the image
		image := svc.Image
		if image == "" && svc.Build != nil {
			// For built images, expect orbex/{jobID}:latest
			image = fmt.Sprintf("orbex/%s:latest", jobID)
		}
		if image == "" {
			result.Error = fmt.Errorf("service %s has no image or build configuration", svcName)
			return result
		}

		// Pull image (skip for locally built images)
		if !strings.HasPrefix(image, "orbex/") {
			if err := o.docker.PullImage(ctx, image); err != nil {
				result.Error = fmt.Errorf("pull image for %s: %w", svcName, err)
				return result
			}
		}

		// Merge environment: service env + job env
		mergedEnv := make(map[string]string)
		for k, v := range svc.ParsedEnv {
			mergedEnv[k] = v
		}
		for k, v := range env {
			mergedEnv[k] = v
		}

		// Create container
		containerName := fmt.Sprintf("orbex-%s-%s-%s", jobID.String()[:8], svcName, uuid.New().String()[:4])
		containerID, err := o.docker.CreateContainer(ctx, docker.ContainerConfig{
			Name:          containerName,
			Image:         image,
			Command:       svc.ParsedCmd,
			Env:           mergedEnv,
			MemoryMB:      512,
			CPUMillicores: 1000,
			NetworkID:     networkID,
			NetworkAlias:  svcName, // service name is the hostname
		})
		if err != nil {
			result.Error = fmt.Errorf("create container for %s: %w", svcName, err)
			return result
		}

		containers = append(containers, containerID)
		containerIDs[svcName] = containerID

		// Start container
		if err := o.docker.StartContainer(ctx, containerID); err != nil {
			result.Error = fmt.Errorf("start %s: %w", svcName, err)
			return result
		}

		log.Printf("[compose] Started service %s (%s)", svcName, containerID[:12])

		// Small delay between service starts for dependencies to initialize
		if svcName != mainService {
			time.Sleep(2 * time.Second)
		}
	}

	// Wait for main service to exit
	mainContainerID, ok := containerIDs[mainService]
	if !ok {
		result.Error = fmt.Errorf("main service %s not found", mainService)
		return result
	}

	log.Printf("[compose] Waiting for main service %s to exit...", mainService)
	exitCode, err := o.docker.WaitContainer(ctx, mainContainerID)
	if err != nil {
		result.Error = fmt.Errorf("wait for %s: %w", mainService, err)
		return result
	}
	result.ExitCode = int(exitCode)

	// Collect logs from all services
	var wg sync.WaitGroup
	var mu sync.Mutex
	for svcName, cid := range containerIDs {
		wg.Add(1)
		go func(name, containerID string) {
			defer wg.Done()
			logs, err := o.docker.GetLogs(ctx, containerID, "500")
			if err != nil {
				logs = fmt.Sprintf("[error getting logs: %v]", err)
			}
			mu.Lock()
			result.Logs[name] = logs
			mu.Unlock()
		}(svcName, cid)
	}
	wg.Wait()

	// Stop non-main services
	for svcName, cid := range containerIDs {
		if svcName != mainService {
			_ = o.docker.StopContainer(ctx, cid, 10)
		}
	}

	log.Printf("[compose] Main service %s exited with code %d", mainService, result.ExitCode)
	return result
}
