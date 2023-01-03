.PHONY: all
all: nwn-ctl

nwn-ctl: main.go
	go build -o $@ main.go
