package option

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
