package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

func GetDatumOsUsers(endpoint string, apiKey string) []User {
	url := endpoint
	bearerToken := apiKey

	// Create a new HTTP client
	client := &http.Client{}

	// Create a new GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}

	// Set the Authorization header
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read the response body
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

	var rawUsersList []interface{}
	usersData, ok := responseData["users"]
	if !ok {
		fmt.Println("Key 'users' not found in the response object.")
		os.Exit(1)
	}

	rawUsersList, ok = usersData.([]interface{})
	if !ok {
		fmt.Fprintf(os.Stderr, "Data under 'users' key is not a list (array). Type is: %T\n", usersData)
		os.Exit(1)
	}

	var typedUsers []User
	for i, userEntry := range rawUsersList {
		userBytes, err := json.Marshal(userEntry) // Marshal the map[string]interface{} back to JSON
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling user entry %d: %v\n", i, err)
			continue
		}
		var typedUser User
		err = json.Unmarshal(userBytes, &typedUser) // Unmarshal JSON into the User struct
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshalling user entry %d into User struct: %v\n", i, err)
			fmt.Fprintf(os.Stderr, "Problematic user entry data: %s\n", string(userBytes))
			continue
		}
		typedUsers = append(typedUsers, typedUser)
	}

	return typedUsers
}
