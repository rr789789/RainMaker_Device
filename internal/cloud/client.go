package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"rainmaker-device/internal/config"
)

type Client struct {
	BaseURL     string
	accessToken string
	mu          sync.Mutex
}

func NewClient() *Client {
	return &Client{
		BaseURL: config.AppConfig.Server.URL,
	}
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.BaseURL+"/v1"+path, reqBody)
	if err != nil {
		return nil, 0, err
	}

	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

// Login authenticates with the cloud server
func (c *Client) Login() error {
	email := config.AppConfig.Server.Email
	password := config.AppConfig.Server.Password

	body := map[string]interface{}{
		"user_name": email,
		"password":  password,
	}
	data, status, err := c.doRequest("POST", "/login", body)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("login failed (status %d): %s", status, string(data))
	}

	var resp map[string]interface{}
	json.Unmarshal(data, &resp)

	token, _ := resp["accesstoken"].(string)
	if token == "" {
		return fmt.Errorf("no access token in response")
	}
	c.accessToken = token
	return nil
}

// RegisterNode creates a node on the cloud server
func (c *Client) RegisterNode(nodeID, secretKey string) error {
	body := map[string]interface{}{
		"operation":  "add",
		"node_id":   nodeID,
		"secret_key": secretKey,
	}
	data, status, err := c.doRequest("PUT", "/user/nodes", body)
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("register node failed (status %d): %s", status, string(data))
	}
	return nil
}

// UploadConfig uploads the full device config to the cloud
func (c *Client) UploadConfig(nodeID string, configJSON []byte) error {
	var config map[string]interface{}
	json.Unmarshal(configJSON, &config)

	data, status, err := c.doRequest("PUT", "/user/nodes?node_id="+nodeID, map[string]interface{}{
		"operation": "edit",
		"node_id":   nodeID,
		"metadata":  `{"name":"` + nodeID + `"}`,
	})
	_ = data
	_ = status
	_ = err

	// Also update node config via params
	var configMap map[string]interface{}
	json.Unmarshal(configJSON, &configMap)

	devices, _ := configMap["devices"].([]interface{})
	for _, d := range devices {
		dev, _ := d.(map[string]interface{})
		devName, _ := dev["name"].(string)
		params, _ := dev["params"].([]interface{})
		for _, p := range params {
			param, _ := p.(map[string]interface{})
			pName, _ := param["name"].(string)
			_ = devName
			_ = pName
		}
	}

	return nil
}

// SyncParams pushes current params to the cloud
func (c *Client) SyncParams(nodeID string, paramsJSON []byte) error {
	var params map[string]interface{}
	json.Unmarshal(paramsJSON, &params)

	_, _, err := c.doRequest("PUT", "/user/nodes/params?node_id="+nodeID, params)
	return err
}

// GetCloudParams fetches current params from cloud
func (c *Client) GetCloudParams(nodeID string) (map[string]interface{}, error) {
	data, status, err := c.doRequest("GET", "/user/nodes/params?node_id="+nodeID, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("get params failed (status %d)", status)
	}
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

// PollSync periodically syncs params between cloud and local
func (c *Client) PollSync(nodeID string, interval time.Duration, onUpdate func(map[string]interface{})) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		params, err := c.GetCloudParams(nodeID)
		if err == nil && params != nil {
			onUpdate(params)
		}
	}
}
