package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	diditSignatureTestSecret = "whsec_test_only_not_a_real_secret"
	diditSignatureTimestamp  = int64(1774970000)
	diditSignatureRawBody    = `{"status":"Approved","session_id":"00000000-0000-4000-8000-000000000101","metadata":{"name":"José","tier":"test"},"timestamp":1774970000,"webhook_type":"status.updated","event_id":"00000000-0000-4000-8000-000000000401"}`
	diditSignatureCanonical  = `{"event_id":"00000000-0000-4000-8000-000000000401","metadata":{"name":"José","tier":"test"},"session_id":"00000000-0000-4000-8000-000000000101","status":"Approved","timestamp":1774970000,"webhook_type":"status.updated"}`
	diditSignatureRawHex     = "ebff26da50aad3564b93322eed0cc99e7bfd03ea0b13747a593dc6c27600743b"
	diditSignatureV2Hex      = "5ba5e3d645df15ebf923030c4ae804d51e948e1a00697550e60a110b36d389c5"
	diditSignatureSimpleHex  = "74330b52437647bd22ad95d4fcf1c1dc24e70d1a3c4218b5d58e0000766786c8"
)

func TestVerifyDiditWebhookSignatures(t *testing.T) {
	now := time.Unix(diditSignatureTimestamp, 0)
	timestamp := "1774970000"

	tests := []struct {
		name            string
		body            string
		headers         DiditWebhookSignatureHeaders
		expectedMethod  DiditSignatureMethod
		decisionTrusted bool
	}{
		{
			name: "v2",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: timestamp,
				V2:        diditSignatureV2Hex,
			},
			expectedMethod:  DiditSignatureV2,
			decisionTrusted: true,
		},
		{
			name: "v2 accepts reordered pretty JSON",
			body: `{
				"event_id":"00000000-0000-4000-8000-000000000401",
				"webhook_type":"status.updated",
				"timestamp":1774970000,
				"metadata":{"tier":"test","name":"José"},
				"session_id":"00000000-0000-4000-8000-000000000101",
				"status":"Approved"
			}`,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: timestamp,
				V2:        diditSignatureV2Hex,
			},
			expectedMethod:  DiditSignatureV2,
			decisionTrusted: true,
		},
		{
			name: "raw",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: timestamp,
				Raw:       diditSignatureRawHex,
			},
			expectedMethod:  DiditSignatureRaw,
			decisionTrusted: true,
		},
		{
			name: "simple is envelope only",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: timestamp,
				Simple:    diditSignatureSimpleHex,
			},
			expectedMethod:  DiditSignatureSimple,
			decisionTrusted: false,
		},
		{
			name: "preference order",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: timestamp,
				V2:        diditSignatureV2Hex,
				Raw:       diditSignatureRawHex,
				Simple:    diditSignatureSimpleHex,
			},
			expectedMethod:  DiditSignatureV2,
			decisionTrusted: true,
		},
		{
			name: "uppercase hex",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: timestamp,
				V2:        strings.ToUpper(diditSignatureV2Hex),
			},
			expectedMethod:  DiditSignatureV2,
			decisionTrusted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := VerifyDiditWebhookSignatures(
				[]byte(test.body),
				test.headers,
				diditSignatureTestSecret,
				now,
			)
			require.NoError(t, err)
			require.Equal(t, test.expectedMethod, result.Method)
			require.Equal(t, test.decisionTrusted, result.DecisionTrusted)
		})
	}
}

