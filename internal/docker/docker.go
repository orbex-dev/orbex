package docker

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
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

	// Verify connection
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

	// Use Wait() to block until the pull completes
	if err := resp.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for image pull %s: %w", imageName, err)
	}
	log.Printf("[docker] Image ready: %s", imageName)
	return nil
}

// CreateContainer creates a new container with resource limits.
func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	// Convert env map to Docker format
	var envSlice []string
	for k, v := range cfg.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	// Container config
	containerCfg := &container.Config{
		Image: cfg.Image,
		Env:   envSlice,
	}
	if len(cfg.Command) > 0 {
		containerCfg.Cmd = cfg.Command
	}

	// Resource limits
	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			Memory:   int64(cfg.MemoryMB) * 1024 * 1024,    // MB to bytes
			NanoCPUs: int64(cfg.CPUMillicores) * 1_000_000, // millicores to nanocpus
		},
		SecurityOpt: []string{"no-new-privileges"},
	}

	result, err := c.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     containerCfg,
		HostConfig: hostCfg,
		Name:       cfg.Name,
	})
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
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

	// Docker multiplexes stdout/stderr with 8-byte binary headers per line.
	// stdcopy.StdCopy demultiplexes this and writes clean text without headers.
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
			log.Printf("[docker] Container %s wait error: %v", containerID[:12], err)
			return -1, fmt.Errorf("waiting for container: %w", err)
		}
		log.Printf("[docker] Container %s wait: nil error received", containerID[:12])
		return -1, fmt.Errorf("unexpected wait state")
	case status := <-waitResult.Result:
		log.Printf("[docker] Container %s exited with code %d", containerID[:12], status.StatusCode)
		return status.StatusCode, nil
	case <-ctx.Done():
		log.Printf("[docker] Container %s wait cancelled: %v", containerID[:12], ctx.Err())
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
