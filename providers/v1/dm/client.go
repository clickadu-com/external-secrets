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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"git.adsrv.wtf/clickadu/domain-manager/modules/apiclient"
	"git.adsrv.wtf/clickadu/domain-manager/modules/models"
	certHandler "git.adsrv.wtf/clickadu/domain-manager/modules/web/handlers/certificate"
	corev1 "k8s.io/api/core/v1"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

var (
	reservedProperties = map[string]bool{
		"cert":    true,
		"key":     true,
		"ca":      true,
		"ca.crt":  true,
		"tls.crt": true,
		"tls.key": true,
	}
)

type apiClientWrapper struct {
	client *apiclient.Client
}

func newClient(baseURL, token string) *apiClientWrapper {
	return &apiClientWrapper{
		client: apiclient.NewClient(baseURL, token),
	}
}

func (c *apiClientWrapper) GetSecret(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef) ([]byte, error) {
	subdomains := ""
	property := ref.Property
	if property != "" && !reservedProperties[property] {
		subdomains = property
		property = ""
	}

	data, err := c.getSecretData(ctx, ref.Key, subdomains)
	if err != nil {
		return nil, err
	}

	if property == "" {
		if val, ok := data["name"]; ok {
			return val, nil
		}
		if val, ok := data["domains"]; ok {
			return val, nil
		}
		bundle := map[string]string{
			"tls.crt": string(data["tls.crt"]),
			"tls.key": string(data["tls.key"]),
		}
		return json.Marshal(bundle)
	}

	if property == "ca.crt" {
		property = "ca"
	}

	val, ok := data[property]
	if !ok {
		return nil, fmt.Errorf("property %s not found in %s", property, ref.Key)
	}
	return val, nil
}

func (c *apiClientWrapper) GetSecretMap(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef) (map[string][]byte, error) {
	subdomains := ""
	if ref.Property != "" && !reservedProperties[ref.Property] {
		subdomains = ref.Property
	}
	return c.getSecretData(ctx, ref.Key, subdomains)
}

func (c *apiClientWrapper) getSecretData(ctx context.Context, key, subdomains string) (map[string][]byte, error) {
	prefix, filter, value, err := parseKey(key)
	if err != nil {
		return nil, err
	}

	switch prefix {
	case "rsa", "ecdsa":
		return c.ensureCertificate(ctx, models.CertificateKeyType(strings.ToUpper(prefix)), filter, value, subdomains)
	case "domain":
		return c.getDomainData(ctx, filter, value)
	default:
		return nil, fmt.Errorf("unsupported key prefix: %s", prefix)
	}
}