func TestVerifyDiditWebhookSignaturesRejectsInvalidInputs(t *testing.T) {
	now := time.Unix(diditSignatureTimestamp, 0)
	validHeaders := DiditWebhookSignatureHeaders{
		Timestamp: "1774970000",
		V2:        diditSignatureV2Hex,
	}

	tests := []struct {
		name        string
		body        string
		headers     DiditWebhookSignatureHeaders
		secret      string
		expectedErr error
	}{
		{
			name:        "wrong secret",
			body:        diditSignatureRawBody,
			headers:     validHeaders,
			secret:      "wrong",
			expectedErr: ErrDiditInvalidSignature,
		},
		{
			name:        "mutated decision",
			body:        strings.Replace(diditSignatureRawBody, `"Approved"`, `"Declined"`, 1),
			headers:     validHeaders,
			secret:      diditSignatureTestSecret,
			expectedErr: ErrDiditInvalidSignature,
		},
		{
			name: "missing signatures",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: "1774970000",
			},
			secret:      diditSignatureTestSecret,
			expectedErr: ErrDiditInvalidSignature,
		},
		{
			name: "malformed digest",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: "1774970000",
				V2:        "not-hex",
			},
			secret:      diditSignatureTestSecret,
			expectedErr: ErrDiditInvalidSignature,
		},
		{
			name: "short digest",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: "1774970000",
				V2:        "abcd",
			},
			secret:      diditSignatureTestSecret,
			expectedErr: ErrDiditInvalidSignature,
		},
		{
			name: "missing timestamp header",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				V2: diditSignatureV2Hex,
			},
			secret:      diditSignatureTestSecret,
			expectedErr: ErrDiditWebhookEnvelope,
		},
		{
			name: "header and body timestamp mismatch",
			body: diditSignatureRawBody,
			headers: DiditWebhookSignatureHeaders{
				Timestamp: "1774970001",
				V2:        diditSignatureV2Hex,
			},
			secret:      diditSignatureTestSecret,
			expectedErr: ErrDiditWebhookEnvelope,
		},
		{
			name:        "trailing JSON",
			body:        diditSignatureRawBody + `{}`,
			headers:     validHeaders,
			secret:      diditSignatureTestSecret,
			expectedErr: ErrDiditWebhookEnvelope,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := VerifyDiditWebhookSignatures(
				[]byte(test.body),
				test.headers,
				test.secret,
				now,
			)
			require.ErrorIs(t, err, test.expectedErr)
		})
	}
}

func TestVerifyDiditWebhookSignaturesTimestampWindow(t *testing.T) {
	tests := []struct {
		name        string
		now         time.Time
		expectedErr error
	}{
		{name: "exactly 300 seconds old", now: time.Unix(diditSignatureTimestamp+300, 0)},
		{name: "exactly 300 seconds future", now: time.Unix(diditSignatureTimestamp-300, 0)},
		{name: "301 seconds old", now: time.Unix(diditSignatureTimestamp+301, 0), expectedErr: ErrDiditStaleWebhook},
		{name: "301 seconds future", now: time.Unix(diditSignatureTimestamp-301, 0), expectedErr: ErrDiditStaleWebhook},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := VerifyDiditWebhookSignatures(
				[]byte(diditSignatureRawBody),
				DiditWebhookSignatureHeaders{
					Timestamp: "1774970000",
					V2:        diditSignatureV2Hex,
				},
				diditSignatureTestSecret,
				test.now,
			)
			if test.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.expectedErr)
			}
		})
	}
}

func TestCanonicalizeDiditWebhookJSON(t *testing.T) {
	canonical, err := CanonicalizeDiditWebhookJSON([]byte(diditSignatureRawBody))
	require.NoError(t, err)
	require.Equal(t, diditSignatureCanonical, string(canonical))

	body := []byte(`{
		"z":[{"whole":99.0,"fraction":99.5}],
		"html":"A&B <SRL>",
		"unicode":"José",
		"separator":"line\u2028separator",
		"negative_zero":-0.0,
		"a":{"b":2,"a":1}
	}`)
	canonical, err = CanonicalizeDiditWebhookJSON(body)
	require.NoError(t, err)
	require.Equal(
		t,
		"{\"a\":{\"a\":1,\"b\":2},\"html\":\"A&B <SRL>\",\"negative_zero\":0,\"separator\":\"line\u2028separator\",\"unicode\":\"José\",\"z\":[{\"fraction\":99.5,\"whole\":99}]}",
		string(canonical),
	)

	canonical, err = CanonicalizeDiditWebhookJSON([]byte(`{"small":1e-7,"large":1e21}`))
	require.NoError(t, err)
	require.Equal(t, `{"large":1000000000000000000000,"small":1e-07}`, string(canonical))
}

func TestDiditSignatureVectorsAreStable(t *testing.T) {
	require.Equal(t, diditSignatureRawHex, signDiditTestPayload([]byte(diditSignatureRawBody)))
	require.Equal(t, diditSignatureV2Hex, signDiditTestPayload([]byte(diditSignatureCanonical)))
	require.Equal(
		t,
		diditSignatureSimpleHex,
		signDiditTestPayload([]byte("1774970000:00000000-0000-4000-8000-000000000101:Approved:status.updated")),
	)
}

func signDiditTestPayload(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(diditSignatureTestSecret))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
