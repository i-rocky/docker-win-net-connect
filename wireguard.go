package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const TunnelConf = `[Interface]
PrivateKey = %s
Address = %s/32
ListenPort = %d
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = %s
AllowedIPs = %s
PersistentKeepalive = 25
`

type Version struct {
	SetupImage string
}

var version = Version{
	SetupImage: "wpkpda/docker-win-net-setup",
}

type Wireguard struct {
	docker         *Docker
	interfaceName  string
	interfaceIndex int
	hostPeerIp     string
	vmPeerIp       string
	hostPrivateKey *wgtypes.Key
	vmPrivateKey   *wgtypes.Key
	vmIpNet        *net.IPNet
	port           int
}

type WireguardOptions struct {
	InterfaceName string
	HostPeerIp    string
	VmPeerIp      string
	Port          int
}

//go:embed bin/wg.exe bin/wireguard.exe
var binaries embed.FS

func NewWireguard(docker *Docker, opts *WireguardOptions) (*Wireguard, error) {
	hostPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, errors.New("failed to generate host private key: " + err.Error())
	}

	vmPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, errors.New("failed to generate VM private key: " + err.Error())
	}

	_, vmIpNet, err := net.ParseCIDR(opts.VmPeerIp + "/32")
	if err != nil {
		return nil, errors.New("failed to parse VM peer CIDR: " + err.Error())
	}

	return &Wireguard{
		docker:         docker,
		interfaceName:  opts.InterfaceName,
		hostPrivateKey: &hostPrivateKey,
		vmPrivateKey:   &vmPrivateKey,
		hostPeerIp:     opts.HostPeerIp,
		vmPeerIp:       opts.VmPeerIp,
		vmIpNet:        vmIpNet,
		port:           opts.Port,
	}, nil
}

func (w *Wireguard) Setup() error {
	err := w.extractBinaries()
	if err != nil {
		return errors.New("failed to extract binaries: " + err.Error())
	}

	err = w.downloadSetup()
	if err != nil {
		return errors.New("failed to download setup: " + err.Error())
	}

	err = w.installTunnel()
	if err != nil {
		return errors.New("failed to install tunnel: " + err.Error())
	}

	time.Sleep(1 * time.Second)

	err = w.updateInterface()
	if err != nil {
		return errors.New("failed to update interface: " + err.Error())
	}

	err = w.findInterfaceIndex()
	if err != nil {
		return errors.New("failed to find interface index: " + err.Error())
	}

	err = w.deleteWireguardRoute()
	if err != nil {
		return errors.New("failed to delete wireguard route: " + err.Error())
	}

	return nil
}

func (w *Wireguard) Teardown() error {
	err := w.uninstallTunnel()
	if err != nil {
		return errors.New("failed to uninstall tunnel: " + err.Error())
	}

	return nil
}

func (w *Wireguard) extractBinaries() error {
	files := 0
	_, err := os.Stat("bin/wg.exe")
	if err == nil {
		files++
	}

	_, err = os.Stat("bin/wireguard.exe")
	if err == nil {
		files++
	}

	if files == 2 {
		return nil
	}

	err = os.MkdirAll("bin", 0755)
	if err != nil {
		return errors.New("failed to create bin directory: " + err.Error())
	}

	wg, err := binaries.ReadFile("bin/wg.exe")
	if err != nil {
		return errors.New("failed to open wg.exe: " + err.Error())
	}

	err = os.WriteFile("bin/wg.exe", wg, 0755)
	if err != nil {
		return errors.New("failed to write wg.exe: " + err.Error())
	}

	wireguard, err := binaries.ReadFile("bin/wireguard.exe")
	if err != nil {
		return errors.New("failed to open wireguard.exe: " + err.Error())
	}

	err = os.WriteFile("bin/wireguard.exe", wireguard, 0755)
	if err != nil {
		return errors.New("failed to write wireguard.exe: " + err.Error())
	}

	return nil
}

func (w *Wireguard) getDockerNetworks() (string, error) {
	subnets, err := w.docker.GetSubnets()
	if err != nil {
		return "", errors.New("failed to get docker subnets: " + err.Error())
	}
	subnets = append(subnets, w.vmIpNet.String())

	return strings.Join(subnets, ", "), nil
}

func (w *Wireguard) getTunnelConf() (string, error) {
	networks, err := w.getDockerNetworks()
	if err != nil {
		return "", errors.New("failed to get docker networks: " + err.Error())
	}

	return fmt.Sprintf(TunnelConf, w.hostPrivateKey.String(), w.hostPeerIp, w.port, w.vmPrivateKey.PublicKey().String(), networks), nil
}

func (w *Wireguard) getTunnelPath() (string, error) {
	tunnelConf, err := w.getTunnelConf()
	if err != nil {
		return "", errors.New("failed to get tunnel config: " + err.Error())
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.New("failed to get current working directory: " + err.Error())
	}

	tunnelPath := filepath.Join(cwd, fmt.Sprintf("%s.conf", w.interfaceName))

	err = os.WriteFile(tunnelPath, []byte(tunnelConf), 0644)
	if err != nil {
		return "", errors.New("failed to write tunnel config: " + err.Error())
	}

	return tunnelPath, nil
}

