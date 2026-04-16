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

	// 1. Попытка через GetSecret (data) -> Должна быть ошибка (создание запрещено)
	_, err := client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/name/test.com",
		Property: "bundle",
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, esv1.NoSecretErr)

	// 2. Попытка через GetSecretMap (dataFrom) -> Должно создать с SAN
	_, err = client.GetSecretMap(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/name/test.com",
		Property: "www,api",
	})
	assert.Error(t, err) // Статус 0 = not ready
	assert.Contains(t, err.Error(), "certificate is not ready")
	assert.Contains(t, err.Error(), "status: 0")
	assert.ElementsMatch(t, []string{"test.com", "www.test.com", "api.test.com"}, provisionedDomains)
}

func TestCertificate_Fetch_Existing_In_Data(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(server.URL, "test-token")

	mux.HandleFunc("/api/v1/certificate/get", func(w http.ResponseWriter, r *http.Request) {
		certs := []map[string]any{
			{
				"id":   1,
				"name": "test.com",
				"pem": map[string]string{
					"public":  "PUB",
					"private": "PRIV",
					"ca":      "CA",
				},
				"status": 30,
			},
		}
		json.NewEncoder(w).Encode(certs)
	})

	ctx := context.Background()

	// 1. Получение конкретного поля (ca)
	val, err := client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/name/test.com",
		Property: "ca",
	})
	assert.NoError(t, err)
	assert.Equal(t, "CA", string(val))

	// 2. Получение без property (ошибка)
	_, err = client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key: "rsa/name/test.com",
	})
	assert.Error(t, err)
	assert.Equal(t, "property is required in data mode", err.Error())

	// 3. Получение в режиме dataFrom (GetSecretMap)
	resMap, err := client.GetSecretMap(ctx, esv1.ExternalSecretDataRemoteRef{
		Key: "rsa/name/test.com",
	})
	assert.NoError(t, err)
	assert.Equal(t, "PUB\nCA", string(resMap["bundle"]))
	assert.Equal(t, "PRIV", string(resMap["key"]))
	assert.Len(t, resMap, 2)
}
