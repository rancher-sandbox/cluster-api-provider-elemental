package agent

import (
	"fmt"
	"os"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs/v4"
)

const (
	configPathDefault = "/etc/elemental/agent/config.yaml"
)

var (
	debugFlag bool
	cfgFile   string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "elemental-agent",
	Short: "elemental-agent interfaces to the CAPI Elemental provider",
	Long: `elemental-agent takes care of the entire lifecycle of an Elemental host, 
first boot registration, installation, CAPI bootstrapping, upgrades, and reset.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// InitAgent initializes the AgentContext with all needed components for normal operation.
func InitAgent() (*context.AgentContext, error) {
	fs := vfs.OSFS
	// Reads and parse the config file
	conf := initConfig()
	// Initialize WorkDir
	if err := utils.CreateDirectory(fs, conf.Agent.WorkDir); err != nil {
		return nil, fmt.Errorf("creating work directory '%s': %w", conf.Agent.WorkDir, err)
	}
	// Initialize Plugin
	pluginLoader := osplugin.NewLoader()
	log.Infof("Loading Plugin: %s", conf.Agent.OSPlugin)
	osPlugin, err := pluginLoader.Load(conf.Agent.OSPlugin)
	if err != nil {
		return nil, fmt.Errorf("loading plugin '%s': %w", conf.Agent.OSPlugin, err)
	}
	log.Info("Initializing Plugin")
	if err := osPlugin.Init(osplugin.PluginContext{
		WorkDir:    conf.Agent.WorkDir,
		ConfigPath: cfgFile,
		Debug:      conf.Agent.Debug || debugFlag,
	}); err != nil {
		return nil, fmt.Errorf("initializing plugin: %w", err)
	}
	// Initialize Identity
	identityManager := identity.NewManager(fs, conf.Agent.WorkDir)
	identity, err := identityManager.LoadSigningKeyOrCreateNew()
	if err != nil {
		return nil, fmt.Errorf("initializing identity: %w", err)
	}
	// Initialize Elemental API Client
	client := client.NewClient(version.Version)
	if err := client.Init(fs, identity, conf); err != nil {
		return nil, fmt.Errorf("initializing Elemental API client: %w", err)
	}
	// Get current hostname
	hostname, err := osPlugin.GetHostname()
	if err != nil {
		return nil, fmt.Errorf("getting current hostname: %w", err)
	}

	return &context.AgentContext{
		Identity:   identity,
		Plugin:     osPlugin,
		Client:     client,
		Config:     conf,
		ConfigPath: cfgFile,
		Hostname:   hostname,
	}, nil

}

func initConfig() config.Config {
	conf := config.DefaultConfig()

	viper.SetConfigFile(cfgFile)
	if err := viper.ReadInConfig(); err != nil {
		cobra.CheckErr(fmt.Errorf("reading config: %w", err))
	}
	log.Infof("Using config file: %s", viper.ConfigFileUsed())
	if err := viper.Unmarshal(&conf); err != nil {
		cobra.CheckErr(fmt.Errorf("unmashalling config: %w", err))
	}
	if debugFlag || conf.Agent.Debug {
		log.EnableDebug()
	}

	return conf
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", configPathDefault, "Config file (default is /etc/elemental/agent/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enables debug logging")
}
