package ipc

import (
	"errors"
	"fmt"
	"net"
	"time"
)

type Client struct {
	sockPath string
	timeout  time.Duration
}

func NewClient(sockPath string) *Client {
	return &Client{
		sockPath: sockPath,
		timeout:  5 * time.Second,
	}
}

func (c *Client) Send(req *Request) (*Response, error) {
	conn, err := net.DialTimeout("unix", c.sockPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to daemon socket: %w — is the daemon running?", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	err = EncodeRequest(conn, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	resp, err := DecodeResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return resp, nil
}

func (c *Client) Ping() (*PingData, error) {
	req, err := NewRequest(TypePing, PingPayload{})
	if err != nil {
		return nil, err
	}

	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}

	if resp.Status == StatusError {
		errData, err := ParseErrorData(resp)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(errData.Message)
	}

	return ParsePingData(resp)
}

func (c *Client) IsDaemonReachable() bool {
	_, err := c.Ping()
	return err == nil
}
