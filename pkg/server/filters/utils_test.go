package filters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authentication/user"
)

func TestUserWithExtra(t *testing.T) {
	tests := []struct {
		name         string
		user         *user.DefaultInfo
		extra        map[string][]string
		expectedUser *user.DefaultInfo
	}{
		{
			name: "nil extra in source user",
			user: &user.DefaultInfo{
				Name:  "test",
				Extra: nil,
			},
			extra: map[string][]string{
				"test": {"value"},
			},
			expectedUser: &user.DefaultInfo{
				Name: "test",
				Extra: map[string][]string{
					"test": {"value"},
				},
			},
		},
		{
			name: "existing extra in source user",
			user: &user.DefaultInfo{
				Name: "test",
				Extra: map[string][]string{
					"existing": {"value"},
				},
			},
			extra: map[string][]string{
				"test": {"value"},
			},
			expectedUser: &user.DefaultInfo{
				Name: "test",
				Extra: map[string][]string{
					"existing": {"value"},
					"test":     {"value"},
				},
			},
		},
		{
			name: "existing extra in source user with key overwrite",
			user: &user.DefaultInfo{
				Name: "test",
				Extra: map[string][]string{
					"existing": {"value"},
				},
			},
			extra: map[string][]string{
				"existing": {"new"},
			},
			expectedUser: &user.DefaultInfo{
				Name: "test",
				Extra: map[string][]string{
					"existing": {"new"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := userWithExtra(tt.user, tt.extra)
			assert.NotSame(t, tt.user, u)
			assert.Equal(t, tt.expectedUser, u)
		})
	}
}
