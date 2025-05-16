package crypto

import (
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/stretchr/testify/require"
)

func init() {
	config.Config.Infura.Secret = "533c2b6ac99b4f11b513d25cfb5dffd1" //test secret, test use only
	config.Config.Infura.ApiUrl = "https://base-sepolia.infura.io/v3/"
}

func TestVerifySafeSignature(t *testing.T) {
	message := "localhost:3000 wants you to sign in with your Ethereum account:\n0xfad0050957E9261660FFd24BB49D42D920430FC8\n\nBy confirming this signature and engaging with our platform, you confirm your status as the rightful account manager or authorized representative for the wallet address 0xfad0050957E9261660FFd24BB49D42D920430FC8. This action grants permission for a login attempt on the https://localhost:3000 portal. Your interaction with our site signifies your acceptance of Ratio1's EULA, Terms of Service, and Privacy Policy, as detailed in our official documentation. You acknowledge having fully reviewed these documents, accessible through our website. We strongly advise familiarizing yourself with these materials to fully understand our data handling practices and your entitlements as a user\n\nURI: https://localhost:3000\nVersion: 1\nChain ID: 84532\nNonce: y2vpv6eRpK0RcanEe\nIssued At: 2025-05-16T08:22:48.790Z"
	signature := "0x8af3db77930e412b88733494ca8a18e04e55673cac7e7040012ebedf8c58157c4861e6c7247467f8037238b62e0bd58257ae541fee775dfa907874e0f9461ecf1b"
	address := "0xfad0050957E9261660FFd24BB49D42D920430FC8"
	err := VerifySafeSignature(address, message, signature)
	require.Nil(t, err)
}
