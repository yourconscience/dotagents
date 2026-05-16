package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const dogfoodHTTPTimeout = 5 * time.Second

func runDogfood(opts runOptions) error {
	fmt.Println("dotagents dogfood")
	fmt.Println()

	fmt.Println("--- sync ---")
	if err := runSync(opts); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	fmt.Println()

	fmt.Println("--- status ---")
	if err := runStatus(opts); err != nil {
		return fmt.Errorf("status check failed after sync: %w", err)
	}
	fmt.Println()

	fmt.Println("--- doctor ---")
	if err := runDoctor(opts); err != nil {
		return fmt.Errorf("doctor found issues: %w", err)
	}
	fmt.Println()

	fmt.Println("--- dashboard ---")
	if err := runDashboardDogfood(); err != nil {
		return fmt.Errorf("dashboard dogfood failed: %w", err)
	}
	fmt.Println()

	fmt.Println("dogfood: all checks passed")
	return nil
}

func runDashboardDogfood() error {
	repoRoot, _, err := findRoots()
	if err != nil {
		return err
	}

	handler := newDashboardHandler(repoRoot, dashboardOptions{NoOpen: true})
	listener, err := listenDashboard("127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
		close(serverErrors)
	}()

	url := dashboardDogfoodURL(listener)
	fmt.Printf("  dashboard: %s\n", url)

	client := &http.Client{Timeout: dogfoodHTTPTimeout}
	if err := dogfoodGetContains(client, url+"/", "dotagents dashboard", "/assets/dashboard.js"); err != nil {
		shutdownDashboardDogfood(server)
		return err
	}
	if err := dogfoodGetContains(client, url+"/assets/dashboard.css", ".metric-row", ".item-row"); err != nil {
		shutdownDashboardDogfood(server)
		return err
	}
	if err := dogfoodGetContains(client, url+"/assets/dashboard.js", "async function render()", "loadJSON(\"/api/overview\")"); err != nil {
		shutdownDashboardDogfood(server)
		return err
	}

	var health dashboardHealth
	if err := dogfoodGetJSON(client, url+"/api/health", &health); err != nil {
		shutdownDashboardDogfood(server)
		return err
	}
	if !health.OK || health.Repo != repoRoot || health.SessionsEnabled {
		shutdownDashboardDogfood(server)
		return fmt.Errorf("unexpected dashboard health: %+v", health)
	}

	var overview dashboardOverview
	if err := dogfoodGetJSON(client, url+"/api/overview", &overview); err != nil {
		shutdownDashboardDogfood(server)
		return err
	}
	if overview.Repo != repoRoot {
		shutdownDashboardDogfood(server)
		return fmt.Errorf("unexpected dashboard overview repo: %q", overview.Repo)
	}

	var skills []dashboardSkill
	if err := dogfoodGetJSON(client, url+"/api/skills", &skills); err != nil {
		shutdownDashboardDogfood(server)
		return err
	}
	if len(skills) == 0 {
		shutdownDashboardDogfood(server)
		return fmt.Errorf("dashboard skills API returned no skills")
	}

	var examples dashboardExamplesResponse
	if err := dogfoodGetJSON(client, url+"/api/examples", &examples); err != nil {
		shutdownDashboardDogfood(server)
		return err
	}
	if len(examples.Examples) == 0 {
		shutdownDashboardDogfood(server)
		return fmt.Errorf("dashboard examples API returned no examples")
	}

	shutdownDashboardDogfood(server)
	select {
	case err := <-serverErrors:
		if err != nil {
			return err
		}
	case <-time.After(dogfoodHTTPTimeout):
		return fmt.Errorf("dashboard server did not stop cleanly")
	}

	fmt.Printf("  dashboard dogfood: %d skills, %d examples\n", len(skills), len(examples.Examples))
	return nil
}

func shutdownDashboardDogfood(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), dogfoodHTTPTimeout)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func dogfoodGetContains(client *http.Client, url string, needles ...string) error {
	body, err := dogfoodGetBody(client, url)
	if err != nil {
		return err
	}
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			return fmt.Errorf("%s missing %q", url, needle)
		}
	}
	return nil
}

func dogfoodGetJSON(client *http.Client, url string, dest interface{}) error {
	body, err := dogfoodGetBody(client, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(body), dest); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

func dogfoodGetBody(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", url, err)
	}
	return string(data), nil
}

func dashboardDogfoodURL(listener net.Listener) string {
	return "http://" + listener.Addr().String()
}
