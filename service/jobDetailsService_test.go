package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_createRequest(t *testing.T) {
	resp, err := GetJobDetails("66", "https://deeploy-api.ratio1.ai/get_oracle_job_details")
	require.Nil(t, err)
	data, _ := json.MarshalIndent(resp, "", " ")
	fmt.Println(string(data))
}
