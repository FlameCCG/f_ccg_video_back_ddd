package search

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const maxCoverImageBytes = 8 << 20

var errCoverUnavailable = errors.New("cover image unavailable for embedding")

func resolveCoverEmbedSource(
	ctx context.Context,
	client *http.Client,
	cover string,
	assetBaseURL string,
	localStorageRoot string,
) (string, error) {
	cover = strings.TrimSpace(cover)
	if cover == "" {
		return "", errCoverUnavailable
	}

	if strings.HasPrefix(strings.ToLower(cover), "data:") {
		return cover, nil
	}

	if parsed, err := url.Parse(cover); err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") {
		if isPubliclyReachableHost(parsed.Hostname()) {
			return cover, nil
		}

		return fetchImageAsDataURI(ctx, client, cover)
	}

	if absolute := joinAssetBaseURL(assetBaseURL, cover); absolute != "" {
		if parsed, err := url.Parse(absolute); err == nil && isPubliclyReachableHost(parsed.Hostname()) {
			return absolute, nil
		}

		return fetchImageAsDataURI(ctx, client, absolute)
	}

	return readLocalImageAsDataURI(localStorageRoot, cover)
}

func joinAssetBaseURL(baseURL, relative string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}

	return baseURL + "/" + strings.TrimLeft(
		strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/"),
		"/",
	)
}

func isPubliclyReachableHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") {
		return false
	}

	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
	}

	return true
}

func fetchImageAsDataURI(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("%w: build cover request failed: %w", errCoverUnavailable, err)
	}
	request.Header.Set("Accept", "image/*")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: fetch cover failed: %w", errCoverUnavailable, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf(
			"%w: fetch cover got status %d",
			errCoverUnavailable,
			response.StatusCode,
		)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, maxCoverImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: read cover failed: %w", errCoverUnavailable, err)
	}

	return buildImageDataURI(data, response.Header.Get("Content-Type"))
}

func readLocalImageAsDataURI(root, cover string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf(
			"%w: local storage is not enabled and assetBaseURL is not configured",
			errCoverUnavailable,
		)
	}

	path, err := safeJoinUnderRoot(root, cover)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%w: stat cover failed: %w", errCoverUnavailable, err)
	}
	if info.Size() > maxCoverImageBytes {
		return "", fmt.Errorf(
			"%w: cover is %d bytes, over the %d limit",
			errCoverUnavailable,
			info.Size(),
			maxCoverImageBytes,
		)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%w: read cover failed: %w", errCoverUnavailable, err)
	}

	return buildImageDataURI(data, "")
}

func safeJoinUnderRoot(root, relative string) (string, error) {
	cleanedRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("%w: resolve storage root failed: %w", errCoverUnavailable, err)
	}

	candidate := filepath.Clean(filepath.FromSlash(
		strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/"),
	))
	if !filepath.IsAbs(candidate) {
		rootBase := filepath.Base(cleanedRoot)
		first, rest, found := strings.Cut(filepath.ToSlash(candidate), "/")
		if found && first == rootBase {
			candidate = filepath.Join(cleanedRoot, rest)
		} else {
			candidate = filepath.Join(cleanedRoot, candidate)
		}
	}

	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("%w: resolve cover path failed: %w", errCoverUnavailable, err)
	}
	relativePath, err := filepath.Rel(cleanedRoot, absolute)
	if err != nil || relativePath == ".." ||
		strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: cover path escapes the storage root", errCoverUnavailable)
	}

	return absolute, nil
}

func buildImageDataURI(data []byte, declaredContentType string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("%w: cover is empty", errCoverUnavailable)
	}
	if len(data) > maxCoverImageBytes {
		return "", fmt.Errorf(
			"%w: cover is over the %d bytes limit",
			errCoverUnavailable,
			maxCoverImageBytes,
		)
	}

	contentType := strings.TrimSpace(strings.Split(declaredContentType, ";")[0])
	if !strings.HasPrefix(contentType, "image/") {
		contentType = strings.Split(http.DetectContentType(data), ";")[0]
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf(
			"%w: cover is not an image (%s)",
			errCoverUnavailable,
			contentType,
		)
	}

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
