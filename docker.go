package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"strings"
	"time"
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

func (d *Docker) WaitRunning() error {
	var counter time.Duration = 0
	var waitTime time.Duration = 0
	timer := time.NewTimer(waitTime)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			_, err := d.cli.Info(context.Background())
			if err == nil {
				return nil
			}

			if strings.Contains(err.Error(), "pipe") && strings.Contains(err.Error(), "docker_engine") {
				waitTime = time.Second * counter * 20
				if counter > 30 {
					waitTime = 600
				}
				_ = elog.Info(1, fmt.Sprintf("Docker not running. Checking again in %f seconds...", waitTime.Seconds()))
				counter++
				timer.Reset(waitTime)
				continue
			}

			return errors.New("something went wrong")
		case <-d.ctx.Done():
			return errors.New("context cancelled")
		}
	}
}
