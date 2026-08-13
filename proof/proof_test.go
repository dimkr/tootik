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

package proof

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

// https://codeberg.org/fediverse/fep/src/commit/3a5942066f989d8317befe6457b48237bc61efe0/fep/8b32/fep-8b32.feature#L3
func TestProof_SignEd25519(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"id":"https://server.example/activities/1","type":"Create","actor":"https://server.example/users/alice","object":{"id":"https://server.example/objects/1","type":"Note","attributedTo":"https://server.example/users/alice","content":"Hello world","location":{"type":"Place","longitude":-71.184902,"latitude":25.273962}}}`)

	var a ap.Activity
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	}

	created, err := time.Parse(time.RFC3339, "2023-02-24T23:36:38Z")
	if err != nil {
		t.Fatalf("Failed to parse creation timestamp: %v", err)
	}

	privKey := ed25519.NewKeyFromSeed(base58.Decode("3u2en7t5LR2WtQH5PfFqMqwVHBeXouLzo6haApm8XHqvjxq")[2:])

	if withProof, err := Add(httpsig.Key{ID: "https://server.example/users/alice#ed25519-key", PrivateKey: privKey}, created, raw); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	} else if err := json.Unmarshal(withProof, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	} else if a.Proof.Value != "zLaewdp4H9kqtwyrLatK4cjY5oRHwVcw4gibPSUDYDMhi4M49v8pcYk3ZB6D69dNpAPbUmY8ocuJ3m9KhKJEEg7z" {
		t.Fatalf("Unexpected proof value: %s", a.Proof.Value)
	} else if err := Verify(privKey.Public(), a.Proof, a.Context, withProof); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
}

// https://codeberg.org/fediverse/fep/src/commit/3a5942066f989d8317befe6457b48237bc61efe0/fep/8b32/fep-8b32.feature#L67
func TestProof_VerifyEd25519(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"id":"https://server.example/activities/1","type":"Create","actor":"https://server.example/users/alice","object":{"id":"https://server.example/objects/1","type":"Note","attributedTo":"https://server.example/users/alice","content":"Hello world","location":{"type":"Place","longitude":-71.184902,"latitude":25.273962}},"proof":{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"type":"DataIntegrityProof","cryptosuite":"eddsa-jcs-2022","verificationMethod":"https://server.example/users/alice#ed25519-key","proofPurpose":"assertionMethod","proofValue":"zLaewdp4H9kqtwyrLatK4cjY5oRHwVcw4gibPSUDYDMhi4M49v8pcYk3ZB6D69dNpAPbUmY8ocuJ3m9KhKJEEg7z","created":"2023-02-24T23:36:38Z"}}`)

	var a ap.Activity
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	}

	if err := Verify(ed25519.PublicKey(base58.Decode("6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2")[2:]), a.Proof, a.Context, raw); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
}