func (w *Wireguard) installTunnel() error {
	tunnelPath, err := w.getTunnelPath()
	if err != nil {
		return errors.New("failed to get tunnel path: " + err.Error())
	}

	err = w.runCommand("wireguard", "/installtunnelservice", tunnelPath)
	if err != nil {
		return errors.New("failed to install tunnel: " + err.Error())
	}

	return nil
}

func (w *Wireguard) findInterfaceIndex() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, i := range interfaces {
		if i.Name == w.interfaceName {
			w.interfaceIndex = i.Index
			break
		}
	}

	return err
}

func (w *Wireguard) deleteWireguardRoute() error {
	err := w.runCommand("route", "DELETE", "0.0.0.0", "MASK", "0.0.0.0", "IF", strconv.Itoa(w.interfaceIndex))
	if err != nil {
		return errors.New("error deleting wireguard route " + err.Error())
	}

	return nil
}

func (w *Wireguard) uninstallTunnel() error {
	err := w.runCommand("wireguard", "/uninstalltunnelservice", w.interfaceName)
	if err != nil {
		return errors.New("failed to uninstall tunnel: " + err.Error())
	}

	return nil
}

func (w *Wireguard) updateInterface() error {
	err := w.runCommand("netsh", "interface", "ip", "set", "address", "name="+w.interfaceName, "static", w.hostPeerIp, "255.255.255.255", w.vmPeerIp)
	if err != nil {
		return errors.New("error updating interface " + err.Error())
	}

	return nil
}

func (w *Wireguard) runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	err := cmd.Run()

	if err != nil {
		return errors.New("error running command " + err.Error())
	}

	return nil
}

func (w *Wireguard) downloadSetup() error {
	_, _, err := w.docker.cli.ImageInspectWithRaw(w.docker.ctx, version.SetupImage)
	if err != nil {
		log.Printf("Setup image doesn't exist locally. Pulling...\n")

		_, err := w.docker.cli.ImagePull(w.docker.ctx, version.SetupImage, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull setup image: %w", err)
		}
	}

	return nil
}

func (w *Wireguard) SetupVM() error {
	resp, err := w.docker.cli.ContainerCreate(w.docker.ctx, &container.Config{
		Image: version.SetupImage,
		Env: []string{
			"SERVER_PORT=" + strconv.Itoa(w.port),
			"HOST_PEER_IP=" + w.hostPeerIp,
			"VM_PEER_IP=" + w.vmPeerIp,
			"HOST_PUBLIC_KEY=" + w.hostPrivateKey.PublicKey().String(),
			"VM_PRIVATE_KEY=" + w.vmPrivateKey.String(),
		},
	}, &container.HostConfig{
		AutoRemove:  true,
		NetworkMode: "host",
		CapAdd:      []string{"NET_ADMIN"},
	}, nil, nil, fmt.Sprintf("wireguard-setup-%d", time.Now().Unix()))
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	err = w.docker.cli.ContainerStart(w.docker.ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	err = func() error {
		reader, err := w.docker.cli.ContainerLogs(w.docker.ctx, resp.ID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		})
		if err != nil {
			return fmt.Errorf("failed to get logs for container %s: %w", resp.ID, err)
		}

		defer func(reader io.ReadCloser) {
			err := reader.Close()
			if err != nil {
				log.Printf("Failed to close reader: %v", err)
			}
		}(reader)

		return nil
	}()
	if err != nil {
		return err
	}

	log.Println("Setup container complete")

	return nil
}

func (w *Wireguard) Start(ctx context.Context) (stop bool) {
	msgs, errsChan := w.docker.cli.Events(w.docker.ctx, types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("type", "network"),
			filters.Arg("event", "create"),
			filters.Arg("event", "destroy"),
		),
	})

	for loop := true; loop; {
		select {
		case err := <-errsChan:
			log.Printf("Error: %v\n", err)
			loop = false
		case msg := <-msgs:
			if msg.Type == "network" && msg.Action == "create" {
				log.Printf("Network created: %s\n", msg.Actor.Attributes["name"])
				log.Printf("Restarting tunnel...\n")
				err := w.Restart()
				if err != nil {
					log.Printf("Error restarting tunnel: %v\n", err)
				}
				continue
			}

			if msg.Type == "network" && msg.Action == "destroy" {
				log.Printf("Network destroyed: %s\n", msg.Actor.Attributes["name"])
				log.Printf("Restarting tunnel...\n")
				err := w.Restart()
				if err != nil {
					log.Printf("Error restarting tunnel: %v\n", err)
				}
				continue
			}
		case <-ctx.Done():
			log.Printf("Context cancelled\n")
			loop = false

			return true
		}
	}

	return false
}

func (w *Wireguard) Restart() error {
	err := w.Teardown()
	if err != nil {
		log.Printf("Error tearing down tunnel: %v\n", err)
		return err
	}

	err = w.Setup()
	if err != nil {
		log.Printf("Error setting up tunnel: %v\n", err)
		return err
	}

	return nil
}
