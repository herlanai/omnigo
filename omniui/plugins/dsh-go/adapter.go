package dshgo

import (
	"context"
	"net/http"

	"github.com/herlanai/omnix/pkg/brand"
	"github.com/herlanai/omnix/pkg/credentials"
	"github.com/herlanai/omnix/pkg/llm/provider_deepseek"
)

// DeepSeekOptions configures omnix's provider, not a separate Chat implementation.
type DeepSeekOptions struct {
	APIKey, BaseURL string
	Client          *http.Client
}

// NewDeepSeek delegates transport, streaming and model protocol handling to omnix.
// Credentials stay in an in-memory omnix store and are not added to session logs.
func NewDeepSeek(o DeepSeekOptions) (*provider_deepseek.DeepSeek, error) {
	// Reuse endpoint validation and redirect protection from the legacy client.
	validated, err := NewClient(ClientOptions{APIKey: o.APIKey, BaseURL: o.BaseURL, Client: o.Client})
	if err != nil {
		return nil, err
	}
	store := credentials.NewMemoryStore()
	if err = store.Set(context.Background(), brand.NewCredentialRef("DEEPSEEK_API_KEY"), o.APIKey); err != nil {
		return nil, err
	}
	options := []provider_deepseek.Option{}
	if o.BaseURL != "" {
		options = append(options, provider_deepseek.WithBaseURL(o.BaseURL))
	}
	if o.Client != nil {
		options = append(options, provider_deepseek.WithHTTPClient(validated.http))
	} else {
		// Use a private connection pool with omnix pool sizes and forbid redirects.
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			options = append(options, provider_deepseek.WithHTTPClient(validated.http))
			return provider_deepseek.NewDeepSeek(store, options...), nil
		}
		transport := base.Clone()
		transport.MaxIdleConns = provider_deepseek.DefaultMaxIdleConns
		transport.MaxIdleConnsPerHost = provider_deepseek.DefaultMaxIdleConnsPerHost
		validated.http.Transport = transport
		options = append(options, provider_deepseek.WithHTTPClient(validated.http))
	}
	return provider_deepseek.NewDeepSeek(store, options...), nil
}
