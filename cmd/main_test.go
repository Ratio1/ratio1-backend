package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestNewApp_ListsMigrationCommand(t *testing.T) {
	app := newApp()
	output := &bytes.Buffer{}
	app.Writer = output

	require.NoError(t, app.Run([]string{"ratio1-api", "--help"}))
	require.Contains(t, output.String(), "migrate")
	require.Contains(t, output.String(), "run database migrations and exit")
}

func TestRunMigrations_RequiresNetwork(t *testing.T) {
	t.Setenv("EE_EVM_NET", "")

	err := newApp().Run([]string{"ratio1-api", "migrate"})
	require.EqualError(t, err, "EE_EVM_NET environment variable not set, cannot load config")
}

func TestNewApp_NoCommandStillRunsAPIAction(t *testing.T) {
	app := newApp()
	called := false
	app.Action = func(_ *cli.Context) error {
		called = true
		return nil
	}

	require.NoError(t, app.Run([]string{"ratio1-api"}))
	require.True(t, called)
}
