package main

import (
	"flag"
	"golang.org/x/sys/windows/svc"
	"log"
	"os"
	"strings"
)

func main() {
	genWinres := flag.Bool("winres", false, "Generate Windows resources")
	flag.Parse()

	if *genWinres {
		generateWinres()
		os.Exit(0)
	}

	svcName := "docker-win-net-connect"

	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}
	if inService {
		runService(svcName, false)
		return
	}

	if len(os.Args) < 2 {
		log.Printf("Usage: %s <command>", os.Args[0])
	}

	installer := NewInstaller(os.Args[0], svcName, "Docker network hacking service")
	manager := NewManager(svcName)

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		runService(svcName, true)
		return
	case "install":
		err = installer.InstallService()
	case "remove", "uninstall":
		err = installer.RemoveService()
	case "start":
		err = manager.StartService()
	case "stop":
		err = manager.ControlService(svc.Stop, svc.Stopped)
	case "pause":
		err = manager.ControlService(svc.Pause, svc.Paused)
	case "continue":
		err = manager.ControlService(svc.Continue, svc.Running)
	default:
		log.Printf("invalid command %s", cmd)
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, svcName, err)
	}
}
