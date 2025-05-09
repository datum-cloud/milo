package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// OrganizationMember defines the structure of a member/owner in an organization
type OrganizationMember struct {
	Email     string `json:"email"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Role      string `json:"role"`
	UID       string `json:"uid"`
	UserID    string `json:"userId"`
}

// OrganizationSpec defines the specification for an organization
type OrganizationSpec struct {
	AvatarRemoteURI      string               `json:"avatarRemoteUri"`
	Description          string               `json:"description"`
	Members              []OrganizationMember `json:"members"`
	Owners               []OrganizationMember `json:"owners"`
	ParentOrganizationID string               `json:"parentOrganizationId"`
}

// OrganizationStatus defines the status of an organization
type OrganizationStatus struct {
	Internal          bool   `json:"internal"`
	Personal          bool   `json:"personal"`
	VerificationState string `json:"verificationState"`
}

// Organization defines the main organization object structure
type Organization struct {
	CreateTime     string             `json:"createTime"`
	DisplayName    string             `json:"displayName"`
	Etag           string             `json:"etag"`
	Name           string             `json:"name"`
	OrganizationID string             `json:"organizationId"`
	Reconciling    bool               `json:"reconciling"`
	Spec           OrganizationSpec   `json:"spec"`
	Status         OrganizationStatus `json:"status"`
	UID            string             `json:"uid"`
	UpdateTime     string             `json:"updateTime"`
}

// OrganizationResponse is the top-level structure for the API response
type OrganizationResponse struct {
	Organizations []Organization `json:"organizations"`
}

// GetDatumOsOrganizations fetches organizations from the Datum OS API.
// baseEndpoint should be the root of the Datum OS API, e.g., "https://api.example.com/datum-os"
func GetDatumOsOrganizations(baseEndpoint string, apiKey string, pageSize int) []Organization {
	path := "/v1alpha/organizations"
	apiURL := strings.TrimSuffix(baseEndpoint, "/") + path
	if pageSize > 0 {
		apiURL = fmt.Sprintf("%s?pageSize=%d", apiURL, pageSize)
	}

	bearerToken := apiKey

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating organizations request: %v\n", err)
		// Consider returning nil or an error for robustness instead of os.Exit
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing organizations request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error fetching organizations: status code %d, body: %s\n", resp.StatusCode, string(bodyBytes))
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading organizations response body: %v\n", err)
		return nil
	}

	// Try to unmarshal assuming the response is wrapped in an object like {"organizations": [...]}
	var orgResponse OrganizationResponse
	err = json.Unmarshal(body, &orgResponse)
	if err == nil && orgResponse.Organizations != nil {
		fmt.Printf("Found %d organizations (via wrapper object).\n", len(orgResponse.Organizations))
		return orgResponse.Organizations
	}

	// If that fails, try to unmarshal as a direct list of organizations []Organization
	// This handles cases where the API might return a raw list.
	fmt.Fprintf(os.Stderr, "Could not unmarshal into OrganizationResponse (key 'organizations' might be missing), trying direct list. Error: %v\n", err)
	var typedOrganizations []Organization
	errDirect := json.Unmarshal(body, &typedOrganizations)
	if errDirect != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshalling organizations JSON directly: %v\n", errDirect)
		fmt.Fprintf(os.Stderr, "Raw response body: %s\n", string(body))
		return nil
	}

	fmt.Printf("Found %d organizations (via direct list).\n", len(typedOrganizations))
	return typedOrganizations
}
