package main

import (
	"errors"
	"github.com/docker/docker/api/types"
	"net"
	"strconv"
)

type NetworkManager struct {
	Utils
	networks       map[string]types.NetworkResource
	interfaceIndex int
	interfaceName  string
}

func NewNetworkManager(interfaceName string) *NetworkManager {
	return &NetworkManager{
		networks:      make(map[string]types.NetworkResource),
		interfaceName: interfaceName,
	}
}

func (n *NetworkManager) findInterfaceIndex() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, i := range interfaces {
		if i.Name == n.interfaceName {
			n.interfaceIndex = i.Index
			break
		}
	}

	return err
}

func (n *NetworkManager) UpdateInterface(hostIp, vmIp string) error {
	err := n.findInterfaceIndex()
	if err != nil {
		return errors.New("error finding interface index " + err.Error())
	}

	err = n.runCommand("netsh", "interface", "ip", "set", "address", "name="+n.interfaceName, "static", hostIp, "255.255.255.255", vmIp)
	if err != nil {
		return errors.New("error updating interface " + err.Error())
	}

	return nil
}

func (n *NetworkManager) AddNetwork(id string, network types.NetworkResource) error {
	for _, config := range network.IPAM.Config {
		subnet := config.Subnet
		_, ipNet, err := net.ParseCIDR(subnet)
		if err != nil {
			return err
		}

		err = n.AddRoute(ipNet.IP.String(), net.IP(ipNet.Mask).String())
		if err != nil {
			return errors.New("error adding route " + err.Error())
		}
	}

	n.networks[id] = network

	return nil
}

func (n *NetworkManager) AddRoute(ip, mask string) error {
	err := n.runCommand("route", "ADD", ip, "MASK", mask, "0.0.0.0", "IF", strconv.Itoa(n.interfaceIndex))
	if err != nil {
		return errors.New("error deleting wireguard route " + err.Error())
	}

	return nil
}

func (n *NetworkManager) RemoveNetwork(id string) error {
	network := n.networks[id]
	for _, config := range network.IPAM.Config {
		subnet := config.Subnet
		_, ipNet, err := net.ParseCIDR(subnet)
		if err != nil {
			return err
		}
		err = n.DeleteRoute(ipNet.IP.String())
		if err != nil {
			return errors.New("error deleting route " + err.Error())
		}
	}
	delete(n.networks, id)

	return nil
}

func (n *NetworkManager) DeleteRoute(ip string) error {
	err := n.runCommand("route", "DELETE", ip, "IF", strconv.Itoa(n.interfaceIndex))
	if err != nil {
		return errors.New("error deleting wireguard route " + err.Error())
	}

	return nil
}
