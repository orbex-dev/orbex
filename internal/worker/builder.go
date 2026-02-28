package worker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/storage"
)

// BuildConfig holds configuration for the build worker.
type BuildConfig struct {
	MaxConcurrent int // Max parallel builds
	PollInterval  time.Duration
}

// RunBuilder polls the build_queue and builds Docker images.
func (w *Worker) RunBuilder(ctx context.Context) {
	log.Println("[builder] Build worker started")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[builder] Build worker stopped")
			return
		case <-ticker.C:
			w.pollAndBuild(ctx)
		}
	}
}

// pollAndBuild picks a pending build from the queue and processes it.
func (w *Worker) pollAndBuild(ctx context.Context) {
	var buildID, jobID, userID uuid.UUID

	err := w.db.Pool.QueryRow(ctx, `
		UPDATE build_queue
		SET status = 'building', started_at = now()
		WHERE id = (
			SELECT id FROM build_queue
			WHERE status = 'pending'
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, job_id, user_id
	`).Scan(&buildID, &jobID, &userID)
	if err != nil {
		return // No pending builds
	}

	log.Printf("[builder] Starting build %s for job %s", buildID, jobID)

	imageTag := fmt.Sprintf("orbex/%s:latest", jobID)
	buildLog, err := w.buildImage(ctx, userID, jobID, imageTag)

	if err != nil {
		log.Printf("[builder] Build %s failed: %v", buildID, err)
		errMsg := err.Error()
		_, _ = w.db.Pool.Exec(ctx, `
			UPDATE build_queue
			SET status = 'failed', error_message = $1, build_log = $2, finished_at = now()
			WHERE id = $3
		`, errMsg, buildLog, buildID)
		return
	}

	log.Printf("[builder] Build %s succeeded: %s", buildID, imageTag)

	// Update build queue
	_, _ = w.db.Pool.Exec(ctx, `
		UPDATE build_queue
		SET status = 'succeeded', image_tag = $1, build_log = $2, finished_at = now()
		WHERE id = $3
	`, imageTag, buildLog, buildID)

	// Update job's image to use the built image
	_, _ = w.db.Pool.Exec(ctx, `
		UPDATE jobs SET image = $1, updated_at = now() WHERE id = $2
	`, imageTag, jobID)
}

// buildImage downloads the build context from MinIO, creates a tar, and builds the Docker image.
func (w *Worker) buildImage(ctx context.Context, userID, jobID uuid.UUID, imageTag string) (string, error) {
	if w.storage == nil {
		return "", fmt.Errorf("storage client not available")
	}

	// Download build context from MinIO
	prefix := fmt.Sprintf("builds/%s/%s/", userID, jobID)
	objects, err := w.storage.List(ctx, prefix)
	if err != nil {
		return "", fmt.Errorf("list build context: %w", err)
	}
	if len(objects) == 0 {
		return "", fmt.Errorf("no build context files found")
	}

	// Create temp dir for build context
	buildDir := filepath.Join(os.TempDir(), "orbex-builds", jobID.String())
	os.MkdirAll(buildDir, 0755)
	defer os.RemoveAll(buildDir)

	// Download all files
	for _, obj := range objects {
		filename := strings.TrimPrefix(obj.Key, prefix)
		if err := downloadFile(ctx, w.storage, obj.Key, filepath.Join(buildDir, filename)); err != nil {
			return "", fmt.Errorf("download %s: %w", filename, err)
		}
	}

	// Find Dockerfile
	dockerfilePath := "Dockerfile"
	if _, err := os.Stat(filepath.Join(buildDir, dockerfilePath)); os.IsNotExist(err) {
		// Try to find a Dockerfile in the context
		entries, _ := os.ReadDir(buildDir)
		for _, e := range entries {
			if strings.EqualFold(e.Name(), "dockerfile") {
				dockerfilePath = e.Name()
				break
			}
		}
	}

	// Create tar archive of build context
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err = filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(buildDir, path)
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("create tar: %w", err)
	}
	tw.Close()

	// Build image using Docker API
	buildOutput, err := w.docker.BuildImage(ctx, &buf, imageTag, dockerfilePath)
	if err != nil {
		return buildOutput, fmt.Errorf("docker build: %w", err)
	}

	return buildOutput, nil
}

// EnqueueBuild adds a build job to the build queue.
func (w *Worker) EnqueueBuild(ctx context.Context, jobID, userID uuid.UUID) (uuid.UUID, error) {
	var buildID uuid.UUID
	err := w.db.Pool.QueryRow(ctx, `
		INSERT INTO build_queue (job_id, user_id)
		VALUES ($1, $2)
		RETURNING id
	`, jobID, userID).Scan(&buildID)
	return buildID, err
}

// downloadFile downloads a file from MinIO and writes it to the local filesystem.
func downloadFile(ctx context.Context, s *storage.Client, key, localPath string) error {
	dir := filepath.Dir(localPath)
	os.MkdirAll(dir, 0755)

	reader, err := s.Download(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}
