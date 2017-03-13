package main

import (
	"fmt"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"sync"
	"bufio"
	"time"
	"encoding/json"
	"io/ioutil"
	"golang.org/x/crypto/ssh"
	//"strings"
	"strconv"
	"errors"
	"bytes"
)

var logger *zap.Logger
var configuration Config
var targetMap map[string]Target
var launchedProcess map[int]StartedProcess

// Structure obtained via jsonutil
type Config struct {
	Processes []Process `json:"processes"`
	Targets   []Target  `json:"target"`
}
type Logs struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}
type Process struct {
	Name string         `json:"name"`
	Arguments  []string `json:"arguments"`
	Target string       `json:"target"`
	Executable string   `json:"executable"`
	Logs Logs           `json:"logs"`
	Number int          `json:"number"`
}
type StartedProcess struct {
	Executable string `json:"executable`
	Server Target     `json:"server"`
	Pid int           `json:"pid"`
	Logs Logs         `json:"logs`
}
type Target struct {
	Auth struct {
		Password   string      `json:"password"`
		PrivateKey string      `json:"private-key"`
	} `json:"auth"`
	Hostname string `json:"hostname"`
	Name string `json:"name"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}

// Initialize the global logger
func initializeLogger() {
	filename := fmt.Sprintf("watchdog-%s.log", time.Now().Format("2006-01-02 15:04:05"))
	logger = createLogger(filename)
}

// Load the configuration file and initialize top level variables
func initializeConfig() {
	launchedProcess = make(map[int]StartedProcess)
	configfile, err := ioutil.ReadFile("config.json")
	if err != nil {
		logger.Fatal("Unable to open configuration file")
		os.Exit(255)
	}
	json.Unmarshal(configfile, &configuration)

	// Convert my JSON array into a map to avoid multiple array walkthrough
	targetMap = make(map[string]Target)
	fmt.Printf("%+v", configuration)
	for _, target := range configuration.Targets {
		targetMap[target.Name] = target
	}
}

func main() {
	var waiting sync.WaitGroup

	initializeLogger()
	initializeConfig()

	// Launch every Command loaded from the config file in a separate goroutine.
	for _, process := range configuration.Processes {
		for i := 0; i < process.Number; i++ {
			waiting.Add(1)
			// This goroutine takes this as a parameter due to the stack
			// architecture to prevent stack overwriting of this
			// variable
			go func(process Process){
				defer waiting.Done()
				if process.Target == "local" {
					createProcess(
						process.Executable,
						process.Logs.Stdout,
						process.Logs.Stderr,
						process.Arguments...
					)
				} else {
					started, err := createRemoteProcess(process, targetMap[process.Target])
					if err != nil {
						logger.Fatal("Unable to create process")
						// TODO add a shutdown process that kills all existing process
						os.Exit(1)
					}
					launchedProcess[started.Pid] = *started
				}
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

// Utility functions to read the PrivateKey file
func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func createSSHSession(server Target) (*ssh.Session, error) {
var sshConfig ssh.ClientConfig
	if server.Auth.PrivateKey == "" && server.Auth.Password != "" {
		logger.Warn("SSH Authentication for " + server.Hostname + " is using an unsafe authentication")
		sshConfig = ssh.ClientConfig{
			User: server.Username,
			Auth: []ssh.AuthMethod{
				ssh.Password(server.Auth.Password),
			},
		}
	} else if server.Auth.Password == "" && server.Auth.PrivateKey != "" {
		sshConfig = ssh.ClientConfig{
			User: server.Username,
			Auth: []ssh.AuthMethod{
				PublicKeyFile(server.Auth.PrivateKey),
			},
		}
	} else {
		logger.Error("Provided target credentials are incomplete")
		return nil, errors.New("Provided target credentials are incomplete")
	}

	// Establish the connection
	connection, err := ssh.Dial("tcp", "" + server.Hostname + ":" + strconv.Itoa(server.Port), &sshConfig)
	if err != nil {
		logger.Error("Impossible to establish the connection")
		return nil, err
	}
	session, err := connection.NewSession()
	if err != nil {
		logger.Error("Impossible to establish the connection")
		return nil, err
	}

	return session, nil
}
func createRemoteProcess(runtime Process, server Target) (*StartedProcess, error) {

	session, err := createSSHSession(server)
	if err != nil {
		logger.Fatal("Failed to obtain an SSH Session")
		return nil, err
	}

	// Create the command string
	//arguments := strings.Join(runtime.Arguments, " ")
	/*command := fmt.Sprintf("daemon -v -E /var/log/watchdog/%s-err.log -O /var/log/watchdog/%s-out.log "+
		"-F /var/run/%s.pid %s %s -n %s && echo /var/run/%s.pid",
		runtime.Name, runtime.Name, runtime.Name, runtime.Name, runtime.Executable, arguments, runtime.Name)
        */
	command := "nohup tail -f /var/log/bootstrap.log >> hello.log 2> error.log & echo -n $!"
	/*stdout, err := session.StdoutPipe()
	if err != nil {
		logger.Error("StdoutPipe() failed")
		return nil, err
	}*/
	var buffer bytes.Buffer
	session.Stdout = &buffer

	err = session.Run(command)
	if err != nil {
		logger.Error("Command failed")
		return nil, err
	}
	if err != nil {
		logger.Error("Impossible to read the buffer")
		return nil, err
	}
	if err != nil {
		fmt.Println(buffer)
		logger.Error("Impossible to convert the buffer to int")
		return nil, err
	}

	output := buffer.String()
	pid, err := strconv.Atoi(output)
	if err != nil {
		logger.Error("Unexpected output : " + output)
		return nil, err
	}

	return &StartedProcess{
		Executable: runtime.Executable,
		Server: server,
		Pid: pid,
		Logs: Logs {
			Stdout: runtime.Name + "-out.log",
			Stderr: runtime.Name + "-err.log",
		},
	}, nil
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
/*
func shutdownAll() error {
	for pid, process := range launchedProcess {
		fmt.Printf("TODO")
	}
	return nil;
}
*/
