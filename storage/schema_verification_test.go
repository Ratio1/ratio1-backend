package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyMigrationIndexDefinition(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		wantError  string
	}{
		{
			name: "PostgreSQL definition",
			definition: "CREATE INDEX idx_allocations_job_latest ON public.allocations USING btree " +
				"(job_id, block_number DESC, allocation_creation DESC, id DESC) " +
				"WHERE ((job_name IS NOT NULL) AND ((job_name)::text <> ''::text))",
		},
		{
			name: "CockroachDB definition",
			definition: "CREATE INDEX idx_allocations_job_latest ON ratio1.public.allocations USING btree " +
				"(job_id ASC, block_number DESC, allocation_creation DESC, id DESC) " +
				"WHERE ((job_name IS NOT NULL) AND (job_name != ''::STRING))",
		},
		{
			name: "unique index",
			definition: "CREATE UNIQUE INDEX idx_allocations_job_latest ON public.allocations USING btree " +
				"(job_id, block_number DESC, allocation_creation DESC, id DESC) " +
				"WHERE ((job_name IS NOT NULL) AND ((job_name)::text <> ''::text))",
			wantError: "verify migrated schema: index idx_allocations_job_latest is unexpectedly unique",
		},
		{
			name: "additional predicate",
			definition: "CREATE INDEX idx_allocations_job_latest ON public.allocations USING btree " +
				"(job_id, block_number DESC, allocation_creation DESC, id DESC) " +
				"WHERE ((job_name IS NOT NULL) AND ((job_name)::text <> ''::text) AND false)",
			wantError: "verify migrated schema: index idx_allocations_job_latest has unexpected predicate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := verifyMigrationIndexDefinition(
				"idx_allocations_job_latest",
				test.definition,
				"using btree (job_id, block_number desc, allocation_creation desc, id desc)",
				"job_name is not null and job_name != ''",
			)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, test.wantError)
		})
	}
}
