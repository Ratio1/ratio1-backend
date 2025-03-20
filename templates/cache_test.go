package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func init() {
	LoadAndCacheTemplates()
}

func TestTemplateGetters(t *testing.T) {
	c, err := GetConfirmEmailTemplate()
	require.Nil(t, err)
	require.Equal(t, c.Name(), emailConfirmFile)

	c, err = GetFinalRejectedEmailTemplate()
	require.Nil(t, err)
	require.Equal(t, c.Name(), emailKycRejectedFile)

	c, err = GetStepRejectedEmailTemplate()
	require.Nil(t, err)
	require.Equal(t, c.Name(), emailStepRejectedFile)

	c, err = GetBlacklistedEmailTemplate()
	require.Nil(t, err)
	require.Equal(t, c.Name(), emailBlacklistedFile)

	c, err = GetKycConfirmedEmailTemplate()
	require.Nil(t, err)
	require.Equal(t, c.Name(), emailKycConfirmedFile)
}
