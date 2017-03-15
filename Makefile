#
# Makefile for watchdog.go
#
GO = go
# make by default should build the binary
default: build

# make build should build the binary
build:
			$(info Get dependencies & build the binary)
			$(GO) get
			$(GO) build

# make clean remove log files (this does not kill the various docker image)
clean:
			$(info Clean log files)
			$(RM) *.log
