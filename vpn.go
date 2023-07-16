package main

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"time"
)

var elog debug.Log

type VPNService struct {
}

func (m *VPNService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const acceptedCommands = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx := context.Background()
	docker, err := NewDocker(ctx)
	if err != nil {
		_ = elog.Info(1, fmt.Sprintf("Failed to create Docker client: %v", err))
		changes <- svc.Status{State: svc.StopPending}
		return ssec, 1
	}
	defer func() {
		err = docker.Close()
		if err != nil {
			_ = elog.Info(1, fmt.Sprintf("Failed to close Docker client: %v", err))
		}
	}()

	wireguardOpts := &WireguardOptions{
		InterfaceName: "docker-win-net-connect",
		HostPeerIp:    "10.20.30.1",
		VmPeerIp:      "10.20.30.2",
		Port:          2030,
	}

	wireguard, err := NewWireguard(docker, wireguardOpts)
	if err != nil {
		_ = elog.Info(1, fmt.Sprintf("Failed to create Wireguard: %v", err))
		changes <- svc.Status{State: svc.StopPending}
		return ssec, 2
	}
	err = wireguard.Setup()
	if err != nil {
		_ = elog.Info(1, fmt.Sprintf("Failed to setup Wireguard: %v", err))
		changes <- svc.Status{State: svc.StopPending}
		return false, 3
	}

	_ = elog.Info(1, fmt.Sprintf("Wireguard server listening\n"))

	defer func() {
		err = wireguard.Teardown()
		if err != nil {
			_ = elog.Info(1, fmt.Sprintf("Failed to teardown Wireguard: %v", err))
		}
	}()

	_ = elog.Info(1, fmt.Sprintf("Starting service\n"))

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			_ = elog.Info(1, fmt.Sprintf("Setting up Wireguard on Docker Desktop VM\n"))
			err := wireguard.SetupVM()
			if err != nil {
				_ = elog.Info(1, fmt.Sprintf("Failed to setup VM: %v", err))
				time.Sleep(1 * time.Second)
				continue
			}

			_ = elog.Info(1, fmt.Sprintf("Watching Docker events\n"))
			stop := wireguard.Start(ctx)
			if stop {
				return
			}

			time.Sleep(1 * time.Second)
		}
	}()

	changes <- svc.Status{State: svc.Running, Accepts: acceptedCommands}

loop:
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			_ = elog.Info(1, fmt.Sprintf("Stopping service\n"))
			break loop
		case svc.Pause:
			changes <- svc.Status{State: svc.Paused, Accepts: acceptedCommands}
		case svc.Continue:
			changes <- svc.Status{State: svc.Running, Accepts: acceptedCommands}
		default:
			_ = elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
		}
	}

	cancel()
	changes <- svc.Status{State: svc.StopPending}

	return
}
