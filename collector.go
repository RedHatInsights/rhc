package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	systemd "github.com/coreos/go-systemd/v22/dbus"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	collectorDirName        = "/usr/lib/rhc/collector.d"
	collectorCacheDirectory = "/var/cache/rhc/collector.d/"
	collectorGroupName      = "rhc-collector"
)

const notDefinedValue = "-"

// CollectorInfo holds information about the collector
type CollectorInfo struct {
	configFilePath string // Configuration file path
	id             string // Get from configuration file name
	Meta           struct {
		Name    string `json:"name" toml:"name"`
		Feature string `json:"feature,omitempty" toml:"feature,omitempty"`
	} `json:"meta" toml:"meta"`
	Exec struct {
		VersionCommand string `json:"version_command" toml:"version_command"`
		User           string `json:"user"`
		Collector      struct {
			Command string `json:"command" toml:"command"`
		}
		Archiver struct {
			Command string `json:"command" toml:"command"`
		}
		Uploader struct {
			Command string `json:"command" toml:"command"`
		}
	} `json:"exec" toml:"exec"`
	Systemd struct {
		Service string `json:"service" toml:"service"`
		Timer   string `json:"timer" toml:"timer"`
	} `json:"systemd" toml:"systemd"`
}

// readCollectorConfig tries to read collector information from the configuration .toml file
func readCollectorConfig(filePath string) (*CollectorInfo, error) {
	var collectorInfo CollectorInfo
	_, err := toml.DecodeFile(filePath, &collectorInfo)
	if err != nil {
		return nil, err
	}
	collectorInfo.configFilePath = filePath
	collectorInfo.id, _ = strings.CutSuffix(filepath.Base(filePath), ".toml")
	return &collectorInfo, nil
}

