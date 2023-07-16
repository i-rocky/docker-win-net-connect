package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	ExitSetupFailed = 1
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	docker, err := NewDocker(ctx)
	if err != nil {
		log.Printf("Failed to create Docker client: %v", err)
		os.Exit(ExitSetupFailed)
	}
	defer func() {
		err = docker.Close()
		if err != nil {
			log.Printf("Failed to close Docker client: %v", err)
		}
	}()

	wireguardOpts := &WireguardOptions{
		InterfaceName: "docker-win-net-connect",
		HostPeerIp:    "10.33.33.1",
		VmPeerIp:      "10.33.33.2",
		Port:          3333,
	}

	wireguard, err := NewWireguard(docker, wireguardOpts)
	if err != nil {
		log.Printf("Failed to create Wireguard: %v", err)
		os.Exit(ExitSetupFailed)
	}

	err = wireguard.Setup()
	if err != nil {
		log.Printf("Failed to setup Wireguard: %v", err)
		os.Exit(ExitSetupFailed)
	}
	defer func() {
		err = wireguard.Teardown()
		if err != nil {
			log.Printf("Failed to teardown Wireguard: %v", err)
		}
	}()

	log.Printf("Wireguard server listening\n")

	go func() {
		for {
			log.Printf("Setting up Wireguard on Docker Desktop VM\n")
			err = wireguard.SetupVM()
			if err != nil {
				log.Printf("Failed to setup VM: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			log.Printf("Watching Docker events\n")
			wireguard.Start()

			time.Sleep(1 * time.Second)
		}
	}()

	term := make(chan os.Signal, 1)
	errs := make(chan error, 1)
	exit := make(chan bool, 1)
	signal.Notify(term, syscall.SIGTERM)
	signal.Notify(term, os.Interrupt)

	go func() {
		select {
		case <-term:
		case <-errs:
		}

		log.Printf("Shutting down\n")
		cancel()

		exit <- true
	}()

	<-exit

	log.Printf("Shutting down\n")
}
