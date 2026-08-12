package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestQueryIndexes(t *testing.T) {
	tests := []struct {
		name    string
		model   any
		indexes map[string][]string
		where   map[string]string
	}{
		{
			name:  "allocations",
			model: &Allocation{},
			indexes: map[string][]string{
				"idx_allocations_draft_creation": {"draft_id", "allocation_creation"},
				"idx_allocations_job_latest":     {"job_id", "block_number desc", "allocation_creation desc", "id desc"},
			},
			where: map[string]string{
				"idx_allocations_job_latest": "job_name IS NOT NULL AND job_name <> ''",
			},
		},
		{
			name:  "burn events",
			model: &BurnEvent{},
			indexes: map[string][]string{
				"idx_burn_events_owner_block": {"csp_owner", "block_number desc"},
			},
		},
		{
			name:  "KYC",
			model: &Kyc{},
			indexes: map[string][]string{
				"idx_kycs_email": {"email"},
			},
		},
		{
			name:  "invoice clients",
			model: &InvoiceClient{},
			indexes: map[string][]string{
				"idx_invoice_clients_block": {"block_number desc"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := schema.Parse(test.model, &sync.Map{}, schema.NamingStrategy{})
			require.NoError(t, err)

			indexes := make(map[string][]string)
			for _, index := range parsed.ParseIndexes() {
				fields := make([]string, 0, len(index.Fields))
				for _, field := range index.Fields {
					name := field.DBName
					if field.Sort != "" {
						name += " " + field.Sort
					}
					fields = append(fields, name)
				}
				indexes[index.Name] = fields
			}

			for name, fields := range test.indexes {
				require.Equal(t, fields, indexes[name], name)
			}
			for name, where := range test.where {
				require.Equal(t, where, parsed.LookIndex(name).Where, name)
			}
		})
	}
}
