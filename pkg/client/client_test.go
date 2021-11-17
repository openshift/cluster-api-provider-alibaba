package client

import (
	"errors"
	"testing"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials/providers"
	corev1 "k8s.io/api/core/v1"
)

func Test_FetchCredentialsInIFromSecret(t *testing.T) {
	for _, tt := range []struct {
		name   string
		secret *corev1.Secret
		expect error
		want   bool
		id     string
		sec    string
		token  string
	}{
		{
			name: "empty credentials",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(""),
				},
			},
			expect: errors.New("section \"default\" does not exist"),
		},
		{
			name: "empty default section",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(`[default]`),
				},
			},
			expect: errors.New("missing credential type in INI secret"),
		},
		{
			name: "type access_key missing id",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(`[default]
					type = access_key`),
				},
			},
			expect: errors.New("missing access_key_id or access_key_secret from credential"),
		},
		{
			name: "type access_key missing id value",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(`[default]
					type = access_key
					access_key_id = 
					access_key_secret = ABCEDEFG`),
				},
			},
			expect: errors.New("access_key_id is empty in credential"),
		},
		{
			name: "type access_key_secret missing secret value",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(`[default]
					type = access_key
					access_key_id = ALIBABA
					access_key_secret =`),
				},
			},
			expect: errors.New("access_key_secret is empty in credential"),
		},
		{
			name: "valid credentials",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(`[default]
					type = access_key
					access_key_id = ALIBABA
					access_key_secret = ABCDEFG`),
				},
			},
			expect: nil,
			id:     "ALIBABA",
			sec:    "ABCDEFG",
		},
		{
			name: "sts type empty token ",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(`[default]
					type = token
					accessKeyStsToken =	`),
				},
			},
			expect: errors.New("accessKeyStsToken is empty in credential"),
		},
		{
			name: "valid sts type token",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"credentials": []byte(`[default]
					type = token
					accessKeyStsToken =	123456789`),
				},
			},
			expect: nil,
			token:  "123456789",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := providers.Configuration{}
			err := fetchCredentialsIniFromSecret(tt.secret, &config)
			if err != nil {
				if tt.expect != nil {
					if err.Error() != tt.expect.Error() {
						t.Fatalf("%#v", err)
					}
				}

			} else if err == nil && tt.expect != nil {
				t.Fatalf("expected %s", tt.expect.Error())
			}
			if config.AccessKeyID != tt.id {
				t.Fatalf("expected %s but got %s", tt.id, config.AccessKeyID)
			}
			if config.AccessKeySecret != tt.sec {
				t.Fatalf("expected %s but got %s", tt.sec, config.AccessKeySecret)
			}
		})
	}

}