// readAllCollectors Tries to readd all collectors from the configuration files
func readAllCollectors() ([]CollectorInfo, error) {
	var collectors []CollectorInfo

	slog.Debug(fmt.Sprintf("Reading collectors from directory %s", collectorDirName))
	files, err := os.ReadDir(collectorDirName)
	if err != nil {
		return collectors, fmt.Errorf("failed to read directory %s: %v", collectorDirName, err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".toml" {
			slog.Debug(fmt.Sprintf("file '%s' is not a TOML file, skipping", file.Name()))
			continue
		}

		filePath := filepath.Join(collectorDirName, file.Name())

		collectorInfo, err := readCollectorConfig(filePath)
		if err != nil {
			slog.Warn(fmt.Sprintf("failed to read TOML file %s: %v\n", file.Name(), err))
			continue
		}

		collectors = append(collectors, *collectorInfo)
	}

	return collectors, nil
}

// getCollectorTimerNextTime tries to return the next time of collector timer.
func getCollectorTimerNextTime(collectorInfo *CollectorInfo) (*time.Time, error) {
	bgCtx := context.Background()
	conn, err := systemd.NewSystemConnectionContext(bgCtx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	collectorTimer := collectorInfo.Systemd.Timer
	if collectorTimer == "" {
		msg := fmt.Sprintf("no timer specified for %s", collectorInfo.id)
		slog.Warn(msg)
		return nil, fmt.Errorf(msg)
	}

	// We have to ask for Timer property. More details about D-Bus properties can be found here:
	// https://www.freedesktop.org/wiki/Software/systemd/dbus/
	properties, err := conn.GetUnitTypePropertiesContext(bgCtx, collectorTimer, "Timer")
	if err != nil {
		msg := fmt.Sprintf("failed to get timer properties of %s: %v", collectorTimer, err)
		slog.Warn(msg)
		return nil, fmt.Errorf(msg)
	}

	propName := "NextElapseUSecRealtime"
	nextTimeUs, exists := properties[propName]
	if !exists {
		msg := fmt.Sprintf("%s not found for %s", propName, collectorTimer)
		slog.Warn(msg)
		return nil, fmt.Errorf(msg)
	}

	microseconds, ok := nextTimeUs.(uint64)
	if !ok {
		msg := fmt.Sprintf("invalid %s type for %s", propName, collectorTimer)
		slog.Warn(msg)
		return nil, fmt.Errorf(msg)
	}

	if microseconds == math.MaxUint64 {
		zeroTime := time.Unix(0, 0)
		return &zeroTime, nil
	}

	nextTime := time.UnixMicro(int64(microseconds))
	return &nextTime, nil
}

type LastRun struct {
	Timestamp string `json:"timestamp"`
}

// writeTimeStampOfLastRun tries to write last_run.json file to cache directory of the collector
func writeTimeStampOfLastRun(collectorConfig *CollectorInfo) error {
	collectorCacheDir := path.Join(collectorCacheDirectory, collectorConfig.id)

	// Try to create a cache directory for this collector
	// Something like /var/cache/rhc/collector.d/<COLLECTOR_ID>/
	if _, err := os.Stat(collectorCacheDir); os.IsNotExist(err) {
		err = os.Mkdir(collectorCacheDir, 0700)
		if err != nil {
			return fmt.Errorf("failed to create collector cache directory %s: %v", collectorCacheDir, err)
		}
	}

	lastRunFilePath := path.Join(collectorCacheDir, "last_run.json")

	// When the previous time stamp exists, then delete it first
	if _, err := os.Stat(lastRunFilePath); err == nil {
		err = os.Remove(lastRunFilePath)
		if err != nil {
			return fmt.Errorf("failed to remove file %s: %v", lastRunFilePath, err)
		}
	}

	timeStamp := fmt.Sprintf("%d", time.Now().UnixMicro())
	lastRun := LastRun{Timestamp: timeStamp}
	data, err := json.MarshalIndent(lastRun, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal last run: %v", err)
	}

	err = os.WriteFile(lastRunFilePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %v", lastRunFilePath, err)
	}

	return nil
}

// runVersionCommand tries to run version command
func runVersionCommand(collectorConfig *CollectorInfo) (*string, error) {
	var outBuffer bytes.Buffer
	if collectorConfig.Exec.VersionCommand == "" {
		return nil, fmt.Errorf("no version command specified in %s", collectorConfig.configFilePath)
	}
	arguments := []string{"-c", collectorConfig.Exec.VersionCommand}
	cmd := exec.Command(bashFilePath, arguments...)
	cmd.Stdout = &outBuffer
	err := cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("failed to run collector '%s': %v", collectorConfig.Exec.VersionCommand, err)
	}

	stdOut := outBuffer.String()
	version := strings.TrimSpace(stdOut)

	return &version, nil
}

// readLastRun tries to read and parse the last run timestamp of the collector from
// the last_run.json file in the collector's cache directory. It returns a pointer to
// time.Time representing when the collector was last run. It returns an error if
// any error occurred during reading or parsing the timestamp. If the file doesn't
// exist or cannot be read/parsed, an error is also returned.
func readLastRun(collectorConfig *CollectorInfo) (*time.Time, error) {
	collectorCacheDir := path.Join(collectorCacheDirectory, collectorConfig.id)
	lastRunFilePath := path.Join(collectorCacheDir, "last_run.json")
	data, err := os.ReadFile(lastRunFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", lastRunFilePath, err)
	}
	var lastRun LastRun
	err = json.Unmarshal(data, &lastRun)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal last run: %v", err)
	}
	microseconds, err := strconv.ParseInt(lastRun.Timestamp, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %v", err)
	}
	lastTime := time.UnixMicro(microseconds)
	return &lastTime, nil
}

