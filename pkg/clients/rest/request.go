/*
 * Copyright 2025 TomTom N.V.
 * Copyright 2017 The Kubernetes Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Request struct {
	err        error
	c          *RESTClient
	params     url.Values
	headers    http.Header
	method     string
	subPath    string
	body       []byte
	timeout    time.Duration
	maxRetries int
}

// Result contains the result of calling Request.Do().
type Result struct {
	Err        error
	Body       []byte
	StatusCode int
}

func NewRequest(c *RESTClient) *Request {
	var timeout time.Duration
	if c.Client != nil {
		timeout = c.Client.Timeout
	}

	r := &Request{
		c:          c,
		timeout:    timeout,
		maxRetries: 3,
	}

	r.SetHeader("Accept", c.contentType+", */*")
	return r
}

func (r *Request) Method(method string) *Request {
	r.method = method
	return r
}

func (r *Request) SetHeader(key string, values ...string) *Request {
	if r.headers == nil {
		r.headers = http.Header{}
	}
	r.headers.Del(key)
	for _, value := range values {
		r.headers.Add(key, value)
	}
	return r
}

func (r *Request) SubPath(subPath string) *Request {
	r.subPath = subPath
	return r
}

func (r *Request) Body(body []byte) *Request {
	r.body = body
	return r
}

// Do formats and executes the request. Returns a Result object for easy response
// processing.
func (r *Request) Do(ctx context.Context) Result {
	var result Result
	err := r.request(ctx, func(req *http.Request, resp *http.Response) {
		result = r.prepareResponse(resp, req)
	})
	if err != nil {
		return Result{Err: err}
	}
	return result
}

// TODO: Add retry logic
func (r *Request) request(ctx context.Context, fn func(*http.Request, *http.Response)) error {
	client := r.c.Client
	if client == nil {
		client = http.DefaultClient
	}

	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	req, err := r.newHTTPRequest(ctx)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	fn(req, resp)

	return nil
}

func (r *Request) newHTTPRequest(ctx context.Context) (*http.Request, error) {
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}

	reqURL := r.URL().String()
	req, err := http.NewRequestWithContext(ctx, r.method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.Header = r.headers
	return req, nil
}

// URL returns the current working URL. Check the result of Error() to ensure
// that the returned URL is valid.
func (r *Request) URL() *url.URL {
	finalURL := &url.URL{}

	if r.c.baseURL != nil {
		*finalURL = *r.c.baseURL
	}

	if len(r.subPath) != 0 {
		finalURL.Path = path.Join(finalURL.Path, strings.ToLower(r.subPath))
	}

	query := url.Values{}
	for key, values := range r.params {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	finalURL.RawQuery = query.Encode()
	return finalURL
}

// transformResponse converts an API response into a structured API object
func (r *Request) prepareResponse(resp *http.Response, req *http.Request) Result {
	var body []byte

	if resp.Body != nil {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			unexpectedErr := fmt.Errorf("unexpected error when reading response body. Original error: %w", err)
			return Result{
				Err: unexpectedErr,
			}
		}
		body = data
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		err := fmt.Errorf("the server responded with the status code %d. Request method %s. Raw response: %v", resp.StatusCode, req.Method, string(body))
		return Result{
			Body:       body,
			StatusCode: resp.StatusCode,
			Err:        err,
		}
	}

	return Result{
		Body:       body,
		StatusCode: resp.StatusCode,
	}
}
