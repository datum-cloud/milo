package filters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authentication/user"
)

func TestDetermineScopeAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		userInfo user.Info
		want     map[string]string
	}{
		{
			name: "project scope",
			userInfo: &user.DefaultInfo{
				Name: "test-user",
				Extra: map[string][]string{
					ParentTypeExtraKey: {"Project"},
					ParentNameExtraKey: {"backend-api"},
				},
			},
			want: map[string]string{
				ScopeTypeKey: ScopeTypeProject,
				ScopeNameKey: "backend-api",
			},
		},
		{
			name: "organization scope",
			userInfo: &user.DefaultInfo{
				Name: "test-user",
				Extra: map[string][]string{
					ParentTypeExtraKey: {"Organization"},
					ParentNameExtraKey: {"acme-corp"},
				},
			},
			want: map[string]string{
				ScopeTypeKey: ScopeTypeOrganization,
				ScopeNameKey: "acme-corp",
			},
		},
		{
			name: "user scope",
			userInfo: &user.DefaultInfo{
				Name: "test-user",
				Extra: map[string][]string{
					ParentTypeExtraKey: {"User"},
					ParentNameExtraKey: {"alice"},
				},
			},
			want: map[string]string{
				ScopeTypeKey: ScopeTypeUser,
				ScopeNameKey: "alice",
			},
		},
		{
			name: "global scope (no parent)",
			userInfo: &user.DefaultInfo{
				Name: "test-user",
				Extra: map[string][]string{},
			},
			want: map[string]string{
				ScopeTypeKey: ScopeTypeGlobal,
				// No scope.name for global
			},
		},
		{
			name: "missing parent name (still sets type)",
			userInfo: &user.DefaultInfo{
				Name: "test-user",
				Extra: map[string][]string{
					ParentTypeExtraKey: {"Organization"},
					// ParentNameExtraKey missing
				},
			},
			want: map[string]string{
				ScopeTypeKey: ScopeTypeOrganization,
				// No scope.name since name is missing
			},
		},
		{
			name: "empty parent name array",
			userInfo: &user.DefaultInfo{
				Name: "test-user",
				Extra: map[string][]string{
					ParentTypeExtraKey: {"Project"},
					ParentNameExtraKey: {}, // Empty array
				},
			},
			want: map[string]string{
				ScopeTypeKey: ScopeTypeProject,
				// No scope.name since array is empty
			},
		},
		{
			name: "unknown parent type defaults to global",
			userInfo: &user.DefaultInfo{
				Name: "test-user",
				Extra: map[string][]string{
					ParentTypeExtraKey: {"UnknownType"},
					ParentNameExtraKey: {"some-name"},
				},
			},
			want: map[string]string{
				ScopeTypeKey: ScopeTypeGlobal,
				// No scope.name for global
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineScopeAnnotations(tt.userInfo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetFirstExtraValue(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string][]string
		key   string
		want  string
	}{
		{
			name: "value exists",
			extra: map[string][]string{
				"test-key": {"value1", "value2"},
			},
			key:  "test-key",
			want: "value1",
		},
		{
			name: "multiple values returns first",
			extra: map[string][]string{
				"test-key": {"first", "second", "third"},
			},
			key:  "test-key",
			want: "first",
		},
		{
			name:  "key missing",
			extra: map[string][]string{},
			key:   "missing-key",
			want:  "",
		},
		{
			name: "empty array",
			extra: map[string][]string{
				"test-key": {},
			},
			key:  "test-key",
			want: "",
		},
		{
			name:  "nil extra map",
			extra: nil,
			key:   "any-key",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFirstExtraValue(tt.extra, tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}
