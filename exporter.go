// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"context"
	"sync"

	fclient "github.com/IBM/fluent-forward-go/fluent/client"
	fproto "github.com/IBM/fluent-forward-go/fluent/protocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
)

type fluentforwardExporter struct {
	config   *Config
	settings component.TelemetrySettings
	client   *fclient.Client
	wg       sync.WaitGroup
}

func newExporter(config *Config, settings component.TelemetrySettings) *fluentforwardExporter {
	settings.Logger.Info("using the Fluent Forward exporter")

	return &fluentforwardExporter{
		config:   config,
		settings: settings,
	}
}

func (f *fluentforwardExporter) convertLogToMap(lr plog.LogRecord) map[string]interface{} {
	// move function into a translator
	m := make(map[string]interface{})
	m["severity"] = lr.SeverityText()
	m["message"] = lr.Body().AsString()
	for key, val := range f.config.DefaultLabelsEnabled {
		if val {
			attribute, found := lr.Attributes().Get(key)
			if found {
				m[key] = attribute.AsString()
			}
		}
	}
	return m
}

func (f *fluentforwardExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	// move for loops into a translator
	entries := []fproto.EntryExt{}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()

		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				entry := fproto.EntryExt{
					Timestamp: fproto.EventTimeNow(),
					Record:    f.convertLogToMap(log),
				}
				entries = append(entries, entry)
			}
		}
	}

	if f.config.CompressGzip {
		return f.SendCompressed(entries)
	}
	return f.SendForward(entries)
}

func (f *fluentforwardExporter) SendCompressed(entries []fproto.EntryExt) error {
	err := f.client.SendCompressed(f.config.Tag, entries)
	if err != nil {
		if errr := f.client.Reconnect(); errr != nil {
			return errr
		}
		err := f.client.SendCompressed(f.config.Tag, entries)
		if err != nil {
			return err
		}
		return err
	}
	return nil
}

func (f *fluentforwardExporter) SendForward(entries []fproto.EntryExt) error {
	err := f.client.SendForward(f.config.Tag, entries)
	if err != nil {
		if errr := f.client.Reconnect(); errr != nil {
			return errr
		}
		err := f.client.SendForward(f.config.Tag, entries)
		if err != nil {
			return err
		}
		return err
	}
	return nil
}

func (f *fluentforwardExporter) start(_ context.Context, host component.Host) error {
	client := fclient.New(fclient.ConnectionOptions{
		Factory: &fclient.ConnFactory{
			Address: f.config.Endpoint,
			Timeout: f.config.ConnectionTimeout,
		},
		RequireAck: f.config.RequireAck,
	})

	if err := client.Connect(); err != nil {
		return err
	}

	f.client = client

	return nil
}

func (f *fluentforwardExporter) stop(context.Context) (err error) {
	f.wg.Wait()
	return nil
}
