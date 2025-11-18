package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// JMAP API endpoints and methods
const (
	apiURL               = "https://api.fastmail.com/jmap/api"
	maskedEmailNamespace = "https://www.fastmail.com/dev/maskedemail"
	methodGet            = "MaskedEmail/get"
	methodSet            = "MaskedEmail/set"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	jmapErrorSuffixLen = 6 // length of "/error" suffix
)

// ErrAliasNotFound is returned when an alias cannot be found
var ErrAliasNotFound = errors.New("alias not found")

type FastmailClient struct {
	AccountID string
	Token     string
	Debug     bool
	client    *http.Client
}

// getMaskedEmail performs a MaskedEmail/get request with the given properties
func (fc *FastmailClient) getMaskedEmail(properties []string) ([]MaskedEmailInfo, error) {
	payload, err := fc.buildRequest(methodCall{
		name: methodGet,
		arguments: struct {
			AccountID  string   `json:"accountId"`
			Properties []string `json:"properties"`
		}{
			AccountID:  fc.AccountID,
			Properties: properties,
		},
		clientID: nil,
	})
	if err != nil {
		return nil, err
	}

	response, err := fc.sendRequest(payload)
	if err != nil {
		return nil, err
	}

	// Validate response structure before accessing
	if err := fc.validateMethodResponse(response, 0, 2); err != nil {
		return nil, err
	}

	var responseData struct {
		List []MaskedEmailInfo `json:"list"`
	}
	if err := json.Unmarshal(response.MethodResponses[0][1], &responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
	}

	return responseData.List, nil
}

// setMaskedEmail performs a MaskedEmail/set request with the given updates or creates
func (fc *FastmailClient) setMaskedEmail(create, update map[string]interface{}) (*MaskedEmailResponse, error) {
	args := struct {
		Create    map[string]interface{} `json:"create,omitempty"`
		Update    map[string]interface{} `json:"update,omitempty"`
		AccountID string                 `json:"accountId"`
	}{
		AccountID: fc.AccountID,
		Create:    create,
		Update:    update,
	}

	payload, err := fc.buildRequest(methodCall{
		name:      methodSet,
		arguments: args,
		clientID:  nil,
	})
	if err != nil {
		return nil, err
	}

	return fc.sendRequest(payload)
}

type MaskedEmailRequest struct {
	Using       []string            `json:"using"`
	MethodCalls [][]json.RawMessage `json:"methodCalls"`
}

type MaskedEmailResponse struct {
	MethodResponses [][]json.RawMessage `json:"methodResponses"`
	MethodErrors    []interface{}       `json:"methodErrors,omitempty"`
}

// JMAPError represents a JMAP method error
type JMAPError struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

// AliasState represents the possible states of a masked email
type AliasState string

const (
	AliasPending  AliasState = "pending"
	AliasEnabled  AliasState = "enabled"
	AliasDisabled AliasState = "disabled"
	AliasDeleted  AliasState = "deleted"
)

type MaskedEmailInfo struct {
	Email     string     `json:"email"`
	ForDomain string     `json:"forDomain"`
	State     AliasState `json:"state"`
	ID        string     `json:"id"`
}

// methodCall represents a JMAP method call
type methodCall struct {
	arguments interface{}
	clientID  interface{}
	name      string
}

func (fc *FastmailClient) buildRequest(calls ...methodCall) (*MaskedEmailRequest, error) {
	methodCalls := make([][]json.RawMessage, len(calls))

	for i, call := range calls {
		name, err := json.Marshal(call.name)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal method name: %w", err)
		}
		args, err := json.Marshal(call.arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal method arguments: %w", err)
		}
		clientID, err := json.Marshal(call.clientID)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal client ID: %w", err)
		}

		methodCalls[i] = []json.RawMessage{name, args, clientID}
	}

	return &MaskedEmailRequest{
		Using:       []string{"urn:ietf:params:jmap:core", "https://www.fastmail.com/dev/maskedemail"},
		MethodCalls: methodCalls,
	}, nil
}

