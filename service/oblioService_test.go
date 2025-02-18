package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_oblio(t *testing.T) {
	name := "John"
	surname := "Doe"
	//companyName := "ACME"
	userEmail := "alberto.bast2001@gmail.com"
	numLicenses := 1
	unitsUsdPrice := 500
	status := model.InvoiceStatusPending
	invoiceData := model.InvoiceClient{
		Uuid:               nil,
		Name:               &name,
		Surname:            &surname,
		CompanyName:        nil,
		UserEmail:          &userEmail,
		IdentificationCode: "1234",
		Address:            "1234 Main St",
		State:              "CA",
		City:               "San Francisco",
		Country:            "USA",

		IsCompany: false,
		Status:    &status,
	}

	InvoiceRequest := model.Event{
		Address:      "0x1234",
		InvoiceID:    "1234",
		NumLicenses:  numLicenses,
		UnitUsdPrice: unitsUsdPrice,
		TokenPaid:    15000,
	}
	url, invoiceNumber, err := generateInvoice(invoiceData, InvoiceRequest)
	require.Nil(t, err)
	fmt.Println(url, invoiceNumber)
}

func Test_ElaborateInvoice(t *testing.T) {
	ElaborateInvoices()
}
func Test_etchfetchEvent(t *testing.T) {
	fetchEvents(nil)
}

func Test_decodeTest(t *testing.T) {
	// Dati dell'evento
	dataAsString := "0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000001f400000000000000000000000000000000000000000000001b1ae4d6e2ef500000"
	data, _ := hex.DecodeString(dataAsString[2:])
	vLog := types.Log{
		Topics: []common.Hash{
			common.HexToHash("0x7b1ae72a7677952e69429bbcf5b43e6f15af8eda659e4c740f79bafa846fade3"),
			common.HexToHash("0x00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c8"), // to address
			common.HexToHash("0x6431386163333938396165373464613339386338616232366465343162623763"), // invoiceUuid
		},
		Data: data,
	}

	// Decodifica dei log
	event, err := decodeLogs(vLog)
	require.Nil(t, err)
	fmt.Println(event.Address)
}

func Test(t *testing.T) {
	uuid := "6431386163333938396165373464613339386338616232366465343162623763"
	new, _ := hex.DecodeString(uuid)
	fmt.Println(string(new))
	uuidExp := "d18ac398-9ae7-4da3-98c8-ab26de41bb7c"
	uuidExp = strings.ReplaceAll(uuidExp, "-", "")
	assert.Equal(t, string(new), uuidExp)
}
