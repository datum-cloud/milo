package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
)

// ProjectSpec defines the specification for a project from the old API.
// Assuming a minimal structure, can be expanded based on actual API response.
type ProjectSpec struct {
	Description string `json:"description"`
	// Add other spec fields if known, e.g., Labels map[string]string `json:"labels"`
}

// ProjectStatus defines the status of a project from the old API.
// Assuming a minimal structure.
type ProjectStatus struct {
	State string `json:"state"` // e.g., "ACTIVE", "DELETED"
}

// Project defines the main project object structure from the old API.
// Field names in `json:"..."` tags are assumptions and may need adjustment
// based on the actual API response.
type Project struct {
	CreateTime  string        `json:"createTime"`
	DisplayName string        `json:"displayName"`
	Etag        string        `json:"etag"`
	Name        string        `json:"name"`      // This might be the fully qualified name, e.g., "organizations/orgId/projects/projId"
	ProjectID   string        `json:"projectId"` // Assuming there's a direct project ID field
	Reconciling bool          `json:"reconciling"`
	Spec        ProjectSpec   `json:"spec"`
	Status      ProjectStatus `json:"status"`
	UID         string        `json:"uid"`
	UpdateTime  string        `json:"updateTime"`
}

// ProjectsResponse is the assumed top-level structure for the API response when listing projects.
type ProjectsResponse struct {
	Projects []Project `json:"projects"` // Assuming projects are nested under a "projects" key
}

// GetDatumOsProjects fetches projects for a given organization from the old Datum OS API.
// It assumes the response is compatible with resourcemanagerpb.ListProjectsResponse.
// organizationName is the identifier used in the API path (e.g., {organization_name}).
// pageSize allows specifying the number of projects to fetch per request.
func GetDatumOsProjects(endpoint string, apiKey string, organizationName string, pageSize int) []*resourcemanagerpb.Project {
	baseURL := strings.TrimSuffix(endpoint, "/") + fmt.Sprintf("/v1alpha/organizations/%s/projects", organizationName)
	apiURL := baseURL
	if pageSize > 0 {
		apiURL = fmt.Sprintf("%s?pageSize=%d", baseURL, pageSize)
	}

	bearerToken := apiKey

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating projects request for org %s: %v\n", organizationName, err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing projects request for org %s: %v\n", organizationName, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error fetching projects for org %s: status code %d, body: %s\n", organizationName, resp.StatusCode, string(bodyBytes))
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading projects response body for org %s: %v\n", organizationName, err)
		return nil
	}

	var listProjectsResponse resourcemanagerpb.ListProjectsResponse
	err = json.Unmarshal(body, &listProjectsResponse)
	if err != nil {
		// Attempt to unmarshal as a direct list if the top-level wrapper is missing
		// This is less likely if it strictly matches ListProjectsResponse, but good for robustness
		var projectsDirectly []*resourcemanagerpb.Project
		errDirect := json.Unmarshal(body, &projectsDirectly)
		if errDirect != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshalling projects response for org %s (tried ListProjectsResponse and direct list): %v / %v\n", organizationName, err, errDirect)
			fmt.Fprintf(os.Stderr, "Raw response body for org %s: %s\n", organizationName, string(body))
			return nil
		}
		fmt.Printf("Found %d projects for organization %s (parsed as direct list of resourcemanagerpb.Project).\n", len(projectsDirectly), organizationName)
		return projectsDirectly
	}

	fmt.Printf("Found %d projects for organization %s (parsed as resourcemanagerpb.ListProjectsResponse).\n", len(listProjectsResponse.Projects), organizationName)
	// TODO: Handle pagination using listProjectsResponse.NextPageToken if necessary
	if listProjectsResponse.NextPageToken != "" {
		fmt.Fprintf(os.Stdout, "Note: Projects response for org %s has a next page token. Pagination not yet implemented in GetDatumOsProjects.\n", organizationName)
	}

	return listProjectsResponse.Projects
}
