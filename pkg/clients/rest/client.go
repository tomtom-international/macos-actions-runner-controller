package rest

import (
	"net/http"
	"net/url"
	"path"
	"strings"
)

type Client interface {
	Post() *Request
	Put() *Request
	Get() *Request
}

type RESTClient struct {
	// base is the root URL for all invocations of the client
	baseURL *url.URL

	// versionedAPIPath is a path segment connecting the base URL to the resource root
	pathPrefix string

	// ContentType specifies the format used to communicate with the server.
	contentType string

	// Set specific behavior of the client.  If not set http.DefaultClient will be used.
	Client *http.Client
}

func NewRESTClient(host string, pathPrefix string, contentType string, client *http.Client) (*RESTClient, error) {
	if len(contentType) == 0 {
		contentType = "application/json"
	}

	baseURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}
	if len(pathPrefix) > 0 {
		baseURL.Path = path.Join(baseURL.Path, pathPrefix)
	}

	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	return &RESTClient{
		baseURL: baseURL,
		Client:  client,
	}, nil
}

// Post begins a POST request. Short for c.Verb("POST").
func (c *RESTClient) Post() *Request {
	return NewRequest(c).Method("POST")
}

// Put begins a PUT request. Short for c.Verb("PUT").
func (c *RESTClient) Put() *Request {
	return NewRequest(c).Method("PUT")
}

// Get begins a GET request. Short for c.Verb("GET").
func (c *RESTClient) Get() *Request {
	return NewRequest(c).Method("GET")
}