func changeCurrentUser(collectorConfig *CollectorInfo) error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %v", err)
	}

	// When the user is defined in the collector config, then try to switch to this user and rhc-collector group
	if collectorConfig.Exec.User != "" && currentUser.Username != collectorConfig.Exec.User {
		// Try to get user rhc-collector group
		collectorUser, err := user.Lookup(collectorConfig.Exec.User)
		if err != nil {
			return fmt.Errorf("failed to lookup user %v %v", collectorConfig.Exec.User, err)
		}
		collectorGroup, err := user.LookupGroup(collectorGroupName)
		if err != nil {
			return fmt.Errorf("failed to lookup group %v: %v", collectorGroupName, err)
		}

		// Try to convert the provided UID and GID to integers
		uid, err := strconv.Atoi(collectorUser.Uid)
		if err != nil {
			return fmt.Errorf("failed to convert uid %s to int: %v", collectorUser.Uid, err)
		}
		gid, err := strconv.Atoi(collectorGroup.Gid)
		if err != nil {
			return fmt.Errorf("failed to convert gid %s to int: %v", collectorGroup.Gid, err)
		}

		// Finally, try to change uid and gid. Note: the following system calls will fail when
		// the current user is not the root user, but it is expected behavior.
		if err := syscall.Setgid(gid); err != nil {
			return fmt.Errorf("failed to set group ID %d: %v (%v)",
				gid, collectorGroupName, err)
		}
		if err := syscall.Setuid(uid); err != nil {
			return fmt.Errorf("failed to set user ID %d: %v (%v)",
				uid, collectorConfig.Exec.User, err)
		}
	}

	return nil
}

// writeCommandOutputsToFiles tries to write command outputs to files
func writeCommandOutputsToFiles(cmd *string, stdoutFilePath string, stderrFilePath string, stdout *string, stderr *string) {
	err := os.WriteFile(stdoutFilePath, []byte(*stdout), 0600)
	if err != nil {
		slog.Warn(fmt.Sprintf("failed to write %s stdout to %s: %v", *cmd, stdoutFilePath, err))
	}
	err = os.WriteFile(stderrFilePath, []byte(*stderr), 0600)
	if err != nil {
		slog.Warn(fmt.Sprintf("failed to write %s stderr to %s: %v", *cmd, stderrFilePath, err))
	}
}

// collectData tries to run a given collector
func collectData(args ...string) (*string, *string, error) {
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	collectorCommand := args[0]
	tempDir := args[1]
	arguments := []string{"-c", collectorCommand}
	cmd := exec.Command(bashFilePath, arguments...)
	cmd.Dir = tempDir
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer
	err := cmd.Run()

	stdOut := stdoutBuffer.String()
	stdErr := stderrBuffer.String()

	if err != nil {
		return &stdOut, &stdErr, fmt.Errorf("failed to run collector '%s -c %s': %v",
			bashFilePath, collectorCommand, err)
	}

	return &stdOut, &stdErr, nil
}

// archiveData tries to run a given archiver
func archiveData(args ...string) (*string, *string, error) {
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	archiverCommand := args[0]
	tempDir := args[1]
	arguments := []string{"-c", archiverCommand + " " + args[2]}
	cmd := exec.Command(bashFilePath, arguments...)
	cmd.Dir = tempDir
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	err := cmd.Run()

	stdOut := stdoutBuffer.String()
	stdErr := stderrBuffer.String()

	if err != nil {
		return &stdOut, &stdErr, fmt.Errorf("failed to run archiver '%s': %v", archiverCommand, err)
	}

	return &stdOut, &stdErr, nil
}

// uploadData tries to run a given uploader
func uploadData(args ...string) (*string, *string, error) {
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	uploaderCommand := args[0]
	tempDir := args[1]
	arguments := []string{"-c", uploaderCommand + " " + args[2]}
	cmd := exec.Command(bashFilePath, arguments...)
	cmd.Dir = tempDir
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	err := cmd.Run()

	stdOut := stdoutBuffer.String()
	stdErr := stderrBuffer.String()

	if err != nil {
		return &stdOut, &stdErr, fmt.Errorf("failed to run uploader '%s': %v", uploaderCommand, err)
	}

	return &stdOut, &stdErr, nil
}
