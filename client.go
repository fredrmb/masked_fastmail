package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// JMAP API endpoints and methods
const (
	apiURL               = "https://api.fastmail.com/jmap/api"
	maskedEmailNamespace = "https://www.fastmail.com/dev/maskedemail"
	methodGet            = "MaskedEmail/get"
	methodSet            = "MaskedEmail/set"
)

// ErrAliasNotFound is returned when an alias cannot be found
var ErrAliasNotFound = errors.New("alias not found")

// ErrUpdateFailed is returned when an alias update fails
var ErrUpdateFailed = errors.New("failed to update alias")

type FastmailClient struct {
	AccountID string
	Token     string
	Debug     bool
}

// getMaskedEmail performs a MaskedEmail/get request with the given properties
func (fc *FastmailClient) getMaskedEmail(properties []string) ([]MaskedEmailInfo, error) {
	payload := fc.buildRequest(methodCall{
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

	response, err := fc.sendRequest(payload)
	if err != nil {
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

	payload := fc.buildRequest(methodCall{
		name:      methodSet,
		arguments: args,
		clientID:  nil,
	})

	return fc.sendRequest(payload)
}

type MaskedEmailRequest struct {
	Using       []string            `json:"using"`
	MethodCalls [][]json.RawMessage `json:"methodCalls"`
}

type MaskedEmailResponse struct {
	MethodResponses [][]json.RawMessage `json:"methodResponses"`
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

func (fc *FastmailClient) buildRequest(calls ...methodCall) *MaskedEmailRequest {
	methodCalls := make([][]json.RawMessage, len(calls))

	for i, call := range calls {
		name, _ := json.Marshal(call.name)
		args, _ := json.Marshal(call.arguments)
		clientID, _ := json.Marshal(call.clientID)

		methodCalls[i] = []json.RawMessage{name, args, clientID}
	}

	return &MaskedEmailRequest{
		Using:       []string{"urn:ietf:params:jmap:core", "https://www.fastmail.com/dev/maskedemail"},
		MethodCalls: methodCalls,
	}
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
		fmt.Fprintf(os.Stderr, "  Authorization: Bearer %s\n", fc.Token)
		fmt.Fprintf(os.Stderr, "DEBUG: Request Body:\n%s\n", string(jsonPayload))
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", fc.Token))

	client := &http.Client{}
	resp, err := client.Do(req)
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

	var result MaskedEmailResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w\nResponse body: %s", err, string(body))
	}

	return &result, nil
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

	var createdAlias struct {
		Created struct {
			MaskedEmail MaskedEmailInfo `json:"MaskedEmail"`
		} `json:"created"`
	}

	err = json.Unmarshal(response.MethodResponses[0][1], &createdAlias)
	if err != nil {
		return nil, err
	}

	return &createdAlias.Created.MaskedEmail, nil
}

// GetAliasByEmail returns a specific alias by its email address
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
