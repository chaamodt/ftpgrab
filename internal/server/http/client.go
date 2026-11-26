package http

import (
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/cavaliergopher/grab/v3"
	"github.com/crazy-max/ftpgrab/v7/internal/config"
	"github.com/crazy-max/ftpgrab/v7/internal/server"
	"github.com/rs/zerolog/log"
)

// Client represents an active http object
type Client struct {
	*server.Client
	config *config.ServerHTTP
	http   *grab.Client
}

// New creates new http instance
func New(config *config.ServerHTTP) (*server.Client, error) {
	var err error
	var client = &Client{config: config}

	/////////////////////////////////////////////////////////////////////////////////////
	// TODO : Basic authentication
	//
	// if len(config.Password) > 0 || len(config.PasswordFile) > 0 {
	// 	password, err := utl.GetSecret(config.Password, config.PasswordFile)
	// 	if err != nil {
	// 		log.Warn().Err(err).Msg("Cannot retrieve password secret for sftp server")
	// 	}
	// }

	// username, err := utl.GetSecret(config.Username, config.UsernameFile)
	// if err != nil {
	// 	log.Warn().Err(err).Msg("Cannot retrieve username secret for sftp server")
	// }
	//
	/////////////////////////////////////////////////////////////////////////////////////

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			// Set InsecureSkipVerify to skip the default validation we are
			// replacing. This will not disable VerifyConnection.
			InsecureSkipVerify: *config.InsecureSkipVerify,
		},
		TLSHandshakeTimeout: *config.Timeout,

		DisableKeepAlives:  *config.DisableKeepAlives,
		DisableCompression: *config.DisableCompression,

		// MaxIdleConns int
		// MaxIdleConnsPerHost int
		// MaxConnsPerHost int

		// IdleConnTimeout time.Duration
		ResponseHeaderTimeout: *config.Timeout,
	}

	client.http = grab.NewClient()
	client.http.HTTPClient = &http.Client{
		Transport: tr,
	}

	return &server.Client{Handler: client}, err
}

// Common return common configuration
func (c *Client) Common() config.ServerCommon {
	return config.ServerCommon{
		Host:    c.config.Host,
		Port:    c.config.Port,
		Sources: c.config.Sources,
	}
}

// ReadDir fetches the contents of a directory, returning a list of os.FileInfo's
// for http urls it heads the url to retrieve size and date
func (c *Client) ReadDir(path string) ([]os.FileInfo, error) {
	var urlStr string
	urlStr = "http"
	if *c.config.TLS {
		urlStr += "s"
	}

	// prepare url
	urlStr += "://" + c.config.Host + ":" + strconv.Itoa(c.config.Port) + path

	log.Debug().Msgf("HTTP Head url %s ...", urlStr)

	req, err := http.NewRequest(http.MethodHead, urlStr, nil)
	if err != nil {
		log.Debug().Str("path", path).Msgf("Cannot create grab head request for url %s", urlStr)
		return nil, err
	}

	resp, err := c.http.HTTPClient.Do(req)
	if err == nil && !needsGetMetadata(path, resp) {
		defer resp.Body.Close()
		return c.fileInfoFromResponse(path, resp, resp.ContentLength)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		log.Warn().
			Str("url", urlStr).
			Err(err).
			Msg("HEAD failed, trying ranged GET")
	} else {
		log.Warn().
			Str("url", urlStr).
			Int("status", resp.StatusCode).
			Int64("size", resp.ContentLength).
			Msg("HEAD returned unusable metadata, trying ranged GET")
	}

	resp, size, err := c.getMetadataWithGet(urlStr, path, true)
	if err != nil {
		log.Warn().
			Str("url", urlStr).
			Err(err).
			Msg("Ranged GET failed, trying regular GET headers")
		resp, size, err = c.getMetadataWithGet(urlStr, path, false)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	return c.fileInfoFromResponse(path, resp, size)
}

func needsGetMetadata(path string, resp *http.Response) bool {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || resp.ContentLength < 0 {
		return true
	}

	return isHTMLForNonHTMLPath(path, resp)
}

func isHTMLForNonHTMLPath(path string, resp *http.Response) bool {
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mediaType != "text/html" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(path))
	return ext != ".htm" && ext != ".html"
}

