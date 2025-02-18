package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func init() {
	/*config.Config.Mail = config.MailConfig{
		ApiUrl:     "https://api.postmarkapp.com",
		ApiKey:     "cca9f805-4cf7-4601-9111-982fb0d4152a",
		ConfirmUrl: "http://127.0.0.1:5000/accounts/email/confirm",
		FromEmail:  "contact.test@bhero.com",
	}*/
}

func TestSendConfirmEmail(t *testing.T) {
	err := SendConfirmEmail("erd1qqqqqqqqqqqqqpgql6pu22ycaclatr6p55ms4mnx3qe2gwd6vens5smjkw", "alessandro@bh.network")
	require.Nil(t, err)
}

func TestSendKycConfirmedEmail(t *testing.T) {
	err := SendKycConfirmedEmail("alessandro@bh.network")
	require.Nil(t, err)
}

func TestSendStepRejectedEmail(t *testing.T) {
	err := SendStepRejectedEmail("alessandro@bh.network")
	require.Nil(t, err)
}
