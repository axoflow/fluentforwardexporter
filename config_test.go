// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/axoflow/fluentforwardexporter/internal/metadata"
)

func TestLoadConfigNewExporter(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				TCPClientSettings: TCPClientSettings{
					Endpoint: Endpoint{
						TCPAddr:               validEndpoint,
						ValidateTCPResolution: false,
					},
					ConnectionTimeout: time.Second * 30,
					ClientConfig: configtls.ClientConfig{
						Insecure:           true,
						InsecureSkipVerify: false,
						Config: configtls.Config{
							CAFile:   "",
							CertFile: "",
							KeyFile:  "",
						},
					},
					SharedKey: "",
				},
				RequireAck:   false,
				Tag:          "tag",
				CompressGzip: false,
				DefaultLabelsEnabled: map[string]bool{
					"timestamp": true,
					"level":     true,
					"message":   true,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueBatchConfig: exporterhelper.QueueBatchConfig{
					Enabled:      true,
					NumConsumers: 10,
					QueueSize:    1000,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc string
		cfg  *Config
		err  error
	}{
		{
			desc: "QueueSettings are invalid",
			cfg: &Config{
				QueueBatchConfig: exporterhelper.QueueBatchConfig{
					QueueSize: -1,
					Enabled:   true,
				},
			},
			err: fmt.Errorf("queue settings has invalid configuration"),
		},
		{
			desc: "Endpoint is invalid",
			cfg: &Config{
				TCPClientSettings: TCPClientSettings{
					Endpoint: Endpoint{
						TCPAddr:               "http://localhost:24224",
						ValidateTCPResolution: true,
					},
					ConnectionTimeout: time.Second * 30,
				},
			},
			err: fmt.Errorf("exporter has an invalid TCP endpoint: address http://localhost:24224: too many colons in address"),
		},
		{
			desc: "Endpoint is invalid with ValidateTCPResolution false throw no error",
			cfg: &Config{
				TCPClientSettings: TCPClientSettings{
					Endpoint: Endpoint{
						TCPAddr:               "http://localhost:24224",
						ValidateTCPResolution: false,
					},
					ConnectionTimeout: time.Second * 30,
				},
			},
			err: nil,
		},
		{
			desc: "Config is valid",
			cfg: &Config{
				TCPClientSettings: TCPClientSettings{
					Endpoint: Endpoint{
						TCPAddr:               validEndpoint,
						ValidateTCPResolution: true,
					},
					ConnectionTimeout: time.Second * 30,
				},
			},
			err: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