// NewFastmailClient creates a new client for interacting with the Fastmail API.
// It requires FASTMAIL_ACCOUNT_ID and FASTMAIL_API_KEY environment variables to be set.
func NewFastmailClient(debug bool) (*FastmailClient, error) {
	accountID := os.Getenv("FASTMAIL_ACCOUNT_ID")
	token := os.Getenv("FASTMAIL_API_KEY")

	if accountID == "" {
		return nil, errors.New("FASTMAIL_ACCOUNT_ID environment variable must be set")
	}
	if token == "" {
		return nil, errors.New("FASTMAIL_API_KEY environment variable must be set")
	}

	return &FastmailClient{
		AccountID: accountID,
		Token:     token,
		Debug:     debug,
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}, nil
}

func (fc *FastmailClient) sendRequest(payload *MaskedEmailRequest) (*MaskedEmailResponse, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if fc.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Request URL: %s\n", apiURL)
		fmt.Fprintf(os.Stderr, "DEBUG: Request Headers:\n")
		fmt.Fprintf(os.Stderr, "  Content-Type: application/json\n")
		fmt.Fprintf(os.Stderr, "  Authorization: Bearer %s\n", redactToken(fc.Token))
		fmt.Fprintf(os.Stderr, "DEBUG: Request Body:\n%s\n", string(jsonPayload))
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", fc.Token))

	resp, err := fc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if fc.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Response Status: %s (%d)\n", resp.Status, resp.StatusCode)
		fmt.Fprintf(os.Stderr, "DEBUG: Response Headers:\n")
		for key, values := range resp.Header {
			for _, value := range values {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", key, value)
			}
		}
		fmt.Fprintf(os.Stderr, "DEBUG: Response Body:\n%s\n", string(body))
	}

	// Check HTTP status code before attempting to unmarshal JSON
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error %d: %s\nResponse body: %s", resp.StatusCode, resp.Status, string(body))
	}

	// Check for empty response body
	if len(body) == 0 {
		return nil, fmt.Errorf("received empty response body")
	}

	var result MaskedEmailResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w\nResponse body: %s", err, string(body))
	}

	// Validate JMAP error responses
	if err := fc.validateJMAPResponse(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// redactToken returns a redacted version of the token showing only the last 4 characters.
// Format: "[redacted token]...1234"
func redactToken(token string) string {
	// If the token is shorter than 4 characters, return the token as is
	if len(token) <= 4 {
		return token
	}
	return "[redacted token]..." + token[len(token)-4:]
}

// validateJMAPResponse checks for JMAP errors in the response
func (fc *FastmailClient) validateJMAPResponse(response *MaskedEmailResponse) error {
	// Check for top-level methodErrors
	if len(response.MethodErrors) > 0 {
		return fmt.Errorf("JMAP method errors in response: %v", response.MethodErrors)
	}

	// Check if MethodResponses is empty
	if len(response.MethodResponses) == 0 {
		return fmt.Errorf("empty MethodResponses array in JMAP response")
	}

	// Check each method response for errors
	for i, methodResponse := range response.MethodResponses {
		if len(methodResponse) == 0 {
			return fmt.Errorf("empty method response at index %d", i)
		}

		// Check if method name indicates an error (e.g., "MaskedEmail/get/error")
		var methodName string
		if err := json.Unmarshal(methodResponse[0], &methodName); err != nil {
			return fmt.Errorf("failed to unmarshal method name at index %d: %w", i, err)
		}

		// JMAP error responses have method names ending with "/error"
		if len(methodName) > jmapErrorSuffixLen && methodName[len(methodName)-jmapErrorSuffixLen:] == "/error" {
			// Try to extract error details
			if len(methodResponse) > 1 {
				var jmapError JMAPError
				if err := json.Unmarshal(methodResponse[1], &jmapError); err == nil {
					return fmt.Errorf("JMAP error: %s - %s", jmapError.Type, jmapError.Message)
				}
				// If we can't parse the error structure, return the raw JSON
				return fmt.Errorf("JMAP error in method '%s': %s", methodName, string(methodResponse[1]))
			}
			return fmt.Errorf("JMAP error in method '%s'", methodName)
		}

		// Validate that the response has at least method name and response data
		if len(methodResponse) < 2 {
			return fmt.Errorf("invalid method response structure at index %d: expected at least 2 elements, got %d", i, len(methodResponse))
		}
	}

	return nil
}

// validateMethodResponse validates that a specific method response in the JMAP response
// has the expected structure before accessing it. Returns an error if the response
// structure is invalid.
func (fc *FastmailClient) validateMethodResponse(response *MaskedEmailResponse, index int, minElements int) error {
	if len(response.MethodResponses) == 0 {
		return fmt.Errorf("invalid response structure: MethodResponses is empty")
	}
	if index >= len(response.MethodResponses) {
		return fmt.Errorf("invalid response structure: method response index %d out of range (have %d responses)", index, len(response.MethodResponses))
	}
	if len(response.MethodResponses[index]) < minElements {
		return fmt.Errorf("invalid response structure: method response at index %d has %d elements, expected at least %d", index, len(response.MethodResponses[index]), minElements)
	}
	return nil
}

func (fc *FastmailClient) GetAliases(domain string) ([]MaskedEmailInfo, error) {
	maskedEmails, err := fc.getMaskedEmail([]string{"email", "forDomain", "state"})
	if err != nil {
		return nil, err
	}

	var filteredAliases []MaskedEmailInfo
	for _, alias := range maskedEmails {
		if alias.ForDomain == domain && alias.State != AliasDeleted {
			filteredAliases = append(filteredAliases, alias)
		}
	}

	return filteredAliases, nil
}

func (fc *FastmailClient) CreateAlias(domain string) (*MaskedEmailInfo, error) {
	create := map[string]interface{}{
		"MaskedEmail": map[string]string{
			"forDomain":   domain,
			"description": domain,
		},
	}

	response, err := fc.setMaskedEmail(create, nil)
	if err != nil {
		return nil, err
	}

	// Validate response structure before accessing
	if err := fc.validateMethodResponse(response, 0, 2); err != nil {
		return nil, err
	}

	var createdAlias struct {
		Created struct {
			MaskedEmail MaskedEmailInfo `json:"MaskedEmail"`
		} `json:"created"`
	}

	err = json.Unmarshal(response.MethodResponses[0][1], &createdAlias)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal created alias: %w", err)
	}

	return &createdAlias.Created.MaskedEmail, nil
}

