package templates

import (
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/stretchr/testify/require"
)

func init() {
	config.Config.EmailTemplatesPath = "../templates/html/"
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

	c, err = GetJobsEndingEmailTemplate()
	require.Nil(t, err)
	require.Equal(t, c.Name(), emailJobsEndingFile)

	c, err = GetNodesOfflineEmailTemplate()
	require.Nil(t, err)
	require.Equal(t, c.Name(), emailNodesOfflineFile)
}
