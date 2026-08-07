package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"

	"github.com/stretchr/testify/require"
)

func TestVerificationSessionModelHasProviderScopedUniqueIndex(t *testing.T) {
	parsed, err := schema.Parse(&VerificationSession{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	indexes := parsed.ParseIndexes()
	index, found := indexes["idx_verification_provider_session"]
	require.True(t, found)
	require.True(t, index.Class == "UNIQUE" || index.Option == "UNIQUE")
	require.Equal(t, []string{"provider", "environment", "provider_session_id"}, indexFields(index))
}

func TestVerificationWebhookEventModelHasProviderScopedUniqueIndex(t *testing.T) {
	parsed, err := schema.Parse(&VerificationWebhookEvent{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	indexes := parsed.ParseIndexes()
	index, found := indexes["idx_verification_provider_event"]
	require.True(t, found)
	require.True(t, index.Class == "UNIQUE" || index.Option == "UNIQUE")
	require.Equal(t, []string{"provider", "environment", "event_id"}, indexFields(index))
}

func indexFields(index schema.Index) []string {
	fields := make([]string, 0, len(index.Fields))
	for _, field := range index.Fields {
		fields = append(fields, field.DBName)
	}
	return fields
}
