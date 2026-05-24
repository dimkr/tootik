/*
Copyright 2025, 2026 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ap

import "encoding/json"

type Proof struct {
	Context            any    `json:"@context,omitempty"`
	Type               string `json:"type"`
	CryptoSuite        string `json:"cryptosuite"`
	VerificationMethod string `json:"verificationMethod"`
	Purpose            string `json:"proofPurpose"`
	Value              string `json:"proofValue,omitempty"`
	Created            string `json:"created"`
}

func (p *Proof) UnmarshalJSON(b []byte) error {
	type proof Proof
	var single proof
	if err := json.Unmarshal(b, &single); err == nil {
		*p = Proof(single)
	} else {
		// forte@6d30fce1d6 send an array at least in some cases
		var arr []json.RawMessage
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		} else if len(arr) > 0 {
			return json.Unmarshal(arr[0], (*proof)(p))
		}
	}

	return nil
}
