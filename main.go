package main

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/go-ps"
	"golang.org/x/sys/unix"
)

func main() {
	if err := runConfigReloader(); err != nil {
		slog.Error("Config realoader main error", "err", err)
		os.Exit(1)
	}
}

func runConfigReloader() error {
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		return errors.New("missing env var CONFIG_DIR is empty, exiting")
	}

	processName := os.Getenv("PROCESS_NAME")
	if processName == "" {
		return errors.New("missing env var PROCESS_NAME is empty, exiting")
	}

	reloadSignal := syscall.SIGHUP
	reloadSignalEnv := os.Getenv("RELOAD_SIGNAL")
	if reloadSignalEnv != "" {
		reloadSignal = unix.SignalNum(reloadSignalEnv)
		if reloadSignal == 0 {
			return fmt.Errorf("cannot find signal for: %s", reloadSignalEnv)
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	err = watcher.Add(configDir)
	if err != nil {
		log.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	slog.Info(fmt.Sprintf("starting watcher with CONFIG_DIR=%s, PROCESS_NAME=%s, RELOAD_SIGNAL=%s\n", configDir, processName, reloadSignal))
outer:
	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Chmod != fsnotify.Chmod {
				slog.Debug(fmt.Sprintf("modified file: %s", event.Name))
				err := reloadProcess(processName, reloadSignal)
				if err != nil {
					slog.Error("Error reloading process", "err", err)
				}
				continue
			}
		case err := <-watcher.Errors:
			return fmt.Errorf("file watch error: %w", err)
		case signal := <-shutdown:
			slog.Info("Received signal", "signal", signal)
			break outer
		}
	}

	err = watcher.Close()
	if err != nil {
		return fmt.Errorf("failed to close file watcher: %w", err)
	}

	return nil
}

func findPID(process string) (int, error) {
	processes, err := ps.Processes()
	if err != nil {
		return -1, fmt.Errorf("failed to list processes: %v", err)
	}

	for _, p := range processes {
		if p.Executable() == process {
			log.Printf("found executable %s (pid: %d)\n", p.Executable(), p.Pid())
			return p.Pid(), nil
		}
	}

	return -1, fmt.Errorf("no process matching %s found", process)
}

func reloadProcess(process string, signal syscall.Signal) error {
	pid, err := findPID(process)
	if err != nil {
		return err
	}

	err = syscall.Kill(pid, signal)
	if err != nil {
		return fmt.Errorf("could not send signal: %v", err)
	}

	slog.Info(fmt.Sprintf("signal %s sent to %s (pid: %d)\n", strings.ToUpper(signal.String()), process, pid), "pid", pid)
	return nil
}
