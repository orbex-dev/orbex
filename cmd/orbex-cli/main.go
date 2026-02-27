package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	apiURL string
	apiKey string
)

func main() {
	root := &cobra.Command{
		Use:   "orbex",
		Short: "Orbex CLI — Run anything. Know everything.",
		Long:  "Orbex is a job orchestration platform.\nManage jobs, trigger runs, and view logs from the command line.",
	}

	root.PersistentFlags().StringVar(&apiURL, "api-url", envOrDefault("ORBEX_API_URL", "http://localhost:8080/api/v1"), "API base URL")
	root.PersistentFlags().StringVar(&apiKey, "api-key", os.Getenv("ORBEX_API_KEY"), "API key (or set ORBEX_API_KEY)")

	root.AddCommand(jobsCmd())
	root.AddCommand(runCmd())
	root.AddCommand(runsCmd())
	root.AddCommand(logsCmd())
	root.AddCommand(pauseCmd())
	root.AddCommand(resumeCmd())
	root.AddCommand(killCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// ─── Jobs ────────────────────────────────────────────────────

func jobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage job definitions",
	}

	// orbex jobs list
	list := &cobra.Command{
		Use:   "list",
		Short: "List all jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/jobs")
			if err != nil {
				return err
			}
			var jobs []map[string]interface{}
			json.Unmarshal(body, &jobs)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tIMAGE\tSCHEDULE\tACTIVE")
			for _, j := range jobs {
				schedule := "—"
				if s, ok := j["schedule"].(string); ok {
					schedule = s
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n",
					truncID(j["id"]), j["name"], j["image"], schedule, j["is_active"])
			}
			w.Flush()
			return nil
		},
	}

	// orbex jobs create
	var name, image, command, schedule string
	var timeout int
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a new job",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]interface{}{
				"name":  name,
				"image": image,
			}
			if command != "" {
				payload["command"] = strings.Split(command, " ")
			}
			if schedule != "" {
				payload["schedule"] = schedule
			}
			if timeout > 0 {
				payload["timeout_seconds"] = timeout
			}

			body, err := apiPost("/jobs", payload)
			if err != nil {
				return err
			}
			var job map[string]interface{}
			json.Unmarshal(body, &job)
			fmt.Printf("✓ Created job: %s (%s)\n", job["name"], truncID(job["id"]))
			return nil
		},
	}
	create.Flags().StringVar(&name, "name", "", "Job name (required)")
	create.Flags().StringVar(&image, "image", "", "Docker image (required)")
	create.Flags().StringVar(&command, "command", "", "Command (space-separated)")
	create.Flags().StringVar(&schedule, "schedule", "", "Cron schedule")
	create.Flags().IntVar(&timeout, "timeout", 0, "Timeout in seconds")
	create.MarkFlagRequired("name")
	create.MarkFlagRequired("image")

	// orbex jobs get <id>
	get := &cobra.Command{
		Use:   "get [job-id]",
		Short: "Get job details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/jobs/" + args[0])
			if err != nil {
				return err
			}
			var job map[string]interface{}
			json.Unmarshal(body, &job)
			printJSON(job)
			return nil
		},
	}

	// orbex jobs delete <id>
	del := &cobra.Command{
		Use:   "delete [job-id]",
		Short: "Delete a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiDelete("/jobs/" + args[0])
			if err != nil {
				return err
			}
			fmt.Println("✓ Job deleted")
			return nil
		},
	}

	cmd.AddCommand(list, create, get, del)
	return cmd
}

// ─── Run (trigger) ────────────────────────────────────────────

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [job-id]",
		Short: "Trigger a job run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiPost("/jobs/"+args[0]+"/run", nil)
			if err != nil {
				return err
			}
			var run map[string]interface{}
			json.Unmarshal(body, &run)
			fmt.Printf("✓ Run triggered: %s (status: %s)\n", truncID(run["id"]), run["status"])
			return nil
		},
	}
}

// ─── Runs ────────────────────────────────────────────

func runsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Manage job runs",
	}

	// orbex runs list <job-id>
	list := &cobra.Command{
		Use:   "list [job-id]",
		Short: "List runs for a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/jobs/" + args[0] + "/runs")
			if err != nil {
				return err
			}
			var runs []map[string]interface{}
			json.Unmarshal(body, &runs)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tEXIT\tDURATION\tCREATED")
			for _, r := range runs {
				exit := "—"
				if e, ok := r["exit_code"].(float64); ok {
					exit = fmt.Sprintf("%d", int(e))
				}
				dur := "—"
				if d, ok := r["duration_ms"].(float64); ok {
					dur = fmt.Sprintf("%.1fs", d/1000)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					truncID(r["id"]), r["status"], exit, dur, truncTime(r["created_at"]))
			}
			w.Flush()
			return nil
		},
	}

	// orbex runs get <run-id>
	get := &cobra.Command{
		Use:   "get [run-id]",
		Short: "Get run details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/runs/" + args[0])
			if err != nil {
				return err
			}
			var run map[string]interface{}
			json.Unmarshal(body, &run)
			printJSON(run)
			return nil
		},
	}

	cmd.AddCommand(list, get)
	return cmd
}

// ─── Logs ────────────────────────────────────────────

func logsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [run-id]",
		Short: "Get logs for a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/runs/" + args[0] + "/logs")
			if err != nil {
				return err
			}
			var data map[string]string
			json.Unmarshal(body, &data)
			fmt.Print(data["logs"])
			return nil
		},
	}
}

// ─── Pause / Resume / Kill ────────────────────────────

func pauseCmd() *cobra.Command {
	return &cobra.Command{
		Use: "pause [run-id]", Short: "Pause a running container",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiPost("/runs/"+args[0]+"/pause", nil)
			if err != nil {
				return err
			}
			fmt.Println("✓ Run paused")
			return nil
		},
	}
}

func resumeCmd() *cobra.Command {
	return &cobra.Command{
		Use: "resume [run-id]", Short: "Resume a paused container",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiPost("/runs/"+args[0]+"/resume", nil)
			if err != nil {
				return err
			}
			fmt.Println("✓ Run resumed")
			return nil
		},
	}
}

func killCmd() *cobra.Command {
	return &cobra.Command{
		Use: "kill [run-id]", Short: "Kill a running container",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiPost("/runs/"+args[0]+"/kill", nil)
			if err != nil {
				return err
			}
			fmt.Println("✓ Run killed")
			return nil
		},
	}
}

// ─── HTTP Helpers ────────────────────────────────────

func apiGet(path string) ([]byte, error) {
	return apiRequest("GET", path, nil)
}

func apiPost(path string, payload interface{}) ([]byte, error) {
	return apiRequest("POST", path, payload)
}

func apiDelete(path string) ([]byte, error) {
	return apiRequest("DELETE", path, nil)
}

func apiRequest(method, path string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, apiURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var apiErr map[string]string
		json.Unmarshal(respBody, &apiErr)
		msg := apiErr["message"]
		if msg == "" {
			msg = string(respBody)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, msg)
	}

	return respBody, nil
}

// ─── Format Helpers ────────────────────────────────────

func truncID(v interface{}) string {
	s, _ := v.(string)
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func truncTime(v interface{}) string {
	s, _ := v.(string)
	if len(s) > 19 {
		return s[:19]
	}
	return s
}

func printJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
