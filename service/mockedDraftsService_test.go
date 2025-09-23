package service

import (
	"os"
	"testing"
)

func Test_mockedData(t *testing.T) {
	BuildMocks()
	i, a := GetMockCspData()
	file, _ := GenerateInvoiceDOC(i[0], a)
	os.WriteFile("csp.html", file, 0644)
	i, a = GetMockOperatorData()
	file, _ = GenerateInvoiceDOC(i[0], a)
	os.WriteFile("operator.html", file, 0644)
}
