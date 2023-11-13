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
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/tls"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/twpayne/go-vfs"
)

var (
	ErrUnexpectedCode = errors.New("unexpected return code")
	ErrInvalidScheme  = errors.New("invalid scheme, use 'https' instead")
)

type Client interface {
	Init(vfs.FS, identity.Identity, config.Config) error
	GetRegistration(token string) (*api.RegistrationResponse, error)
	CreateHost(newHost api.HostCreateRequest, registrationToken string) error
	DeleteHost(hostname string) error
	PatchHost(patch api.HostPatchRequest, hostname string) (*api.HostResponse, error)
	GetBootstrap(hostname string) (*api.BootstrapResponse, error)
}

var _ Client = (*client)(nil)

type client struct {
	userAgent       string
	registrationURI string
	httpClient      http.Client
	identity        identity.Identity
}

func NewClient(version string) Client {
	userAgent := fmt.Sprintf("elemental-agent/%s", version)
	return &client{
		userAgent: userAgent,
	}
}

func (c *client) Init(fs vfs.FS, identity identity.Identity, conf config.Config) error {
	log.Debug("Initializing Client")
	c.identity = identity

	url, err := url.Parse(conf.Registration.URI)
	if err != nil {
		return fmt.Errorf("parsing registration URI: %w", err)
	}

	scheme := strings.ToLower(url.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unknown scheme '%s': %w", url.Scheme, ErrInvalidScheme)
	}

	if !conf.Agent.InsecureAllowHTTP && scheme != "https" {
		return fmt.Errorf("using '%s' scheme: %w", url.Scheme, ErrInvalidScheme)
	}

	caCert, err := tls.GetCACert(fs, conf.Registration.CACert)
	if err != nil {
		return fmt.Errorf("reading CA Cert from configuration: %w", err)
	}

	tlsConfig, err := tls.GetTLSClientConfig(caCert, conf.Agent.UseSystemCertPool, conf.Agent.InsecureSkipTLSVerify)
	if err != nil {
		return fmt.Errorf("configuring TLS client: %w", err)
	}

	c.registrationURI = conf.Registration.URI
	c.httpClient = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return nil
}

func (c *client) GetRegistration(registrationToken string) (*api.RegistrationResponse, error) {
	log.Debugf("Getting registration: %s", c.registrationURI)
	request, err := c.newRequest(http.MethodGet, c.registrationURI, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing GET registration request: %w", err)
	}
	c.addRegistrationHeader(&request.Header, registrationToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("getting registration: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getting registration returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading registration response body: %w", err)
	}

	registration := api.RegistrationResponse{}
	if err := json.Unmarshal(responseBody, &registration); err != nil {
		return nil, fmt.Errorf("unmarshalling registration response: %w", err)
	}

	return &registration, nil
}

func (c *client) CreateHost(newHost api.HostCreateRequest, registrationToken string) error {
	log.Debugf("Creating new host: %s", newHost.Name)
	requestBody, err := json.Marshal(newHost)
	if err != nil {
		return fmt.Errorf("marshalling new host request body: %w", err)
	}

	url := fmt.Sprintf("%s/hosts", c.registrationURI)
	request, err := c.newAuthenticatedRequest(newHost.Name, http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("preparing POST host request: %w", err)
	}
	request.Header.Add("Content-Type", "application/json")
	c.addRegistrationHeader(&request.Header, registrationToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("creating new host: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating new host returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	return nil
}

func (c *client) DeleteHost(hostname string) error {
	log.Debugf("Marking host for deletion: %s", hostname)
	url := fmt.Sprintf("%s/hosts/%s", c.registrationURI, hostname)
	request, err := c.newAuthenticatedRequest(hostname, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("preparing DELETE host request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("deleting host: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("deleting host returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}
	return nil
}

func (c *client) PatchHost(patch api.HostPatchRequest, hostname string) (*api.HostResponse, error) {
	log.Debugf("Patching Host '%s': %+v", hostname, patch)
	requestBody, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshalling patch host request body: %w", err)
	}

	url := fmt.Sprintf("%s/hosts/%s", c.registrationURI, hostname)
	request, err := c.newAuthenticatedRequest(hostname, http.MethodPatch, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("preparing PATCH host request: %w", err)
	}
	request.Header.Add("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("patching host: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("patching host returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading host response body: %w", err)
	}

	host := api.HostResponse{}
	if err := json.Unmarshal(responseBody, &host); err != nil {
		return nil, fmt.Errorf("unmarshalling host response: %w", err)
	}

	return &host, nil
}

func (c *client) GetBootstrap(hostname string) (*api.BootstrapResponse, error) {
	log.Debugf("Getting bootstrap for host: %s", hostname)
	url := fmt.Sprintf("%s/hosts/%s/bootstrap", c.registrationURI, hostname)
	request, err := c.newAuthenticatedRequest(hostname, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing get bootstrap request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("getting bootstrap: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getting bootstrap returned code '%d': %w", response.StatusCode, ErrUnexpectedCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading bootstrap response body: %w", err)
	}

	bootstrap := api.BootstrapResponse{}
	if err := json.Unmarshal(responseBody, &bootstrap); err != nil {
		return nil, fmt.Errorf("unmarshalling bootstrap response: %w", err)
	}

	return &bootstrap, nil
}

func (c *client) newRequest(method string, url string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}
	c.addUserAgentHeader(&request.Header)
	return request, nil
}

func (c *client) newAuthenticatedRequest(forHostname string, method string, url string, body io.Reader) (*http.Request, error) {
	request, err := c.newRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating new request: %w", err)
	}
	if err := c.addAuthHeader(&request.Header, forHostname); err != nil {
		return nil, fmt.Errorf("setting Authorization header: %w", err)
	}
	return request, nil
}

func (c *client) addRegistrationHeader(header *http.Header, registrationToken string) {
	header.Add("Registration-Authorization", fmt.Sprintf("Bearer %s", registrationToken))
}

func (c *client) addUserAgentHeader(header *http.Header) {
	header.Add("User-Agent", c.userAgent)
}

func (c *client) addAuthHeader(header *http.Header, hostname string) error {
	token, err := c.newToken(hostname)
	if err != nil {
		return fmt.Errorf("generating new token: %w", err)
	}
	header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return nil
}

func (c *client) newToken(hostname string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Minute)),
		Issuer:    c.userAgent,
		Subject:   hostname,
		Audience:  []string{c.registrationURI},
	}
	token, err := c.identity.Sign(claims)
	if err != nil {
		return "", fmt.Errorf("signing JWT claims: %w", err)
	}
	return token, nil
}
