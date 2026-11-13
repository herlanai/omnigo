package dshgo

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/herlanai/omnix/pkg/brand"
	"github.com/herlanai/omnix/pkg/credentials"
	"github.com/herlanai/omnix/pkg/imagegen/provider_ark"
	"github.com/herlanai/omnix/pkg/tools"
)

// ImageOptions explicitly enables omnix's Ark image tool. Empty APIKey and Model
// read ARK_API_KEY and ARK_IMAGE_MODEL at construction. Credentials never enter
// tool arguments or UI state. Every call uses the existing Approve policy.
type ImageOptions struct {
	APIKey, Model, Endpoint string
	Client                  *http.Client
}

// GeneratedImage is verified provider output, not a model-authored Markdown URL.
// URLs may expire. Applications own opening, downloading and storing images.
type GeneratedImage struct{ URL, Size string }

func newImageTool(o ImageOptions) (*tools.Tool, error) {
	key := strings.TrimSpace(o.APIKey)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("ARK_API_KEY"))
	}
	if key == "" {
		return nil, errors.New("dshgo: image generation requires ARK_API_KEY or ImageOptions.APIKey")
	}
	endpoint := o.Endpoint
	if endpoint == "" {
		endpoint = provider_ark.DefaultEndpoint
	}
	// Share the host endpoint and redirect policy; provider_ark owns the actual API.
	validated, err := NewClient(ClientOptions{BaseURL: endpoint, Client: o.Client})
	if err != nil {
		return nil, err
	}
	if o.Client == nil {
		validated.http.Timeout = 3 * time.Minute
	}
	store := credentials.NewMemoryStore()
	if err = store.Set(context.Background(), brand.NewCredentialRef("ARK_API_KEY"), key); err != nil {
		return nil, err
	}
	options := []provider_ark.Option{provider_ark.WithEndpoint(endpoint), provider_ark.WithHTTPClient(validated.http)}
	model := strings.TrimSpace(o.Model)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("ARK_IMAGE_MODEL"))
	}
	if model != "" {
		options = append(options, provider_ark.WithModel(model))
	}
	return provider_ark.New(store, options...).Tool(), nil
}

const imageInstructions = "When image creation is requested, use ark_generate_image. Preserve requested text, style and composition. Use transparent background only when requested, with PNG. Show only image URLs actually returned by the tool. Do not claim success on errors or repeat a failed generation unchanged."
