package gorp

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestResetApplicationFlagsRemovesImportedFlags(t *testing.T) {
	original := flag.CommandLine
	t.Cleanup(func() {
		flag.CommandLine = original
	})

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.Int("quickchecks", 100, "The default number of iterations for each check")

	resetApplicationFlags()

	if flag.Lookup("quickchecks") != nil {
		t.Fatal("quickchecks flag should be removed")
	}
	if flag.Lookup("config") != nil || flag.Lookup("config-dir") != nil {
		t.Fatal("application flags should be registered after reset")
	}
}

func TestInvalidReloadKeepsCurrentServerRunning(t *testing.T) {
	if configFile := os.Getenv("GORP_RELOAD_TEST_CHILD_CONFIG"); configFile != "" {
		Start(configFile, true)
		return
	}

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener address: %v", err)
	}
	address := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatalf("release listener address: %v", err)
	}

	configFile := t.TempDir() + "/config.yaml"
	validConfig := fmt.Sprintf(`listeners:
  - name: mock
    type: mock
    address: %q
    mock:
      scenarios:
        healthy:
          match:
            path: /
          response:
            status: 200
            body: previous
`, address)
	if err := os.WriteFile(configFile, []byte(validConfig), 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestInvalidReloadKeepsCurrentServerRunning$")
	command.Env = append(os.Environ(), "GORP_RELOAD_TEST_CHILD_CONFIG="+configFile)
	outputReader, outputWriter := io.Pipe()
	command.Stdout = outputWriter
	command.Stderr = outputWriter
	if err := command.Start(); err != nil {
		t.Fatalf("start child server: %v", err)
	}
	reloadError := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(outputReader)
		reported := false
		for scanner.Scan() {
			if !reported && strings.Contains(scanner.Text(), "failed to build reloaded configuration") {
				reloadError <- scanner.Text()
				reported = true
			}
		}
	}()
	done := make(chan struct{})
	var processErr error
	go func() {
		processErr = command.Wait()
		_ = outputWriter.Close()
		close(done)
	}()
	defer func() {
		select {
		case <-done:
		default:
			_ = command.Process.Signal(os.Interrupt)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = command.Process.Kill()
				<-done
			}
		}
	}()

	endpoint := "http://" + address + "/"
	if err := waitForResponse(endpoint, "previous", 5*time.Second); err != nil {
		t.Fatalf("initial server did not start: %v", err)
	}

	invalidConfig := strings.Replace(validConfig, "type: mock", "type: unsupported", 1)
	if err := os.WriteFile(configFile, []byte(invalidConfig), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	select {
	case message := <-reloadError:
		if !strings.Contains(message, "invalid configuration") {
			t.Fatalf("reload error = %q, want a validation error", message)
		}
	case <-done:
		t.Fatalf("server process exited after invalid reload: %v", processErr)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not report the invalid reload")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			t.Fatalf("server process exited after invalid reload: %v", processErr)
		default:
		}
		if err := checkResponse(endpoint, "previous"); err != nil {
			t.Fatalf("previous configuration stopped serving after invalid reload: %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func waitForResponse(endpoint, expectedBody string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := checkResponse(endpoint, expectedBody); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	return lastErr
}

func checkResponse(endpoint, expectedBody string) error {
	client := http.Client{Timeout: time.Second}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-Mock-Scenario", "healthy")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || string(body) != expectedBody {
		return fmt.Errorf("response = %d %q, want 200 %q", response.StatusCode, body, expectedBody)
	}
	return nil
}
