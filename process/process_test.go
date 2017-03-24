package process

import (
	"testing"
	"encoding/json"
	"reflect"
	"syscall"
	"os"
)

// -----------------------------------------------------------------------------
// Test code related to loading the configuration
// -----------------------------------------------------------------------------

// Ensure that Unmarshaling of Process returns a Struct compliant to our defined model
func TestProcessUnmarshaling(t *testing.T) {
	expected := Process {
		Name: "unit-test",
		Arguments: []string{"-p", "should pass"},
		Target: "test-machine-1",
		Executable: "/bin/executable",
		Logs: Logs{
			Stdout: "/var/log/stdout.log",
			Stderr: "/var/log/stderr.log",
		},
		Number: 1,
	}

	data := "{" +
            "\"name\": \"unit-test\"," +
            "\"executable\": \"/bin/executable\"," +
            "\"arguments\": [\"-p\", \"should pass\"]," +
            "\"target\": \"test-machine-1\"," +
            "\"number\": 1," +
            "\"logs\": {" +
                "\"stdout\": \"/var/log/stdout.log\"," +
                "\"stderr\": \"/var/log/stderr.log\"" +
            "}" +
        "}"

	var test Process
	err := json.Unmarshal([]byte(data), &test)
	if err != nil {
		t.Errorf("Unmarshaling Process failed " + err.Error())
	}

	if !reflect.DeepEqual(expected, test) {
		t.Errorf("Expected %+v got %+v", expected, test)
	}
}

// Test Unmarshaling of Target returns a Struct compliant to our defined model
func TestTargetUnmarshaling(t *testing.T) {
	expected := Target {
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "test-center",
		Name: "test-machine-1",
		Port: 80,
		Username: "root",
	}

	data := "{" +
        	"\"auth\": {" +
		"\"password\":\"password\"," +
		"\"private-key\": \"\"" +
		"}," +
		"\"hostname\": \"test-center\"," +
		"\"name\": \"test-machine-1\"," +
		"\"port\": 80," +
		"\"username\": \"root\"" +
		"}"

	var test Target
	err := json.Unmarshal([]byte(data), &test)
	if err != nil {
		t.Errorf("Unmarshaling Target failed " + err.Error())
	}

	if !reflect.DeepEqual(expected, test) {
		t.Errorf("Expected %+v got %+v", expected, test)
	}
}

// -----------------------------------------------------------------------------
// Test code related to run Process
// -----------------------------------------------------------------------------

// Test to run a process (working scenario)
func TestRunProcess(t *testing.T) {
	_, err := RunProcess("/bin/ls", "vms/logErr.err", "vms/logErr.err", "ls", "-la")

	if err != nil {
		t.Errorf("Expected nil got %s", err.Error())
	}
}

// -----------------------------------------------------------------------------
// Test code related to run RemoteProcess
// -----------------------------------------------------------------------------

// Test to run a remote process on a server (ideal scenario)
func TestRunRemoteProcess(t *testing.T) {
	process := Process{
		Name: "test",
		Arguments: []string{"-l", "-a"},
		Target: "null",
		Executable: "/bin/ls",
		Logs: Logs{
			Stdout: "out.log",
			Stderr: "err.log",
		},
		Number: 1,
	}

	target := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "localhost",
		Name: "localhost",
		Port: 10000,
		Username: "root",
	}

	_, err := process.RunRemoteProcess(target)
	if err != nil {
		t.Errorf("Expected nil got %s", err.Error())
	}
}
// Test to run a remote process on a non existing server
func TestRunRemoteProcessOnNonExistingServer(t *testing.T) {
	dummyProcess := Process{
		Name: "test",
		Arguments: []string{"test"},
		Target: "I-do-not-exist",
		Executable: "/bin/notfound",
		Logs: Logs{
			Stdout: "nf.log",
			Stderr: "nf.log",
		},
		Number: 1,
	}

	dummyTarget := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "Idonotexist",
		Name: "I-do-not-exist",
		Port: 9999,
		Username: "root",
	}

	_, err := dummyProcess.RunRemoteProcess(dummyTarget)
	if err == nil {
		t.Errorf("Received nil expected error")
	}
}

// -----------------------------------------------------------------------------
// Test code related to Signal (and Kill which is a signal shortcut)
// -----------------------------------------------------------------------------

// Try to send a signal to a local process
func TestSignal(t *testing.T) {
	started := StartedProcess{
		Executable: "useless",
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
		Pid: os.Getpid(),
		Logs: Logs{
			Stdout: "vms/log",
			Stderr: "vms/log",
		},
		Name: "useless",
	}

	err := started.Signal(syscall.SIGUSR1)
	if err != nil {
		t.Errorf("Expected nil got %s", err.Error())
	}
}

// Try to send a kill signal to a local process
func TestKill(t *testing.T) {
	t.Skipf("This test is currently not working due to tail -f being infinite")
	started, err := RunProcess("/usr/bin/tail", "out.log", "err.log", "tail",
		"-f", "vms/log")

	if err != nil {
		t.Errorf("Fatal Error : failed to create preliminary process")
		t.Fatalf("Fatal Error : %+v ", err)
	}

	err = started.Kill()
	if err != nil {
		t.Errorf("Expected nil got %s", err.Error())
	}
}

// Try to send a signal to a remote process
func TestSignalRemoteProcess(t *testing.T) {
	proc := Process{
		Name: "tail",
		Arguments: []string{"-f", "out.logs"},
		Target: "test-center-1",
		Executable: "/usr/bin/tail",
		Logs: Logs{
			Stdout: "out.log",
			Stderr: "err.log",
		},
		Number: 1,
	}

	target := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "localhost",
		Port: 10000,
		Username: "root",
	}

	started, err := proc.RunRemoteProcess(target)
	if err != nil {
		t.Fatalf("Fatal Error : %+v", err)
	}

	err = started.Signal(syscall.SIGTERM)
	if err != nil {
		t.Errorf("Expected nil got %s", err.Error())
	}
}

