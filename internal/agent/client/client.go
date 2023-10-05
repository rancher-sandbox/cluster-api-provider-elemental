package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/tls"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
)

var (
	ErrUnexpectedCode = errors.New("unexpected return code")
	ErrInvalidScheme  = errors.New("invalid scheme, use 'https' instead")
)

type Client interface {
	GetRegistration() (api.RegistrationResponse, error)
	CreateMachineHost(host api.HostCreateRequest) error
	PatchMachineHost(patch api.HostPatchRequest, hostname string) (api.HostResponse, error)
	GetBootstrap(hostname string) (api.BootstrapResponse, error)
}

var _ Client = (*client)(nil)

type client struct {
	registrationURI string
	httpClient      http.Client
	identity        host.Identity
}

func NewClient(fs vfs.FS, conf config.Config) (Client, error) {
	url, err := url.Parse(conf.Registration.URI)
	if err != nil {
		return nil, fmt.Errorf("parsing registration URI: %w", err)
	}

	if conf.Agent.InsecureAllowHTTP && strings.ToLower(url.Scheme) != "https" {
		return nil, fmt.Errorf("using '%s' scheme: %w", url.Scheme, ErrInvalidScheme)
	}

	caCert, err := tls.GetCACert(fs, conf.Registration.CACert)
	if err != nil {
		return nil, fmt.Errorf("reading CA Cert from configuration: %w", err)
	}

	tlsConfig, err := tls.GetTLSClientConfig(caCert, conf.Agent.UseSystemCertPool, conf.Agent.InsecureSkipTLSVerify)
	if err != nil {
		return nil, fmt.Errorf("configuring TLS client: %w", err)
	}

	// Initialize Identity
	identityManager := host.NewDummyManager(fs, conf.Agent.WorkDir)
	identity, err := identityManager.GetOrCreateIdentity()
	if err != nil {
		return nil, fmt.Errorf("initializing host identity: %w", err)
	}

	return &client{
		registrationURI: conf.Registration.URI,
		httpClient: http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
		identity: identity,
	}, nil
}

func (c *client) GetRegistration() (api.RegistrationResponse, error) {
	response, err := c.httpClient.Get(c.registrationURI)
	if err != nil {
		return api.RegistrationResponse{}, fmt.Errorf("getting registration: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return api.RegistrationResponse{}, fmt.Errorf("getting registration returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return api.RegistrationResponse{}, fmt.Errorf("reading registration response body: %w", err)
	}

	registration := api.RegistrationResponse{}
	if err := json.Unmarshal(responseBody, &registration); err != nil {
		return api.RegistrationResponse{}, fmt.Errorf("unmarshalling registration response: %w", err)
	}

	return registration, nil
}

func (c *client) CreateMachineHost(newHost api.HostCreateRequest) error {
	requestBody, err := json.Marshal(newHost)
	if err != nil {
		return fmt.Errorf("marshalling new host request body: %w", err)
	}

	response, err := c.httpClient.Post(fmt.Sprintf("%s/hosts", c.registrationURI), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("creating new host: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating new host returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	return nil
}

func (c *client) PatchMachineHost(patch api.HostPatchRequest, hostname string) (api.HostResponse, error) {
	requestBody, err := json.Marshal(patch)
	if err != nil {
		return api.HostResponse{}, fmt.Errorf("marshalling patch host request body: %w", err)
	}

	request, err := http.NewRequest("PATCH", fmt.Sprintf("%s/hosts/%s", c.registrationURI, hostname), bytes.NewBuffer(requestBody))
	if err != nil {
		return api.HostResponse{}, fmt.Errorf("preparing host patch request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return api.HostResponse{}, fmt.Errorf("patching host: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return api.HostResponse{}, fmt.Errorf("patching host returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return api.HostResponse{}, fmt.Errorf("reading host response body: %w", err)
	}

	host := api.HostResponse{}
	if err := json.Unmarshal(responseBody, &host); err != nil {
		return api.HostResponse{}, fmt.Errorf("unmarshalling host response: %w", err)
	}

	return host, nil
}

func (c *client) GetBootstrap(hostname string) (api.BootstrapResponse, error) {
	response, err := c.httpClient.Get(fmt.Sprintf("%s/hosts/%s/bootstrap", c.registrationURI, hostname))
	if err != nil {
		return api.BootstrapResponse{}, fmt.Errorf("getting bootstrap: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return api.BootstrapResponse{}, fmt.Errorf("getting bootstrap returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return api.BootstrapResponse{}, fmt.Errorf("reading bootstrap response body: %w", err)
	}

	bootstrap := api.BootstrapResponse{}
	if err := json.Unmarshal(responseBody, &bootstrap); err != nil {
		return api.BootstrapResponse{}, fmt.Errorf("unmarshalling bootstrap response: %w", err)
	}

	return bootstrap, nil
}
