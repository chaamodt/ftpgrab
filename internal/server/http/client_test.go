package http

import (
	"bytes"
	"net"
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cavaliergopher/grab/v3"
	"github.com/crazy-max/ftpgrab/v7/internal/config"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler stdhttp.Handler) (*Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	tls := false
	httpClient := grab.NewClient()
	httpClient.HTTPClient = server.Client()

	return &Client{
		config: &config.ServerHTTP{
			Host:      host,
			Port:      port,
			TLS:       &tls,
			UserAgent: "Wget",
		},
		http: httpClient,
	}, server
}

func TestReadDirUsesHeadWhenMetadataIsUsable(t *testing.T) {
	requests := 0
	client, server := newTestClient(t, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		requests++
		require.Equal(t, stdhttp.MethodHead, r.Method)
		require.Equal(t, "Wget", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Length", "123")
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	entries, err := client.ReadDir("/file.zip")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, 123, entries[0].Size())
	require.Equal(t, 1, requests)
}

func TestReadDirAcceptsUnknownSizeWithLastModified(t *testing.T) {
	client := &Client{}
	resp := &stdhttp.Response{
		StatusCode:    stdhttp.StatusOK,
		ContentLength: -1,
		Header: stdhttp.Header{
			"Content-Type":  {"application/zip"},
			"Last-Modified": {"Sun, 01 Nov 2026 05:00:21 GMT"},
		},
	}

	require.False(t, needsGetMetadata("/file.zip", resp))
	entries, err := client.fileInfoFromResponse("/file.zip", resp, resp.ContentLength)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, -1, entries[0].Size())
}

func TestReadDirUsesRangedGetWhenHeadReturnsHTML(t *testing.T) {
	var methods []string
	client, server := newTestClient(t, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		methods = append(methods, r.Method)
		if r.Method == stdhttp.MethodHead {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Length", "244")
			w.WriteHeader(stdhttp.StatusOK)
			return
		}

		require.Equal(t, "bytes=0-0", r.Header.Get("Range"))
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Range", "bytes 0-0/123456789")
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(stdhttp.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()

	entries, err := client.ReadDir("/file.zip")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, 123456789, entries[0].Size())
	require.Equal(t, []string{stdhttp.MethodHead, stdhttp.MethodGet}, methods)
}

func TestReadDirFallsBackToRegularGetWhenRangeIsIgnoredWithHTML(t *testing.T) {
	var methods []string
	client, server := newTestClient(t, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		methods = append(methods, r.Method)
		switch {
		case r.Method == stdhttp.MethodHead:
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Length", "244")
			w.WriteHeader(stdhttp.StatusOK)
		case r.Header.Get("Range") != "":
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Length", "244")
			w.WriteHeader(stdhttp.StatusOK)
		default:
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Length", "123456789")
			w.WriteHeader(stdhttp.StatusOK)
		}
	}))
	defer server.Close()

	entries, err := client.ReadDir("/file.zip")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, 123456789, entries[0].Size())
	require.Equal(t, []string{stdhttp.MethodHead, stdhttp.MethodGet, stdhttp.MethodGet}, methods)
}

func TestRetrieveRejectsHTMLForZip(t *testing.T) {
	client, server := newTestClient(t, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		require.Equal(t, "Wget", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte("<html>not a zip</html>"))
	}))
	defer server.Close()

	var dest bytes.Buffer
	err := client.Retrieve("/file.zip", &dest)
	require.ErrorContains(t, err, "returned HTML for a non-HTML file")
	require.Empty(t, dest.Bytes())
}
