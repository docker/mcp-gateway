package catalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/tui"
)

func isValidURL(u string) bool {
	parsedURL, err := url.ParseRequestURI(u)
	if err != nil {
		return false
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return false
	}
	return true
}

func DownloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %q (status code: %d)", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func Import(ctx context.Context, nameOrURL string) error {
	// Accept urls or catalog names.
	url := nameOrURL
	if urlFromAlias, ok := aliasToURL[nameOrURL]; ok {
		url = urlFromAlias
	}

	var (
		catalogContent []byte
		err            error
	)
	if isValidURL(url) {
		catalogContent, err = DownloadFile(ctx, url)
	} else {
		url, err = filepath.Abs(url)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		catalogContent, err = os.ReadFile(url)
	}
	if err != nil {
		return err
	}

	metaData, err := readCatalogMetaData(catalogContent)
	if err != nil {
		return fmt.Errorf("failed to read catalog meta data: %w", err)
	}

	if metaData.Name == "" {
		userMeta, err := askUserForMetaData()
		if err != nil {
			return err
		}
		metaData = userMeta
		data, err := setCatalogMetaData(catalogContent, *metaData)
		if err != nil {
			return fmt.Errorf("failed to set catalog meta data: %w", err)
		}
		catalogContent = data
	}
	if metaData.DisplayName == "" {
		metaData.DisplayName = metaData.Name
	}

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}

	cfg.Catalogs[metaData.Name] = Catalog{
		DisplayName: metaData.DisplayName,
		URL:         url,
	}
	if err := WriteConfig(cfg); err != nil {
		return err
	}
	return WriteCatalogFile(metaData.Name, catalogContent)
}

func askUserForMetaData() (*MetaData, error) {
	name, err := tui.ReadUserInput("Please provide a name for the catalog: ")
	if err != nil {
		return nil, fmt.Errorf("failed to read user input: %w", err)
	}
	return &MetaData{
		Name:        name,
		DisplayName: name,
	}, nil
}
