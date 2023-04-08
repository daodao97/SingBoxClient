package option

type ClashAPIOptions struct {
	ExternalController string `json:"external_controller,omitempty"`
	ExternalUI         string `json:"external_ui,omitempty"`
	Secret             string `json:"secret,omitempty"`
	DefaultMode        string `json:"default_mode,omitempty"`
	StoreSelected      bool   `json:"store_selected,omitempty"`
	StoreFakeIP        bool   `json:"store_fakeip,omitempty"`
	CacheFile          string `json:"cache_file,omitempty"`
}

type SelectorOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Default   string   `json:"default,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	URL       string   `json:"url,omitempty"`
	Interval  Duration `json:"interval,omitempty"`
	Tolerance uint16   `json:"tolerance,omitempty"`
}

type ProviderOutboundOptions struct {
	ProviderType    string           `json:"provider_type"`
	Url             Listable[string] `json:"url,omitempty"`
	Path            Listable[string] `json:"path,omitempty"`
	Default         string           `json:"default,omitempty"`
	Interval        string           `json:"interval"`
	Policy          string           `json:"policy"`
	UrlTest         *UrlTest         `json:"url_test"`
	IncludeKeyWords Listable[string] `json:"include_key_words"`
	ExcludeKeyWords Listable[string] `json:"exclude_key_words"`
}

type UrlTest struct {
	Url       string `json:"url,omitempty"`
	Interval  string `json:"interval"`
	Tolerance int    `json:"tolerance"`
}
