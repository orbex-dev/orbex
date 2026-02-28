package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// ContainerConfig holds the parameters for creating a container.
type ContainerConfig struct {
	Image         string
	Command       []string
	Env           map[string]string
	MemoryMB      int
	CPUMillicores int // 1000 = 1 core
	Name          string
	Binds         []string // Host:Container bind mounts
	NetworkID     string   // Optional Docker network to connect to
	NetworkAlias  string   // Optional alias for the container on the network
}

// Client wraps the Docker Engine API client.
type Client struct {
	cli *client.Client
}

// New creates a new Docker client.
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	_, err = cli.Ping(context.Background(), client.PingOptions{})
	if err != nil {
		return nil, fmt.Errorf("connecting to docker daemon: %w", err)
	}

	return &Client{cli: cli}, nil
}

// Close closes the Docker client.
func (c *Client) Close() error {
	return c.cli.Close()
}

// PullImage pulls a Docker image if not already present.
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	log.Printf("[docker] Pulling image: %s", imageName)
	resp, err := c.cli.ImagePull(ctx, imageName, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", imageName, err)
	}
	if err := resp.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for image pull %s: %w", imageName, err)
	}
	log.Printf("[docker] Image ready: %s", imageName)
	return nil
}

// CreateContainer creates a new container with resource limits.
func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	var envSlice []string
	for k, v := range cfg.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	containerCfg := &container.Config{
		Image: cfg.Image,
		Env:   envSlice,
	}
	if len(cfg.Command) > 0 {
		containerCfg.Cmd = cfg.Command
	}

	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			Memory:   int64(cfg.MemoryMB) * 1024 * 1024,
			NanoCPUs: int64(cfg.CPUMillicores) * 1_000_000,
		},
		SecurityOpt: []string{"no-new-privileges"},
		Binds:       cfg.Binds,
	}

	result, err := c.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     containerCfg,
		HostConfig: hostCfg,
		Name:       cfg.Name,
	})
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	// Connect to network if specified
	if cfg.NetworkID != "" {
		var aliases []string
		if cfg.NetworkAlias != "" {
			aliases = []string{cfg.NetworkAlias}
		}
		_, err := c.cli.NetworkConnect(ctx, cfg.NetworkID, client.NetworkConnectOptions{
			Container: result.ID,
			EndpointConfig: &network.EndpointSettings{
				Aliases: aliases,
			},
		})
		if err != nil {
			log.Printf("[docker] Warning: failed to connect container to network: %v", err)
		}
	}

	return result.ID, nil
}

// StartContainer starts a container.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	_, err := c.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	return err
}

// StopContainer gracefully stops a container with a timeout.
func (c *Client) StopContainer(ctx context.Context, containerID string, timeoutSeconds int) error {
	_, err := c.cli.ContainerStop(ctx, containerID, client.ContainerStopOptions{
		Timeout: &timeoutSeconds,
	})
	return err
}

// PauseContainer freezes a running container via cgroup freezer.
func (c *Client) PauseContainer(ctx context.Context, containerID string) error {
	_, err := c.cli.ContainerPause(ctx, containerID, client.ContainerPauseOptions{})
	return err
}

// UnpauseContainer resumes a paused container.
func (c *Client) UnpauseContainer(ctx context.Context, containerID string) error {
	_, err := c.cli.ContainerUnpause(ctx, containerID, client.ContainerUnpauseOptions{})
	return err
}

// GetLogs retrieves stdout and stderr from a container.
func (c *Client) GetLogs(ctx context.Context, containerID string, tail string) (string, error) {
	result, err := c.cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	})
	if err != nil {
		return "", fmt.Errorf("getting logs: %w", err)
	}
	defer result.Close()

	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, result)
	if err != nil {
		return "", fmt.Errorf("reading logs: %w", err)
	}

	return buf.String(), nil
}

// WaitContainer blocks until the container exits and returns the exit code.
func (c *Client) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	log.Printf("[docker] Waiting for container %s to exit", containerID[:12])
	waitResult := c.cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNextExit,
	})

	select {
	case err := <-waitResult.Error:
		if err != nil {
			return -1, fmt.Errorf("waiting for container: %w", err)
		}
		return -1, fmt.Errorf("unexpected wait state")
	case status := <-waitResult.Result:
		log.Printf("[docker] Container %s exited with code %d", containerID[:12], status.StatusCode)
		return status.StatusCode, nil
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

// RemoveContainer removes a container.
func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	_, err := c.cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		Force: true,
	})
	return err
}

// InspectContainer returns the current state of a container.
func (c *Client) InspectContainer(ctx context.Context, containerID string) (*client.ContainerInspectResult, error) {
	resp, err := c.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspecting container: %w", err)
	}
	return &resp, nil
}

// BuildImage builds a Docker image from a tar build context.
func (c *Client) BuildImage(ctx context.Context, buildContext io.Reader, imageTag, dockerfilePath string) (string, error) {
	log.Printf("[docker] Building image: %s (Dockerfile: %s)", imageTag, dockerfilePath)

	resp, err := c.cli.ImageBuild(ctx, buildContext, client.ImageBuildOptions{
		Dockerfile:  dockerfilePath,
		Tags:        []string{imageTag},
		Remove:      true,
		ForceRemove: true,
	})
	if err != nil {
		return "", fmt.Errorf("starting build: %w", err)
	}
	defer resp.Body.Close()

	var buildLog strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return buildLog.String(), fmt.Errorf("reading build output: %w", err)
		}
		if msg.Error != "" {
			return buildLog.String(), fmt.Errorf("build error: %s", msg.Error)
		}
		if msg.Stream != "" {
			buildLog.WriteString(msg.Stream)
		}
	}

	log.Printf("[docker] Image built successfully: %s", imageTag)
	return buildLog.String(), nil
}

// CreateNetwork creates a Docker network and returns its ID.
func (c *Client) CreateNetwork(ctx context.Context, name string) (string, error) {
	resp, err := c.cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return "", fmt.Errorf("creating network %s: %w", name, err)
	}
	log.Printf("[docker] Created network: %s (%s)", name, resp.ID[:12])
	return resp.ID, nil
}

// RemoveNetwork removes a Docker network.
func (c *Client) RemoveNetwork(ctx context.Context, networkID string) error {
	_, err := c.cli.NetworkRemove(ctx, networkID, client.NetworkRemoveOptions{})
	return err
}

// ConnectNetwork connects a container to a Docker network with an optional alias.
func (c *Client) ConnectNetwork(ctx context.Context, networkID, containerID string, aliases []string) error {
	_, err := c.cli.NetworkConnect(ctx, networkID, client.NetworkConnectOptions{
		Container: containerID,
		EndpointConfig: &network.EndpointSettings{
			Aliases: aliases,
		},
	})
	return err
}
