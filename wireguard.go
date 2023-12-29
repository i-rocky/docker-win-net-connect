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
	docker            *Docker
	interfaceName     string
	interfaceIndex    int
	hostPeerIp        string
	vmPeerIp          string
	hostPrivateKey    *wgtypes.Key
	vmPrivateKey      *wgtypes.Key
	vmIpNet           *net.IPNet
	port              int
	networkManager    *NetworkManager
	exePath           string
	binDirWg          string
	exBinDirWg        string
	binDirWireguard   string
	exBinDirWireguard string
	Utils
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

	// get the directory where the executable is located
	exe, err := os.Executable()
	if err != nil {
		return nil, errors.New("failed to get executable path: " + err.Error())
	}

	exePath := filepath.Dir(exe)

	return &Wireguard{
		docker:          docker,
		interfaceName:   opts.InterfaceName,
		hostPrivateKey:  &hostPrivateKey,
		vmPrivateKey:    &vmPrivateKey,
		hostPeerIp:      opts.HostPeerIp,
		vmPeerIp:        opts.VmPeerIp,
		vmIpNet:         vmIpNet,
		port:            opts.Port,
		networkManager:  NewNetworkManager(opts.InterfaceName),
		exePath:         exePath,
		binDirWg:        "bin/wg.exe",
		binDirWireguard: "bin/wireguard.exe",
	}, nil
}

func (w *Wireguard) Setup() error {
	_ = elog.Info(36, "Extracting binaries")
	err := w.extractBinaries()
	if err != nil {
		return errors.New("failed to extract binaries: " + err.Error())
	}

	_ = elog.Info(37, "Downloading setup image")
	err = w.downloadSetup()
	if err != nil {
		return errors.New("failed to download setup: " + err.Error())
	}

	_ = elog.Info(38, "Installing tunnel")
	err = w.installTunnel(true)
	if err != nil {
		return errors.New("failed to install tunnel: " + err.Error())
	}

	time.Sleep(1 * time.Second)

	_ = elog.Info(39, "Updating interface")
	err = w.networkManager.UpdateInterface(w.hostPeerIp, w.vmPeerIp)
	if err != nil {
		return errors.New("failed to update interface: " + err.Error())
	}

	_ = elog.Info(41, "Deleting wireguard route")
	err = w.networkManager.DeleteRoute("0.0.0.0")
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

	w.exBinDirWg = filepath.Join(w.exePath, w.binDirWg)
	w.exBinDirWireguard = filepath.Join(w.exePath, w.binDirWireguard)

	_, err := os.Stat(w.exBinDirWg)
	if err == nil {
		files++
	}

	_, err = os.Stat(w.exBinDirWireguard)
	if err == nil {
		files++
	}

	if files == 2 {
		return nil
	}

	err = os.MkdirAll(filepath.Dir(w.binDirWg), 0755)
	if err != nil {
		return errors.New("failed to create bin directory: " + err.Error())
	}

	wg, err := binaries.ReadFile(w.binDirWg)
	if err != nil {
		return errors.New("failed to open wg.exe: " + err.Error())
	}

	err = os.WriteFile(w.exBinDirWg, wg, 0755)
	if err != nil {
		return errors.New("failed to write wg.exe: " + err.Error())
	}

	wireguard, err := binaries.ReadFile(w.binDirWireguard)
	if err != nil {
		return errors.New("failed to open wireguard.exe: " + err.Error())
	}

	err = os.WriteFile(w.exBinDirWireguard, wireguard, 0755)
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

	tunnelPath := filepath.Join(w.exePath, fmt.Sprintf("%s.conf", w.interfaceName))

	err = os.WriteFile(tunnelPath, []byte(tunnelConf), 0644)
	if err != nil {
		return "", errors.New("failed to write tunnel config: " + err.Error())
	}

	return tunnelPath, nil
}

func (w *Wireguard) installTunnel(first bool) error {
	tunnelPath, err := w.getTunnelPath()
	if err != nil {
		return errors.New("failed to get tunnel path: " + err.Error())
	}

	err = w.runCommand(w.exBinDirWireguard, "/installtunnelservice", tunnelPath)
	if err != nil {
		if first && strings.Contains(err.Error(), "Tunnel already installed and running") {
			if err := w.uninstallTunnel(); err != nil {
				return errors.New("tunnel running, failed to uninstall tunnel: " + err.Error())
			}

			if err := w.installTunnel(false); err != nil {
				return errors.New("tunnel was running, stopped but failed to install tunnel: " + err.Error())
			}
		}
		return errors.New("failed to install tunnel: " + err.Error())
	}

	return nil
}

func (w *Wireguard) uninstallTunnel() error {
	err := w.runCommand(w.exBinDirWireguard, "/uninstalltunnelservice", w.interfaceName)
	if err != nil {
		return errors.New("failed to uninstall tunnel: " + err.Error())
	}

	return nil
}

func (w *Wireguard) downloadSetup() error {
	err := w.docker.WaitRunning()
	if err != nil {
		return err
	}

	_, _, err = w.docker.cli.ImageInspectWithRaw(w.docker.ctx, version.SetupImage)
	if err != nil {
		_ = elog.Info(17, "Setup image doesn't exist locally. Pulling...\n")

		_, err := w.docker.cli.ImagePull(w.docker.ctx, version.SetupImage, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull setup image: %w", err)
		}
	}

	return nil
}

func (w *Wireguard) SetupVM() error {
	err := w.docker.WaitRunning()
	if err != nil {
		return err
	}

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
				_ = elog.Info(18, fmt.Sprintf("Failed to close reader: %v", err))
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
			_ = elog.Info(19, fmt.Sprintf("Error: %v\n", err))
			loop = false
		case msg := <-msgs:
			if msg.Type == "network" && msg.Action == "create" {
				_ = elog.Info(20, fmt.Sprintf("Network created: %s\n", msg.Actor.Attributes["name"]))
				network, err := w.docker.cli.NetworkInspect(ctx, msg.Actor.ID, types.NetworkInspectOptions{})
				if err != nil {
					_ = elog.Error(21, fmt.Sprintf("Failed to inspect new Docker network: %v", err))
					continue
				}
				err = w.networkManager.AddNetwork(network.ID, network)
				if err != nil {
					_ = elog.Info(1, fmt.Sprintf("Error restarting tunnel: %v\n", err))
				}
				continue
			}

			if msg.Type == "network" && msg.Action == "destroy" {
				_ = elog.Info(22, fmt.Sprintf("Network destroyed: %s\n", msg.Actor.Attributes["name"]))
				err := w.networkManager.RemoveNetwork(msg.Actor.ID)
				if err != nil {
					_ = elog.Info(24, fmt.Sprintf("Error restarting tunnel: %v\n", err))
				}
				continue
			}
		case <-ctx.Done():
			_ = elog.Info(25, "Context cancelled\n")
			loop = false

			return true
		}
	}

	return false
}
