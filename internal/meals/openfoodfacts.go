package meals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OFFClient calls the Open Food Facts API to look up products by barcode.
type OFFClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOFFClient creates an OFFClient pointing at the official Open Food Facts API.
func NewOFFClient() *OFFClient {
	return &OFFClient{
		baseURL:    "https://world.openfoodfacts.org",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// newOFFClientWithBase creates an OFFClient with a custom base URL and HTTP client.
// Used in tests only.
func newOFFClientWithBase(baseURL string, c *http.Client) *OFFClient {
	return &OFFClient{baseURL: baseURL, httpClient: c}
}

type offResponse struct {
	Status  int        `json:"status"`
	Product offProduct `json:"product"`
}

type offProduct struct {
	Name            string `json:"product_name"`
	IngredientsText string `json:"ingredients_text"`
}

// splitIngredients parses a comma-separated ingredients_text into a trimmed slice.
// Empty text returns nil.
func splitIngredients(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Fetch looks up a product by barcode.
// Returns ErrProductNotFound if the barcode is unknown.
func (c *OFFClient) Fetch(ctx context.Context, barcode string) (Product, error) {
	url := c.baseURL + "/api/v2/product/" + barcode + ".json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Product{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Product{}, fmt.Errorf("fetch product %s: %w", barcode, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Product{}, fmt.Errorf("Open Food Facts returned HTTP %d for barcode %s", resp.StatusCode, barcode)
	}

	var result offResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Product{}, fmt.Errorf("decode response: %w", err)
	}

	if result.Status == 0 {
		return Product{}, ErrProductNotFound
	}

	ingredients := splitIngredients(result.Product.IngredientsText)
	return Product{Name: result.Product.Name, Ingredients: ingredients}, nil
}
