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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"git.adsrv.wtf/clickadu/domain-manager/modules/apiclient"
	"git.adsrv.wtf/clickadu/domain-manager/modules/models"
	"git.adsrv.wtf/clickadu/domain-manager/modules/web/handlers/certificate"
	corev1 "k8s.io/api/core/v1"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

type apiClientWrapper struct {
	client *apiclient.Client
}

func newClient(baseURL, token string) *apiClientWrapper {
	return &apiClientWrapper{client: apiclient.NewClient(baseURL, token)}
}

func (c *apiClientWrapper) GetSecret(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef) ([]byte, error) {
	if ref.Property == "" {
		return nil, errors.New("property is required in data mode")
	}

	data, err := c.fetchData(ctx, ref, false)
	if err != nil {
		return nil, err
	}

	val, ok := data[ref.Property]
	if !ok {
		return nil, fmt.Errorf("property %s not found", ref.Property)
	}

	return val, nil
}

func (c *apiClientWrapper) GetSecretMap(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef) (map[string][]byte, error) {
	data, err := c.fetchData(ctx, ref, true)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{"bundle": data["bundle"], "key": data["key"]}, nil
}

func (c *apiClientWrapper) fetchData(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef, isMap bool) (map[string][]byte, error) {
	parts := strings.Split(ref.Key, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid key: %s", ref.Key)
	}
	prefix, filter, value := strings.ToLower(parts[0]), strings.ToLower(parts[1]), parts[2]
	if prefix != "rsa" && prefix != "ecdsa" {
		return nil, fmt.Errorf("unsupported prefix: %s", prefix)
	}

	kt := models.CertificateKeyType(strings.ToUpper(prefix))
	req := &certificate.GetReq{KeyType: &kt, NoCache: true}

	switch filter {
	case "id":
		id, _ := strconv.ParseUint(value, 10, 32)
		u32id := uint32(id)
		req.ID = &u32id
	case "name":
		req.Name = &value
	default:
		return nil, fmt.Errorf("unsupported filter: %s", filter)
	}

	resp, err := c.client.CertificateGet(ctx, req)

	var cert *certificate.GetEntity
	if err == nil && len(resp) > 0 {
		cert = resp[0]
	} else if isMap && filter == "name" {
		domains := []string{value}
		for _, sub := range strings.Split(ref.Property, ",") {
			if s := strings.TrimSpace(sub); s != "" {
				if strings.Contains(s, ".") {
					domains = append(domains, s)
				} else {
					domains = append(domains, s+"."+value)
				}
			}
		}
		createResp, err := c.client.CertificateCreateNew(ctx, &certificate.CreateReq{
			Name: value, KeyType: kt, Domains: domains, RequestedBy: ptr("ESO"), Sync: true,
		})

		if err == nil && createResp.PEM.Public != "" {
			cert = &certificate.GetEntity{
				Certificate: createResp.Certificate,
				PEM:         createResp.PEM,
			}
		} else {
			var nerr *apiclient.NotAllowedError
			if err != nil && !errors.As(err, &nerr) {
				return nil, fmt.Errorf("failed to provision certificate: %w", err)
			}

			// Try to get the newly created cert to verify accessibility and get its status
			resp, err := c.client.CertificateGet(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("certificate created but retrieval failed: %w", err)
			}
			if len(resp) == 0 {
				return nil, fmt.Errorf("certificate created but not found in subsequent request")
			}
			cert = resp[0]
		}
	}

	if cert == nil {
		return nil, esv1.NoSecretErr
	}

	bundle := cert.PEM.Public
	if cert.PEM.CA != "" {
		bundle += "\n" + cert.PEM.CA
	}
	return map[string][]byte{
		"cert": []byte(cert.PEM.Public), "key": []byte(cert.PEM.Private),
		"ca": []byte(cert.PEM.CA), "bundle": []byte(bundle),
	}, nil
}

func (c *apiClientWrapper) Validate() (esv1.ValidationResult, error) {
	return esv1.ValidationResultReady, nil
}
func (c *apiClientWrapper) Close(_ context.Context) error { return nil }

func (c *apiClientWrapper) PushSecret(_ context.Context, _ *corev1.Secret, _ esv1.PushSecretData) error {
	return errors.New("not supported")
}
func (c *apiClientWrapper) DeleteSecret(_ context.Context, _ esv1.PushSecretRemoteRef) error {
	return errors.New("not supported")
}
func (c *apiClientWrapper) SecretExists(_ context.Context, _ esv1.PushSecretRemoteRef) (bool, error) {
	return false, errors.New("not supported")
}
func (c *apiClientWrapper) GetAllSecrets(_ context.Context, _ esv1.ExternalSecretFind) (map[string][]byte, error) {
	return nil, errors.New("not supported")
}

func ptr[T any](v T) *T {
	return &v
}
