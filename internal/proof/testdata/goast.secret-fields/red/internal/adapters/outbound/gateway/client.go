package gateway

// Client keeps its credential in a bare string.
type Client struct {
	APIKey  string
	BaseURL string
}
