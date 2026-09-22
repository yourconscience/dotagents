package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type herdrPluginCommand struct {
	Command   []string `json:"command"`
	Platforms []string `json:"platforms"`
}

type herdrPlugin struct {
	PluginID   string               `json:"plugin_id"`
	PluginRoot string               `json:"plugin_root"`
	Actions    []herdrPluginCommand `json:"actions"`
	Events     []herdrPluginCommand `json:"events"`
	Panes      []herdrPluginCommand `json:"panes"`
}

type herdrPluginListResponse struct {
	Result struct {
		Plugins []herdrPlugin `json:"plugins"`
	} `json:"result"`
}

type herdrPluginLog struct {
	PluginID     string   `json:"plugin_id"`
	Command      []string `json:"command"`
	Status       string   `json:"status"`
	Error        string   `json:"error"`
	Stderr       string   `json:"stderr"`
	LogID        string   `json:"log_id"`
	StartedUnixM int64    `json:"started_unix_ms"`
}

type herdrPluginLogResponse struct {
	Result struct {
		Logs []herdrPluginLog `json:"logs"`
	} `json:"result"`
}

type executableLookup func(string) (string, error)

func checkHerdrPluginHealth() checkResult {
	if os.Getenv("HERDR_ENV") != "1" {
		return checkResult{"herdr plugins", checkStatusPass, "not running inside Herdr, skipped"}
	}
	herdr, err := exec.LookPath("herdr")
	if err != nil {
		return checkResult{"herdr plugins", checkStatusWarn, "Herdr session detected but herdr CLI is not on PATH"}
	}

	pluginData, err := exec.Command(herdr, "plugin", "list", "--json").Output()
	if err != nil {
		return checkResult{"herdr plugins", checkStatusWarn, fmt.Sprintf("cannot inspect installed plugins: %v", err)}
	}
	logData, err := exec.Command(herdr, "plugin", "log", "list").Output()
	if err != nil {
		return checkResult{"herdr plugins", checkStatusWarn, fmt.Sprintf("cannot inspect plugin command logs: %v", err)}
	}

	var plugins herdrPluginListResponse
	if err := json.Unmarshal(pluginData, &plugins); err != nil {
		return checkResult{"herdr plugins", checkStatusWarn, fmt.Sprintf("cannot parse plugin inventory: %v", err)}
	}
	var logs herdrPluginLogResponse
	if err := json.Unmarshal(logData, &logs); err != nil {
		return checkResult{"herdr plugins", checkStatusWarn, fmt.Sprintf("cannot parse plugin command logs: %v", err)}
	}
	return assessHerdrPluginHealth(plugins.Result.Plugins, logs.Result.Logs, exec.LookPath)
}

func assessHerdrPluginHealth(plugins []herdrPlugin, logs []herdrPluginLog, lookup executableLookup) checkResult {
	latest := make(map[string]herdrPluginLog)
	for _, log := range logs {
		key := herdrCommandKey(log.PluginID, log.Command)
		if prior, ok := latest[key]; !ok || log.StartedUnixM > prior.StartedUnixM {
			latest[key] = log
		}
	}

	var issues []string
	for _, plugin := range plugins {
		commands := append([]herdrPluginCommand{}, plugin.Actions...)
		commands = append(commands, plugin.Events...)
		commands = append(commands, plugin.Panes...)
		for _, declared := range commands {
			if len(declared.Command) == 0 || !herdrCommandApplies(declared.Platforms, runtime.GOOS) {
				continue
			}
			missingPath := missingHerdrCommandPath(plugin.PluginRoot, declared.Command)
			if missingPath != "" {
				issues = append(issues, fmt.Sprintf("%s declares missing command file %s; reinstall or update the plugin", plugin.PluginID, missingPath))
				continue
			}
			executable := declared.Command[0]
			if !strings.ContainsRune(executable, filepath.Separator) {
				if _, err := lookup(executable); err != nil {
					issues = append(issues, fmt.Sprintf("%s requires executable %q but doctor cannot resolve it; install it or use an absolute executable path in the plugin manifest", plugin.PluginID, executable))
					continue
				}
			}

			log, ok := latest[herdrCommandKey(plugin.PluginID, declared.Command)]
			if !ok || log.Status != "failed" {
				continue
			}
			if strings.Contains(strings.ToLower(log.Error), "no such file or directory") {
				if !strings.ContainsRune(executable, filepath.Separator) {
					if resolved, err := lookup(executable); err == nil {
						issues = append(issues, fmt.Sprintf("%s cannot start %q from the Herdr server PATH although doctor resolves it at %s; use that absolute executable in the plugin manifest or restart Herdr with its directory on PATH", plugin.PluginID, executable, resolved))
						continue
					}
				}
				issues = append(issues, fmt.Sprintf("%s cannot start %q; install it or use an absolute executable path in the plugin manifest", plugin.PluginID, executable))
				continue
			}
			detail := strings.TrimSpace(log.Error)
			if detail == "" {
				detail = strings.TrimSpace(log.Stderr)
			}
			if detail == "" {
				detail = "command failed"
			}
			issues = append(issues, fmt.Sprintf("%s latest command log %s failed: %s", plugin.PluginID, log.LogID, detail))
		}
	}

	if len(issues) > 0 {
		sort.Strings(issues)
		return checkResult{"herdr plugins", checkStatusWarn, strings.Join(issues, "; ")}
	}
	return checkResult{"herdr plugins", checkStatusPass, fmt.Sprintf("%d installed plugins have valid commands and no current command failures", len(plugins))}
}

func herdrCommandApplies(platforms []string, goos string) bool {
	if len(platforms) == 0 {
		return true
	}
	if goos == "darwin" {
		goos = "macos"
	}
	for _, platform := range platforms {
		if platform == goos {
			return true
		}
	}
	return false
}

func herdrCommandKey(pluginID string, command []string) string {
	return pluginID + "\x00" + strings.Join(command, "\x00")
}

func missingHerdrCommandPath(pluginRoot string, command []string) string {
	for i, token := range command {
		if i == 0 && !strings.ContainsRune(token, filepath.Separator) {
			continue
		}
		if strings.HasPrefix(token, "-") || strings.ContainsAny(token, " \t\n\r$\"'`") || (!strings.HasPrefix(token, ".") && !strings.ContainsRune(token, filepath.Separator)) {
			continue
		}
		candidate := token
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(pluginRoot, candidate)
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return ""
}