func (c *apiClientWrapper) ensureCertificate(ctx context.Context, keyType models.CertificateKeyType, filter, value, subdomains string) (map[string][]byte, error) {
	var requestedID uint32
	if filter == "id" || filter == "domain_id" {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid id format: %w", err)
		}
		requestedID = uint32(id)
	}

	list, err := c.client.CertificateList(ctx)
	if err != nil {
		return nil, err
	}

	var foundID uint32
	var foundStatus models.CertificateStatus
	exists := false

	for _, cert := range list {
		if cert.KeyType != keyType {
			continue
		}
		match := false
		switch filter {
		case "id":
			match = cert.ID == requestedID
		case "domain_id":
			match = cert.DomainID != nil && *cert.DomainID == requestedID
		case "domain", "name":
			match = cert.Name == value
		}
		if match {
			foundID = cert.ID
			foundStatus = cert.Status
			exists = true
			break
		}
	}

	if !exists && (filter == "domain" || filter == "domain_id") {
		certName := value
		domains := []string{value}

		if filter == "domain_id" {
			domainList, err := c.client.DomainList(ctx)
			if err != nil {
				return nil, err
			}
			var dName string
			for _, d := range domainList {
				if d.ID == requestedID {
					dName = d.Name
					break
				}
			}
			if dName == "" {
				return nil, fmt.Errorf("domain with id %d not found to provision certificate", requestedID)
			}
			certName = dName
			domains = []string{dName}
		}

		if subdomains != "" {
			for _, sub := range strings.Split(subdomains, ",") {
				sub = strings.TrimSpace(sub)
				if sub == "" {
					continue
				}
				if strings.Contains(sub, ".") {
					domains = append(domains, sub)
				} else {
					domains = append(domains, sub+"."+certName)
				}
			}
		}

		req := &certHandler.CreateReq{
			Name:    certName,
			KeyType: keyType,
			Domains: domains,
			Sync:    true,
		}
		resp, err := c.client.CertificateCreate(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to provision certificate: %w", err)
		}
		foundID = resp.ID
		foundStatus = resp.Status
	} else if !exists {
		return nil, esv1.NoSecretError{}
	}

	if foundStatus != models.CertificateStatusOk {
		if foundStatus == models.CertificateStatusIssueFailed || foundStatus == models.CertificateStatusRenewFailed {
			return nil, fmt.Errorf("certificate issuance failed (status: %d)", foundStatus)
		}
		return nil, fmt.Errorf("certificate is not ready yet (status: %d)", foundStatus)
	}

	certs, err := c.client.CertificateGet(ctx)
	if err != nil {
		return nil, err
	}

	for _, cert := range certs {
		if cert.ID == foundID {
			fullchain := cert.PEM.Public
			if cert.PEM.CA != "" {
				fullchain = cert.PEM.Public + "\n" + cert.PEM.CA
			}
			return map[string][]byte{
				"tls.crt": []byte(fullchain),
				"tls.key": []byte(cert.PEM.Private),
				"cert":    []byte(fullchain),
				"key":     []byte(cert.PEM.Private),
				"ca":      []byte(cert.PEM.CA),
				"ca.crt":  []byte(cert.PEM.CA),
			}, nil
		}
	}

	return nil, fmt.Errorf("certificate ready but payload missing in get response")
}

func (c *apiClientWrapper) getDomainData(ctx context.Context, filter, value string) (map[string][]byte, error) {
	domains, err := c.client.DomainList(ctx)
	if err != nil {
		return nil, err
	}

	if filter == "type_id" {
		typeID, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid type_id format: %w", err)
		}
		validNames := []string{}
		for _, d := range domains {
			if d.TypeID == uint16(typeID) && d.Status >= 40 && d.Status < 58 {
				validNames = append(validNames, d.Name)
			}
		}
		return map[string][]byte{
			"domains": []byte(strings.Join(validNames, ",")),
		}, nil
	}

	var requestedID uint32
	if filter == "id" {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid id format: %w", err)
		}
		requestedID = uint32(id)
	}

	for _, d := range domains {
		match := false
		if filter == "id" {
			match = d.ID == requestedID
		} else if filter == "name" {
			match = d.Name == value
		}

		if match {
			return map[string][]byte{
				"name": []byte(d.Name),
			}, nil
		}
	}

	return nil, esv1.NoSecretError{}
}

func parseKey(key string) (prefix, filter, value string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid key format: %s", key)
	}
	return parts[0], parts[1], parts[2], nil
}

func (c *apiClientWrapper) PushSecret(ctx context.Context, secret *corev1.Secret, data esv1.PushSecretData) error {
	return errors.New("push secret is not supported by dm provider")
}

func (c *apiClientWrapper) DeleteSecret(ctx context.Context, ref esv1.PushSecretRemoteRef) error {
	return errors.New("delete secret is not supported by dm provider")
}

func (c *apiClientWrapper) SecretExists(ctx context.Context, ref esv1.PushSecretRemoteRef) (bool, error) {
	return false, errors.New("push is not supported")
}

func (c *apiClientWrapper) Validate() (esv1.ValidationResult, error) {
	return esv1.ValidationResultReady, nil
}

func (c *apiClientWrapper) GetAllSecrets(ctx context.Context, ref esv1.ExternalSecretFind) (map[string][]byte, error) {
	return nil, errors.New("get all secrets is not supported")
}

func (c *apiClientWrapper) Close(ctx context.Context) error {
	return nil
}
