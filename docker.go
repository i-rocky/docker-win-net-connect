package main

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Docker struct {
	cli *client.Client
	ctx context.Context
}

func NewDocker(ctx context.Context) (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &Docker{
		cli: cli,
		ctx: ctx,
	}, nil
}

func (d *Docker) Close() error {
	return d.cli.Close()
}

func (d *Docker) CreateContainer() error {
	return nil
}

func (d *Docker) GetSubnets() ([]string, error) {
	networks, err := d.cli.NetworkList(d.ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}

	var subnets []string
	for _, network := range networks {
		configs := network.IPAM.Config
		if len(configs) == 0 {
			continue
		}

		subnets = append(subnets, configs[0].Subnet)
	}

	return subnets, nil
}
