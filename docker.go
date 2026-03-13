package main

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// newDockerClient returns a Docker client configured from the environment.
// It handles Unix sockets (Linux/macOS) and Windows named pipes automatically,
// and negotiates the API version with the daemon.
func newDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
}

// listContainers returns the IDs of all running containers.
// When all is true, stopped containers are included as well.
func listContainers(ctx context.Context, cli *client.Client, all bool) ([]string, error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}
	return ids, nil
}

// inspectContainers inspects each container ID concurrently and returns the
// full inspect response for each. Returns on the first error encountered.
func inspectContainers(ctx context.Context, cli *client.Client, ids []string) ([]container.InspectResponse, error) {
	results := make([]container.InspectResponse, len(ids))
	errs := make([]error, len(ids))

	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = cli.ContainerInspect(ctx, id)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}