func (c *Client) getMetadataWithGet(urlStr, path string, ranged bool) (*http.Response, int64, error) {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, 0, err
	}
	if ranged {
		req.Header.Set("Range", "bytes=0-0")
	}

	resp, err := c.http.HTTPClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, 0, err
	}

	if (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) &&
		!(ranged && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable) {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("GET %s returned status %s", urlStr, resp.Status)
	}
	if isHTMLForNonHTMLPath(path, resp) {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("GET %s returned HTML for a non-HTML file", urlStr)
	}

	var size int64
	switch resp.StatusCode {
	case http.StatusPartialContent:
		size, err = contentRangeSize(resp.Header.Get("Content-Range"))
	case http.StatusRequestedRangeNotSatisfiable:
		size, err = contentRangeSize(resp.Header.Get("Content-Range"))
	default:
		size = resp.ContentLength
		if size < 0 {
			err = fmt.Errorf("GET %s did not provide a content length", urlStr)
		}
	}
	if err != nil {
		resp.Body.Close()
		return nil, 0, err
	}

	return resp, size, nil
}

func contentRangeSize(header string) (int64, error) {
	unit, value, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || unit != "bytes" {
		return 0, fmt.Errorf("invalid Content-Range %q", header)
	}
	_, total, ok := strings.Cut(value, "/")
	if !ok || total == "*" {
		return 0, fmt.Errorf("invalid Content-Range %q", header)
	}

	size, err := strconv.ParseInt(total, 10, 64)
	if err != nil || size < 0 {
		return 0, fmt.Errorf("invalid Content-Range %q", header)
	}
	return size, nil
}

func (c *Client) fileInfoFromResponse(path string, resp *http.Response, size int64) ([]os.FileInfo, error) {
	// decode time
	var resptime time.Time
	var err error

	if resp.Header.Get("last-modified") != "" {
		// header last-modified
		resptime, err = dateparse.ParseLocal(resp.Header.Get("last-modified"))
	} else if resp.Header.Get("date") != "" {
		// header date
		resptime, err = dateparse.ParseLocal(resp.Header.Get("date"))
	} else {
		// current date
		resptime = time.Now()
	}

	if err != nil {
		log.Debug().Str("path", path).Err(err).Msg("Error decoding HTTP response date")
		return nil, err
	}

	// log.Debug().Msgf("Decoded date for url %s : %v ", urlStr, resptime)

	var entries []os.FileInfo

	var mode os.FileMode

	fileInfo := &fileInfo{
		name:  filepath.Base(path),
		mode:  mode,
		mtime: resptime,
		size:  size,
	}

	entries = append(entries, fileInfo)

	return entries, nil
}

// Retrieve file "path" from server and write bytes to "dest".
// func (c *Client) Retrieve(path string, dest io.Writer) error {
// 	var urlStr string
// 	urlStr = "http"
// 	if *c.config.TLS {
// 		urlStr += "s"
// 	}

// 	// prepare url
// 	urlStr += "://" + c.config.Host + ":" + strconv.Itoa(c.config.Port) + path

// 	tempfile, err := os.CreateTemp("", "*")
// 	if err != nil {
// 		log.Debug().Str("path", path).Err(err).Msgf("Cannot create temporary file for url %s !", urlStr)
// 		return err
// 	}

// 	log.Debug().Msgf("HTTP Retrieve url %s ...", urlStr)

// 	req, err := grab.NewRequest(tempfile.Name(), urlStr)
// 	if err != nil {
// 		log.Debug().Str("path", path).Err(err).Msgf("Cannot create grab request for url %s !", urlStr)
// 		return err
// 	}

// 	resp := c.http.Do(req)
// 	if err := resp.Err(); err != nil {
// 		log.Debug().Str("path", path).Err(err).Msgf("Error exec grab request for url %s !", urlStr)
// 		return err
// 	}

// 	if _, err := io.Copy(dest, tempfile); err != nil {
// 		log.Debug().Str("path", path).Str("temfile", tempfile.Name()).Err(err).Msgf("Error copy temp file to destination for url %s !", urlStr)
// 		return err
// 	}

// 	defer os.Remove(tempfile.Name())

//		return nil
//	}
func (c *Client) Retrieve(path string, dest io.Writer) error {
	var urlStr string
	urlStr = "http"
	if *c.config.TLS {
		urlStr += "s"
	}

	urlStr += "://" + c.config.Host + ":" + strconv.Itoa(c.config.Port) + path

	// requête GET simple
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}

	// client HTTP existant (proxy / TLS / transport déjà configuré)
	resp, err := c.http.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// gestion des erreurs HTTP propres
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	if isHTMLForNonHTMLPath(path, resp) {
		return fmt.Errorf("GET %s returned HTML for a non-HTML file", urlStr)
	}

	// copie du flux (download réel)
	_, err = io.Copy(dest, resp.Body)
	return err
}

// Close closes http connection
func (c *Client) Close() error {
	// http client in grab doesn't have close method
	// nothing to do ?
	return nil
}
