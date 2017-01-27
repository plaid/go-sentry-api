package sentry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	// DefaultEndpoint is the default endpoint
	DefaultEndpoint = "https://sentry.io/api/0/"
	// DefaultTimeout is the default timeout and is set to 60 seconds
	DefaultTimeout = time.Duration(60) * time.Second
)

// Client is used to talk to a sentry endpoint.
// Needs a auth token.
// If no endpoint this defaults to https://sentry.io/api/0/
type Client struct {
	AuthToken  string
	Endpoint   string
	HTTPClient *http.Client
}

// NewClient takes a auth token a optional endpoint and optional timeout and
// will return back a client and error
func NewClient(authtoken string, endpoint *string, timeout *int) (*Client, error) {
	var (
		clientEndpoint string
		clientTimeout  time.Duration
	)

	if endpoint == nil {
		clientEndpoint = DefaultEndpoint
	} else {
		if *endpoint == "" {
			return nil, fmt.Errorf("Endpoint can not be a empty string")
		}
		clientEndpoint = *endpoint
	}

	if timeout == nil {
		clientTimeout = DefaultTimeout
	} else {
		clientTimeout = time.Duration(*timeout) * time.Second
	}

	return &Client{
		AuthToken: authtoken,
		Endpoint:  clientEndpoint,
		HTTPClient: &http.Client{
			Timeout: clientTimeout,
		},
	}, nil
}

func (c *Client) hasError(response *http.Response) error {

	if response.StatusCode > 299 || response.StatusCode < 200 {
		apierror := APIError{
			StatusCode: response.StatusCode,
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(body, &apierror); err != nil {
			return err
		}

		return error(apierror)
	}
	return nil
}

func (c *Client) decodeOrError(response *http.Response, out interface{}) error {

	if err := c.hasError(response); err != nil {
		return err
	}

	defer response.Body.Close()

	if out != nil {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) encodeOrError(in interface{}) (io.Reader, error) {
	bytedata, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(bytedata), nil
}

func (c *Client) newRequest(method, endpoint string, in interface{}) (*http.Request, error) {

	var bodyreader io.Reader

	if in != nil {
		newbodyreader, err := c.encodeOrError(&in)
		if err != nil {
			return nil, err
		}
		bodyreader = newbodyreader
	}

	req, err := http.NewRequest(method, c.Endpoint+endpoint+"/", bodyreader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.AuthToken))
	req.Close = true

	return req, nil
}

func (c *Client) doWithQuery(method string, endpoint string, out interface{}, in interface{}, query QueryReq) error {
	request, err := c.newRequest(method, endpoint, in)
	if err != nil {
		return err
	}
	request.URL.RawQuery = query.ToQueryString()
	return c.send(request, out)
}

func (c *Client) do(method string, endpoint string, out interface{}, in interface{}) error {
	request, err := c.newRequest(method, endpoint, in)
	if err != nil {
		return err
	}

	// TODO: Remove this
	if in != nil && method == "GET" {
		request.URL.RawQuery = in.(QueryReq).ToQueryString()
		log.Printf("Added query params url is now %s", request.URL)
	}
	return c.send(request, out)
}

func (c *Client) send(req *http.Request, out interface{}) error {
	response, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	return c.decodeOrError(response, out)
}
