package snowflake

import (
	"errors"
	"fmt"
	"sync"

	bwmsnowflake "github.com/bwmarrin/snowflake"
)

var ErrBootstrapAlreadyInitialized = errors.New("snowflake bootstrap owner already initialized")

type Generator interface {
	NextID() int64
}

type generator struct {
	node *bwmsnowflake.Node
}

func NewGenerator(nodeID int64) (Generator, error) {
	node, err := bwmsnowflake.NewNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("create snowflake node %d: %w", nodeID, err)
	}
	return &generator{node: node}, nil
}

func (g *generator) NextID() int64 {
	return g.node.Generate().Int64()
}

type BootstrapOwner struct {
	mu          sync.Mutex
	initialized bool
}

func NewBootstrapOwner() *BootstrapOwner {
	return &BootstrapOwner{}
}

func (o *BootstrapOwner) Init(nodeID int64) (Generator, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.initialized {
		return nil, ErrBootstrapAlreadyInitialized
	}

	generator, err := NewGenerator(nodeID)
	if err != nil {
		return nil, err
	}
	o.initialized = true
	return generator, nil
}
