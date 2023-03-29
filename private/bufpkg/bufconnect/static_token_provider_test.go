package bufconnect

import (
	"testing"

	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/stretchr/testify/assert"
)

func TestNewTokenProviderFromContainer(t *testing.T) {
	tokenSet, err := NewTokenProviderFromContainer(app.NewEnvContainer(map[string]string{
		tokenEnvKey: "default",
	}))
	assert.NoError(t, err)
	token := tokenSet.RemoteToken("fake")
	assert.True(t, tokenSet.IsFromEnvVar())
	assert.Equal(t, "default", token)
}

func TestNewTokenProviderFromString(t *testing.T) {
	tokenProvider, err := NewTokenProviderFromString("default")
	assert.NoError(t, err)
	assert.Equal(t, "default", tokenProvider.RemoteToken("host"))
	tokenProvider, err = NewTokenProviderFromString("token1@host1")
	assert.NoError(t, err)
	assert.Equal(t, "token1", tokenProvider.RemoteToken("host1"))
	tokenProvider, err = NewTokenProviderFromString("token1@remote1,token2@remote2")
	assert.NoError(t, err)
	assert.Equal(t, "token1", tokenProvider.RemoteToken("remote1"))
	assert.Equal(t, "token2", tokenProvider.RemoteToken("remote2"))
	_, err = NewTokenProviderFromString("")
	assert.NoError(t, err)
}

func TestInvalidTokens(t *testing.T) {
	invalidTokens := []string{
		"user1@remote1,user2@remote1",
		"user1@remote1,user2@remote2,",
		",token1@host1",
		"token1@host1,",
		"token1@",
		"token1@host1@",
		"@token1",
		"token1@host1,token2",
		",",
		"token,",
		",token",
	}

	for _, token := range invalidTokens {
		_, err := NewTokenProviderFromString(token)
		assert.Error(t, err, "expected %s to be an invalid token, but it wasn't", token)
		_, err = NewTokenProviderFromContainer(app.NewEnvContainer(map[string]string{
			tokenEnvKey: token,
		}))
		assert.Error(t, err, "expected %s to be an invalid token, but it wasn't", token)
	}
}