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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	collectorDirName        = "/usr/lib/rhc/collector.d"
	collectorCacheDirectory = "/var/cache/rhc/collector.d/"
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

	if _, err := os.Stat(collectorCacheDir); os.IsNotExist(err) {
		err = os.Mkdir(collectorCacheDir, 0700)
		if err != nil {
			return fmt.Errorf("failed to create collector cache directory %s: %v", collectorCacheDir, err)
		}
	}

	timeStamp := fmt.Sprintf("%d", time.Now().UnixMicro())
	lastRun := LastRun{Timestamp: timeStamp}
	data, err := json.MarshalIndent(lastRun, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal last run: %v", err)
	}

	lastRunFilePath := path.Join(collectorCacheDir, "last_run.json")

	err = os.Remove(lastRunFilePath)
	if err != nil {
		return fmt.Errorf("failed to remove file %s: %v", lastRunFilePath, err)
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

// collectData tries to run a given collector
func collectData(args ...string) (*string, error) {
	var outBuffer bytes.Buffer
	collectorCommand := args[0]
	tempDir := args[1]
	arguments := []string{"-c", collectorCommand}
	cmd := exec.Command(bashFilePath, arguments...)
	cmd.Dir = tempDir
	cmd.Stdout = &outBuffer
	err := cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("failed to run collector '%s -c %s': %v",
			bashFilePath, collectorCommand, err)
	}

	stdOut := outBuffer.String()

	return &stdOut, nil
}

// archiveData tries to run a given archiver
func archiveData(args ...string) (*string, error) {
	var outBuffer bytes.Buffer
	archiverCommand := args[0]
	tempDir := args[1]
	arguments := []string{"-c", archiverCommand + " " + args[2]}
	cmd := exec.Command(bashFilePath, arguments...)
	cmd.Dir = tempDir
	cmd.Stdout = &outBuffer
	err := cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("failed to run archiver '%s': %v", archiverCommand, err)
	}

	stdOut := outBuffer.String()

	return &stdOut, nil
}

func uploadData(args ...string) (*string, error) {
	var outBuffer bytes.Buffer
	uploaderCommand := args[0]
	tempDir := args[1]
	arguments := []string{"-c", uploaderCommand + " " + args[2]}
	cmd := exec.Command(bashFilePath, arguments...)
	cmd.Dir = tempDir
	cmd.Stdout = &outBuffer
	err := cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("failed to run uploader '%s': %v", uploaderCommand, err)
	}

	stdOut := outBuffer.String()
	return &stdOut, nil
}
