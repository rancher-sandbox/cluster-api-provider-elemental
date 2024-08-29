package context

import (
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
)

type AgentContext struct {
	Identity   identity.Identity
	Plugin     osplugin.Plugin
	Client     client.Client
	Config     config.Config
	ConfigPath string
	Hostname   string
}