// Try to kill a remote process (SIGTERM)
func TestKillRemoteProcess(t *testing.T) {
	proc := Process{
		Name: "tail",
		Arguments: []string{"-f", "out.logs"},
		Target: "test-center-1",
		Executable: "/usr/bin/tail",
		Logs: Logs{
			Stdout: "out.log",
			Stderr: "err.log",
		},
		Number: 1,
	}

	target := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "localhost",
		Port: 10000,
		Username: "root",
	}

	started, err := proc.RunRemoteProcess(target)
	if err != nil {
		t.Fatalf("Fatal Error : %+v", err)
	}

	err = started.Kill()
	if err != nil {
		t.Errorf("Expected nil got %s", err.Error())
	}
}

// -----------------------------------------------------------------------------
// Test code related to createSSHSession
// -----------------------------------------------------------------------------

// Test to create a SSH Session without any credentials
// (require SSHServer container to be up on port 10000)
func TestCreateSSHSessionIncompleteCredentials(t *testing.T) {
	dummyTarget := Target{
		Auth: Auth{
			Password: "",
			PrivateKey: "",
		},
		Hostname: "localhost",
		Name: "localhost",
		Port: 10000,
		Username: "root",
	}

	_, err := createSSHSession(dummyTarget);
	if err == nil {
		t.Errorf("Received nil expected error")
	}
}
// Try to start a SSH Session on a non-existing server using passworded authentication
func TestCreateSSHSessionPassword(t *testing.T) {
	dummyTarget := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "Idonotexist",
		Name: "I-do-not-exist",
		Port: 9999,
		Username: "root",
	}

	_, err := createSSHSession(dummyTarget);
	if err == nil {
		t.Errorf("Received nil expected error")
	}
}

// Try to connect to a SSH Server with no shell (require NoSSHShell container to be up
// on port 9999)
func TestCreateSSHSessionNoShell(t *testing.T) {
	noSSHShell := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "localhost",
		Port: 9999,
		Username: "root",
	}

	_, err := createSSHSession(noSSHShell);
	if err == nil {
		t.Errorf("Received nil expected error")
	}
}

// Test the connection to a working SSH Server with password
// (require SSHServer container to be up on port 10000)
func TestCreateSSHSessionPassword1(t *testing.T) {
	target := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "localhost",
		Name: "localhost",
		Port: 10000,
		Username: "root",
	}

	_, err := createSSHSession(target);
	if err != nil {
		t.Errorf("Received %s expected nil", err.Error())
	}

}

// -----------------------------------------------------------------------------
// Test code related to Watch
// -----------------------------------------------------------------------------
func TestWatchInvalidParameter(t *testing.T) {
	proc := Process{
		Name: "tail",
		Arguments: []string{"-f", "out.logs"},
		Target: "test-center-1",
		Executable: "/usr/bin/tail",
		Logs: Logs{
			Stdout: "out.log",
			Stderr: "err.log",
		},
		Number: 1,
	}

	target := Target{
		Auth: Auth{
			Password: "password",
			PrivateKey: "",
		},
		Hostname: "localhost",
		Port: 10000,
		Username: "root",
	}

	started, err := proc.RunRemoteProcess(target)
	if err != nil {
		t.Fatalf("Fatal Error : %+v", err)
	}

	err = started.Watch(-1, func(StartedProcess) (string, error){
		return "", nil
	}, func(*StartedProcess) (error){
		return nil
	})

	if err == nil {
		t.Errorf("Expected error got nil")
	}
}

// -----------------------------------------------------------------------------
// Test code related to publicKeyFile
// -----------------------------------------------------------------------------

// Test if the file doesn't exist
func TestPublicKeyFileFileNotFound(t *testing.T) {
	result := publicKeyFile("i-do-not-exist.dne")
	if result != nil {
		t.Errorf("Expected nil got %v", result)
	}
}

// Test with a valid key
func TestPublicKeyFile(t *testing.T) {
	result := publicKeyFile("vms/compromised")
	if result == nil {
		t.Errorf("Expected ssh.AuthMethod got nil")
	}
}

// Test with an invalid key
func TestPublicKeyFileCorrupted(t *testing.T) {
	result := publicKeyFile("vms/corrupted")
	if result != nil {
		t.Errorf("Expected nil got %+v", result)
	}
}

// -----------------------------------------------------------------------------
// Test code related to createLogger
// -----------------------------------------------------------------------------

// Call the logger on a locked file
func TestCreateLogger(t *testing.T) {
	t.Skipf("For an unknown reason this test crash in CI but in CI only")
	_, error := createLogger("vms/locked.log")
	if error == nil {
		t.Errorf("Expected error got nil")
	}
}

// Call the logger on a standard file
func TestCreateLogger1(t *testing.T) {
	_, error := createLogger("vms/log")
	if error != nil {
		t.Errorf("Expected nil got %s", error.Error())
	}
}

// -----------------------------------------------------------------------------
// Test code related to createCommand
// -----------------------------------------------------------------------------
func TestCreateCommand(t *testing.T) {
	expected := "nohup ls -l -a >> output 2> error & echo -n $!"
	command := createCommand("ls", []string{"-l", "-a"}, Logs{
		Stdout: "output",
		Stderr: "error",
	})

	if command != expected {
		t.Errorf("Expected %s got %s", expected, command)
	}
}
