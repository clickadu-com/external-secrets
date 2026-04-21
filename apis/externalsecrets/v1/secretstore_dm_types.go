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

package v1

import (
	esmeta "github.com/external-secrets/external-secrets/apis/meta/v1"
)

// DMProvider configures a store to sync secrets using Domain Manager provider.
type DMProvider struct {
	// BaseURL is the Domain Manager API base URL.
	// +kubebuilder:validation:MinLength:=1
	BaseURL string `json:"baseURL"`

	Auth DMAuth `json:"auth"`
}

// DMAuth contains a secretRef for credentials.
type DMAuth struct {
	// +optional
	SecretRef *DMAuthSecretRef `json:"secretRef,omitempty"`
}

// DMAuthSecretRef holds secret references for Domain Manager API credentials.
type DMAuthSecretRef struct {
	// APIToken is used as a Bearer token for API authentication.
	APIToken esmeta.SecretKeySelector `json:"apiToken"`
}
