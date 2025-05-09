package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings" // Added for strings.TrimSuffix
)

// UserSpec defines the specification for a user
type UserSpec struct {
	AuthProvider     string          `json:"authProvider"`
	AvatarRemoteURI  string          `json:"avatarRemoteUri"`
	AvatarUpdateTime string          `json:"avatarUpdateTime"`
	Email            string          `json:"email"`
	Firstname        string          `json:"firstname"`
	LastSeenTime     string          `json:"lastSeenTime"`
	Lastname         string          `json:"lastname"`
	Orgs             []OrgMembership `json:"orgs"`
	PhoneNumbers     []string        `json:"phoneNumbers"` // Assuming phoneNumbers is an array of strings
	Role             string          `json:"role"`
	Setting          UserSetting     `json:"setting"`
}

// UserSetting defines user UI settings
type UserSetting struct {
	UITheme string `json:"uiTheme"`
}

// OrgMembership defines the structure of an organization membership object
type OrgMembership struct {
	DisplayName    string `json:"displayName"`
	OrganizationID string `json:"organizationId"`
	Role           string `json:"role"`
	UID            string `json:"uid"`
}

// UserStatus defines the status of a user
type UserStatus struct {
	Status string `json:"status"`
}

// User defines the main user object structure
type User struct {
	CreateTime  string     `json:"createTime"`
	DisplayName string     `json:"displayName"`
	Etag        string     `json:"etag"`
	Name        string     `json:"name"`
	Reconciling bool       `json:"reconciling"`
	Spec        UserSpec   `json:"spec"`
	Status      UserStatus `json:"status"`
	UID         string     `json:"uid"`
	UpdateTime  string     `json:"updateTime"`
	UserID      string     `json:"userId"`
}

// UsersResponse is the assumed top-level structure for the API response when listing users.
// If the API returns a direct list, this might not be strictly needed.
type UsersResponse struct {
	Users []User `json:"users"`
	// Potentially add NextPageToken string `json:"nextPageToken,omitempty"` if API supports pagination
}

// GetDatumOsUsers fetches users from the Datum OS API.
// baseEndpoint should be the root of the Datum OS API, e.g., "https://api.example.com/datum-os"
func GetDatumOsUsers(baseEndpoint string, apiKey string, pageSize int) []User {
	path := "/v1alpha/users"
	apiURL := strings.TrimSuffix(baseEndpoint, "/") + path
	if pageSize > 0 {
		apiURL = fmt.Sprintf("%s?pageSize=%d", apiURL, pageSize)
	}

	bearerToken := apiKey

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating users request: %v\n", err)
		return nil // Return nil or empty slice on error
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing users request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error fetching users: status code %d, body: %s\n", resp.StatusCode, string(bodyBytes))
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading users response body: %v\n", err)
		return nil
	}

	// Try to unmarshal assuming the response is wrapped in an object like {"users": [...]}
	var usersResponse UsersResponse
	err = json.Unmarshal(body, &usersResponse)
	if err == nil && usersResponse.Users != nil {
		fmt.Printf("Found %d users (via wrapper object).\n", len(usersResponse.Users))
		return usersResponse.Users
	}

	// If that fails, try to unmarshal as a direct list of users []User
	fmt.Fprintf(os.Stderr, "Could not unmarshal into UsersResponse (key 'users' might be missing), trying direct list. Error: %v\n", err)
	var typedUsers []User
	errDirect := json.Unmarshal(body, &typedUsers)
	if errDirect != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshalling users JSON directly: %v\n", errDirect)
		fmt.Fprintf(os.Stderr, "Raw response body for users: %s\n", string(body))
		return nil
	}

	fmt.Printf("Found %d users (via direct list).\n", len(typedUsers))
	return typedUsers
}
