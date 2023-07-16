package main

import (
	"fmt"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"time"
)

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	_ = elog.Info(2, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &VPNService{})
	if err != nil {
		_ = elog.Error(4, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	_ = elog.Info(3, fmt.Sprintf("%s service stopped", name))
}

type Manager struct {
	name string
}

func NewManager(name string) *Manager {
	return &Manager{name: name}
}

func (m *Manager) StartService() error {
	_m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer _m.Disconnect()
	s, err := _m.OpenService(m.name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func (m *Manager) ControlService(c svc.Cmd, to svc.State) error {
	_m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer _m.Disconnect()
	s, err := _m.OpenService(m.name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

type Installer struct {
	path string
	name string
	desc string
}

func NewInstaller(path, name, desc string) *Installer {
	return &Installer{
		path: path,
		name: name,
		desc: desc,
	}
}

func (i *Installer) InstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(i.name)
	if err == nil {
		_ = s.Close()
		return fmt.Errorf("service %s already exists", i.name)
	}
	s, err = m.CreateService(i.name, i.path, mgr.Config{DisplayName: i.desc}, "is", "auto-started")
	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(i.name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		_ = s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func (i *Installer) RemoveService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(i.name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", i.name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(i.name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}
