package authn

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type AuthnToken interface {
	// Parse from JSON. Required before further usage.
	FromJSON(data []byte) error
	// Raw token as obtained from the authentication service.
	Raw() []byte
	// Whether the token will expire soon.
	ShouldRefresh() bool
}

type AuthnToken4 struct {
	bytes     []byte
	Data      string `json:"data"`
	timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
	Key       string `json:"key"`
	Timestamp time.Time
}

// Sample token
// {"protected":"eyJhbGciOiJjb25qdXIub3JnL3Nsb3NpbG8vdjIiLCJraWQiOiI5M2VjNTEwODRmZTM3Zjc3M2I1ODhlNTYyYWVjZGMxMSJ9","payload":"eyJzdWIiOiJhZG1pbiIsImlhdCI6MTUxMDc1MzI1OX0=","signature":"raCufKOf7sKzciZInQTphu1mBbLhAdIJM72ChLB4m5wKWxFnNz_7LawQ9iYEI_we1-tdZtTXoopn_T1qoTplR9_Bo3KkpI5Hj3DB7SmBpR3CSRTnnEwkJ0_aJ8bql5Cbst4i4rSftyEmUqX-FDOqJdAztdi9BUJyLfbeKTW9OGg-QJQzPX1ucB7IpvTFCEjMoO8KUxZpbHj-KpwqAMZRooG4ULBkxp5nSfs-LN27JupU58oRgIfaWASaDmA98O2x6o88MFpxK_M0FeFGuDKewNGrRc8lCOtTQ9cULA080M5CSnruCqu1Qd52r72KIOAfyzNIiBCLTkblz2fZyEkdSKQmZ8J3AakxQE2jyHmMT-eXjfsEIzEt-IRPJIirI3Qm"}
// https://www.conjur.org/reference/cryptography.html
type AuthnToken5 struct {
	bytes     []byte
	Protected string `json:"protected"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
	iat       time.Time
	exp       *time.Time
}

func hasField(fields map[string]string, name string) (hasField bool) {
	_, hasField = fields[name]
	return
}

func NewToken(data []byte) (token AuthnToken, err error) {
	fields := make(map[string]string)
	if err = json.Unmarshal(data, &fields); err != nil {
		err = fmt.Errorf("Unable to unmarshal token : %s", err)
		return
	}

	if hasField(fields, "protected") && hasField(fields, "payload") && hasField(fields, "signature") {
		t := &AuthnToken5{}
		token = t
	} else if hasField(fields, "data") && hasField(fields, "timestamp") && hasField(fields, "signature") && hasField(fields, "key") {
		t := &AuthnToken4{}
		token = t
	} else {
		err = fmt.Errorf("Unrecognized token format")
		return
	}
	return
}

func (t *AuthnToken5) FromJSON(data []byte) (err error) {
	t.bytes = data

	err = json.Unmarshal(data, &t)
	if err != nil {
		err = fmt.Errorf("Unable to unmarshal v5 access token %s", err)
		return
	}

	// Example: {"sub":"admin","iat":1510753259}
	payloadFields := make(map[string]interface{})
	var payloadJSON []byte
	payloadJSON, err = base64.StdEncoding.DecodeString(t.Payload)
	if err != nil {
		err = fmt.Errorf("v5 access token field 'payload' is not valid base64")
		return
	}
	err = json.Unmarshal(payloadJSON, &payloadFields)
	if err != nil {
		err = fmt.Errorf("Unable to unmarshal v5 access token field 'payload' : %s", err)
		return
	}

	iat_v, ok := payloadFields["iat"]
	if !ok {
		err = fmt.Errorf("v5 access token field 'payload' does not contain 'iat'")
		return
	}
	iat_f := iat_v.(float64)
	// In the absence of exp, the token expires at iat+8 minutes
	t.iat = time.Unix(int64(iat_f), 0)

	exp_v, ok := payloadFields["exp"]
	if ok {
		exp_f := exp_v.(float64)
		exp := time.Unix(int64(exp_f), 0)
		t.exp = &exp
		if t.iat.After(*t.exp) {
			err = fmt.Errorf("v5 access token expired before it was issued")
			return
		}
	}

	return
}

func (t *AuthnToken4) FromJSON(data []byte) (err error) {
	t.bytes = data

	err = json.Unmarshal(data, &t)
	if err != nil {
		err = fmt.Errorf("Unable to unmarshal v4 access token %s", err)
		return
	}

	t.Timestamp, err = time.Parse("2006-01-02 15:04:05 MST", t.timestamp)
	if err != nil {
		err = fmt.Errorf("Unable to parse v4 access token field 'timestamp' %s : %s", t.Timestamp, err)
		return
	}

	return
}

func (t *AuthnToken5) Raw() []byte {
	return t.bytes
}

func (t *AuthnToken5) ShouldRefresh() bool {
	if t.exp != nil {
		// Expire when the token is 85% expired
		lifespan := t.exp.Sub(t.iat)
		duration := float32(lifespan) * 0.85
		return time.Now().After(t.iat.Add(time.Duration(duration)))
	} else {
		// Token expires 8 minutes after issue, by default
		return time.Now().After(t.iat.Add(5 * time.Minute))
	}
}

func (t *AuthnToken4) Raw() []byte {
	return t.bytes
}

func (t *AuthnToken4) ShouldRefresh() bool {
	return t.Timestamp.Add(5 * time.Minute).After(time.Now())
}
