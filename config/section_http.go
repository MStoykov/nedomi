package config

import "encoding/json"

// BaseHTTP contains the basic configuration options for HTTP.
type BaseHTTP struct {
	Listen         string            `json:"listen"`
	Servers        []json.RawMessage `json:"virtual_hosts"`
	MaxHeadersSize int               `json:"max_headers_size"`
	ReadTimeout    uint32            `json:"read_timeout"`
	WriteTimeout   uint32            `json:"write_timeout"`

	// Defaults for vhosts:
	DefaultHandlerType  string        `json:"default_handler"`
	DefaultUpstreamType string        `json:"default_upstream_type"`
	DefaultCacheZone    string        `json:"default_cache_zone"`
	Logger              LoggerSection `json:"logger"`
}

// HTTP contains all configuration options for HTTP.
type HTTP struct {
	BaseHTTP
	Servers []*VirtualHost `json:"virtual_hosts"`
	parent  *Config
}

// UnmarshalJSON is a custom JSON unmashalling that also implements inheritance,
// custom field initiation and data validation for the HTTP config.
func (h *HTTP) UnmarshalJSON(buff []byte) error {
	if err := json.Unmarshal(buff, &h.BaseHTTP); err != nil {
		return err
	}

	// Inherit HTTP values to vhosts
	baseVhost := VirtualHost{parent: h, BaseVirtualHost: BaseVirtualHost{
		HandlerType:  h.DefaultHandlerType,
		UpstreamType: h.DefaultUpstreamType,
		CacheZone:    h.DefaultCacheZone,
		Logger:       &h.Logger,
	}}

	// Parse all the vhosts
	for _, vhostBuff := range h.BaseHTTP.Servers {
		vhost := baseVhost
		if err := json.Unmarshal(vhostBuff, &vhost); err != nil {
			return err
		}
		h.Servers = append(h.Servers, &vhost)
	}

	h.BaseHTTP.Servers = nil // Cleanup
	return h.Validate()
}

// Validate checks the HTTP config for logical errors.
func (h *HTTP) Validate() error {
	//!TODO: implement
	return nil
}