// https://www.w3.org/TR/vc-di-quantum-resistant-1.0/#cryptosuite-mldsa44-jcs-2024
func TestProof_VerifyMLDSA(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"@context":["https://www.w3.org/ns/credentials/v2","https://w3id.org/citizenship/v4rc1"],"type":["VerifiableCredential","EmploymentAuthorizationDocumentCredential"],"issuer":{"id":"did:key:zDnaegE6RR3atJtHKwTRTWHsJ3kNHqFwv7n9YjTgmU7TyfU76","image":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIW2NgUPr/HwADaAIhG61j/AAAAABJRU5ErkJggg=="},"credentialSubject":{"type":["Person","EmployablePerson"],"givenName":"JOHN","additionalName":"JACOB","familyName":"SMITH","image":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIW2Ng+M/wHwAEAQH/7yMK/gAAAABJRU5ErkJggg==","gender":"Male","residentSince":"2015-01-01","birthCountry":"Bahamas","birthDate":"1999-07-17","employmentAuthorizationDocument":{"type":"EmploymentAuthorizationDocument","identifier":"83627465","lprCategory":"C09","lprNumber":"999-999-999"}},"name":"Employment Authorization Document","description":"Example Employment Authorization Document.","validFrom":"2019-12-03T00:00:00Z","validUntil":"2029-12-03T00:00:00Z","proof":{"type":"DataIntegrityProof","cryptosuite":"mldsa44-jcs-2024","created":"2023-02-24T23:36:38Z","verificationMethod":"did:key:ukCRKDtY8Do_dXzYGyuX7BY-1dDYM4FuSiw0gFdO-eJXFH0eqlt4_CP4sEISGAzNlKDzLpUJWoInRywXOpd7FCp_QAJAlL7iRo4cepKhzhlq8xt6qd5jkhYF9tNH8z3RGDl9aunNy_06fWLYNScWd5RmGg46Po8T-kIjMMkJftaqqZcGDxktpu9Et2bnaZMx4K98YyG1urUpM9lgvldgg2qv-6XCrm2uXlJ9U-HN4xtQKn4Ug-5xPwbhPGR2pbcBScTFotkhBqLc2eQLL6zPutWF83sSZbOhD_11BjMkeiLyJbMeHCIhz5GDIbPksEFIaSho3MdFo5fpQ8QZoqCit3Jn4ddfuShfIoLU1Hw5EZ0xBiqOU7e-TINd7-7HsgHLmYMGnpqljm1ot3c3cfalYsg87WQuSscO7XNH3Ewa-cgU6Bnj1SGn0plTy6Yq-GxU8XUBPwsK_IoIJbXWC0UD97c9UYpNiZi0-ECnbB-Y5_SM8auMfoIeap_buAOdXlJZmcp8xNCXI59AN9-96Sdhks5L-JmsELzyjAgjqNx8Zt3KPFc2jSwNjDVC1fEa8FdDdDT3WNkF6KTt65lb5_aIkFh20nOvT7kIJcKTgmhRGNJZgGSPYVbMypaQoaac8dtEoQjvYgnO-rM_RcsiWMHNc29br3o5wdiLXdr63MoX1lEWu_THBfeP1JuxrSbUmHOByepWbubbSM4iVQITCxBHZT0Mj2bWwIxd3nZUajzebyEnsfitV01kpzlO7bzY2uxSzyplTkRfppc_7YH0y0PHaggw0cIXNSh73wVqNZmzmJx5W0_akrvy5oSz9ZB1Io2p_fTxzibefwO700bUqbElV_yuCjD7EJ_Hfqbog80y_g9TK6koX7wYwqFNQxBVavKC-HbcT7yPdvzs9hlC2MNWCT3W7gVgeYr4AFgbV9EgMcH0GtJDKYw8vkpB_vTsaSTGZAj3TNKalAwiGO50VAmF5tknF96kOrWmNL0MdkXhnm1vXgDpP68bMt4r2Qr-hNdJ4s3_nqmSDYTnZRA4qjXjrgKQfO19txt0tX7LifE1GZ1bQyS7NqHWXyMEhw6_F8pc_tS16VhvJO_FM7CX51mLLkLCGl7DsmbnEIsVUW9qlCxb6bj53UyijTYdu6uLZW9JISE2B4EevxzwDu9UGcJPHmJYi1rRQAP__jH97GiQC8FvkdAEfKqcwV9jAbBPQPG6lUkBLcoijgR3Bcwd-ta92oeZmcpoJ97PzzBbCL-NrppJ2HHQ1SMsYWoPveZTmZc66YBA0P9YfT4hZ0RQiP2gxB4snTvMFI0Ot6Q2nQ0p5DMxmWqIaCKW53rqn16AVXQeqC2TJjlbjA9sC6pr8GEGY2OQUgEmWu5GmnOSz1lNY7fNHJypChnieI_hyYiy06qouUpoHA5z_IUtfzZoMIG0yJiGUUpF9BJvYChDECCqaUM1kWnO5tKcohSKq5Hqwu_EWDRYF2tj7igSimZkS4Pts41tu8nIaVk5EkzAX9gCR2EX3Lk869mIxSyBS3MyG_NotPcbm6uXDn_YkV5Z0HkxUxYRA9hIG-UhKhK3VOaHZP8GcQN8noOMa2CnPd208X6HOzlIlxs7SRbzppUs_fHN1eROglNy-2oJWGmo-xOy0Qd44TtY0S_bYhu6iH6inrx3-yncSrWFxEiYosvYJD4ZBSyrV4d6UsfeNSHYS0ODTsdPqz4SYTeloZbIx8XWz7fxLXlNyLr3s9tp-Q25f1vTIrmQL","proofPurpose":"assertionMethod","proofValue":"uTSucVLvXmOpmjGGNB-B9rM-u4HzBxN8ZIuZbpTHrjOTNBnahoE4PSdkeD-IzLLXykJn0aYq_APExy-Ka0BcJNMvKgkdjbbP33WmUwkzljno3szRUDrN9KX2DMH7j0iOBakU4ByjD-hTSO1iR6rlxsZPHJM1H-WLMhzVSggBILAuglItzstl663Gz5bFjEfbKAgfe50L4v4PjLFSDbJYcg65GtCKRXISkWrnJRuToWwvTVdcnIBOQwPBFKsvApPJMKrUTkIuZf4-V1uJ81zzct4o20O-DqLQ5bHfOR2n5Y4DSy6e5zg0-S3ADKtMtuPaQ8cAPUTEKRXGRQnSndnrtgMh2dimvpSaaw0TDy7zY6vrDxJa1tkrS0ulKf3Xz8xsNrNIkx4SaKYWPTjhRvdKqjdrpbGRt3mRSFFc0VE8vK44F_EVFIhwouL-4Rm4mXU2QkiO0YkwuAJM-QdWUACqzJ7TSf2QrrU8zAwOLbrGS5uZ1qLGD1PcgWfg2d0zTAYmcWP4LP63fTnFxwr-L0N_3MLFXixHNEp5osMlo2lhl5noDCmQpqCgluxkd5gXs1NSpOBbWVQyYWcj0WMBtMam8AeqXpA39L7oqvYxqbEpiwvKrmHsXIEZrnsHKCk2P0yc10AFCCtsIapvTHwIAjbDhX11HFU5cci4X5vCdG2BUzRsgmGeiYiUClCHmqsBW4z2GA9r0d9jtHZ03nMie_qS95XPsXuAFqypsP1HOfcIUAHHS9Wn4XGFz3hXoqMsmoUGRg9vEpC2j_nkcYZQphYLs54veWq5BBzoMqPuvYhhRdawdCnn-LTf7AxQgVGoRTpTy4IkXxr_pC1LUJZJkdKeG-2TuQzyHSkbMPu3YbWsGy2KxdGeFN2yUI8TTQ-MFHl-_jDCRBrAYyCVOgML4NAtbGqvs7h2tGZYI-m9MfjG0vjp7CUyIPD8BV-Yhku_bHd0hcrseKtYYyUjxISf2wveo4dfQ2AnCVdDAbBmznjPIDlkqx0316sRc-vXJGRQmfXOW1dNk-7WNBrJVQbnT9m6cf0UEl3mEgagk1_lLOxTgjzZRpWcOB827VB3hPi7RdI6U6knXuOflHPt9BZN7i6OAl76k69uMFH2KNH3Abm0GDhOv_nu1lEaOH8aXdOqL3U8Yo0cp5roOoTw5fJP2gxwI3DY0TOWeNOCfLXmnodgoGaKG2Vns4_-gN_Mg7g0ZinguwJMwKACx07H__ffh8jYQdc87EjCNyH4m8hJICvcC81J7CKb89YTZm7IM0D1_qTR5t-DkuU4ypxNuFOCxWpN9y2QiLAAobDdc1Y_S3nXFFkLmsn7hUNhcgXxPC3jLifiM0IV7DAqmQpk2ZGE59l0VTKb3F2Ualj9JcqKtLg_b2KqprUol9WtjFlkbxqJPYyCKnSEitzDnDsfxTRFEIViTx5-1SFb0NjPE_hv5MewCkizNpfo0b-m-FxvyWJnDYt4Igv8JtgF0K_xMRC9Tf3NaQgFHb1OgkBz04C3wsxoPLqTgMWoxcZ2-2x7TRRvX2Nh1Ye-ZrpmF5hVeRMK0ECj_t5HLPaq2md18rqnhwsZv84-V0eReDyXVIhkE2eAKedCM9t13UjTfF1qFoUQ3D8xQ6MfR7zFwf6X78Tb0EFs0cBQ1TatzWysbE2b_0k-YbT78G4Ko8FlmTljBN1b0StKxzOE1Kp1h4nDBY9jZYYPNnVrtAGn2AKN3HWr0bhhF8fW_G4SA_MGu2r6LnobB3MLCSj1lbVZSk5YtCtMnkldAsDagoI8lRBlKW1R7PFvXatSHcwbe352nVuvZYxs9QJVSylf2QS1xeUQUMS4AHd4h8Y9HqmnGPr5JX65uch8sr1bWcpGiNlwnMlFy89pnEZU2v7IiC_foLi8JbxMud1k2XRDwThhepEf5bxqlBcgjF8vAbGEKAJg5Oahl0GCBffuHhuUXmuF6XwOxLovlJUM8oFCTayQL42NGz_Z2cuFltmiy-cbI8NmEpvlOPnGZBj00ZIrp9tAgwepYvlt5ticFn9ufU-f8xZpBpZb-rf5sVTNqYmoOYvhKVhE5cCPpCbzZ5GvW5ukB8yLLJC5sc6df9oSoujovfA_VAXqsvmBuKU4cHcNTNEqzsGEg_l0ln5FIHKH3CRUTDemKN2vbtmTz1snn4VfdBFAlhJhBItpBmd3HibH-1q2WD313j-cE5nW_QgQeDn_JFJ7zwlORQVRUkeB942HJjkHWXCJMXz9LKxdncKalaVNm-yVjDkQdgA1tKl9bc_QnAL7HWhtHFc_XhzJvQxqLJ0aLUkkrnphrqscG6D_Kdh7aTTkDjDSA-dmgRQBh51YVBnfnp5V28AwmGXXglBAWCWChGmAtac6xeLbxW143426J4HMAUIpLgNhjetQoQqKzVTIpzcyo-GK8L3C0calt57orTswSRqDjxg_6zAQ6RPNoThToRqb2QgHr-gOop6NEJkEy0K8GwPg3nYAFgVVjcCyDtMEbIrHXJ9WKg3oTd14eC7GNZgQ75aP3HpUAqT5gqWdxer35Ohs3n1FylwreS1kOZ5Z4OVW5PVJkNHhKPfaNq4mQT5vBAWlWphOotPHwTbN7oMiOGYu-AMTnsLPCn0A3VEXx16EROpu0zlVisEZo1sya8nQaJYiI_i3MEWqvV2ypvcMDYt_ArFjxMU3tjV5tNcJgIoE5E5UCGyo2HQMvN03T0GdHZr7txswg9HpRJqntiJzm0iAr9BhRPfErg4HLQyc92gOH6UdczP4hvbweP3mcW67yUT3lH31vznZ5lIJ0pth3H_7khwUff5daIROar2usWxoMWTItY5HC7v5HBjnfqj4EoVi1A4Uw7RvHhaCMkfDrqqntNM0TDDSfDP1uCB1RwZtcuWpNfLWYyP1B9XqwKg3EworHhIq1vI74gZROvebyYqx8UCFeiLubTfXHJrC2evT99ha6jf7vg8zKgew6Cj3Jz_RSRE5rF5uQQno6PsevbKKtZsQmn0PQphfNzWqacMpred2yrmetGOEG_JvYxx5Scmu8w8Yg-3V7yhMeDsOh0DZ8sjTw5d-w3zKUNeU2IZzz5wvaJ7-8RRxyJqxFsUFaBv7WxUZ0bmWg4pRXdBUKNW53mEUGDigpRF6Fla-6wc4BCDhwfJWdpaixu8Lf4fkRKCs0Y4-lrsTH1-zwNkKBudbd6v4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAwbKDA"}}`)

	var a struct {
		Context any      `json:"@context"`
		Proof   ap.Proof `json:"proof"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	}

	pubBytes, err := base64.RawURLEncoding.DecodeString("kCRKDtY8Do_dXzYGyuX7BY-1dDYM4FuSiw0gFdO-eJXFH0eqlt4_CP4sEISGAzNlKDzLpUJWoInRywXOpd7FCp_QAJAlL7iRo4cepKhzhlq8xt6qd5jkhYF9tNH8z3RGDl9aunNy_06fWLYNScWd5RmGg46Po8T-kIjMMkJftaqqZcGDxktpu9Et2bnaZMx4K98YyG1urUpM9lgvldgg2qv-6XCrm2uXlJ9U-HN4xtQKn4Ug-5xPwbhPGR2pbcBScTFotkhBqLc2eQLL6zPutWF83sSZbOhD_11BjMkeiLyJbMeHCIhz5GDIbPksEFIaSho3MdFo5fpQ8QZoqCit3Jn4ddfuShfIoLU1Hw5EZ0xBiqOU7e-TINd7-7HsgHLmYMGnpqljm1ot3c3cfalYsg87WQuSscO7XNH3Ewa-cgU6Bnj1SGn0plTy6Yq-GxU8XUBPwsK_IoIJbXWC0UD97c9UYpNiZi0-ECnbB-Y5_SM8auMfoIeap_buAOdXlJZmcp8xNCXI59AN9-96Sdhks5L-JmsELzyjAgjqNx8Zt3KPFc2jSwNjDVC1fEa8FdDdDT3WNkF6KTt65lb5_aIkFh20nOvT7kIJcKTgmhRGNJZgGSPYVbMypaQoaac8dtEoQjvYgnO-rM_RcsiWMHNc29br3o5wdiLXdr63MoX1lEWu_THBfeP1JuxrSbUmHOByepWbubbSM4iVQITCxBHZT0Mj2bWwIxd3nZUajzebyEnsfitV01kpzlO7bzY2uxSzyplTkRfppc_7YH0y0PHaggw0cIXNSh73wVqNZmzmJx5W0_akrvy5oSz9ZB1Io2p_fTxzibefwO700bUqbElV_yuCjD7EJ_Hfqbog80y_g9TK6koX7wYwqFNQxBVavKC-HbcT7yPdvzs9hlC2MNWCT3W7gVgeYr4AFgbV9EgMcH0GtJDKYw8vkpB_vTsaSTGZAj3TNKalAwiGO50VAmF5tknF96kOrWmNL0MdkXhnm1vXgDpP68bMt4r2Qr-hNdJ4s3_nqmSDYTnZRA4qjXjrgKQfO19txt0tX7LifE1GZ1bQyS7NqHWXyMEhw6_F8pc_tS16VhvJO_FM7CX51mLLkLCGl7DsmbnEIsVUW9qlCxb6bj53UyijTYdu6uLZW9JISE2B4EevxzwDu9UGcJPHmJYi1rRQAP__jH97GiQC8FvkdAEfKqcwV9jAbBPQPG6lUkBLcoijgR3Bcwd-ta92oeZmcpoJ97PzzBbCL-NrppJ2HHQ1SMsYWoPveZTmZc66YBA0P9YfT4hZ0RQiP2gxB4snTvMFI0Ot6Q2nQ0p5DMxmWqIaCKW53rqn16AVXQeqC2TJjlbjA9sC6pr8GEGY2OQUgEmWu5GmnOSz1lNY7fNHJypChnieI_hyYiy06qouUpoHA5z_IUtfzZoMIG0yJiGUUpF9BJvYChDECCqaUM1kWnO5tKcohSKq5Hqwu_EWDRYF2tj7igSimZkS4Pts41tu8nIaVk5EkzAX9gCR2EX3Lk869mIxSyBS3MyG_NotPcbm6uXDn_YkV5Z0HkxUxYRA9hIG-UhKhK3VOaHZP8GcQN8noOMa2CnPd208X6HOzlIlxs7SRbzppUs_fHN1eROglNy-2oJWGmo-xOy0Qd44TtY0S_bYhu6iH6inrx3-yncSrWFxEiYosvYJD4ZBSyrV4d6UsfeNSHYS0ODTsdPqz4SYTeloZbIx8XWz7fxLXlNyLr3s9tp-Q25f1vTIrmQL")
	if err != nil {
		t.Fatalf("Failed to decode public key: %v", err)
	}

	var pub mldsa44.PublicKey
	if err := pub.UnmarshalBinary(pubBytes[2:]); err != nil {
		t.Fatalf("Failed to decode key: %v", err)
	}

	if err := Verify(&pub, a.Proof, a.Context, raw); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
}
