package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

func GetDatumOsOrganizations(endpoint string, apiKey string) []Organization {
	url := endpoint
	bearerToken := apiKey

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response body: %v\n", err)
		os.Exit(1)
	}

	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshalling JSON response object: %v\n", err)
		fmt.Fprintf(os.Stderr, "Raw response body: %s\n", string(body))
		os.Exit(1)
	}

	var rawOrganizationsList []interface{}
	organizationsData, ok := responseData["organizations"]
	if !ok {
		fmt.Println("Key 'organizations' not found in the response object.")
		os.Exit(1)
	}

	rawOrganizationsList, ok = organizationsData.([]interface{})
	if !ok {
		fmt.Fprintf(os.Stderr, "Data under 'organizations' key is not a list (array). Type is: %T\n", organizationsData)
		os.Exit(1)
	}

	var typedOrganizations []Organization
	for i, orgEntry := range rawOrganizationsList {
		orgBytes, err := json.Marshal(orgEntry) // Marshal the map[string]interface{} back to JSON
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling organization entry %d: %v\n", i, err)
			continue
		}
		var typedOrg Organization
		err = json.Unmarshal(orgBytes, &typedOrg) // Unmarshal JSON into the Organization struct
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshalling organization entry %d into Organization struct: %v\n", i, err)
			fmt.Fprintf(os.Stderr, "Problematic organization entry data: %s\n", string(orgBytes))
			continue
		}
		typedOrganizations = append(typedOrganizations, typedOrg)
	}

	fmt.Printf("Found %d organizations.\n", len(typedOrganizations))
	fmt.Println("Organizations:")

	return typedOrganizations
}