// GetAliasByEmail retrieves a specific alias by its email address.
// Returns ErrAliasNotFound if the alias doesn't exist.
func (fc *FastmailClient) GetAliasByEmail(email string) (*MaskedEmailInfo, error) {
	aliases, err := fc.getMaskedEmail([]string{"email", "state", "forDomain", "id"})
	if err != nil {
		return nil, fmt.Errorf("failed to get aliases: %w", err)
	}

	for _, alias := range aliases {
		if alias.Email == email {
			return &alias, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrAliasNotFound, email)
}

// UpdateAliasStatus changes the state of an existing alias.
// Returns an error if the alias is already in the requested state or if the update fails.
func (fc *FastmailClient) UpdateAliasStatus(alias *MaskedEmailInfo, state AliasState) error {
	// Print current state for user feedback
	fmt.Printf("Setting '%s' for '%s' to '%s'\n", alias.Email, alias.ForDomain, state)

	if state == alias.State {
		return fmt.Errorf("'%s' is already '%s'", alias.Email, state)
	}

	update := map[string]interface{}{
		alias.ID: map[string]string{
			"state": string(state),
		},
	}

	response, err := fc.setMaskedEmail(nil, update)
	if err != nil {
		return fmt.Errorf("failed to update alias: %w", err)
	}

	// Validate response structure before accessing
	if err := fc.validateMethodResponse(response, 0, 2); err != nil {
		return err
	}

	// Verify the update was successful
	var updateResponse struct {
		Updated map[string]interface{} `json:"updated"`
	}
	if err := json.Unmarshal(response.MethodResponses[0][1], &updateResponse); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if _, ok := updateResponse.Updated[alias.ID]; !ok {
		return fmt.Errorf("server did not confirm the update")
	}

	fmt.Println("Success")
	return nil
}
