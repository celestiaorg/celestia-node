package authtoken

import (
	"encoding/json"
	"testing"

	"github.com/cristalhq/jwt"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/stretchr/testify/assert"
)

type badTokenBuilder struct{}

func (j *badTokenBuilder) MarshalBinary() (data []byte, err error) {
	return []byte("````````bad89045353@@@"), nil
}

func TestMalformedPermissionsToken(t *testing.T) {
	signer, err := jwt.NewHS256(make([]byte, 32))
	if err != nil {
		t.Fatal(err)

	}
	tk := &badTokenBuilder{}
	token, err := jwt.NewTokenBuilder(signer).BuildBytes(tk)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ExtractSignedPermissions(signer, string(token))
	assert.Error(t, err)
	// assert.Equal(t, []auth.Permission(nil), perms)
}

type invalidTokenBuilder struct{ Message string }

func (j *invalidTokenBuilder) MarshalBinary() (data []byte, err error) {
	return json.Marshal(j)
}

func TestNoPermissions(t *testing.T) {
	signer, err := jwt.NewHS256(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}

	tk := &invalidTokenBuilder{
		Message: "i'm bad",
	}
	token, err := jwt.NewTokenBuilder(signer).BuildBytes(tk)
	if err != nil {
		t.Fatal(err)
	}

	perms, err := ExtractSignedPermissions(signer, string(token))
	assert.Nil(t, err)
	assert.Equal(t, []auth.Permission(nil), perms)
}

func TestExtractSignedPermissions(t *testing.T) {
	signer, err := jwt.NewHS256(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		token         string
		expectedPerms []auth.Permission
		expectedErr   error
	}{
		{
			name: "ValidToken",
			expectedPerms: []auth.Permission{
				"read",
			},
			expectedErr: nil,
		},
		{
			name:          "InvalidToken",
			expectedPerms: nil,
			expectedErr:   nil,
		},
		// Add more test cases as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := NewSignedJWT(signer, tt.expectedPerms)
			if err != nil {
				t.Fatal(err)
			}
			perms, err := ExtractSignedPermissions(signer, token)
			assert.Equal(t, tt.expectedPerms, perms)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}
