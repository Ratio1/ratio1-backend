package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"
)

const diditWebhookTimestampWindow = 5 * time.Minute

var (
	ErrDiditInvalidSignature = errors.New("invalid didit webhook signature")
	ErrDiditStaleWebhook     = errors.New("didit webhook timestamp is outside the allowed window")
	ErrDiditWebhookEnvelope  = errors.New("invalid didit webhook envelope")
)

type DiditSignatureMethod string

const (
	DiditSignatureV2     DiditSignatureMethod = "v2"
	DiditSignatureRaw    DiditSignatureMethod = "raw"
	DiditSignatureSimple DiditSignatureMethod = "simple"
)

type DiditWebhookSignatureHeaders struct {
	Timestamp string
	V2        string
	Raw       string
	Simple    string
}

type DiditSignatureVerification struct {
	Method          DiditSignatureMethod
	DecisionTrusted bool
}

func VerifyDiditWebhookSignatures(
	body []byte,
	headers DiditWebhookSignatureHeaders,
	secret string,
	now time.Time,
) (DiditSignatureVerification, error) {
	if secret == "" {
		return DiditSignatureVerification{}, ErrDiditInvalidSignature
	}

	headerTimestamp, err := strconv.ParseInt(strings.TrimSpace(headers.Timestamp), 10, 64)
	if err != nil {
		return DiditSignatureVerification{}, ErrDiditWebhookEnvelope
	}
	delta := now.Unix() - headerTimestamp
	if delta > int64(diditWebhookTimestampWindow/time.Second) ||
		delta < -int64(diditWebhookTimestampWindow/time.Second) {
		return DiditSignatureVerification{}, ErrDiditStaleWebhook
	}

	envelope, canonicalBody, err := decodeDiditWebhookEnvelope(body)
	if err != nil {
		return DiditSignatureVerification{}, err
	}
	if envelope.Timestamp != headerTimestamp {
		return DiditSignatureVerification{}, ErrDiditWebhookEnvelope
	}

	if headers.V2 != "" && diditSignatureMatches(canonicalBody, headers.V2, secret) {
		return DiditSignatureVerification{
			Method:          DiditSignatureV2,
			DecisionTrusted: true,
		}, nil
	}
	if headers.Raw != "" && diditSignatureMatches(body, headers.Raw, secret) {
		return DiditSignatureVerification{
			Method:          DiditSignatureRaw,
			DecisionTrusted: true,
		}, nil
	}
	if headers.Simple != "" {
		simplePayload := fmt.Sprintf(
			"%d:%s:%s:%s",
			envelope.Timestamp,
			envelope.SessionId,
			envelope.Status,
			envelope.WebhookType,
		)
		if diditSignatureMatches([]byte(simplePayload), headers.Simple, secret) {
			return DiditSignatureVerification{
				Method:          DiditSignatureSimple,
				DecisionTrusted: false,
			}, nil
		}
	}

	return DiditSignatureVerification{}, ErrDiditInvalidSignature
}

func CanonicalizeDiditWebhookJSON(body []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, ErrDiditWebhookEnvelope
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, ErrDiditWebhookEnvelope
	}

	var canonical bytes.Buffer
	if err := writeDiditCanonicalJSON(&canonical, value); err != nil {
		return nil, ErrDiditWebhookEnvelope
	}
	return canonical.Bytes(), nil
}

type diditWebhookEnvelope struct {
	Timestamp   int64
	SessionId   string
	Status      string
	WebhookType string
}

func decodeDiditWebhookEnvelope(body []byte) (diditWebhookEnvelope, []byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var fields map[string]interface{}
	if err := decoder.Decode(&fields); err != nil {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}

	timestampNumber, ok := fields["timestamp"].(json.Number)
	if !ok {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}
	normalizedTimestamp, err := normalizeDiditJSONNumber(timestampNumber)
	if err != nil || strings.ContainsAny(normalizedTimestamp, ".eE") {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}
	timestamp, err := strconv.ParseInt(normalizedTimestamp, 10, 64)
	if err != nil {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}

	sessionId, _ := fields["session_id"].(string)
	status, ok := fields["status"].(string)
	if !ok || status == "" {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}
	webhookType, ok := fields["webhook_type"].(string)
	if !ok || webhookType == "" {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}

	var canonical bytes.Buffer
	if err = writeDiditCanonicalJSON(&canonical, fields); err != nil {
		return diditWebhookEnvelope{}, nil, ErrDiditWebhookEnvelope
	}

	return diditWebhookEnvelope{
		Timestamp:   timestamp,
		SessionId:   sessionId,
		Status:      status,
		WebhookType: webhookType,
	}, canonical.Bytes(), nil
}

func diditSignatureMatches(payload []byte, providedHex string, secret string) bool {
	provided, err := hex.DecodeString(strings.TrimSpace(providedHex))
	if err != nil || len(provided) != sha256.Size {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return hmac.Equal(mac.Sum(nil), provided)
}

func writeDiditCanonicalJSON(destination *bytes.Buffer, value interface{}) error {
	switch typed := value.(type) {
	case nil:
		destination.WriteString("null")
	case bool:
		destination.WriteString(strconv.FormatBool(typed))
	case string:
		writeDiditCanonicalJSONString(destination, typed)
	case json.Number:
		normalized, err := normalizeDiditJSONNumber(typed)
		if err != nil {
			return err
		}
		destination.WriteString(normalized)
	case []interface{}:
		destination.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				destination.WriteByte(',')
			}
			if err := writeDiditCanonicalJSON(destination, item); err != nil {
				return err
			}
		}
		destination.WriteByte(']')
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		destination.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				destination.WriteByte(',')
			}
			if err := writeDiditCanonicalJSON(destination, key); err != nil {
				return err
			}
			destination.WriteByte(':')
			if err := writeDiditCanonicalJSON(destination, typed[key]); err != nil {
				return err
			}
		}
		destination.WriteByte('}')
	default:
		return errors.New("unsupported JSON value")
	}
	return nil
}

func writeDiditCanonicalJSONString(destination *bytes.Buffer, value string) {
	const hexCharacters = "0123456789abcdef"

	destination.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"', '\\':
			destination.WriteByte('\\')
			destination.WriteRune(character)
		case '\b':
			destination.WriteString(`\b`)
		case '\f':
			destination.WriteString(`\f`)
		case '\n':
			destination.WriteString(`\n`)
		case '\r':
			destination.WriteString(`\r`)
		case '\t':
			destination.WriteString(`\t`)
		default:
			if character < 0x20 {
				destination.WriteString(`\u00`)
				destination.WriteByte(hexCharacters[byte(character)>>4])
				destination.WriteByte(hexCharacters[byte(character)&0x0f])
			} else {
				destination.WriteRune(character)
			}
		}
	}
	destination.WriteByte('"')
}

func normalizeDiditJSONNumber(number json.Number) (string, error) {
	value := number.String()
	if !strings.ContainsAny(value, ".eE") {
		if _, ok := new(big.Int).SetString(value, 10); !ok {
			return "", errors.New("invalid JSON number")
		}
		return value, nil
	}

	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(floatValue, 0) || math.IsNaN(floatValue) {
		return "", errors.New("invalid JSON number")
	}
	if math.Trunc(floatValue) == floatValue {
		if floatValue == 0 {
			return "0", nil
		}
		return strconv.FormatFloat(floatValue, 'f', -1, 64), nil
	}
	return strconv.FormatFloat(floatValue, 'g', -1, 64), nil
}
