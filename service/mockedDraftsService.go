package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
)

// ------------------------ MOCKS ------------------------
var (
	mutexForMock sync.Mutex

	mockedInvoiceDraftsForCSP       []model.InvoiceDraft
	mockedInvoiceDraftsForNodeOwner []model.InvoiceDraft
	mockedCSPAllocations            []model.Allocation
	mockedOperatorAllocations       []model.Allocation
)

func BuildMocks() {
	mutexForMock.Lock()
	now := time.Now().UTC()

	cspOwnerAddr := "0xCSP00000000000000000000000000000000000001"
	nodeOwnerAddr := "0xNODE000000000000000000000000000000000001"

	cspPool := []string{
		cspOwnerAddr,
		"0xCSP00000000000000000000000000000000000002",
		"0xCSP00000000000000000000000000000000000003",
		"0xCSP00000000000000000000000000000000000004",
	}

	nodeAddresses := []string{
		"0xNODE000000000000000000000000000000000002",
		"0xNODE000000000000000000000000000000000003",
		"0xNODE000000000000000000000000000000000004",
	}

	clientCount := 24
	clientUsers := make([]string, 0, clientCount)
	for i := 1; i <= clientCount; i++ {
		addr := fmt.Sprintf("0xUSER%038d", i)
		clientUsers = append(clientUsers, addr)
	}

	userAddressSet := map[string]struct{}{}

	allUserInfos := map[string]model.UserInfo{}
	allUserInfos[cspOwnerAddr] = model.UserInfo{
		BlockchainAddress:  cspOwnerAddr,
		Email:              "billing@prime-csp.example",
		Name:               strPtr("Prime"),
		Surname:            strPtr("CSP"),
		CompanyName:        strPtr("Prime CSP S.p.A."),
		IdentificationCode: "IT-CSP-001",
		Address:            "Via Roma 1",
		State:              "Lazio",
		City:               "Roma",
		Country:            "IT",
		IsCompany:          true,
	}
	for i := 2; i <= 4; i++ {
		addr := fmt.Sprintf("0xCSP%038d", i)
		allUserInfos[addr] = model.UserInfo{
			BlockchainAddress:  addr,
			Email:              fmt.Sprintf("info@csp-%d.example", i),
			Name:               strPtr("CSP"),
			Surname:            strPtr(fmt.Sprintf("N.%d", i)),
			CompanyName:        strPtr(fmt.Sprintf("Cloud %d SRL", i)),
			IdentificationCode: fmt.Sprintf("IT-CSP-%03d", i),
			Address:            "Via Milano 10",
			State:              "Lombardia",
			City:               "Milano",
			Country:            "IT",
			IsCompany:          true,
		}
	}

	allUserInfos[nodeOwnerAddr] = model.UserInfo{
		BlockchainAddress:  nodeOwnerAddr,
		Email:              "owner@node.example",
		Name:               strPtr("Node"),
		Surname:            strPtr("Owner"),
		CompanyName:        strPtr("Node Ops SRL"),
		IdentificationCode: "IT-NODE-001",
		Address:            "Corso Francia 22",
		State:              "Piemonte",
		City:               "Torino",
		Country:            "IT",
		IsCompany:          true,
	}

	// Client users profile
	for i, addr := range clientUsers {
		allUserInfos[addr] = model.UserInfo{
			BlockchainAddress:  addr,
			Email:              fmt.Sprintf("user%d@example.com", i+1),
			Name:               strPtr("User"),
			Surname:            strPtr(fmt.Sprintf("N.%d", i+1)),
			CompanyName:        nil,
			IdentificationCode: fmt.Sprintf("IT-USER-%03d", i+1),
			Address:            "Via Firenze 12",
			State:              "Toscana",
			City:               "Firenze",
			Country:            "IT",
			IsCompany:          false,
		}
	}

	// ---------------- DRAFTS PER CSP (≥20) ----------------
	minDrafts := 20
	for i := 0; i < minDrafts; i++ {
		userAddr := clientUsers[i%len(clientUsers)]
		userAddressSet[userAddr] = struct{}{}

		invSeries := "CSP-2025"
		invNumber := 1000 + i + 1

		draftID := uuid.New()
		usdc := 50.0 + float64(i%7)*12.5
		vat := 22.0
		lc := "EUR"
		ratio := 1.00

		ud := allUserInfos[userAddr]
		cd := allUserInfos[cspOwnerAddr]

		d := model.InvoiceDraft{
			DraftId:                    draftID,
			CreationTimestamp:          now.Add(-time.Duration(24*i) * time.Hour),
			UserAddress:                userAddr,
			CspOwner:                   cspOwnerAddr,
			TotalUsdcAmount:            usdc,
			VatApplied:                 vat,
			InvoiceSeries:              invSeries,
			InvoiceNumber:              invNumber,
			ExtraText:                  strPtr("mocked data, non existent values"),
			ExtraTaxes:                 strPtr(genExtraTaxesJSON()),
			LocalCurrency:              lc,
			LocalCurrencyExchangeRatio: ratio,
			CspProfile:                 cd,
			UserProfile:                ud,
		}
		mockedInvoiceDraftsForCSP = append(mockedInvoiceDraftsForCSP, d)

		alloc := model.Allocation{
			Id:                 uint(i + 1),
			AllocationCreation: d.CreationTimestamp.Add(30 * time.Minute),
			BlockNumber:        int64(10_000 + i),
			TxHash:             fmt.Sprintf("0xALLOCTx%064d", i+1),
			JobId:              fmt.Sprintf("job-csp-%04d", i+1),
			JobName:            fmt.Sprintf("Render batch %d", i+1),
			JobType:            model.JobType(i % 2),
			ProjectName:        "CSP Workloads",
			NodeAddress:        nodeAddresses[i%len(nodeAddresses)],
			UserAddress:        userAddr,
			CspAddress:         cspOwnerAddr,
			CspOwner:           cspOwnerAddr,
			UsdcAmountPayed:    fmt.Sprintf("%.2f", usdc),
			DraftId:            &d.DraftId,
			CspProfile:         cd,
			UserProfile:        ud,
		}
		mockedCSPAllocations = append(mockedCSPAllocations, alloc)
	}

	// ------------- DRAFTS PER NODE OWNER (≥20) -------------
	for i := 0; i < minDrafts; i++ {
		userAddr := nodeOwnerAddr
		userAddressSet[userAddr] = struct{}{}

		csp := cspPool[i%len(cspPool)]
		invSeries := "NODE-2025"
		invNumber := 2000 + i + 1

		draftID := uuid.New()
		usdc := 80.0 + float64(i%5)*20.0
		vat := 22.0
		lc := "EUR"
		ratio := 1.00

		ud := allUserInfos[userAddr]
		cd := allUserInfos[csp]

		d := model.InvoiceDraft{
			DraftId:                    draftID,
			CreationTimestamp:          now.Add(-time.Duration(12*i) * time.Hour),
			UserAddress:                userAddr,
			CspOwner:                   csp,
			TotalUsdcAmount:            usdc,
			VatApplied:                 vat,
			InvoiceSeries:              invSeries,
			InvoiceNumber:              invNumber,
			ExtraText:                  strPtr("mocked data, non existent values"),
			ExtraTaxes:                 strPtr(genExtraTaxesJSON()),
			LocalCurrency:              lc,
			LocalCurrencyExchangeRatio: ratio,
			CspProfile:                 cd,
			UserProfile:                ud,
		}
		mockedInvoiceDraftsForNodeOwner = append(mockedInvoiceDraftsForNodeOwner, d)

		alloc := model.Allocation{
			Id:                 uint(i + 1),
			AllocationCreation: d.CreationTimestamp.Add(20 * time.Minute),
			BlockNumber:        int64(20_000 + i),
			TxHash:             fmt.Sprintf("0xNODETx%064d", i+1),
			JobId:              fmt.Sprintf("job-node-%04d", i+1),
			JobName:            fmt.Sprintf("Inference task %d", i+1),
			JobType:            model.JobType((i + 1) % 2),
			ProjectName:        "Node Owner Jobs",
			NodeAddress:        nodeAddresses[(i+1)%len(nodeAddresses)],
			UserAddress:        userAddr,
			CspAddress:         csp,
			CspOwner:           csp,
			UsdcAmountPayed:    fmt.Sprintf("%.2f", usdc),
			DraftId:            &d.DraftId,
			CspProfile:         cd,
			UserProfile:        ud,
		}
		mockedOperatorAllocations = append(mockedOperatorAllocations, alloc)
	}
	mutexForMock.Unlock()
}

func GetCspData() ([]model.InvoiceDraft, []model.Allocation) {
	mutexForMock.Lock()
	defer mutexForMock.Unlock()
	return mockedInvoiceDraftsForCSP, mockedCSPAllocations
}

func GetOperatorData() ([]model.InvoiceDraft, []model.Allocation) {
	mutexForMock.Lock()
	defer mutexForMock.Unlock()
	return mockedInvoiceDraftsForNodeOwner, mockedOperatorAllocations
}

// ------------------------ Helpers ------------------------
func strPtr(s string) *string { return &s }

func genExtraTaxesJSON() string {
	n := rand.Intn(4) + 1
	items := make([]model.ExtraTax, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, model.ExtraTax{
			Description: fmt.Sprintf("Random fee %d", rand.Intn(1000)),
			TaxType:     model.TaxTypeEnum(rand.Intn(2)),
			Value:       rand.Float64()*30 + rand.Float64(),
		})
	}
	b, _ := json.Marshal(items)
	return string(b)
}
