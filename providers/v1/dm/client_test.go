/*
Copyright © The ESO Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	"github.com/stretchr/testify/assert"
)

func TestCertificate_Provisioning_Only_In_DataFrom(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(server.URL, "test-token")

	provisionedDomains := []string{}
	mux.HandleFunc("/api/v1/certificate/list", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{})
	})

	mux.HandleFunc("/api/v1/certificate/create/new", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domains     []string `json:"domains"`
			Name        string   `json:"name"`
			RequestedBy string   `json:"requestedBy"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		provisionedDomains = req.Domains
		assert.Equal(t, "test.com", req.Name)
		assert.Equal(t, "ESO", req.RequestedBy)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": 100, "status": 0})
	})

	mux.HandleFunc("/api/v1/certificate/get", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		id := r.URL.Query().Get("id")

		if name == "test.com" && id == "" {
			// (1) findCertificate for GetSecret (data)
			// Return empty to simulate NOT FOUND
			json.NewEncoder(w).Encode([]any{})
			return
		}

		if id == "100" {
			// (3) fetchCertificateEntity after provisioning
			certs := []map[string]any{
				{
					"id":     100,
					"name":   "test.com",
					"status": 0,
					"pem":    map[string]string{},
				},
			}
			json.NewEncoder(w).Encode(certs)
			return
		}

		json.NewEncoder(w).Encode([]any{})
	})

	ctx := context.Background()

	// 1. Попытка через GetSecret (data) -> Должно попытаться создать, но вернуть ошибку (не готов)
	_, err := client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/name/test.com",
		Property: "bundle",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate is not ready (status: 0)")

	// 2. Попытка через GetSecretMap (dataFrom) -> Должно создать с SAN
	_, err = client.GetSecretMap(ctx, esv1.ExternalSecretDataRemoteRef{
		Key: "rsa/name/test.com",
		GeneratorMetaData: &esv1.ExternalSecretGeneratorPayload{
			DNSNames: []string{"www", "api"},
		},
	})
	assert.Error(t, err) // Статус 0 = not ready
	assert.Contains(t, err.Error(), "certificate is not ready (status: 0)")
	assert.ElementsMatch(t, []string{"test.com", "www.test.com", "api.test.com"}, provisionedDomains)
	}

	func TestCertificate_GeneratorParams(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(server.URL, "test-token")

	var capturedReq map[string]any
	mux.HandleFunc("/api/v1/certificate/create/new", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id":     100,
			"name":   "test.com",
			"status": 30, // Ready
			"pem": map[string]string{
				"public":  "PUB",
				"private": "PRIV",
				"ca":      "CA",
			},
		})
	})

	mux.HandleFunc("/api/v1/certificate/get", func(w http.ResponseWriter, r *http.Request) {
		// Mock for the first check (find)
		json.NewEncoder(w).Encode([]any{})
	})

	ctx := context.Background()

	ref := esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/name/test.com",
		Property: "bundle",
		GeneratorMetaData: &esv1.ExternalSecretGeneratorPayload{
			ProviderName: "MY_ISSUER",
			ProviderType: "ZEROSSL",
			Subject: &esv1.ExternalSecretGeneratorSubject{
				CommonName: "Test CN",
			},
			IPAddresses: []string{"1.1.1.1"},
			DNSNames:    []string{"web"},
			Sync:        true,
		},
		}
		resMap, err := client.GetSecretMap(ctx, ref)
		assert.NoError(t, err)
		assert.Len(t, resMap, 1)
		assert.Contains(t, resMap, "bundle")

		assert.Equal(t, "MY_ISSUER", capturedReq["providerName"])
		assert.Equal(t, "ZEROSSL", capturedReq["providerType"])
		assert.Equal(t, true, capturedReq["sync"])
		assert.Equal(t, []any{"1.1.1.1"}, capturedReq["ips"])
		assert.Equal(t, []any{"test.com", "web.test.com"}, capturedReq["domains"])

		subject := capturedReq["subject"].(map[string]any)
		assert.Equal(t, "Test CN", subject["CommonName"])
		}

