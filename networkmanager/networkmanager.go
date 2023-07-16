package networkmanager

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"

	"github.com/docker/docker/api/types"
)

type NetworkManager struct {
	DockerNetworks map[string]types.NetworkResource
	interfaceIndex int
}

func New() NetworkManager {
	return NetworkManager{
		DockerNetworks: map[string]types.NetworkResource{},
	}
}

// SetInterfaceAddress Set the point-to-point IP address configuration on a network interface.
func (manager *NetworkManager) SetInterfaceAddress(ip string, peerIp string, iface string) (string, string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", "", err
	}

	for _, i := range interfaces {
		if i.Name == iface {
			manager.interfaceIndex = i.Index
			log.Printf("Interface ID: %d Name: %s", i.Index, i.Name)
			break
		}
	}

	cmd := exec.Command("netsh", "interface", "ip", "set", "address", "name="+iface, "static", ip, "255.255.255.255", peerIp)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	return stdout.String(), stderr.String(), err
}

// AddRoute Add a route to the macOS routing table.
func (manager *NetworkManager) AddRoute(netStr string) (string, string, error) {
	_, ipNet, err := net.ParseCIDR(netStr)

	cmd := exec.Command("route", "ADD", ipNet.IP.String(), "MASK", net.IP(ipNet.Mask).String(), "0.0.0.0", "IF", strconv.Itoa(manager.interfaceIndex))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	return stdout.String(), stderr.String(), err
}

// DeleteRoute Delete a route from the macOS routing table.
func (manager *NetworkManager) DeleteRoute(netStr string) (string, string, error) {
	_, ipNet, err := net.ParseCIDR(netStr)
	if err != nil {
		return "", "", err
	}

	cmd := exec.Command("route", "DELETE", ipNet.IP.String())

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	return stdout.String(), stderr.String(), err
}

// DeleteWireguardRoute Delete a route from the macOS routing table.
func (manager *NetworkManager) DeleteWireguardRoute() (string, string, error) {
	cmd := exec.Command("route", "DELETE", "0.0.0.0", "MASK", "0.0.0.0", "IF", strconv.Itoa(manager.interfaceIndex))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.String(), stderr.String(), err
}

func (manager *NetworkManager) ProcessDockerNetworkCreate(network types.NetworkResource, iface string) {
	manager.DockerNetworks[network.ID] = network

	for _, config := range network.IPAM.Config {
		if network.Scope == "local" {
			fmt.Printf("Adding route for %s -> %s (%s)\n", config.Subnet, iface, network.Name)

			stdout, stderr, err := manager.AddRoute(config.Subnet)

			log.Printf("Output: %s, Error: %s", stdout, stderr)

			if err != nil {
				_ = fmt.Errorf("Failed to add route: %v. %v\n", err, stderr)
			}
		}
	}
}

func (manager *NetworkManager) ProcessDockerNetworkDestroy(network types.NetworkResource) {
	for _, config := range network.IPAM.Config {
		if network.Scope == "local" {
			fmt.Printf("Deleting route for %s (%s)\n", config.Subnet, network.Name)

			stdout, stderr, err := manager.DeleteRoute(config.Subnet)
			log.Printf("Output: %s, Error: %s", stdout, stderr)

			if err != nil {
				_ = fmt.Errorf("Failed to delete route: %v. %v\n", err, stderr)
			}
		}
	}
	delete(manager.DockerNetworks, network.ID)
}
