package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBearerToken_Valid(t *testing.T) {
	token, err := ExtractBearerToken("Bearer my-token-123")
	assert.NoError(t, err)
	assert.Equal(t, "my-token-123", token)
}

func TestExtractBearerToken_Empty(t *testing.T) {
	_, err := ExtractBearerToken("")
	assert.Error(t, err)
}

func TestExtractBearerToken_NoBearer(t *testing.T) {
	_, err := ExtractBearerToken("Basic abc123")
	assert.Error(t, err)
}

func TestExtractBearerToken_BearerOnly(t *testing.T) {
	_, err := ExtractBearerToken("Bearer ")
	assert.Error(t, err)
}
