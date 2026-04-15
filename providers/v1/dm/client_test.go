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

func TestGetSecret_Certificate_DomainID_Provisioning(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(server.URL, "test-token")

	provisionedDomains := []string{}
	mux.HandleFunc("/api/v1/certificate/list", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{})
	})

	mux.HandleFunc("/api/v1/domain/list", func(w http.ResponseWriter, r *http.Request) {
		domains := []map[string]any{
			{"id": 42, "name": "resolved.com", "typeID": 1, "status": 40},
		}
		json.NewEncoder(w).Encode(domains)
	})

	mux.HandleFunc("/api/v1/certificate/create", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string   `json:"name"`
			Domains []string `json:"domains"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		provisionedDomains = req.Domains
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "status": 0})
	})

	ctx := context.Background()

	// Тест: запрос по domain_id с субдоменами в property
	_, err := client.GetSecretMap(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/domain_id/42",
		Property: "www,api",
	})

	assert.Error(t, err) // Статус 0, ожидаем "not ready"
	assert.ElementsMatch(t, []string{"resolved.com", "www.resolved.com", "api.resolved.com"}, provisionedDomains)
}

func TestGetSecret_Certificate_Aliases(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(server.URL, "test-token")

	mux.HandleFunc("/api/v1/certificate/list", func(w http.ResponseWriter, r *http.Request) {
		certs := []map[string]any{
			{"id": 1, "name": "test.com", "keyType": "RSA", "status": 30},
		}
		json.NewEncoder(w).Encode(certs)
	})

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
			},
		}
		json.NewEncoder(w).Encode(certs)
	})

	ctx := context.Background()

	// Проверяем ca.crt
	val, err := client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/domain/test.com",
		Property: "ca.crt",
	})
	assert.NoError(t, err)
	assert.Equal(t, "CA", string(val))

	// Проверяем fullchain (tls.crt)
	val, err = client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/domain/test.com",
		Property: "tls.crt",
	})
	assert.NoError(t, err)
	assert.Equal(t, "PUB\nCA", string(val))

	// Проверяем ca.crt (новый алиас)
	val, err = client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key:      "rsa/domain/test.com",
		Property: "ca.crt",
	})
	assert.NoError(t, err)
	assert.Equal(t, "CA", string(val))
}

func TestGetSecret_Domain_TypeID(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(server.URL, "test-token")

	mux.HandleFunc("/api/v1/domain/list", func(w http.ResponseWriter, r *http.Request) {
		domains := []map[string]any{
			{"id": 1, "name": "a.com", "typeID": 42, "status": 40},
			{"id": 2, "name": "b.com", "typeID": 42, "status": 40},
			{"id": 3, "name": "c.com", "typeID": 1, "status": 40},
		}
		json.NewEncoder(w).Encode(domains)
	})

	ctx := context.Background()

	// Тест: запрос списка доменов по type_id
	val, err := client.GetSecret(ctx, esv1.ExternalSecretDataRemoteRef{
		Key: "domain/type_id/42",
	})
	assert.NoError(t, err)
	assert.Equal(t, "a.com,b.com", string(val))
}
