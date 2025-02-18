package templates

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfirmEmailTemplate(t *testing.T) {
	templ, err := LoadConfirmEmailTemplate()
	require.Nil(t, err)
	require.True(t, templ.Name() == "email.confirm.html")
}

func TestLoadConfirmEmailTemplate_ExecuteReplace(t *testing.T) {
	templ, err := LoadConfirmEmailTemplate()
	require.Nil(t, err)

	var body bytes.Buffer
	err = templ.Execute(&body, struct {
		Url string
	}{
		Url: "https://localhost:5000/hexhexhexhex",
	})
	require.Nil(t, err)
	t.Log(body.String())
}
