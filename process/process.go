package process

import (
	"sync"
	"go.uber.org/zap"
	"os/exec"
	"errors"
	"bufio"
	"golang.org/x/crypto/ssh"
	"bytes"
	"fmt"
	"strconv"
	"syscall"
	"time"
	"strings"
	"os"
	"io/ioutil"
)

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
	Auth Auth `json:"auth"`
	Hostname string `json:"hostname"`
	Name string `json:"name"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}

type Auth struct {
	Password   string `json:"password"`
	PrivateKey string `json:"private-key"`
}

type Logs struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// Create and Run a Process locally and return a startedProcess
func RunProcess(executable, stdoutLogfile, stderrLogfile string, arguments... string) (StartedProcess, error) {
	var waiting sync.WaitGroup
	var empty StartedProcess
	stderrLogger := createLogger(stderrLogfile)
	stdoutLogger := createLogger(stdoutLogfile)
	command := exec.Command(executable, arguments...)
	stderr, err := command.StderrPipe()
	if err != nil {
		return empty, errors.New("CreateProcess() impossible to pipe stderr")
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return empty, errors.New("CreateProcess() impossible to pipe stdout")
	}

	if err := command.Start(); err != nil {
		return empty, errors.New("CreateProcess() impossible to create the process")
	}

	waiting.Add(1)
	go func(){
		defer waiting.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			stdoutLogger.Info(scanner.Text())
		}
	}()

	waiting.Add(1)
	go func() {
		defer waiting.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderrLogger.Info(scanner.Text())
		}
	}()

	if err := command.Wait(); err != nil {
		return empty, errors.New("CreateProcess() Failed to create process for " + executable)
	}

	waiting.Wait()

	return StartedProcess {
		Executable: executable,
		Server: Target {
			Auth: Auth{
				Password: "",
				PrivateKey: "",
			},
			Hostname: "local",
			Name: "local",
			Port: 0,
			Username: "",
		},
		Pid: command.Process.Pid,
		Logs: Logs{
			Stdout: stdoutLogfile,
			Stderr: stderrLogfile,
		},
	}, nil
}

//------------------------------------------------------------------------------
// Process type functions (non exported)
//------------------------------------------------------------------------------
// Run a Process on a remote server
func (runtime Process) RunRemoteProcess(server Target) (*StartedProcess, error) {
	session, err := createSSHSession(server)
	if err != nil {
		return nil, errors.New("Failed to obtain an SSH session")
	}

	var buffer bytes.Buffer
	session.Stdout = &buffer

	// Create the command string
	arguments := strings.Join(runtime.Arguments, " ")
	command := fmt.Sprintf("nohup %s %s >> %s 2> %s & echo -n $!", runtime.Name, arguments, runtime.Logs.Stdout,
		runtime.Logs.Stderr)

	err = session.Run(command)
	if err != nil {
		return nil, errors.New("Command : " + command + " : failed")
	}

	output := buffer.String()
	pid, err := strconv.Atoi(output)
	if err != nil {
		return nil, errors.New("Unexpected output")
	}

	return &StartedProcess{
		Executable: runtime.Executable,
		Server: server,
		Pid: pid,
		Logs: Logs {
			Stdout: runtime.Logs.Stdout,
			Stderr: runtime.Logs.Stderr,
		},
	}, nil

}


//------------------------------------------------------------------------------
// StartedProcess type functions (non exported)
//------------------------------------------------------------------------------

// Send a signal to a specific process
// TODO Get stdout and stderr
func (process StartedProcess) Signal(signal syscall.Signal) error {
	if process.Server.Name != "local" {
		command := fmt.Sprintf("kill -s %d %d", signal, process.Pid)
		session, err := createSSHSession(process.Server)
		if err != nil {
			return errors.New("Failed to send signal")
		}
		err = session.Run(command)
		if err != nil {
			return errors.New("Failed to send signal")
		}
	} else {
		executable := "/bin/kill"
		arguments := []string{"-s", signal.String(), strconv.Itoa(process.Pid)}
		command := exec.Command(executable, arguments...)
		if err := command.Start(); err != nil {
			return errors.New("Failed to send signal")
		}
	}
	return nil
}

// Execute the function passed in parameter at the define frequency (in millisecond) on the given process
// Go count in nanosecond but we multiply by time.Millisecond
// TODO Solve the error passing issue
func (process StartedProcess) Watch(frequency int, onTick func(StartedProcess) (string, error),
	onCrash func(*StartedProcess) error) error {

	if frequency <= 0 {
		return errors.New("frequency must be greater than 0")
	}

	ticker := time.NewTicker(time.Duration(frequency) * time.Millisecond)
	quit := make(chan(struct{}))
	go func() {
		for {
			select {
			case <- ticker.C:
				_, err := onTick(process)
				if err != nil {
					onCrash(&process)
				}
			case <- quit:
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (process StartedProcess) Kill() error {
	return process.Signal(syscall.SIGTERM)
}

//------------------------------------------------------------------------------
// Utility functions (non exported)
//------------------------------------------------------------------------------

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

func createSSHSession(server Target) (*ssh.Session, error) {
	var sshConfig ssh.ClientConfig
	if server.Auth.PrivateKey == "" && server.Auth.Password != "" {
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
				publicKeyFile(server.Auth.PrivateKey),
			},
		}
	} else {
		return nil, errors.New("Incomplete credentials")
	}

	connection, err := ssh.Dial("tcp", "" + server.Hostname + ":" + strconv.Itoa(server.Port), &sshConfig)
	if err != nil {
		return nil, errors.New("Impossible to establish the connection")
	}
	session, err := connection.NewSession()
	if err != nil {
		return nil, errors.New("Impossible to establish the connection")
	}

	return session, nil
}

func publicKeyFile(file string) ssh.AuthMethod {
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