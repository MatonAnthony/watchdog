package main

import (
	"fmt"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"sync"
	"bufio"
	"time"
	"os/signal"
	"encoding/json"
	"io/ioutil"
)

var logger *zap.Logger
var configuration Config

// Structure obtained via jsonutil
type Config struct {
	Processes []Process `json:"processes"`
}

type Logs struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}
type Process struct {
	Arguments  []string `json:"arguments"`
	Executable string   `json:"executable"`
	Logs Logs           `json:"logs"`
	Number int          `json:"number"`
}

type StartedProcess struct {
	Executable string `json:"executable`
	Server string     `json:"server"`
	Pid int           `json:"pid"`
	Logs Logs         `json:"logs`
}

// initialize the global logger & read the configuration file
func initializer() {
	filename := fmt.Sprintf("watchdog-%s.log", time.Now().Format("2006-01-02 15:04:05"))
	logger = createLogger(filename)

	configfile, err := ioutil.ReadFile("config.json")
	if err != nil {
		logger.Fatal("Unable to open configuration file")
		os.Exit(255)
	}
	// Read the configuration file
	json.Unmarshal(configfile, &configuration)
}


func main() {
	var waiting sync.WaitGroup

	initializer()

	// Launch every Command loaded from the config file in a separate goroutine.
	for _, process := range configuration.Processes {
		for i := 0; i < process.Number; i++ {
			waiting.Add(1)
			// This goroutine takes this as a parameter due to the stack
			// architecture to prevent stack overwriting of this
			// variable
			go func(process Process){
				defer waiting.Done()
				createProcess(
					process.Executable,
					process.Logs.Stdout,
					process.Logs.Stderr,
					process.Arguments...
				)
			}(process)
		}
	}

	waiting.Wait()
}

// Create a new Process and keep it running even if the watchdog is killed
func createProcess(executable, stdoutLogfile, stderrLogfile string, arguments ...string) error {
	var waiting sync.WaitGroup
	stderr_logger := createLogger(stderrLogfile)
	stdout_logger := createLogger(stdoutLogfile)
	command := exec.Command(executable, arguments...)
	stderr, err := command.StderrPipe()
	if err != nil {
		logger.Error("createProcess() impossible to pipe stderr")
		return err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		logger.Error("createProcess() impossible to pipe stdout")
		return err
	}

	if err := command.Start(); err != nil {
		logger.Fatal("createProcess() impossible to create the Queue")
		return err
	}

	// Goroutine to log stdout
	waiting.Add(1)
	go func(){
		defer waiting.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			stdout_logger.Info(scanner.Text())
		}
	}()
	waiting.Add(1)

	// Goroutine to log stderr
	go func(){
		defer waiting.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderr_logger.Info(scanner.Text())
		}
	}()

	if err := command.Wait(); err != nil {
		logger.Error("Processus : " + executable + "crashed")
		logger.Error("Relaunching attempt")
		err := createProcess(executable, stdoutLogfile, stderrLogfile, arguments...)
		if err != nil {
			logger.Fatal("Recovery attempt failed for "+ executable)
			return err
		}
	}
	waiting.Wait()
	return nil;
}

// Create a Logger writing to the path specified in parameter
func createLogger(filepath string) *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{filepath}
	logger, err := cfg.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Logger instantiation error")
		os.Exit(-1)
	}
	logger.Info("New logger created -> " + filepath)

	return logger
}
