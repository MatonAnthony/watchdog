package main

import (
	"fmt"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
	"encoding/json"
	"io/ioutil"
	"strconv"
	"syscall"
	"watchdog/process"
	"os/signal"
)

var logger *zap.Logger
var configuration Config
var targetMap map[string]process.Target
var loadedProcess map[string]process.Process
var launchedProcess map[int]process.StartedProcess

type Process process.Process
// Structure obtained via jsonutil
type Config struct {
	Processes []process.Process `json:"processes"`
	Targets   []process.Target  `json:"target"`
}

// Initialize the global logger
func initializeLogger() {
	filename := fmt.Sprintf("watchdog-%s.log", time.Now().Format("2006-01-02 15:04:05"))
	logger = createLogger(filename)
}

// Load the configuration file and initialize top level variables
func initializeConfig() {
	launchedProcess = make(map[int]process.StartedProcess)
	configfile, err := ioutil.ReadFile("config.json")
	if err != nil {
		logger.Fatal("Unable to open configuration file")
		os.Exit(255)
	}
	json.Unmarshal(configfile, &configuration)

	// Convert my JSON array into a map to avoid multiple array walkthrough
	targetMap = make(map[string]process.Target)
	for _, target := range configuration.Targets {
		targetMap[target.Name] = target
	}

	loadedProcess = make(map[string]process.Process)
	for _, process := range configuration.Processes {
		loadedProcess[process.Name] = process
	}
}

func main() {
	var waiting sync.WaitGroup

	initializeLogger()
	initializeConfig()

	// Launch every Command loaded from the config file in a separate goroutine.
	for _, processus := range configuration.Processes {
		for i := 0; i < processus.Number; i++ {
			waiting.Add(1)
			// This goroutine takes this as a parameter due to the stack
			// architecture to prevent stack overwriting of this
			// variable
			go func(processus process.Process){
				defer waiting.Done()
				if processus.Target == "local" {
					started, err := process.RunProcess(
						processus.Executable,
						processus.Logs.Stdout,
						processus.Logs.Stderr,
						processus.Name,
						processus.Arguments...
					)
					if err != nil {
						logger.Fatal("Unable to create local process")
						killAll()
						os.Exit(1)
					}
					logger.Info("Local process started")
					launchedProcess[started.Pid] = started
				} else {
					started, err := processus.RunRemoteProcess(targetMap[processus.Target])
					if err != nil {
						logger.Fatal("Unable to create remote process")
						killAll()
						os.Exit(1)
					}
					logger.Info("Remote process started")
					launchedProcess[started.Pid] = *started
				}
			}(processus)
		}
	}

	waiting.Wait()
	setupWatcher()

	// Setup a trap on CTRL + C and on CTRL + D which call killAll()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		<-sigs
		killAll()
		os.Exit(1)
	}()
	waiting.Add(1)
	waiting.Wait()
}

// Set a watcher on every occurence of this process
func watch(processName string, frequency int, onTick func(process.StartedProcess) (string, error),
	onCrash func(*process.StartedProcess) error) {

	for _, processus := range launchedProcess {
		if processName == processus.Name {
			logger.Info("Add watcher on " + processName)
			go processus.Watch(frequency, onTick, onCrash)
		}
	}
}

// Kill every process started by the watchdog
func killAll() error {
	var err error
	for index, process := range launchedProcess {
		err = process.Kill()
		if err != nil {
			logger.Error("Failed to kill properly " + strconv.Itoa(process.Pid) + " on " +
				process.Server.Name)
			return err
		}
		delete(launchedProcess, index)
	}
	return nil
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
	return logger
}

// Place to write the watcher code and condition
func setupWatcher() {
	logger.Info("Starting watcher setup")
	// Example
	watch("tail", 5000, func(process.StartedProcess) (string, error){
		fmt.Println("Tick - Tack")
		return "", nil
	}, func(*process.StartedProcess) (error){
		fmt.Println("Oops crashed")
		return nil
	})
}
