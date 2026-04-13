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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	"github.com/external-secrets/external-secrets/runtime/esutils/resolvers"
)

var _ esv1.Provider = &Provider{}

var (
	errMissingStore     = errors.New("missing store specification")
	errInvalidSpec      = errors.New("invalid specification for dm provider")
	errMissingBaseURL   = errors.New("missing dm baseURL")
	errMissingSecretRef = errors.New("missing dm auth.secretRef")
	errMissingTokenRef  = errors.New("missing dm auth.secretRef.apiToken")
)

// Provider implements Domain Manager provider.
type Provider struct{}

// NewClient creates a new Domain Manager client.
func (p *Provider) NewClient(ctx context.Context, store esv1.GenericStore, kube client.Client, namespace string) (esv1.SecretsClient, error) {
	cfg, err := getConfig(store)
	if err != nil {
		return nil, err
	}

	storeKind := store.GetKind()
	token, err := resolvers.SecretKeyRef(ctx, kube, storeKind, namespace, &cfg.Auth.SecretRef.APIToken)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dm api token: %w", err)
	}

	return newClient(strings.TrimRight(cfg.BaseURL, "/"), token), nil
}

// ValidateStore validates provider config.
func (p *Provider) ValidateStore(store esv1.GenericStore) (admission.Warnings, error) {
	_, err := getConfig(store)
	return nil, err
}

// Capabilities returns provider capabilities.
func (p *Provider) Capabilities() esv1.SecretStoreCapabilities {
	return esv1.SecretStoreReadOnly
}

// NewProvider creates a new provider.
func NewProvider() esv1.Provider {
	return &Provider{}
}

// ProviderSpec returns provider spec.
func ProviderSpec() *esv1.SecretStoreProvider {
	return &esv1.SecretStoreProvider{
		DM: &esv1.DMProvider{},
	}
}

// MaintenanceStatus returns maintenance status of provider.
func MaintenanceStatus() esv1.MaintenanceStatus {
	return esv1.MaintenanceStatusMaintained
}

func getConfig(store esv1.GenericStore) (*esv1.DMProvider, error) {
	if store == nil {
		return nil, errMissingStore
	}

	storeSpec := store.GetSpec()
	if storeSpec == nil || storeSpec.Provider == nil || storeSpec.Provider.DM == nil {
		return nil, errInvalidSpec
	}

	cfg := storeSpec.Provider.DM
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errMissingBaseURL
	}
	if cfg.Auth.SecretRef == nil {
		return nil, errMissingSecretRef
	}
	if strings.TrimSpace(cfg.Auth.SecretRef.APIToken.Name) == "" || strings.TrimSpace(cfg.Auth.SecretRef.APIToken.Key) == "" {
		return nil, errMissingTokenRef
	}

	return cfg, nil
}
