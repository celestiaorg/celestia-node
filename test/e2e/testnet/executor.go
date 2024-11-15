package testnet

import (
	"context"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/celestiaorg/knuu/pkg/instance"
	"github.com/celestiaorg/knuu/pkg/knuu"
)

const (
	executorDefaultImage = "docker.io/nicolaka/netshoot:latest"
	sleepCommand         = "sleep"
	infinityArg          = "infinity"
)

type Executor struct {
	Kn *knuu.Knuu
}

var (
	executorMemoryLimit = resource.MustParse("100Mi")
	executorCpuLimit    = resource.MustParse("100m")
)

func (e *Executor) NewInstance(ctx context.Context, name string) (*instance.Instance, error) {
	i, err := e.Kn.NewInstance(name)
	if err != nil {
		return nil, err
	}

	if err := i.Build().SetImage(ctx, executorDefaultImage); err != nil {
		return nil, err
	}

	if err := i.Build().Commit(ctx); err != nil {
		return nil, err
	}

	if err := i.Build().SetArgs(sleepCommand, infinityArg); err != nil {
		return nil, err
	}

	if err := i.Resources().SetMemory(executorMemoryLimit, executorMemoryLimit); err != nil {
		return nil, err
	}

	if err := i.Resources().SetCPU(executorCpuLimit); err != nil {
		return nil, err
	}

	if err := i.Execution().Start(ctx); err != nil {
		return nil, err
	}

	return i, nil
}
